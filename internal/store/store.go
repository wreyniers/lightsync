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
	mu       sync.Mutex
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

func (s *Store) GetDevices() []lights.Device {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]lights.Device(nil), s.config.Devices...)
}

func (s *Store) SetDevices(devices []lights.Device) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.Devices = devices
	return s.saveLocked()
}

func (s *Store) GetScenes() []Scene {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Scene(nil), s.config.Scenes...)
}

func (s *Store) SetScenes(scenes []Scene) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.Scenes = scenes
	return s.saveLocked()
}

func (s *Store) GetSettings() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config.Settings
}

func (s *Store) SetSettings(settings Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.Settings = settings
	return s.saveLocked()
}

func (s *Store) GetHueBridges() []HueBridge {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]HueBridge(nil), s.config.HueBridges...)
}

func (s *Store) SetHueBridges(bridges []HueBridge) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.HueBridges = bridges
	return s.saveLocked()
}

func (s *Store) UpsertScene(scene Scene) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, sc := range s.config.Scenes {
		if sc.ID == scene.ID {
			s.config.Scenes[i] = scene
			return s.saveLocked()
		}
	}
	s.config.Scenes = append(s.config.Scenes, scene)
	return s.saveLocked()
}

func (s *Store) DeleteScene(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, sc := range s.config.Scenes {
		if sc.ID == id {
			s.config.Scenes = append(s.config.Scenes[:i], s.config.Scenes[i+1:]...)
			break
		}
	}
	return s.saveLocked()
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

// saveLocked marshals config and writes atomically. Caller must hold s.mu.
func (s *Store) saveLocked() error {
	data, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	tmp := s.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.filePath)
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
