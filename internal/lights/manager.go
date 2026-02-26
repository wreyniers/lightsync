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

func (m *Manager) DiscoverAll(ctx context.Context) ([]Device, error) {
	return m.DiscoverAllWithProgress(ctx, nil)
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
		m.devices[d.ID] = d
	}
	m.mu.Unlock()

	if len(errs) > 0 && len(allDevices) == 0 {
		return nil, fmt.Errorf("discovery failed: %v", errs)
	}

	return allDevices, nil
}

func (m *Manager) SetDeviceState(ctx context.Context, deviceID string, state DeviceState) error {
	brand := brandFromDeviceID(deviceID)
	ctrl, ok := m.GetController(brand)
	if !ok {
		return fmt.Errorf("no controller for brand %q", brand)
	}
	log.Printf("[manager] SetDeviceState %s: on=%v brightness=%.2f", deviceID, state.On, state.Brightness)
	if err := ctrl.SetState(ctx, deviceID, state); err != nil {
		log.Printf("[manager] SetDeviceState %s failed: %v", deviceID, err)
		return err
	}
	return nil
}

func (m *Manager) GetDeviceState(ctx context.Context, deviceID string) (DeviceState, error) {
	brand := brandFromDeviceID(deviceID)
	ctrl, ok := m.GetController(brand)
	if !ok {
		return DeviceState{}, fmt.Errorf("no controller for brand %q", brand)
	}
	return ctrl.GetState(ctx, deviceID)
}

func (m *Manager) TurnOn(ctx context.Context, deviceID string) error {
	brand := brandFromDeviceID(deviceID)
	ctrl, ok := m.GetController(brand)
	if !ok {
		return fmt.Errorf("no controller for brand %q", brand)
	}
	log.Printf("[manager] TurnOn %s", deviceID)
	if err := ctrl.TurnOn(ctx, deviceID); err != nil {
		log.Printf("[manager] TurnOn %s failed: %v", deviceID, err)
		return err
	}
	return nil
}

func (m *Manager) TurnOff(ctx context.Context, deviceID string) error {
	brand := brandFromDeviceID(deviceID)
	ctrl, ok := m.GetController(brand)
	if !ok {
		return fmt.Errorf("no controller for brand %q", brand)
	}
	log.Printf("[manager] TurnOff %s", deviceID)
	if err := ctrl.TurnOff(ctx, deviceID); err != nil {
		log.Printf("[manager] TurnOff %s failed: %v", deviceID, err)
		return err
	}
	return nil
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
