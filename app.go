package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v3/pkg/application"

	"lightsync/internal/discovery"
	"lightsync/internal/lights"
	"lightsync/internal/scenes"
	"lightsync/internal/screensync"
	"lightsync/internal/screensync/capture"
	"lightsync/internal/store"
	"lightsync/internal/webcam"
)

type App struct {
	mainWindow application.Window

	store        *store.Store
	lightManager *lights.Manager
	sceneManager *scenes.Manager
	webcamMon    *webcam.Monitor
	scanner      *discovery.Scanner
	lifxCtrl     *lights.LIFXController
	hueCtrl      *lights.HueController
	elgatoCtrl   *lights.ElgatoController
	goveeCtrl    *lights.GoveeController

	screenSyncEngine      *screensync.Engine
	screenSyncActiveScene string
	preSyncStates        map[string]lights.DeviceState

	quitConfirmed bool
}

func NewApp() *App {
	return &App{}
}

func (a *App) setMainWindow(w application.Window) {
	a.mainWindow = w
}

// prewarmControllers populates LIFX and Elgato controller caches from stored
// devices so the frontend can fetch light states immediately on load.
func (a *App) prewarmControllers(ctx context.Context, devices []lights.Device) {
	// Elgato: register known devices by IP so getClient finds them without reconnect race.
	for _, d := range devices {
		if d.Brand != lights.BrandElgato || d.LastIP == "" {
			continue
		}
		a.elgatoCtrl.AddDevice(d.LastIP)
	}

	// LIFX: run discovery in background. Frontend may still hit before it completes,
	// but we reduce the race window. refreshDevices retries after 2s if some devices miss.
	go func() {
		time.Sleep(200 * time.Millisecond) // let startup finish
		lifxCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if discovered, err := a.lifxCtrl.Discover(lifxCtx); err == nil && len(discovered) > 0 {
			slog.Info("Pre-warmed LIFX devices", "count", len(discovered))
		}
	}()
}

func (a *App) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	s, err := store.New()
	if err != nil {
		slog.Error("Failed to initialize store", "err", err)
		return err
	}
	a.store = s

	a.lightManager = lights.NewManager()
	a.lifxCtrl = lights.NewLIFXController()
	a.hueCtrl = lights.NewHueController()
	a.elgatoCtrl = lights.NewElgatoController()
	a.goveeCtrl = lights.NewGoveeController()
	a.lightManager.RegisterController(a.lifxCtrl)
	a.lightManager.RegisterController(a.hueCtrl)
	a.lightManager.RegisterController(a.elgatoCtrl)
	a.lightManager.RegisterController(a.goveeCtrl)

	bridges := a.store.GetHueBridges()
	for _, bridge := range bridges {
		if err := a.hueCtrl.AddBridge(bridge.IP, bridge.Username); err != nil {
			slog.Warn("Failed to add Hue bridge", "ip", bridge.IP, "err", err)
		}
	}
	if len(bridges) > 0 {
		hueCtx, hueCancel := context.WithTimeout(ctx, 10*time.Second)
		if discovered, err := a.hueCtrl.Discover(hueCtx); err == nil && len(discovered) > 0 {
			slog.Info("Loaded Hue lights from bridge(s)", "count", len(discovered))
		}
		hueCancel()
	}

	devices := a.store.GetDevices()
	a.lightManager.SetDevices(devices)

	// Pre-populate controller caches from stored devices so the frontend can
	// fetch light states immediately. Without this, LIFX/Elgato would need
	// per-device discovery on first access, causing races when both main
	// window and popup load in parallel (Wails v3).
	a.prewarmControllers(ctx, devices)

	a.scanner = discovery.NewScanner(a.lightManager, a.elgatoCtrl)
	a.sceneManager = scenes.NewManager(a.store, a.lightManager)
	a.sceneManager.OnChange(func(scene store.Scene) {
		application.Get().Event.Emit("scene:active", scene)
	})

	a.screenSyncEngine = screensync.NewEngine(a.lightManager)
	a.screenSyncEngine.OnColors(func(colors []lights.Color) {
		application.Get().Event.Emit("screensync:colors", colors)
	})
	a.screenSyncEngine.OnStats(func(s screensync.Stats) {
		application.Get().Event.Emit("screensync:stats", s)
	})
	a.screenSyncEngine.OnState(func(running bool) {
		application.Get().Event.Emit("screensync:state", map[string]interface{}{
			"running": running,
			"sceneId": a.screenSyncActiveScene,
		})
	})

	settings := a.store.GetSettings()
	interval := time.Duration(settings.PollIntervalMs) * time.Millisecond
	if interval < 500*time.Millisecond {
		interval = time.Second
	}
	a.webcamMon = webcam.NewMonitor(interval)
	a.webcamMon.OnChange(func(cameraOn bool) {
		application.Get().Event.Emit("camera:state", cameraOn)
		a.sceneManager.OnCameraStateChange(ctx, cameraOn)
	})
	go a.webcamMon.Start(ctx)

	a.setupTray()
	a.initPopupWindow()

	if lastID := a.store.GetLastSceneID(); lastID != "" {
		go func() {
			time.Sleep(400 * time.Millisecond)
			if scene, err := a.sceneManager.GetScene(lastID); err == nil {
				application.Get().Event.Emit("app:last-scene", scene)
			}
		}()
	}
	return nil
}

