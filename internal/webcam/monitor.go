package webcam

import (
	"context"
	"log"
	"sync"
	"time"
)

type StateChangeHandler func(cameraOn bool)

type Monitor struct {
	mu       sync.RWMutex
	interval time.Duration
	active   bool
	onChange StateChangeHandler
	enabled  bool
}

func NewMonitor(interval time.Duration) *Monitor {
	return &Monitor{
		interval: interval,
		enabled:  true,
	}
}

func (m *Monitor) OnChange(handler StateChangeHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onChange = handler
}

func (m *Monitor) SetEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = enabled
}

func (m *Monitor) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

func (m *Monitor) IsActive() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active
}

func (m *Monitor) SetInterval(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.interval = d
}

func (m *Monitor) Start(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	log.Printf("[webcam] Monitor started (interval: %v)", m.interval)

	for {
		select {
		case <-ctx.Done():
			log.Println("[webcam] Monitor stopped")
			return
		case <-ticker.C:
			m.mu.RLock()
			enabled := m.enabled
			m.mu.RUnlock()

			if !enabled {
				continue
			}

			cameraOn := isCameraOn()

			m.mu.Lock()
			changed := m.active != cameraOn
			m.active = cameraOn
			handler := m.onChange
			m.mu.Unlock()

			if changed {
				log.Printf("[webcam] Camera state changed: on=%v", cameraOn)
				if handler != nil {
					handler(cameraOn)
				}
			}
		}
	}
}

func (m *Monitor) CheckNow() (bool, error) {
	cameraOn := isCameraOn()
	log.Printf("[webcam] CheckNow: on=%v", cameraOn)

	m.mu.Lock()
	m.active = cameraOn
	m.mu.Unlock()

	return cameraOn, nil
}
