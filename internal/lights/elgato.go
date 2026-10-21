package lights

import (
	"context"
	"fmt"
	"log"
	"math"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/mdlayher/keylight"
)

type ElgatoController struct {
	mu      sync.RWMutex
	clients map[string]*keylight.Client
	addrs   map[string]string
}

func NewElgatoController() *ElgatoController {
	return &ElgatoController{
		clients: make(map[string]*keylight.Client),
		addrs:   make(map[string]string),
	}
}

func (c *ElgatoController) Brand() Brand {
	return BrandElgato
}

// elgatoBaseURL returns the Key Light HTTP base URL (e.g. http://192.168.4.73:9123).
// Accepts a bare IP/hostname, host:port, or a full http(s) URL — avoids double "http://".
func elgatoBaseURL(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	lower := strings.ToLower(addr)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		u, err := url.Parse(addr)
		if err != nil || u.Host == "" {
			return ""
		}
		host := u.Hostname()
		port := u.Port()
		if port == "" {
			port = "9123"
		}
		return "http://" + net.JoinHostPort(host, port)
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
		port = "9123"
	}
	return "http://" + net.JoinHostPort(host, port)
}

// elgatoDeviceIDSuffix builds the stable part after "elgato:" (host, or host:port if non-default).
func elgatoDeviceIDSuffix(addr string) string {
	base := elgatoBaseURL(addr)
	if base == "" {
		return strings.TrimSpace(addr)
	}
	u, err := url.Parse(base)
	if err != nil {
		return addr
	}
	h := u.Hostname()
	p := u.Port()
	if p == "" || p == "9123" {
		return h
	}
	return net.JoinHostPort(h, p)
}

func (c *ElgatoController) Discover(ctx context.Context) ([]Device, error) {
	c.mu.RLock()
	existing := make(map[string]string, len(c.addrs))
	for k, v := range c.addrs {
		existing[k] = v
	}
	c.mu.RUnlock()

	log.Printf("[elgato] Discover: %d known address(es) to probe", len(existing))

	var result []Device
	for id, addr := range existing {
		log.Printf("[elgato] Probing %s at %s", id, addr)
		client, err := keylight.NewClient(addr, nil)
		if err != nil {
			log.Printf("[elgato] Failed to create client for %s: %v", id, err)
			continue
		}
		d, err := client.AccessoryInfo(ctx)
		if err != nil {
			log.Printf("[elgato] Failed to get accessory info for %s: %v", id, err)
			continue
		}

		c.mu.Lock()
		c.clients[id] = client
		c.mu.Unlock()

		log.Printf("[elgato] Discovered: %s (%s) at %s", d.DisplayName, d.ProductName, addr)
		result = append(result, Device{
			ID:              id,
			Brand:           BrandElgato,
			Name:            d.DisplayName,
			Model:           d.ProductName,
			LastIP:          elgatoBaseURL(addr),
			LastSeen:        time.Now(),
			SupportsColor:   false,
			SupportsKelvin:  true,
			MinKelvin:       2900,
			MaxKelvin:       7000,
			KelvinStep:      50,
			FirmwareVersion: d.FirmwareVersion,
		})
	}

	return result, nil
}

