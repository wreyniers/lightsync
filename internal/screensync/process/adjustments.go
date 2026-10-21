// Package process implements color adjustments applied after extraction.
package process

import (
	"lightsync/internal/lights"
	"lightsync/internal/store"
)

// ApplyAdjustments applies saturation boost and brightness clamping to a slice
// of extracted colors. It returns a new slice; the input is not modified.
//
// Hue is passed through unchanged. A prior "warm hue correction" (+12° push on
// 0°–60°) was removed when the screen-sync send path switched to matching the
// preview swatches' HSL colour space, which made the compensation unnecessary
// and overshoot into yellow.
func ApplyAdjustments(colors []lights.Color, cfg store.ScreenSyncConfig) []lights.Color {
	result := make([]lights.Color, len(colors))
	minB, maxB := brightnessLimits(cfg.BrightnessMode)

	for i, c := range colors {
		s := applySaturationBoost(c.S, cfg.SaturationBoost)
		b := applyBrightness(c.B, cfg.BrightnessMultiplier, minB, maxB)
		result[i] = lights.Color{H: c.H, S: s, B: b}
	}
	return result
}

func applySaturationBoost(s, boost float64) float64 {
	if boost == 0 || boost == 1 {
		return s
	}
	s = s * boost
	if s > 1 {
		return 1
	}
	return s
}

func applyBrightness(b, multiplier, minB, maxB float64) float64 {
	b = b * multiplier
	if b < minB {
		return minB
	}
	if b > maxB {
		return maxB
	}
	return b
}

// brightnessLimits returns the (min, max) brightness range for the given mode.
func brightnessLimits(mode store.BrightnessMode) (float64, float64) {
	switch mode {
	case store.BrightnessModeDark:
		return 0, 0.10
	case store.BrightnessModeMedium:
		return 0.45, 0.75
	case store.BrightnessModeBright:
		return 0.75, 1.0
	case store.BrightnessModeFullBright:
		return 1.0, 1.0
	default: // fully_dynamic
		return 0, 1.0
	}
}
