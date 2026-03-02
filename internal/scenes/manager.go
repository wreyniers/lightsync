package scenes

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"

	"lightsync/internal/lights"
	"lightsync/internal/store"
)

type Manager struct {
	mu           sync.RWMutex
	store        *store.Store
	lightManager *lights.Manager
	activeScene  string
	onChange      func(scene store.Scene)
}

func NewManager(s *store.Store, lm *lights.Manager) *Manager {
	return &Manager{
		store:        s,
		lightManager: lm,
	}
}

func (m *Manager) OnChange(fn func(scene store.Scene)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onChange = fn
}

func (m *Manager) GetScenes() []store.Scene {
	return m.store.GetScenes()
}

func (m *Manager) GetScene(id string) (store.Scene, error) {
	for _, s := range m.store.GetScenes() {
		if s.ID == id {
			return s, nil
		}
	}
	return store.Scene{}, fmt.Errorf("scene %s not found", id)
}

func (m *Manager) triggerInUse(trigger, excludeID string) bool {
	if trigger == "" {
		return false
	}
	// Multiple screen_sync scenes are allowed; only one can be active at a time
	// (enforced by the engine lifecycle in app.go, not by the trigger index).
	if trigger == "screen_sync" {
		return false
	}
	for _, s := range m.store.GetScenes() {
		if s.ID != excludeID && s.Trigger == trigger {
			return true
		}
	}
	return false
}

func (m *Manager) CreateScene(name, trigger string, devices map[string]lights.DeviceState, globalColor *lights.Color, globalKelvin *int, screenSync *store.ScreenSyncConfig) (store.Scene, error) {
	if m.triggerInUse(trigger, "") {
		return store.Scene{}, fmt.Errorf("trigger %q is already used by another scene", trigger)
	}

	scene := store.Scene{
		ID:           uuid.New().String(),
		Name:         name,
		Trigger:      trigger,
		Devices:      devices,
		GlobalColor:  globalColor,
		GlobalKelvin: globalKelvin,
		ScreenSync:   screenSync,
	}

	if err := m.store.UpsertScene(scene); err != nil {
		return store.Scene{}, err
	}

	return scene, nil
}

func (m *Manager) UpdateScene(scene store.Scene) error {
	if m.triggerInUse(scene.Trigger, scene.ID) {
		return fmt.Errorf("trigger %q is already used by another scene", scene.Trigger)
	}
	return m.store.UpsertScene(scene)
}

func (m *Manager) DeleteScene(id string) error {
	return m.store.DeleteScene(id)
}

func (m *Manager) GetActiveScene() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeScene
}

func (m *Manager) ActivateScene(ctx context.Context, id string) error {
	scene, err := m.GetScene(id)
	if err != nil {
		return err
	}

	// Persist last activated scene so it can be restored on next app launch.
	_ = m.store.SetLastSceneID(id)

	// Emit scene:active immediately so the UI updates in one coherent step.
	// Device state changes run afterward; the frontend applies preset states
	// optimistically when it receives scene:active.
	m.mu.Lock()
	m.activeScene = id
	fn := m.onChange
	m.mu.Unlock()

	if fn != nil {
		fn(scene)
	}

	for deviceID, state := range scene.Devices {
		if err := m.lightManager.SetDeviceState(ctx, deviceID, state); err != nil {
			continue
		}
	}

	return nil
}

// MarkActive sets the active scene ID and fires the onChange callback without
// applying device states. Used by Screen Sync scenes where the engine controls
// lights directly.
func (m *Manager) MarkActive(id string) error {
	scene, err := m.GetScene(id)
	if err != nil {
		return err
	}

	// Persist last activated scene so it can be restored on next app launch.
	_ = m.store.SetLastSceneID(id)

	m.mu.Lock()
	m.activeScene = id
	fn := m.onChange
	m.mu.Unlock()

	if fn != nil {
		fn(scene)
	}
	return nil
}

// ClearActive clears the in-memory active scene without emitting any event.
func (m *Manager) ClearActive() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeScene = ""
}

func (m *Manager) OnCameraStateChange(ctx context.Context, cameraOn bool) {
	trigger := "camera_off"
	if cameraOn {
		trigger = "camera_on"
	}

	for _, scene := range m.store.GetScenes() {
		if scene.Trigger == trigger {
			_ = m.ActivateScene(ctx, scene.ID)
			return
		}
	}
}
