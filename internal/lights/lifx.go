package lights

import (
	"bytes"
	"context"
	"encoding/binary"
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

	rediscoverMu sync.Mutex // serializes rediscoverDevice to avoid thundering herd
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

// lifxBroadcastAddrs returns the subnet-directed broadcast address for every
// active IPv4 interface (e.g. 192.168.4.255 for a /24) plus the global
// 255.255.255.255 fallback. Subnet-directed broadcasts reach the correct
// interface regardless of the OS routing table, which matters on multi-NIC
// Windows machines (e.g. WSL virtual adapter present).
func lifxBroadcastAddrs() []*net.UDPAddr {
	global, _ := net.ResolveUDPAddr("udp4", net.JoinHostPort(lifxlan.DefaultBroadcastHost, lifxlan.DefaultBroadcastPort))
	addrs := []*net.UDPAddr{global}

	ifaces, err := net.Interfaces()
	if err != nil {
		return addrs
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		ifAddrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range ifAddrs {
			ipNet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipNet.IP.To4()
			if ip4 == nil {
				continue
			}
			bcast := make(net.IP, 4)
			for i := range 4 {
				bcast[i] = ip4[i] | ^ipNet.Mask[i]
			}
			addr, err := net.ResolveUDPAddr("udp4", net.JoinHostPort(bcast.String(), lifxlan.DefaultBroadcastPort))
			if err == nil {
				addrs = append(addrs, addr)
			}
		}
	}
	return addrs
}

// discoverLIFX sends LIFX GetService broadcasts over an explicit IPv4 socket
// and returns discovered devices on a channel that is closed when ctx expires.
// This replaces lifxlan.Discover which uses a dual-stack "udp" socket that
// fails to broadcast on Windows with newer Go versions (no SO_BROADCAST on
// IPv6 sockets).
func discoverLIFX(ctx context.Context) (<-chan lifxlan.Device, error) {
	msg, err := lifxlan.GenerateMessage(
		lifxlan.Tagged,
		0, // source
		lifxlan.AllDevices,
		0, // flags
		0, // sequence
		lifxlan.GetService,
		nil, // payload
	)
	if err != nil {
		return nil, fmt.Errorf("generate GetService: %w", err)
	}

	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return nil, fmt.Errorf("listen udp4: %w", err)
	}

	targets := lifxBroadcastAddrs()

	ch := make(chan lifxlan.Device, 64)
	go func() {
		defer close(ch)
		defer conn.Close()

		// Send 3 rounds of broadcasts 500ms apart for reliability (UDP is lossy).
		const broadcastRounds = 3
		const broadcastInterval = 500 * time.Millisecond
		go func() {
			for i := range broadcastRounds {
				if ctx.Err() != nil {
					return
				}
				for _, target := range targets {
					if _, err := conn.WriteTo(msg, target); err != nil {
						log.Printf("[lifx] broadcast to %s failed: %v", target.IP, err)
					}
				}
				if i < broadcastRounds-1 {
					time.Sleep(broadcastInterval)
				}
			}
		}()

		buf := make([]byte, 4096)
		for {
			if ctx.Err() != nil {
				return
			}
			if err := conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
				return
			}
			n, addr, err := conn.ReadFrom(buf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				return
			}

			host, _, err := net.SplitHostPort(addr.String())
			if err != nil {
				continue
			}
			resp, err := lifxlan.ParseResponse(buf[:n])
			if err != nil {
				continue
			}
			if resp.Message != lifxlan.StateService {
				continue
			}

			var payload lifxlan.RawStateServicePayload
			if err := binary.Read(bytes.NewReader(resp.Payload), binary.LittleEndian, &payload); err != nil {
				continue
			}
			if payload.Service != lifxlan.ServiceUDP {
				continue
			}

			dev := lifxlan.NewDevice(
				net.JoinHostPort(host, fmt.Sprintf("%d", payload.Port)),
				payload.Service,
				resp.Target,
			)
			select {
			case ch <- dev:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

func (c *LIFXController) Discover(ctx context.Context) ([]Device, error) {
	log.Printf("[lifx] Discovering via UDP4 broadcast (port 56700)...")
	discoverCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	ch, err := discoverLIFX(discoverCtx)
	if err != nil {
		return nil, fmt.Errorf("lifx discovery: %w", err)
	}

	// Drain responses with a silence timeout: stop collecting once no new
	// unique device has appeared for 1.5s. This avoids blocking for the full
	// 5s discovery window when all devices respond within the first second.
	seen := make(map[string]bool)
	var rawDevices []lifxlan.Device
	silence := time.NewTimer(1500 * time.Millisecond)
	defer silence.Stop()
drain:
	for {
		select {
		case raw, ok := <-ch:
			if !ok {
				break drain
			}
			target := raw.Target().String()
			if seen[target] {
				continue
			}
			seen[target] = true
			rawDevices = append(rawDevices, raw)
			log.Printf("[lifx] Got response from device %s", target)
			silence.Reset(1500 * time.Millisecond)
		case <-silence.C:
			break drain
		}
	}
	cancel() // stop the broadcast goroutine early

	var result []Device
	for _, raw := range rawDevices {
		target := raw.Target().String()

		labelCtx, labelCancel := context.WithTimeout(ctx, 2*time.Second)
		ld, err := light.Wrap(labelCtx, raw, false)
		labelCancel()
		if err != nil {
			log.Printf("[lifx] Failed to wrap device %s (dropped): %v", target, err)
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
		var host string
		if conn, dialErr := raw.Dial(); dialErr == nil {
			host, _, _ = net.SplitHostPort(conn.RemoteAddr().String())
			conn.Close()
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

	log.Printf("[lifx] Found %d device(s)", len(result))
	return result, nil
}

const lifxTransition = 200 * time.Millisecond

func (c *LIFXController) SetState(ctx context.Context, deviceID string, state DeviceState) error {
	ld, conn, err := c.dialWithRetry(ctx, deviceID)
	if err != nil {
		return err
	}
	defer conn.Close()

	if !state.On {
		return ld.SetLightPower(ctx, conn, lifxlan.PowerOff, lifxTransition, false)
	}

	if err := ld.SetLightPower(ctx, conn, lifxlan.PowerOn, lifxTransition, false); err != nil {
		return err
	}

	color := stateToLIFXColor(state)
	return ld.SetColor(ctx, conn, &color, lifxTransition, false)
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
	ld, conn, err := c.dialWithRetry(ctx, deviceID)
	if err != nil {
		return err
	}
	defer conn.Close()
	return ld.SetLightPower(ctx, conn, power, lifxTransition, false)
}

func (c *LIFXController) dialWithRetry(ctx context.Context, deviceID string) (light.Device, net.Conn, error) {
	ld, err := c.getLight(ctx, deviceID)
	if err != nil {
		return nil, nil, err
	}

	conn, err := ld.Dial()
	if err != nil {
		log.Printf("[lifx] Dial failed for %s, re-discovering: %v", deviceID, err)
		ld, err = c.rediscoverDevice(ctx, deviceID)
		if err != nil {
			return nil, nil, err
		}
		conn, err = ld.Dial()
		if err != nil {
			return nil, nil, fmt.Errorf("dial %s after re-discovery: %w", deviceID, err)
		}
	}
	return ld, conn, nil
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
	c.rediscoverMu.Lock()
	defer c.rediscoverMu.Unlock()

	// Re-check: a concurrent caller may have populated the cache while we waited.
	c.mu.RLock()
	ld, ok := c.lights[deviceID]
	c.mu.RUnlock()
	if ok {
		return ld, nil
	}

	discoverCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, _ = c.Discover(discoverCtx)

	c.mu.RLock()
	ld, ok = c.lights[deviceID]
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
	kelvin := uint16(DefaultKelvin)
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
