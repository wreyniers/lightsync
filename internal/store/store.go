package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"lightsync/internal/lights"
)

type Scene struct {
	ID      string                        `json:"id"`
	Name    string                        `json:"name"`
	Trigger string                        `json:"trigger"`
	Devices map[string]lights.DeviceState `json:"devices"`
	// GlobalColor/GlobalKelvin persist the editor's global override so it can
	// be restored when the scene is re-opened for editing.
	GlobalColor  *lights.Color `json:"globalColor,omitempty"`
	GlobalKelvin *int          `json:"globalKelvin,omitempty"`
}

type Settings struct {
	PollIntervalMs int  `json:"pollIntervalMs"`
	StartMinimized bool `json:"startMinimized"`
	LaunchAtLogin  bool `json:"launchAtLogin"`
}

type Config struct {
	Devices  []lights.Device `json:"devices"`
	Scenes   []Scene         `json:"scenes"`
	Settings Settings        `json:"settings"`

	HueBridges []HueBridge `json:"hueBridges,omitempty"`
}

type HueBridge struct {
	ID       string `json:"id"`
	IP       string `json:"ip"`
	Username string `json:"username"`
}

type Store struct {
	mu       sync.RWMutex
	config   Config
	filePath string
}

func New() (*Store, error) {
	p, err := configPath()
	if err != nil {
		return nil, err
	}

	s := &Store{
		filePath: p,
		config: Config{
			Settings: Settings{
				PollIntervalMs: 1000,
			},
		},
	}

	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return s, nil
}

func (s *Store) GetConfig() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

func (s *Store) GetDevices() []lights.Device {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.Devices
}

func (s *Store) SetDevices(devices []lights.Device) error {
	s.mu.Lock()
	s.config.Devices = devices
	s.mu.Unlock()
	return s.save()
}

func (s *Store) GetScenes() []Scene {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.Scenes
}

func (s *Store) SetScenes(scenes []Scene) error {
	s.mu.Lock()
	s.config.Scenes = scenes
	s.mu.Unlock()
	return s.save()
}

func (s *Store) GetSettings() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.Settings
}

func (s *Store) SetSettings(settings Settings) error {
	s.mu.Lock()
	s.config.Settings = settings
	s.mu.Unlock()
	return s.save()
}

func (s *Store) GetHueBridges() []HueBridge {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.HueBridges
}

func (s *Store) SetHueBridges(bridges []HueBridge) error {
	s.mu.Lock()
	s.config.HueBridges = bridges
	s.mu.Unlock()
	return s.save()
}

func (s *Store) UpsertDevice(device lights.Device) error {
	s.mu.Lock()
	found := false
	for i, d := range s.config.Devices {
		if d.ID == device.ID {
			s.config.Devices[i] = device
			found = true
			break
		}
	}
	if !found {
		s.config.Devices = append(s.config.Devices, device)
	}
	s.mu.Unlock()
	return s.save()
}

func (s *Store) UpsertScene(scene Scene) error {
	s.mu.Lock()
	found := false
	for i, sc := range s.config.Scenes {
		if sc.ID == scene.ID {
			s.config.Scenes[i] = scene
			found = true
			break
		}
	}
	if !found {
		s.config.Scenes = append(s.config.Scenes, scene)
	}
	s.mu.Unlock()
	return s.save()
}

func (s *Store) DeleteScene(id string) error {
	s.mu.Lock()
	for i, sc := range s.config.Scenes {
		if sc.ID == id {
			s.config.Scenes = append(s.config.Scenes[:i], s.config.Scenes[i+1:]...)
			break
		}
	}
	s.mu.Unlock()
	return s.save()
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return json.Unmarshal(data, &s.config)
}

func (s *Store) save() error {
	s.mu.RLock()
	data, err := json.MarshalIndent(s.config, "", "  ")
	s.mu.RUnlock()
	if err != nil {
		return err
	}

	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0644)
}

func configPath() (string, error) {
	var dir string
	switch runtime.GOOS {
	case "windows":
		dir = os.Getenv("APPDATA")
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, "Library", "Application Support")
	default:
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "lightsync", "config.json"), nil
}
