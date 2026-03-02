// Package process implements color adjustments applied after extraction.
package process

import (
	"math"

	"lightsync/internal/lights"
	"lightsync/internal/store"
)

// ApplyAdjustments applies hue correction, saturation boost, and brightness
// clamping to a slice of extracted colors. It returns a new slice; the input
// is not modified.
func ApplyAdjustments(colors []lights.Color, cfg store.ScreenSyncConfig) []lights.Color {
	result := make([]lights.Color, len(colors))
	minB, maxB := brightnessLimits(cfg.BrightnessMode)

	for i, c := range colors {
		h := applyWarmHueCorrection(c.H)
		s := applySaturationBoost(c.S, cfg.SaturationBoost)
		b := applyBrightness(c.B, cfg.BrightnessMultiplier, minB, maxB)
		result[i] = lights.Color{H: h, S: s, B: b}
	}
	return result
}

// applyWarmHueCorrection pushes orange hues (H ≈ 5°–55°) toward yellow by up
// to +12° at the peak (H ≈ 30°). This compensates for the perceptual mismatch
// between sRGB screen orange and physical LED orange: a screen pixel at H=25°
// reads as "orange" to human eyes, but the same hue on a light bulb looks
// distinctly red. Nudging it toward H≈37° matches perceptual expectations.
//
// The correction is bell-shaped (sin envelope) so it tapers smoothly to zero
// at pure red (H=0°) and yellow (H=60°), leaving those anchors unchanged.
// Hues outside [0°, 60°] are returned unmodified.
func applyWarmHueCorrection(h float64) float64 {
	const warmRange = 60.0 // correction active window: 0° → 60°
	const peakPush = 12.0  // maximum shift in degrees (at H≈30°)

	if h <= 0 || h >= warmRange {
		return h
	}
	push := peakPush * math.Sin(h/warmRange*math.Pi)
	return h + push
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
