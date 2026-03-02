// Package assign implements color-to-device assignment algorithms for Screen Sync.
package assign

import (
	"math"

	"lightsync/internal/lights"
)

// Fixed perceptual weights for HSB distance. Hue dominates because it is the
// strongest human perceptual cue; saturation and brightness play supporting roles.
const (
	hueWeight    = 0.65
	satWeight    = 0.20
	brightWeight = 0.15
)

// colorDist returns the weighted perceptual distance between two HSB colors.
// Hue uses shortest-path angular distance normalised to [0, 1] (max = 180°).
func colorDist(a, b lights.Color) float64 {
	hDeg := math.Abs(hueShortest(a.H, b.H))
	hNorm := hDeg / 180.0
	dS := a.S - b.S
	dB := a.B - b.B
	return math.Sqrt(
		hueWeight*hueWeight*hNorm*hNorm +
			satWeight*satWeight*dS*dS +
			brightWeight*brightWeight*dB*dB,
	)
}

// hueShortest returns the signed shortest angular difference a → b in degrees.
func hueShortest(a, b float64) float64 {
	d := b - a
	for d > 180 {
		d -= 360
	}
	for d < -180 {
		d += 360
	}
	return d
}

// lerpColor linearly interpolates between two HSB colors using shortest-path hue.
func lerpColor(a, b lights.Color, t float64) lights.Color {
	dh := hueShortest(a.H, b.H)
	h := a.H + dh*t
	for h < 0 {
		h += 360
	}
	for h >= 360 {
		h -= 360
	}
	return lights.Color{
		H: h,
		S: a.S + (b.S-a.S)*t,
		B: a.B + (b.B-a.B)*t,
	}
}
