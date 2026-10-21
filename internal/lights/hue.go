package lights

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/openhue/openhue-go"
)

type HueBridge struct {
	IP       string
	Username string
}

type HueController struct {
	mu      sync.RWMutex
	bridges map[string]*hueConnection
}

type hueConnection struct {
	bridge  HueBridge
	client  *openhue.ClientWithResponses
	devices map[string]hueDeviceInfo
}

type hueDeviceInfo struct {
	lightID string
	name    string
}

func NewHueController() *HueController {
	return &HueController{
		bridges: make(map[string]*hueConnection),
	}
}

// NewHueHTTPClient returns an HTTP client configured for Hue bridges.
// ForceAttemptHTTP2 enables HTTP/2 multiplexing even with a custom TLS config,
// allowing all concurrent per-light requests in the screen sync hot path to
// share a single TCP connection instead of each paying a full TLS handshake.
func NewHueHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
			MaxIdleConns:        20,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     90 * time.Second,
			ForceAttemptHTTP2:   true,
		},
	}
}

func (c *HueController) Brand() Brand {
	return BrandHue
}

func (c *HueController) AddBridge(ip, username string) error {
	httpClient := NewHueHTTPClient(0)

	apiURL := fmt.Sprintf("https://%s", ip)
	client, err := openhue.NewClientWithResponses(
		apiURL,
		openhue.WithHTTPClient(httpClient),
		openhue.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			req.Header.Set("hue-application-key", username)
			return nil
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to create Hue client for %s: %w", ip, err)
	}

	c.mu.Lock()
	c.bridges[ip] = &hueConnection{
		bridge:  HueBridge{IP: ip, Username: username},
		client:  client,
		devices: make(map[string]hueDeviceInfo),
	}
	c.mu.Unlock()

	return nil
}

func (c *HueController) Discover(ctx context.Context) ([]Device, error) {
	c.mu.RLock()
	bridges := make([]*hueConnection, 0, len(c.bridges))
	for _, b := range c.bridges {
		bridges = append(bridges, b)
	}
	c.mu.RUnlock()

	log.Printf("[hue] Discover: %d bridge(s) registered", len(bridges))

	var result []Device

	for _, conn := range bridges {
		log.Printf("[hue] Querying bridge %s for lights...", conn.bridge.IP)
		resp, err := conn.client.GetLightsWithResponse(ctx)
		if err != nil {
			log.Printf("[hue] Bridge %s GetLights error: %v", conn.bridge.IP, err)
			continue
		}
		if resp.HTTPResponse != nil {
			log.Printf("[hue] Bridge %s responded with HTTP %d", conn.bridge.IP, resp.HTTPResponse.StatusCode)
		}
		if resp.JSON200 == nil || resp.JSON200.Data == nil {
			log.Printf("[hue] Bridge %s returned no light data (JSON200=%v)", conn.bridge.IP, resp.JSON200 != nil)
			continue
		}

		// Fetch Hue Device resources to get model and firmware info (best-effort).
		// LightGet.Owner.Rid points to the owning DeviceGet, which has ProductData.
		type hueDeviceMeta struct {
			modelName       string
			firmwareVersion string
		}
		deviceMeta := make(map[string]hueDeviceMeta) // key: Hue device resource ID
		if devResp, err := conn.client.GetDevicesWithResponse(ctx); err == nil &&
			devResp.JSON200 != nil && devResp.JSON200.Data != nil {
			for _, hd := range *devResp.JSON200.Data {
				if hd.Id == nil || hd.ProductData == nil {
					continue
				}
				meta := hueDeviceMeta{}
				if v := hd.ProductData.ProductName; v != nil {
					meta.modelName = *v
				} else if v := hd.ProductData.ModelId; v != nil {
					meta.modelName = *v
				}
				if v := hd.ProductData.SoftwareVersion; v != nil {
					meta.firmwareVersion = *v
				}
				deviceMeta[*hd.Id] = meta
			}
		}

		bridgeCount := 0
		for _, l := range *resp.JSON200.Data {
			if l.Id == nil {
				continue
			}
			deviceID := fmt.Sprintf("hue:%s", *l.Id)
			name := "Hue Light"
			if l.Metadata != nil && l.Metadata.Name != nil {
				name = *l.Metadata.Name
			}

			c.mu.Lock()
			conn.devices[deviceID] = hueDeviceInfo{
				lightID: *l.Id,
				name:    name,
			}
			c.mu.Unlock()

			var minKelvin, maxKelvin int
			if l.ColorTemperature != nil && l.ColorTemperature.MirekSchema != nil {
				if v := l.ColorTemperature.MirekSchema.MirekMaximum; v != nil && *v > 0 {
					minKelvin = 1_000_000 / *v
				}
				if v := l.ColorTemperature.MirekSchema.MirekMinimum; v != nil && *v > 0 {
					maxKelvin = 1_000_000 / *v
				}
			}

			var modelName, firmwareVersion string
			if l.Owner != nil && l.Owner.Rid != nil {
				if meta, ok := deviceMeta[*l.Owner.Rid]; ok {
					modelName = meta.modelName
					firmwareVersion = meta.firmwareVersion
				}
			}

			result = append(result, Device{
				ID:              deviceID,
				Brand:           BrandHue,
				Name:            name,
				Model:           modelName,
				LastIP:          conn.bridge.IP,
				LastSeen:        time.Now(),
				SupportsColor:   l.Color != nil,
				SupportsKelvin:  l.ColorTemperature != nil,
				MinKelvin:       minKelvin,
				MaxKelvin:       maxKelvin,
				KelvinStep:      1,
				FirmwareVersion: firmwareVersion,
			})
			bridgeCount++
		}
		log.Printf("[hue] Bridge %s: found %d light(s)", conn.bridge.IP, bridgeCount)
	}

	return result, nil
}

