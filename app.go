package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"time"

	"github.com/getlantern/systray"
	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"lightsync/internal/discovery"
	"lightsync/internal/lights"
	"lightsync/internal/scenes"
	"lightsync/internal/screensync"
	"lightsync/internal/screensync/capture"
	"lightsync/internal/store"
	"lightsync/internal/webcam"
)

type App struct {
	ctx          context.Context
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
	screenSyncActiveScene string // sceneID of the running screen sync scene
	// preSyncStates holds device states captured just before screen sync started
	// so they can be restored when screen sync is deactivated.
	preSyncStates map[string]lights.DeviceState

	// quitConfirmed is set to true when the user explicitly chooses "Exit" in
	// the close dialog. OnBeforeClose checks this to allow the quit through.
	quitConfirmed bool
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	s, err := store.New()
	if err != nil {
		runtime.LogErrorf(ctx, "Failed to initialize store: %v", err)
		return
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
			runtime.LogWarningf(ctx, "Failed to add Hue bridge %s: %v", bridge.IP, err)
		}
	}

	if len(bridges) > 0 {
		hueCtx, hueCancel := context.WithTimeout(ctx, 10*time.Second)
		if discovered, err := a.hueCtrl.Discover(hueCtx); err == nil && len(discovered) > 0 {
			runtime.LogInfof(ctx, "Loaded %d Hue light(s) from bridge(s)", len(discovered))
		}
		hueCancel()
	}

	a.lightManager.SetDevices(a.store.GetDevices())

	a.scanner = discovery.NewScanner(a.lightManager, a.elgatoCtrl)
	a.sceneManager = scenes.NewManager(a.store, a.lightManager)
	a.sceneManager.OnChange(func(scene store.Scene) {
		runtime.EventsEmit(a.ctx, "scene:active", scene)
	})

	// Screen Sync engine.
	a.screenSyncEngine = screensync.NewEngine(a.lightManager)
	a.screenSyncEngine.OnColors(func(colors []lights.Color) {
		runtime.EventsEmit(a.ctx, "screensync:colors", colors)
	})
	a.screenSyncEngine.OnStats(func(s screensync.Stats) {
		runtime.EventsEmit(a.ctx, "screensync:stats", s)
	})
	a.screenSyncEngine.OnState(func(running bool) {
		runtime.EventsEmit(a.ctx, "screensync:state", map[string]interface{}{
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
		runtime.EventsEmit(a.ctx, "camera:state", cameraOn)
		a.sceneManager.OnCameraStateChange(a.ctx, cameraOn)
	})

	go a.webcamMon.Start(ctx)

	a.setupTray()

	// Emit last scene for sidebar display. Delayed so the frontend has time to
	// load and register listeners (avoids production-build race where frontend
	// mounts before bridge is ready; push from backend avoids frontend polling).
	if lastID := a.store.GetLastSceneID(); lastID != "" {
		go func() {
			time.Sleep(400 * time.Millisecond)
			if scene, err := a.sceneManager.GetScene(lastID); err == nil {
				runtime.EventsEmit(a.ctx, "app:last-scene", scene)
			}
		}()
	}
}

func (a *App) shutdown(ctx context.Context) {
	if a.screenSyncEngine != nil {
		a.screenSyncEngine.Stop()
	}
	if a.lightManager != nil {
		_ = a.lightManager.Close()
	}
}

// --- Discovery ---

type DiscoverResult struct {
	Devices []lights.Device `json:"devices"`
	Errors  []string        `json:"errors,omitempty"`
}

func (a *App) DiscoverLights() DiscoverResult {
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()

	r := a.scanner.ScanAll(ctx, func(p discovery.ScanProgress) {
		runtime.EventsEmit(a.ctx, "scan:progress", p)
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

func (a *App) SetDeviceRoom(deviceID, room string) error {
	a.lightManager.SetDeviceRoom(deviceID, room)
	return a.store.SetDevices(a.lightManager.GetDevices())
}

// --- Light Control ---

func (a *App) SetLightState(deviceID string, state lights.DeviceState) error {
	ctx, cancel := context.WithTimeout(a.ctx, 5*time.Second)
	defer cancel()
	return a.lightManager.SetDeviceState(ctx, deviceID, state)
}

func (a *App) GetLightState(deviceID string) (lights.DeviceState, error) {
	ctx, cancel := context.WithTimeout(a.ctx, 5*time.Second)
	defer cancel()
	return a.lightManager.GetDeviceState(ctx, deviceID)
}

func (a *App) TurnOnLight(deviceID string) error {
	ctx, cancel := context.WithTimeout(a.ctx, 5*time.Second)
	defer cancel()
	return a.lightManager.TurnOn(ctx, deviceID)
}

func (a *App) TurnOffLight(deviceID string) error {
	ctx, cancel := context.WithTimeout(a.ctx, 5*time.Second)
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

	// Stop any running screen sync before activating another scene.
	if a.screenSyncEngine.IsRunning() {
		a.stopScreenSync()
	}

	if scene.Trigger == "screen_sync" && scene.ScreenSync != nil {
		store.NormalizeScreenSyncConfig(scene.ScreenSync)
		// Capture pre-sync device states for later restore.
		a.preSyncStates = a.captureDeviceStates(scene.ScreenSync.DeviceIDs)
		// Emit scene:active without applying static device states.
		if err := a.sceneManager.MarkActive(id); err != nil {
			return err
		}
		a.screenSyncActiveScene = id
		// Blackout, start engine immediately. Engine calibrates (runs pipeline
		// without sending) for 2s, then fades brightness up.
		a.blackoutDevices(scene.ScreenSync.DeviceIDs)
		return a.screenSyncEngine.Start(*scene.ScreenSync)
	}

	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()
	return a.sceneManager.ActivateScene(ctx, id)
}

// captureDeviceStates reads and returns the current state of the given devices.
func (a *App) captureDeviceStates(deviceIDs []string) map[string]lights.DeviceState {
	states := make(map[string]lights.DeviceState, len(deviceIDs))
	ctx, cancel := context.WithTimeout(a.ctx, 3*time.Second)
	defer cancel()
	for _, id := range deviceIDs {
		if s, err := a.lightManager.GetDeviceState(ctx, id); err == nil {
			states[id] = s
		}
	}
	return states
}

// blackoutDevices sets all given devices to brightness 0 (lights on, fully dimmed).
func (a *App) blackoutDevices(deviceIDs []string) {
	ctx, cancel := context.WithTimeout(a.ctx, 5*time.Second)
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

// stopScreenSync stops the engine and restores pre-sync light states.
func (a *App) stopScreenSync() {
	a.screenSyncEngine.Stop()
	a.screenSyncActiveScene = ""
	if len(a.preSyncStates) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(a.ctx, 5*time.Second)
	defer cancel()
	for id, state := range a.preSyncStates {
		_ = a.lightManager.SetDeviceState(ctx, id, state)
	}
	a.preSyncStates = nil
}

func (a *App) GetActiveScene() string {
	return a.sceneManager.GetActiveScene()
}

// GetLastSceneID returns the ID of the last scene that was activated, persisted
// across app restarts. Returns an empty string if no scene has ever been activated.
func (a *App) GetLastSceneID() string {
	return a.store.GetLastSceneID()
}

// DeactivateScene clears the active scene without touching any lights.
// Used when the user explicitly stops a scene from the sidebar.
func (a *App) DeactivateScene() {
	a.sceneManager.ClearActive()
}

// QuitApp is called from the frontend when the user confirms they want to exit.
// It sets a bypass flag so OnBeforeClose allows the quit through, then tears
// down the systray and requests application exit.
func (a *App) QuitApp() {
	a.quitConfirmed = true
	systray.Quit()
	runtime.Quit(a.ctx)
}

// CloneScene creates a copy of the given scene with " (Copy)" appended to the name.
// The clone gets a new ID and its trigger is cleared (cloned scenes start as manual).
func (a *App) CloneScene(id string) (store.Scene, error) {
	scene, err := a.sceneManager.GetScene(id)
	if err != nil {
		return store.Scene{}, err
	}

	// Clones are always manual (no trigger) to avoid duplicate trigger conflicts.
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
	runtime.EventsEmit(a.ctx, "monitoring:state", enabled)
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
	bridges = append(bridges, store.HueBridge{
		ID:       uuid.New().String(),
		IP:       ip,
		Username: username,
	})
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
	ctx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
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

// ScreenSyncState describes the current engine state returned to the frontend.
type ScreenSyncState struct {
	Running bool   `json:"running"`
	SceneID string `json:"sceneId"`
}

// GetScreenSyncState returns whether the engine is currently running and for which scene.
func (a *App) GetScreenSyncState() ScreenSyncState {
	return ScreenSyncState{
		Running: a.screenSyncEngine.IsRunning(),
		SceneID: a.screenSyncActiveScene,
	}
}

// StopScreenSync stops the engine and restores lights to their pre-sync states.
func (a *App) StopScreenSync() {
	a.stopScreenSync()
}

// UpdateScreenSyncConfig hot-reloads the engine's config and persists it.
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

// GetMonitors returns layout information about all active displays.
func (a *App) GetMonitors() []capture.MonitorInfo {
	return capture.GetMonitors()
}

// GetWindows returns a list of visible application windows.
func (a *App) GetWindows() []capture.WindowInfo {
	return capture.EnumWindows()
}

// GetWindowThumbnail captures a small thumbnail of the given window and returns
// it as a base64-encoded PNG string. Returns an empty string on failure.
func (a *App) GetWindowThumbnail(hwnd uint64) string {
	img, err := capture.CaptureThumbnail(hwnd)
	if err != nil {
		return ""
	}
	// Encode as PNG, base64.
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

// StartRegionSelect opens the full-screen overlay and blocks until the user
// selects a region. The result is emitted as a "screensync:region-selected" event.
// Runs in a goroutine so it does not block the Wails RPC call.
func (a *App) StartRegionSelect() {
	go func() {
		result := capture.StartRegionOverlay()
		if result.Cancelled {
			runtime.EventsEmit(a.ctx, "screensync:region-selected", map[string]interface{}{
				"cancelled": true,
			})
			return
		}
		runtime.EventsEmit(a.ctx, "screensync:region-selected", map[string]interface{}{
			"cancelled": false,
			"x":         result.Region.X,
			"y":         result.Region.Y,
			"width":     result.Region.Width,
			"height":    result.Region.Height,
		})
	}()
}

// GetDefaultScreenSyncConfig returns the default configuration for a new Screen Sync scene.
func (a *App) GetDefaultScreenSyncConfig() store.ScreenSyncConfig {
	return store.DefaultScreenSyncConfig()
}

// GetCapturePreview returns the most recent 1-fps JPEG preview of the captured
// image as a base64-encoded string (no data-URI prefix). Returns an empty
// string when the engine is not running or no frame has been captured yet.
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
