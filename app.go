package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"lightsync/internal/discovery"
	"lightsync/internal/lights"
	"lightsync/internal/scenes"
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
}

func (a *App) shutdown(ctx context.Context) {
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
}

func (a *App) CreateScene(req CreateSceneRequest) (store.Scene, error) {
	return a.sceneManager.CreateScene(req.Name, req.Trigger, req.Devices, req.GlobalColor, req.GlobalKelvin)
}

func (a *App) UpdateScene(scene store.Scene) error {
	return a.sceneManager.UpdateScene(scene)
}

func (a *App) DeleteScene(id string) error {
	return a.sceneManager.DeleteScene(id)
}

func (a *App) ActivateScene(id string) error {
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()
	return a.sceneManager.ActivateScene(ctx, id)
}

func (a *App) GetActiveScene() string {
	return a.sceneManager.GetActiveScene()
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
