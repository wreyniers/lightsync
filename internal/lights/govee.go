package lights

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	govee "github.com/swrm-io/go-vee"
)

type GoveeController struct {
	mu         sync.RWMutex
	controller *govee.Controller
	deviceMap  map[string]*govee.Device
	started    bool
}

func NewGoveeController() *GoveeController {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	ctrl := govee.NewController(logger)

	return &GoveeController{
		controller: ctrl,
		deviceMap:  make(map[string]*govee.Device),
	}
}

func (c *GoveeController) Brand() Brand {
	return BrandGovee
}

func (c *GoveeController) ensureStarted() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.started {
		return
	}
	go func() {
		_ = c.controller.Start()
	}()
	c.started = true
}

func (c *GoveeController) Discover(ctx context.Context) ([]Device, error) {
	c.ensureStarted()

	time.Sleep(3 * time.Second)

	devices := c.controller.Devices()
	var result []Device

	c.mu.Lock()
	for _, d := range devices {
		ip := d.IP()
		sku := d.SKU()
		deviceID := fmt.Sprintf("govee:%s", ip)
		c.deviceMap[deviceID] = d
		result = append(result, Device{
			ID:             deviceID,
			Brand:          BrandGovee,
			Name:           fmt.Sprintf("Govee %s (%s)", sku, ip),
			Model:          sku,
			LastIP:         ip,
			LastSeen:       time.Now(),
			SupportsColor:  true,
			SupportsKelvin: true,
		})
	}
	c.mu.Unlock()

	return result, nil
}

func (c *GoveeController) Connect(ctx context.Context, deviceID string) error {
	c.mu.RLock()
	_, ok := c.deviceMap[deviceID]
	c.mu.RUnlock()
	if ok {
		return nil
	}
	return fmt.Errorf("device %s not found, run discovery first", deviceID)
}

func (c *GoveeController) SetState(ctx context.Context, deviceID string, state DeviceState) error {
	c.mu.RLock()
	dev, ok := c.deviceMap[deviceID]
	c.mu.RUnlock()
	if !ok {
		return fmt.Errorf("device %s not connected", deviceID)
	}

	if !state.On {
		return dev.TurnOff()
	}

	if err := dev.TurnOn(); err != nil {
		return err
	}

	if err := dev.SetBrightness(govee.NewBrightness(uint(state.Brightness * 100))); err != nil {
		return err
	}

	if state.Color != nil {
		r, g, b := HSBToRGB(state.Color.H, state.Color.S, state.Color.B)
		return dev.SetColor(govee.Color{R: uint(r), G: uint(g), B: uint(b)})
	}

	if state.Kelvin != nil {
		return dev.SetColorKelvin(govee.NewColorKelvin(uint(*state.Kelvin)))
	}

	return nil
}

func (c *GoveeController) GetState(_ context.Context, deviceID string) (DeviceState, error) {
	c.mu.RLock()
	_, ok := c.deviceMap[deviceID]
	c.mu.RUnlock()
	if !ok {
		return DeviceState{}, fmt.Errorf("device %s not connected", deviceID)
	}
	return DeviceState{On: true, Brightness: 1.0}, nil
}

func (c *GoveeController) TurnOn(_ context.Context, deviceID string) error {
	c.mu.RLock()
	dev, ok := c.deviceMap[deviceID]
	c.mu.RUnlock()
	if !ok {
		return fmt.Errorf("device %s not connected", deviceID)
	}
	return dev.TurnOn()
}

func (c *GoveeController) TurnOff(_ context.Context, deviceID string) error {
	c.mu.RLock()
	dev, ok := c.deviceMap[deviceID]
	c.mu.RUnlock()
	if !ok {
		return fmt.Errorf("device %s not connected", deviceID)
	}
	return dev.TurnOff()
}

func (c *GoveeController) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.started {
		c.controller.Shutdown()
		c.started = false
	}
	return nil
}
