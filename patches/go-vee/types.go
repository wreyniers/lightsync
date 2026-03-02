package govee

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

// Message represents a message sent to or from a device.
type Message struct {
	IP      string
	Payload any
}

// Version represents a semantic version number used to identify
// device firmware and hardware versions.
type Version struct {
	Major int
	Minor int
	Patch int
}

// NewVersion creates a new Version instance.
func NewVersion(major, minor, patch int) Version {
	return Version{Major: major, Minor: minor, Patch: patch}
}

// String returns the string representation of the version.
func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// UnmarshalJSON parses the JSON-encoded version string.
func (v *Version) UnmarshalJSON(b []byte) error {
	val := bytes.Split(bytes.Trim(b, `"`), []byte{'.'})
	if len(val) != 3 {
		return ErrInvalidVersionFormat
	}
	var err error

	v.Major, err = strconv.Atoi(string(val[0]))
	if err != nil {
		v.Major = 0
		v.Minor = 0
		v.Patch = 0
		return fmt.Errorf("invalid major version: %w", ErrInvalidVersionFormat)
	}
	v.Minor, err = strconv.Atoi(string(val[1]))
	if err != nil {
		v.Major = 0
		v.Minor = 0
		v.Patch = 0
		return fmt.Errorf("invalid minor version: %w", ErrInvalidVersionFormat)
	}
	v.Patch, err = strconv.Atoi(string(val[2]))
	if err != nil {
		v.Major = 0
		v.Minor = 0
		v.Patch = 0
		return fmt.Errorf("invalid patch version: %w", ErrInvalidVersionFormat)
	}
	return nil
}

// MarshalJSON returns the JSON representation of the version.
func (v Version) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.String())
}

// State represents the on/off state of a device.
type State uint

// NewState creates a new State value.
func NewState(value uint) State {
	return State(value)
}

// String returns the string representation of the state.
func (s State) String() string {
	if s == 1 {
		return "On"
	}
	return "Off"
}

// Brightness represents the brightness level of a device.
type Brightness uint

// NewBrightness creates a new Brightness value
// with a maximum of 100%.
func NewBrightness(value uint) Brightness {
	if value > 100 {
		return NewBrightness(100)
	}
	return Brightness(value)
}

// String returns the string representation of the brightness.
func (b Brightness) String() string {
	return fmt.Sprintf("%d%%", b)
}

// Color represents an RGB color.
type Color struct {
	R uint `json:"r"`
	G uint `json:"g"`
	B uint `json:"b"`
}

// NewColor creates a new Color instance
// with RGB values clamped to the [0, 255] range.
func NewColor(r, g, b uint) Color {
	if r > 255 {
		r = 255
	}
	if g > 255 {
		g = 255
	}
	if b > 255 {
		b = 255
	}
	return Color{R: r, G: g, B: b}
}

// String returns the string representation of the color.
func (c Color) String() string {
	return fmt.Sprintf("rgb(%d, %d, %d)", c.R, c.G, c.B)
}

// ColorKelvin represents a color temperature in Kelvin.
type ColorKelvin uint

// NewColorKelvin creates a new ColorKelvin value
// with a minimum of 2000K and a maximum of 9000K.
func NewColorKelvin(value uint) ColorKelvin {
	if value < 2000 {
		value = 2000
	}
	if value > 9000 {
		value = 9000
	}
	return ColorKelvin(value)
}

// String returns the string representation of the color temperature in Kelvin.
func (c ColorKelvin) String() string {
	return fmt.Sprintf("%dK", c)
}