func (a *App) ServiceShutdown() error {
	done := make(chan struct{})
	go func() {
		a.shutdown()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(4 * time.Second):
		slog.Warn("Shutdown timed out after 4s, proceeding")
	}
	return nil
}

func (a *App) shutdown() {
	if a.screenSyncEngine != nil {
		safeStop(a.screenSyncEngine.Stop)
	}

	// Brief delay so in-flight light send goroutines (from the last frame) can finish
	// before we close the light manager. They use a 2s timeout max.
	time.Sleep(300 * time.Millisecond)

	if a.lightManager != nil {
		safeClose(a.lightManager.Close)
	}
}

func safeStop(fn func()) {
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("Recovered from panic during engine stop", "panic", r)
		}
	}()
	fn()
}

func safeClose(fn func() error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("Recovered from panic during close", "panic", r)
		}
	}()
	_ = fn()
}

// --- Discovery ---

type DiscoverResult struct {
	Devices []lights.Device `json:"devices"`
	Errors  []string        `json:"errors,omitempty"`
}

func (a *App) DiscoverLights() DiscoverResult {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	r := a.scanner.ScanAll(ctx, func(p discovery.ScanProgress) {
		application.Get().Event.Emit("scan:progress", p)
	})

	if err := a.store.SetDevices(a.lightManager.GetDevices()); err != nil {
		r.Errors = append(r.Errors, err.Error())
	}

	return DiscoverResult{
		Devices: r.Devices,
		Errors:  r.Errors,
	}
}

func (a *App) GetDevices() []lights.Device {
	return a.lightManager.GetDevices()
}

func (a *App) RemoveDevice(deviceID string) error {
	a.lightManager.RemoveDevice(deviceID)
	return a.store.SetDevices(a.lightManager.GetDevices())
}

// AddElgatoByIP adds an Elgato Key Light by IP when auto-discovery fails
// (e.g. mDNS is blocked on Windows). Use the device's LAN IP (e.g. 192.168.4.73).
func (a *App) AddElgatoByIP(ip string) (lights.Device, error) {
	if ip == "" {
		return lights.Device{}, fmt.Errorf("IP cannot be empty")
	}
	a.elgatoCtrl.AddDevice(ip)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	devices, err := a.elgatoCtrl.Discover(ctx)
	if err != nil {
		return lights.Device{}, fmt.Errorf("elgato discover: %w", err)
	}
	var added lights.Device
	targetID := fmt.Sprintf("elgato:%s", ip)
	for _, d := range devices {
		if d.ID == targetID || strings.Contains(d.ID, ip) {
			added = d
			break
		}
	}
	if added.ID == "" {
		return lights.Device{}, fmt.Errorf("could not reach Elgato at %s (check IP and that the light is on)", ip)
	}
	existing := a.lightManager.GetDevices()
	merged := append(existing, added)
	a.lightManager.SetDevices(merged)
	if err := a.store.SetDevices(merged); err != nil {
		return lights.Device{}, fmt.Errorf("saved but store failed: %w", err)
	}
	return added, nil
}

