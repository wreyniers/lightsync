package lights

import (
	"math"
	"time"
)

type Brand string

const (
	BrandLIFX   Brand = "lifx"
	BrandHue    Brand = "hue"
	BrandElgato Brand = "elgato"
	BrandGovee  Brand = "govee"
)

const DefaultKelvin = 4000

type Device struct {
	ID             string    `json:"id"`
	Brand          Brand     `json:"brand"`
	Name           string    `json:"name"`
	Model          string    `json:"model,omitempty"`
	LastIP         string    `json:"lastIp"`
	LastSeen       time.Time `json:"lastSeen"`
	SupportsColor  bool      `json:"supportsColor"`
	SupportsKelvin bool      `json:"supportsKelvin"`
	// MinKelvin/MaxKelvin are the device's supported colour-temperature range in Kelvin.
	// Zero means the field is not known (use the UI default).
	MinKelvin  int `json:"minKelvin,omitempty"`
	MaxKelvin  int `json:"maxKelvin,omitempty"`
	KelvinStep int `json:"kelvinStep,omitempty"`
	// FirmwareVersion is the device's reported software/firmware version string.
	FirmwareVersion string `json:"firmwareVersion,omitempty"`
	// Room is the user-assigned room label for grouping (e.g. "Bedroom", "Office").
	Room string `json:"room,omitempty"`
}

type DeviceState struct {
	On         bool    `json:"on"`
	Brightness float64 `json:"brightness"`
	Color      *Color  `json:"color,omitempty"`
	Kelvin     *int    `json:"kelvin,omitempty"`
}

type Color struct {
	H float64 `json:"h"`
	S float64 `json:"s"`
	B float64 `json:"b"`
}

func HSBToRGB(h, s, b float64) (r, g, bl uint8) {
	if s == 0 {
		v := uint8(b * 255)
		return v, v, v
	}

	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	hh := h / 60.0
	i := int(hh)
	ff := hh - float64(i)
	p := b * (1.0 - s)
	q := b * (1.0 - s*ff)
	t := b * (1.0 - s*(1.0-ff))

	var rr, gg, bb float64
	switch i {
	case 0:
		rr, gg, bb = b, t, p
	case 1:
		rr, gg, bb = q, b, p
	case 2:
		rr, gg, bb = p, b, t
	case 3:
		rr, gg, bb = p, q, b
	case 4:
		rr, gg, bb = t, p, b
	default:
		rr, gg, bb = b, p, q
	}

	return uint8(rr * 255), uint8(gg * 255), uint8(bb * 255)
}
