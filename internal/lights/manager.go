package lights

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
)

type Manager struct {
	mu          sync.RWMutex
	controllers map[Brand]Controller
	devices     map[string]Device
}

func NewManager() *Manager {
	return &Manager{
		controllers: make(map[Brand]Controller),
		devices:     make(map[string]Device),
	}
}

func (m *Manager) RegisterController(c Controller) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.controllers[c.Brand()] = c
}

func (m *Manager) GetController(brand Brand) (Controller, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.controllers[brand]
	return c, ok
}

// DiscoverAllWithProgress runs discovery across all controllers concurrently.
// onDevices is called (under an internal lock, so serially) each time a
// controller finishes, with only the devices that controller returned.
func (m *Manager) DiscoverAllWithProgress(ctx context.Context, onDevices func([]Device)) ([]Device, error) {
	m.mu.RLock()
	controllers := make([]Controller, 0, len(m.controllers))
	for _, c := range m.controllers {
		controllers = append(controllers, c)
	}
	m.mu.RUnlock()

	var (
		allDevices []Device
		mu         sync.Mutex
		wg         sync.WaitGroup
		errs       []error
	)

	for _, c := range controllers {
		wg.Add(1)
		go func(ctrl Controller) {
			defer wg.Done()
			devices, err := ctrl.Discover(ctx)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", ctrl.Brand(), err))
				return
			}
			allDevices = append(allDevices, devices...)
			if onDevices != nil && len(devices) > 0 {
				onDevices(devices)
			}
		}(c)
	}

	wg.Wait()

	m.mu.Lock()
	for _, d := range allDevices {
		if existing, ok := m.devices[d.ID]; ok {
			d.Room = existing.Room
		}
		m.devices[d.ID] = d
	}
	m.mu.Unlock()

	if len(errs) > 0 && len(allDevices) == 0 {
		return nil, fmt.Errorf("discovery failed: %v", errs)
	}

	return allDevices, nil
}

func (m *Manager) controllerFor(deviceID string) (Controller, error) {
	brand := brandFromDeviceID(deviceID)
	ctrl, ok := m.GetController(brand)
	if !ok {
		return nil, fmt.Errorf("no controller for brand %q", brand)
	}
	return ctrl, nil
}

func (m *Manager) SetDeviceState(ctx context.Context, deviceID string, state DeviceState) error {
	ctrl, err := m.controllerFor(deviceID)
	if err != nil {
		return err
	}
	log.Printf("[manager] SetDeviceState %s: on=%v brightness=%.2f", deviceID, state.On, state.Brightness)
	return ctrl.SetState(ctx, deviceID, state)
}

func (m *Manager) GetDeviceState(ctx context.Context, deviceID string) (DeviceState, error) {
	ctrl, err := m.controllerFor(deviceID)
	if err != nil {
		return DeviceState{}, err
	}
	return ctrl.GetState(ctx, deviceID)
}

func (m *Manager) TurnOn(ctx context.Context, deviceID string) error {
	ctrl, err := m.controllerFor(deviceID)
	if err != nil {
		return err
	}
	log.Printf("[manager] TurnOn %s", deviceID)
	return ctrl.TurnOn(ctx, deviceID)
}

func (m *Manager) TurnOff(ctx context.Context, deviceID string) error {
	ctrl, err := m.controllerFor(deviceID)
	if err != nil {
		return err
	}
	log.Printf("[manager] TurnOff %s", deviceID)
	return ctrl.TurnOff(ctx, deviceID)
}

func (m *Manager) GetDevices() []Device {
	m.mu.RLock()
	defer m.mu.RUnlock()
	devices := make([]Device, 0, len(m.devices))
	for _, d := range m.devices {
		devices = append(devices, d)
	}
	sort.Slice(devices, func(i, j int) bool {
		if devices[i].Brand != devices[j].Brand {
			return devices[i].Brand < devices[j].Brand
		}
		return devices[i].Name < devices[j].Name
	})
	return devices
}

func (m *Manager) SetDevices(devices []Device) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range devices {
		m.devices[d.ID] = d
	}
}

func (m *Manager) RemoveDevice(deviceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.devices, deviceID)
}

func (m *Manager) SetDeviceRoom(deviceID, room string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if d, ok := m.devices[deviceID]; ok {
		d.Room = room
		m.devices[deviceID] = d
	}
}

func (m *Manager) Close() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, c := range m.controllers {
		_ = c.Close()
	}
	return nil
}

func brandFromDeviceID(id string) Brand {
	parts := strings.SplitN(id, ":", 2)
	if len(parts) < 2 {
		return ""
	}
	return Brand(parts[0])
}
