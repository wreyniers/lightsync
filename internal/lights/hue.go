package lights

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
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

func (c *HueController) Brand() Brand {
	return BrandHue
}

func (c *HueController) AddBridge(ip, username string) error {
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

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

			// Derive Kelvin range from Mirek schema (1 000 000 / mirek = Kelvin).
			var minKelvin, maxKelvin int
			if l.ColorTemperature != nil && l.ColorTemperature.MirekSchema != nil {
				if v := l.ColorTemperature.MirekSchema.MirekMaximum; v != nil && *v > 0 {
					minKelvin = 1_000_000 / *v
				}
				if v := l.ColorTemperature.MirekSchema.MirekMinimum; v != nil && *v > 0 {
					maxKelvin = 1_000_000 / *v
				}
			}

			// Look up product model/firmware via the light's owning device.
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
		}
		log.Printf("[hue] Bridge %s: found %d light(s)", conn.bridge.IP, len(result))
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
	return c.SetState(ctx, deviceID, DeviceState{On: true, Brightness: 1.0})
}

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
		return 4000
	}
	return 1000000 / mirek
}

func hsbToXY(h, s, b float64) [2]float64 {
	r, g, bl := HSBToRGB(h, s, b)
	rf := gammaCorrect(float64(r) / 255.0)
	gf := gammaCorrect(float64(g) / 255.0)
	bf := gammaCorrect(float64(bl) / 255.0)

	x := rf*0.664511 + gf*0.154324 + bf*0.162028
	y := rf*0.283881 + gf*0.668433 + bf*0.047685
	z := rf*0.000088 + gf*0.072310 + bf*0.986039

	sum := x + y + z
	if sum == 0 {
		return [2]float64{0.3127, 0.3290}
	}
	return [2]float64{x / sum, y / sum}
}

func gammaCorrect(v float64) float64 {
	if v > 0.04045 {
		return pow((v+0.055)/1.055, 2.4)
	}
	return v / 12.92
}

func pow(base, exp float64) float64 {
	if base <= 0 {
		return 0
	}
	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}
	frac := exp - float64(int(exp))
	if frac > 0 {
		result *= (1.0 + frac*(base-1.0))
	}
	return result
}