func (c *ElgatoController) AddDevice(addr string) {
	fullAddr := elgatoBaseURL(addr)
	if fullAddr == "" {
		log.Printf("[elgato] Invalid address %q", addr)
		return
	}
	deviceID := "elgato:" + elgatoDeviceIDSuffix(addr)
	log.Printf("[elgato] Adding device %s at %s", deviceID, fullAddr)
	client, err := keylight.NewClient(fullAddr, nil)
	if err != nil {
		log.Printf("[elgato] Failed to create client for %s: %v", fullAddr, err)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.clients[deviceID] = client
	c.addrs[deviceID] = fullAddr
	log.Printf("[elgato] Device %s registered successfully", deviceID)
}

func (c *ElgatoController) SetState(ctx context.Context, deviceID string, state DeviceState) error {
	client, err := c.getClient(deviceID)
	if err != nil {
		return err
	}

	temp := 4000
	if state.Kelvin != nil {
		temp = *state.Kelvin
	}
	if temp < 2900 {
		temp = 2900
	}
	if temp > 7000 {
		temp = 7000
	}

	// Library requires brightness in [3, 100]; use Round to avoid float truncation.
	brightness := int(math.Round(state.Brightness * 100))
	if brightness < 3 {
		brightness = 3
	}
	if brightness > 100 {
		brightness = 100
	}

	ll := []*keylight.Light{
		{
			On:          state.On,
			Brightness:  brightness,
			Temperature: temp,
		},
	}

	if err := client.SetLights(ctx, ll); err != nil {
		log.Printf("[elgato] SetLights failed for %s, reconnecting: %v", deviceID, err)
		client, err = c.reconnect(deviceID)
		if err != nil {
			return err
		}
		return client.SetLights(ctx, ll)
	}
	log.Printf("[elgato] SetState applied for %s: on=%v brightness=%d temp=%d", deviceID, state.On, int(state.Brightness*100), temp)
	return nil
}

func (c *ElgatoController) GetState(ctx context.Context, deviceID string) (DeviceState, error) {
	client, err := c.getClient(deviceID)
	if err != nil {
		return DeviceState{}, err
	}

	ll, err := client.Lights(ctx)
	if err != nil {
		return DeviceState{}, err
	}

	if len(ll) == 0 {
		return DeviceState{}, fmt.Errorf("no lights found on device %s", deviceID)
	}

	l := ll[0]
	kelvin := l.Temperature
	return DeviceState{
		On:         l.On,
		Brightness: float64(l.Brightness) / 100.0,
		Kelvin:     &kelvin,
	}, nil
}

func (c *ElgatoController) TurnOn(ctx context.Context, deviceID string) error {
	return c.setPower(ctx, deviceID, true)
}

func (c *ElgatoController) TurnOff(ctx context.Context, deviceID string) error {
	return c.setPower(ctx, deviceID, false)
}

func (c *ElgatoController) setPower(ctx context.Context, deviceID string, on bool) error {
	client, err := c.getClient(deviceID)
	if err != nil {
		return err
	}

	ll, err := client.Lights(ctx)
	if err != nil {
		log.Printf("[elgato] Lights() failed for %s, reconnecting: %v", deviceID, err)
		client, err = c.reconnect(deviceID)
		if err != nil {
			return err
		}
		ll, err = client.Lights(ctx)
		if err != nil {
			return fmt.Errorf("lights %s after reconnect: %w", deviceID, err)
		}
	}
	if len(ll) == 0 {
		return fmt.Errorf("no lights found on device %s", deviceID)
	}

	ll[0].On = on
	if err := client.SetLights(ctx, ll); err != nil {
		log.Printf("[elgato] SetLights failed for %s: %v", deviceID, err)
		return err
	}
	log.Printf("[elgato] Power set to %v for %s", on, deviceID)
	return nil
}

// getClient returns an existing client or creates one from the device ID's embedded IP.
func (c *ElgatoController) getClient(deviceID string) (*keylight.Client, error) {
	c.mu.RLock()
	client, ok := c.clients[deviceID]
	c.mu.RUnlock()
	if ok {
		return client, nil
	}

	// Extract IP from "elgato:<ip>" and reconnect
	log.Printf("[elgato] Client for %s not in cache, attempting reconnect", deviceID)
	return c.reconnect(deviceID)
}

func (c *ElgatoController) reconnect(deviceID string) (*keylight.Client, error) {
	suffix := strings.TrimPrefix(deviceID, "elgato:")
	if suffix == deviceID || suffix == "" {
		return nil, fmt.Errorf("cannot parse Elgato device ID %q", deviceID)
	}

	fullAddr := elgatoBaseURL(suffix)
	if fullAddr == "" {
		return nil, fmt.Errorf("invalid address in device ID %q", deviceID)
	}
	log.Printf("[elgato] Reconnecting %s at %s", deviceID, fullAddr)
	client, err := keylight.NewClient(fullAddr, nil)
	if err != nil {
		return nil, fmt.Errorf("reconnect %s: %w", deviceID, err)
	}

	c.mu.Lock()
	c.clients[deviceID] = client
	c.addrs[deviceID] = fullAddr
	c.mu.Unlock()

	log.Printf("[elgato] Reconnected %s successfully", deviceID)
	return client, nil
}

func (c *ElgatoController) Close() error {
	return nil
}

