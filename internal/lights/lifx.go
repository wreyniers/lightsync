package lights

import (
	"context"
	"fmt"
	"log"
	"math"
	"net"
	"sync"
	"time"

	"go.yhsif.com/lifxlan"
	"go.yhsif.com/lifxlan/light"
)

type LIFXController struct {
	mu      sync.RWMutex
	devices map[string]lifxlan.Device
	lights  map[string]light.Device
}

func NewLIFXController() *LIFXController {
	return &LIFXController{
		devices: make(map[string]lifxlan.Device),
		lights:  make(map[string]light.Device),
	}
}

func (c *LIFXController) Brand() Brand {
	return BrandLIFX
}

func (c *LIFXController) Discover(ctx context.Context) ([]Device, error) {
	discoverCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	ch := make(chan lifxlan.Device)
	go func() {
		_ = lifxlan.Discover(discoverCtx, ch, "")
	}()

	seen := make(map[string]bool)
	var result []Device

	for raw := range ch {
		target := raw.Target().String()
		if seen[target] {
			continue
		}
		seen[target] = true

		labelCtx, labelCancel := context.WithTimeout(ctx, 2*time.Second)
		ld, err := light.Wrap(labelCtx, raw, false)
		labelCancel()
		if err != nil {
			continue
		}

		// Fetch hardware version and firmware (best-effort; errors are non-fatal).
		versionCtx, versionCancel := context.WithTimeout(ctx, 2*time.Second)
		_ = raw.GetHardwareVersion(versionCtx, nil)
		versionCancel()

		firmwareCtx, firmwareCancel := context.WithTimeout(ctx, 2*time.Second)
		_ = raw.GetFirmware(firmwareCtx, nil)
		firmwareCancel()

		supportsColor := true
		supportsKelvin := true
		var minKelvin, maxKelvin int
		var productName string
		if product := raw.HardwareVersion().Parse(); product != nil {
			supportsColor = product.Features.Color.Get()
			productName = product.ProductName
			tr := product.Features.TemperatureRange
			if tr.Valid() {
				minKelvin = int(tr.Min())
				maxKelvin = int(tr.Max())
			}
		}

		var firmwareVersion string
		if fw := raw.Firmware(); fw.String() != lifxlan.EmptyFirmware {
			firmwareVersion = fmt.Sprintf("%d.%d", fw.Major, fw.Minor)
		}

		deviceID := fmt.Sprintf("lifx:%s", target)
		host, _, _ := net.SplitHostPort(raw.Target().String())
		if host == "" {
			host = target
		}

		c.mu.Lock()
		c.devices[deviceID] = raw
		c.lights[deviceID] = ld
		c.mu.Unlock()

		name := ld.Label().String()
		if name == lifxlan.EmptyLabel {
			name = fmt.Sprintf("LIFX %s", target)
		}

		result = append(result, Device{
			ID:              deviceID,
			Brand:           BrandLIFX,
			Name:            name,
			Model:           productName,
			LastIP:          host,
			LastSeen:        time.Now(),
			SupportsColor:   supportsColor,
			SupportsKelvin:  supportsKelvin,
			MinKelvin:       minKelvin,
			MaxKelvin:       maxKelvin,
			KelvinStep:      1,
			FirmwareVersion: firmwareVersion,
		})
	}

	return result, nil
}

func (c *LIFXController) SetState(ctx context.Context, deviceID string, state DeviceState) error {
	ld, err := c.getLight(ctx, deviceID)
	if err != nil {
		return err
	}

	conn, err := ld.Dial()
	if err != nil {
		log.Printf("[lifx] Dial failed for %s, re-discovering: %v", deviceID, err)
		ld, err = c.rediscoverDevice(ctx, deviceID)
		if err != nil {
			return err
		}
		conn, err = ld.Dial()
		if err != nil {
			return fmt.Errorf("dial %s after re-discovery: %w", deviceID, err)
		}
	}
	defer conn.Close()

	if !state.On {
		if err := ld.SetLightPower(ctx, conn, lifxlan.PowerOff, 200*time.Millisecond, false); err != nil {
			log.Printf("[lifx] SetLightPower(off) failed for %s: %v", deviceID, err)
			return err
		}
		return nil
	}

	if err := ld.SetLightPower(ctx, conn, lifxlan.PowerOn, 200*time.Millisecond, false); err != nil {
		log.Printf("[lifx] SetLightPower(on) failed for %s: %v", deviceID, err)
		return err
	}

	color := stateToLIFXColor(state)
	if err := ld.SetColor(ctx, conn, &color, 200*time.Millisecond, false); err != nil {
		log.Printf("[lifx] SetColor failed for %s: %v", deviceID, err)
		return err
	}
	return nil
}