func (c *HueController) findDevice(deviceID string) (*hueConnection, hueDeviceInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, conn := range c.bridges {
		if info, ok := conn.devices[deviceID]; ok {
			return conn, info, true
		}
	}
	return nil, hueDeviceInfo{}, false
}

func (c *HueController) SetState(ctx context.Context, deviceID string, state DeviceState) error {
	conn, info, ok := c.findDevice(deviceID)
	if !ok {
		return fmt.Errorf("device %s not connected", deviceID)
	}

	on := openhue.On{On: &state.On}
	body := openhue.UpdateLightJSONRequestBody{
		On: &on,
	}

	brightness := openhue.Brightness(state.Brightness * 100.0)
	body.Dimming = &openhue.Dimming{Brightness: &brightness}

	if state.Color != nil {
		xy := hsbToXY(state.Color.H, state.Color.S, state.Color.B)
		x := float32(xy[0])
		y := float32(xy[1])
		body.Color = &openhue.Color{
			Xy: &openhue.GamutPosition{X: &x, Y: &y},
		}
	}

	if state.Kelvin != nil {
		mirek := kelvinToMirek(*state.Kelvin)
		body.ColorTemperature = &openhue.ColorTemperature{
			Mirek: &mirek,
		}
	}

	resp, err := conn.client.UpdateLightWithResponse(ctx, info.lightID, body)
	if err != nil {
		log.Printf("[hue] UpdateLight %s error: %v", deviceID, err)
		return err
	}
	if resp.HTTPResponse != nil && resp.HTTPResponse.StatusCode != 200 {
		log.Printf("[hue] UpdateLight %s returned HTTP %d", deviceID, resp.HTTPResponse.StatusCode)
		return fmt.Errorf("bridge returned HTTP %d", resp.HTTPResponse.StatusCode)
	}
	return nil
}

func (c *HueController) GetState(ctx context.Context, deviceID string) (DeviceState, error) {
	conn, info, ok := c.findDevice(deviceID)
	if !ok {
		return DeviceState{}, fmt.Errorf("device %s not connected", deviceID)
	}

	resp, err := conn.client.GetLightWithResponse(ctx, info.lightID)
	if err != nil {
		return DeviceState{}, err
	}

	if resp.JSON200 == nil || resp.JSON200.Data == nil || len(*resp.JSON200.Data) == 0 {
		return DeviceState{}, fmt.Errorf("no data for device %s", deviceID)
	}

	l := (*resp.JSON200.Data)[0]
	state := DeviceState{Brightness: 1.0}

	if l.On != nil && l.On.On != nil {
		state.On = *l.On.On
	}
	if l.Dimming != nil && l.Dimming.Brightness != nil {
		state.Brightness = float64(*l.Dimming.Brightness) / 100.0
	}
	if l.ColorTemperature != nil && l.ColorTemperature.Mirek != nil {
		kelvin := mirekToKelvin(*l.ColorTemperature.Mirek)
		state.Kelvin = &kelvin
	}

	return state, nil
}

func (c *HueController) TurnOn(ctx context.Context, deviceID string) error {
	conn, info, ok := c.findDevice(deviceID)
	if !ok {
		return fmt.Errorf("device %s not connected", deviceID)
	}
	on := openhue.On{On: boolPtr(true)}
	body := openhue.UpdateLightJSONRequestBody{On: &on}
	_, err := conn.client.UpdateLightWithResponse(ctx, info.lightID, body)
	return err
}

func boolPtr(b bool) *bool { return &b }