func (a *App) SetDeviceRoom(deviceID, room string) error {
	a.lightManager.SetDeviceRoom(deviceID, room)
	return a.store.SetDevices(a.lightManager.GetDevices())
}

// --- Light Control ---

func (a *App) SetLightState(deviceID string, state lights.DeviceState) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return a.lightManager.SetDeviceState(ctx, deviceID, state)
}

func (a *App) GetLightState(deviceID string) (lights.DeviceState, error) {
	// 8s allows slow HTTP devices (Elgato) to respond when many are queried in parallel at startup
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	return a.lightManager.GetDeviceState(ctx, deviceID)
}

func (a *App) TurnOnLight(deviceID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return a.lightManager.TurnOn(ctx, deviceID)
}

func (a *App) TurnOffLight(deviceID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return a.lightManager.TurnOff(ctx, deviceID)
}

// --- Scenes ---

func (a *App) GetScenes() []store.Scene {
	return a.sceneManager.GetScenes()
}

func (a *App) GetScene(id string) (store.Scene, error) {
	return a.sceneManager.GetScene(id)
}

type CreateSceneRequest struct {
	Name         string                        `json:"name"`
	Trigger      string                        `json:"trigger"`
	Devices      map[string]lights.DeviceState `json:"devices"`
	GlobalColor  *lights.Color                 `json:"globalColor,omitempty"`
	GlobalKelvin *int                          `json:"globalKelvin,omitempty"`
	ScreenSync   *store.ScreenSyncConfig       `json:"screenSync,omitempty"`
}

func (a *App) CreateScene(req CreateSceneRequest) (store.Scene, error) {
	return a.sceneManager.CreateScene(req.Name, req.Trigger, req.Devices, req.GlobalColor, req.GlobalKelvin, req.ScreenSync)
}

func (a *App) UpdateScene(scene store.Scene) error {
	return a.sceneManager.UpdateScene(scene)
}

func (a *App) DeleteScene(id string) error {
	return a.sceneManager.DeleteScene(id)
}

