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

	wakeCh chan struct{}
}

func NewMonitor(interval time.Duration) *Monitor {
	return &Monitor{
		interval: interval,
		enabled:  true,
		wakeCh:   make(chan struct{}, 1),
	}
}

func (m *Monitor) OnChange(handler StateChangeHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onChange = handler
}

func (m *Monitor) SetEnabled(enabled bool) {
	m.mu.Lock()
	m.enabled = enabled
	m.mu.Unlock()
	m.wake()
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

// SetInterval sets the poll interval. It only applies when the platform has
// no push source for camera activity.
func (m *Monitor) SetInterval(d time.Duration) {
	m.mu.Lock()
	m.interval = d
	m.mu.Unlock()
	m.wake()
}

func (m *Monitor) wake() {
	select {
	case m.wakeCh <- struct{}{}:
	default:
	}
}

func (m *Monitor) Start(ctx context.Context) {
	events := cameraEvents()

	// Without a push source, fall back to polling on the configured interval.
	var ticker *time.Ticker
	var tick <-chan time.Time
	if events == nil {
		m.mu.RLock()
		interval := m.interval
		m.mu.RUnlock()
		ticker = time.NewTicker(interval)
		defer ticker.Stop()
		tick = ticker.C
		log.Printf("[webcam] Monitor started (polling every %v)", interval)
	} else {
		log.Println("[webcam] Monitor started (event-driven)")
	}

	m.evaluate()

	for {
		select {
		case <-ctx.Done():
			log.Println("[webcam] Monitor stopped")
			return
		case <-m.wakeCh:
			if ticker != nil {
				m.mu.RLock()
				newInterval := m.interval
				m.mu.RUnlock()
				ticker.Reset(newInterval)
			}
			m.evaluate()
		case <-tick:
			m.evaluate()
		case <-events:
			m.evaluate()
		}
	}
}

// evaluate reads the current camera state and fires onChange if it changed.
func (m *Monitor) evaluate() {
	m.mu.RLock()
	enabled := m.enabled
	m.mu.RUnlock()

	if !enabled {
		return
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

func (m *Monitor) CheckNow() (bool, error) {
	cameraOn := isCameraOn()
	log.Printf("[webcam] CheckNow: on=%v", cameraOn)

	m.mu.Lock()
	m.active = cameraOn
	m.mu.Unlock()

	return cameraOn, nil
}