func (c *HueController) TurnOff(ctx context.Context, deviceID string) error {
	return c.SetState(ctx, deviceID, DeviceState{On: false})
}

func (c *HueController) Close() error {
	return nil
}

func kelvinToMirek(kelvin int) int {
	if kelvin < 2000 {
		kelvin = 2000
	}
	if kelvin > 6535 {
		kelvin = 6535
	}
	return 1000000 / kelvin
}

func mirekToKelvin(mirek int) int {
	if mirek <= 0 {
		return DefaultKelvin
	}
	return 1000000 / mirek
}

// hsbToXY converts an HSB triple to a CIE xy chromaticity clamped to Hue
// gamut C (the widest of the Hue gamuts — covers newer bulbs; older gamut-A/B
// bulbs still accept gamut-C points but clamp them internally, which is the
// same behaviour the bridge applies anyway).
//
// Pipeline: HSB → sRGB → linearised sRGB → standard sRGB-D65 XYZ matrix →
// xy → clamp to gamut-C triangle. This replaces Philips' legacy "Wide RGB
// D65" matrix which mapped saturated sRGB primaries well outside any Hue
// gamut and biased clamped outputs toward magenta.
func hsbToXY(h, s, b float64) [2]float64 {
	r, g, bl := HSBToRGB(h, s, b)
	rf := srgbToLinear(float64(r) / 255.0)
	gf := srgbToLinear(float64(g) / 255.0)
	bf := srgbToLinear(float64(bl) / 255.0)

	// Standard sRGB D65 → XYZ (Bradford-adapted).
	x := rf*0.4124564 + gf*0.3575761 + bf*0.1804375
	y := rf*0.2126729 + gf*0.7151522 + bf*0.0721750
	z := rf*0.0193339 + gf*0.1191920 + bf*0.9503041

	sum := x + y + z
	if sum == 0 {
		return [2]float64{0.3127, 0.3290}
	}
	return clampToHueGamutC([2]float64{x / sum, y / sum})
}

func srgbToLinear(v float64) float64 {
	if v > 0.04045 {
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return v / 12.92
}

// Hue gamut C triangle vertices (R, G, B primaries in CIE xy).
// Source: Philips Hue developer documentation.
var (
	hueGamutCRed   = [2]float64{0.6915, 0.3083}
	hueGamutCGreen = [2]float64{0.1700, 0.7000}
	hueGamutCBlue  = [2]float64{0.1532, 0.0475}
)

// clampToHueGamutC projects p onto the closest point inside the gamut-C
// triangle when p is outside. Points inside the triangle are returned
// unchanged. Without this, out-of-gamut xy values (common for saturated sRGB
// primaries) are clamped edge-wise by the bridge in ways that bias hue.
func clampToHueGamutC(p [2]float64) [2]float64 {
	if pointInTriangle(p, hueGamutCRed, hueGamutCGreen, hueGamutCBlue) {
		return p
	}
	ab := closestPointOnSegment(p, hueGamutCRed, hueGamutCGreen)
	bc := closestPointOnSegment(p, hueGamutCGreen, hueGamutCBlue)
	ca := closestPointOnSegment(p, hueGamutCBlue, hueGamutCRed)

	dAB := distSq(p, ab)
	dBC := distSq(p, bc)
	dCA := distSq(p, ca)

	best := ab
	bestD := dAB
	if dBC < bestD {
		best = bc
		bestD = dBC
	}
	if dCA < bestD {
		best = ca
	}
	return best
}

func pointInTriangle(p, a, b, c [2]float64) bool {
	s1 := sign(p, a, b)
	s2 := sign(p, b, c)
	s3 := sign(p, c, a)
	hasNeg := s1 < 0 || s2 < 0 || s3 < 0
	hasPos := s1 > 0 || s2 > 0 || s3 > 0
	return !(hasNeg && hasPos)
}

func sign(p, a, b [2]float64) float64 {
	return (p[0]-b[0])*(a[1]-b[1]) - (a[0]-b[0])*(p[1]-b[1])
}

func closestPointOnSegment(p, a, b [2]float64) [2]float64 {
	ab := [2]float64{b[0] - a[0], b[1] - a[1]}
	ap := [2]float64{p[0] - a[0], p[1] - a[1]}
	denom := ab[0]*ab[0] + ab[1]*ab[1]
	if denom == 0 {
		return a
	}
	t := (ap[0]*ab[0] + ap[1]*ab[1]) / denom
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return [2]float64{a[0] + t*ab[0], a[1] + t*ab[1]}
}

func distSq(a, b [2]float64) float64 {
	dx := a[0] - b[0]
	dy := a[1] - b[1]
	return dx*dx + dy*dy
}