func (c *LIFXController) GetState(ctx context.Context, deviceID string) (DeviceState, error) {
	ld, err := c.getLight(ctx, deviceID)
	if err != nil {
		return DeviceState{}, err
	}

	conn, err := ld.Dial()
	if err != nil {
		return DeviceState{}, fmt.Errorf("dial %s: %w", deviceID, err)
	}
	defer conn.Close()

	power, err := ld.GetPower(ctx, conn)
	if err != nil {
		return DeviceState{}, err
	}

	color, err := ld.GetColor(ctx, conn)
	if err != nil {
		return DeviceState{}, err
	}

	return lifxColorToState(power, color), nil
}

func (c *LIFXController) TurnOn(ctx context.Context, deviceID string) error {
	return c.setPower(ctx, deviceID, lifxlan.PowerOn)
}

func (c *LIFXController) TurnOff(ctx context.Context, deviceID string) error {
	return c.setPower(ctx, deviceID, lifxlan.PowerOff)
}

func (c *LIFXController) setPower(ctx context.Context, deviceID string, power lifxlan.Power) error {
	ld, err := c.getLight(ctx, deviceID)
	if err != nil {
		return err
	}

	conn, err := ld.Dial()
	if err != nil {
		log.Printf("[lifx] Dial failed for %s, re-discovering: %v", deviceID, err)
		ld, err = c.rediscoverDevice(ctx, deviceID)
		if err != nil {
			return err
		}
		conn, err = ld.Dial()
		if err != nil {
			return fmt.Errorf("dial %s after re-discovery: %w", deviceID, err)
		}
	}
	defer conn.Close()

	if err := ld.SetLightPower(ctx, conn, power, 200*time.Millisecond, false); err != nil {
		log.Printf("[lifx] SetLightPower failed for %s: %v", deviceID, err)
		return err
	}
	log.Printf("[lifx] Power set to %v for %s", power.On(), deviceID)
	return nil
}

// getLight retrieves a known light, or re-discovers if missing.
func (c *LIFXController) getLight(ctx context.Context, deviceID string) (light.Device, error) {
	c.mu.RLock()
	ld, ok := c.lights[deviceID]
	c.mu.RUnlock()
	if ok {
		return ld, nil
	}
	log.Printf("[lifx] Device %s not in cache, running discovery", deviceID)
	return c.rediscoverDevice(ctx, deviceID)
}

func (c *LIFXController) rediscoverDevice(ctx context.Context, deviceID string) (light.Device, error) {
	discoverCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, _ = c.Discover(discoverCtx)

	c.mu.RLock()
	ld, ok := c.lights[deviceID]
	c.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("device %s not found after re-discovery", deviceID)
	}
	log.Printf("[lifx] Re-discovered %s successfully", deviceID)
	return ld, nil
}

func (c *LIFXController) Close() error {
	return nil
}

func stateToLIFXColor(state DeviceState) lifxlan.Color {
	kelvin := uint16(3500)
	if state.Kelvin != nil {
		kelvin = uint16(*state.Kelvin)
	}

	if state.Color != nil {
		return lifxlan.Color{
			Hue:        uint16(state.Color.H / 360.0 * math.MaxUint16),
			Saturation: uint16(state.Color.S * math.MaxUint16),
			Brightness: uint16(state.Brightness * math.MaxUint16),
			Kelvin:     kelvin,
		}
	}

	return lifxlan.Color{
		Hue:        0,
		Saturation: 0,
		Brightness: uint16(state.Brightness * math.MaxUint16),
		Kelvin:     kelvin,
	}
}

func lifxColorToState(power lifxlan.Power, color *lifxlan.Color) DeviceState {
	kelvin := int(color.Kelvin)
	state := DeviceState{
		On:         power.On(),
		Brightness: float64(color.Brightness) / math.MaxUint16,
		Kelvin:     &kelvin,
	}

	if color.Saturation > 0 {
		state.Color = &Color{
			H: float64(color.Hue) / math.MaxUint16 * 360.0,
			S: float64(color.Saturation) / math.MaxUint16,
			B: float64(color.Brightness) / math.MaxUint16,
		}
	}

	return state
}