func (a *App) ActivateScene(id string) error {
	scene, err := a.sceneManager.GetScene(id)
	if err != nil {
		return err
	}

	if a.screenSyncEngine.IsRunning() {
		a.stopScreenSync()
	}

	if scene.Trigger == "screen_sync" && scene.ScreenSync != nil {
		store.NormalizeScreenSyncConfig(scene.ScreenSync)
		a.preSyncStates = a.captureDeviceStates(scene.ScreenSync.DeviceIDs)
		if err := a.sceneManager.MarkActive(id); err != nil {
			return err
		}
		a.screenSyncActiveScene = id
		a.blackoutDevices(scene.ScreenSync.DeviceIDs)
		return a.screenSyncEngine.Start(*scene.ScreenSync)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return a.sceneManager.ActivateScene(ctx, id)
}

func (a *App) captureDeviceStates(deviceIDs []string) map[string]lights.DeviceState {
	states := make(map[string]lights.DeviceState, len(deviceIDs))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for _, id := range deviceIDs {
		if s, err := a.lightManager.GetDeviceState(ctx, id); err == nil {
			states[id] = s
		}
	}
	return states
}

func (a *App) blackoutDevices(deviceIDs []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	state := lights.DeviceState{
		On:         true,
		Brightness: 0,
		Color:      &lights.Color{H: 0, S: 0, B: 1.0},
	}
	for _, id := range deviceIDs {
		_ = a.lightManager.SetDeviceState(ctx, id, state)
	}
}

func (a *App) stopScreenSync() {
	a.screenSyncEngine.Stop()
	a.screenSyncActiveScene = ""
	if len(a.preSyncStates) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for id, state := range a.preSyncStates {
		_ = a.lightManager.SetDeviceState(ctx, id, state)
	}
	a.preSyncStates = nil
}

func (a *App) GetActiveScene() string {
	return a.sceneManager.GetActiveScene()
}

func (a *App) GetLastSceneID() string {
	return a.store.GetLastSceneID()
}

func (a *App) DeactivateScene() {
	a.sceneManager.ClearActive()
}

func (a *App) QuitApp() {
	a.quitConfirmed = true

	// Safety net: Wails' Quit() blocks indefinitely in WebView2 teardown on
	// Windows (AppHangXProcB1). Our ServiceShutdown completes in ~300ms, so
	// give it 1.5s total then force-exit.
	go func() {
		time.Sleep(1500 * time.Millisecond)
		os.Exit(0)
	}()

	if popupWindow != nil {
		popupWindow.Close()
	}

	application.Get().Quit()
}

func (a *App) CloneScene(id string) (store.Scene, error) {
	scene, err := a.sceneManager.GetScene(id)
	if err != nil {
		return store.Scene{}, err
	}
	return a.sceneManager.CreateScene(
		scene.Name+" (Copy)",
		"",
		scene.Devices,
		scene.GlobalColor,
		scene.GlobalKelvin,
		scene.ScreenSync,
	)
}

// --- Webcam ---

func (a *App) GetCameraState() bool {
	return a.webcamMon.IsActive()
}

func (a *App) CheckCameraNow() (bool, error) {
	return a.webcamMon.CheckNow()
}

func (a *App) SetMonitoringEnabled(enabled bool) {
	a.webcamMon.SetEnabled(enabled)
	application.Get().Event.Emit("monitoring:state", enabled)
}

func (a *App) IsMonitoringEnabled() bool {
	return a.webcamMon.IsEnabled()
}

// --- Settings ---

func (a *App) GetSettings() store.Settings {
	return a.store.GetSettings()
}

func (a *App) UpdateSettings(settings store.Settings) error {
	if settings.PollIntervalMs > 0 {
		a.webcamMon.SetInterval(time.Duration(settings.PollIntervalMs) * time.Millisecond)
	}
	return a.store.SetSettings(settings)
}

// --- Hue Bridge ---

func (a *App) AddHueBridge(ip, username string) error {
	if err := a.hueCtrl.AddBridge(ip, username); err != nil {
		return err
	}
	bridges := a.store.GetHueBridges()
	// Upsert by IP: update existing or append new
	found := false
	for i := range bridges {
		if bridges[i].IP == ip {
			bridges[i].Username = username
			found = true
			break
		}
	}
	if !found {
		bridges = append(bridges, store.HueBridge{
			ID:       uuid.New().String(),
			IP:       ip,
			Username: username,
		})
	}
	return a.store.SetHueBridges(bridges)
}

func (a *App) GetHueBridges() []store.HueBridge {
	return a.store.GetHueBridges()
}

func (a *App) RemoveHueBridge(id string) error {
	bridges := a.store.GetHueBridges()
	filtered := make([]store.HueBridge, 0, len(bridges))
	for _, b := range bridges {
		if b.ID != id {
			filtered = append(filtered, b)
		}
	}
	return a.store.SetHueBridges(filtered)
}

func (a *App) DiscoverHueBridges() []discovery.DiscoveredHueBridge {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	bridges := a.scanner.DiscoverHueBridges(ctx)
	if bridges == nil {
		return []discovery.DiscoveredHueBridge{}
	}
	return bridges
}

type PairResult struct {
	Success  bool   `json:"success"`
	Username string `json:"username"`
	Error    string `json:"error,omitempty"`
}

func (a *App) PairHueBridge(ip string) PairResult {
	httpClient := lights.NewHueHTTPClient(5 * time.Second)

	body, _ := json.Marshal(map[string]interface{}{
		"devicetype":        "lightsync#app",
		"generateclientkey": true,
	})

	resp, err := httpClient.Post(
		fmt.Sprintf("https://%s/api", ip),
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return PairResult{Error: fmt.Sprintf("cannot reach bridge: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return PairResult{Error: fmt.Sprintf("bridge returned HTTP %d", resp.StatusCode)}
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return PairResult{Error: "failed to read response"}
	}

	var results []struct {
		Success *struct {
			Username string `json:"username"`
		} `json:"success,omitempty"`
		Error *struct {
			Type        int    `json:"type"`
			Description string `json:"description"`
		} `json:"error,omitempty"`
	}
	if err := json.Unmarshal(respBody, &results); err != nil {
		return PairResult{Error: "unexpected response from bridge"}
	}

	if len(results) == 0 {
		return PairResult{Error: "empty response from bridge"}
	}

	if results[0].Error != nil {
		if results[0].Error.Type == 101 {
			return PairResult{Error: "link button not pressed"}
		}
		return PairResult{Error: results[0].Error.Description}
	}

	if results[0].Success != nil && results[0].Success.Username != "" {
		username := results[0].Success.Username
		if err := a.AddHueBridge(ip, username); err != nil {
			return PairResult{Error: fmt.Sprintf("paired but failed to save: %v", err)}
		}
		return PairResult{Success: true, Username: username}
	}

	return PairResult{Error: "unexpected response from bridge"}
}

// --- Screen Sync ---

type ScreenSyncState struct {
	Running bool   `json:"running"`
	SceneID string `json:"sceneId"`
}

func (a *App) GetScreenSyncState() ScreenSyncState {
	return ScreenSyncState{
		Running: a.screenSyncEngine.IsRunning(),
		SceneID: a.screenSyncActiveScene,
	}
}

func (a *App) StopScreenSync() {
	a.stopScreenSync()
}

func (a *App) UpdateScreenSyncConfig(sceneID string, cfg store.ScreenSyncConfig) error {
	store.NormalizeScreenSyncConfig(&cfg)
	scene, err := a.sceneManager.GetScene(sceneID)
	if err != nil {
		return err
	}
	scene.ScreenSync = &cfg
	if err := a.sceneManager.UpdateScene(scene); err != nil {
		return err
	}
	if a.screenSyncEngine.IsRunning() && a.screenSyncActiveScene == sceneID {
		a.screenSyncEngine.UpdateConfig(cfg)
	}
	return nil
}

func (a *App) GetMonitors() []capture.MonitorInfo {
	return capture.GetMonitors()
}

func (a *App) GetWindows() []capture.WindowInfo {
	return capture.EnumWindows()
}

func (a *App) GetWindowThumbnail(hwnd uint64) string {
	img, err := capture.CaptureThumbnail(hwnd)
	if err != nil {
		return ""
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func (a *App) StartRegionSelect() {
	go func() {
		result := capture.StartRegionOverlay()
		if result.Cancelled {
			application.Get().Event.Emit("screensync:region-selected", map[string]interface{}{
				"cancelled": true,
			})
			return
		}
		application.Get().Event.Emit("screensync:region-selected", map[string]interface{}{
			"cancelled": false,
			"x":         result.Region.X,
			"y":         result.Region.Y,
			"width":     result.Region.Width,
			"height":    result.Region.Height,
		})
	}()
}

func (a *App) GetDefaultScreenSyncConfig() store.ScreenSyncConfig {
	return store.DefaultScreenSyncConfig()
}

func (a *App) GetCapturePreview() string {
	if a.screenSyncEngine == nil || !a.screenSyncEngine.IsRunning() {
		return ""
	}
	data := a.screenSyncEngine.GetPreviewFrame()
	if len(data) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(data)
}
