// Package extract implements color extraction algorithms for the Screen Sync engine.
package extract

import (
	"image"
	"math"

	"lightsync/internal/lights"
	"lightsync/internal/store"
)

// Extractor extracts one or more representative colors from a captured frame.
type Extractor interface {
	Extract(img image.Image, cfg store.ScreenSyncConfig) []lights.Color
}

// New returns a DispatchExtractor that re-reads cfg on every Extract call.
// This means colorMode, multiColorApproach, and extractionMethod changes are
// picked up immediately without restarting the engine.
func New(_ store.ScreenSyncConfig) Extractor {
	return &DispatchExtractor{}
}

// DispatchExtractor reads the full config on every Extract call and delegates
// to the appropriate algorithm, so live config changes take effect next frame.
// It holds a persistent PaletteExtractor so palette temporal blending survives
// across frames — the extractor is recreated on each engine Start.
type DispatchExtractor struct {
	palette PaletteExtractor
}

func (d *DispatchExtractor) Extract(img image.Image, cfg store.ScreenSyncConfig) []lights.Color {
	if cfg.ColorMode == store.ColorModeMulti {
		switch cfg.MultiColorApproach {
		case store.MultiColorScenePalette:
			return d.palette.Extract(img, cfg)
		default: // spatial_grid
			return (&SpatialExtractor{}).Extract(img, cfg)
		}
	}
	return singleColorExtract(img, cfg)
}

// singleColorExtract applies the single-color extraction method from cfg.
func singleColorExtract(img image.Image, cfg store.ScreenSyncConfig) []lights.Color {
	bounds := img.Bounds()
	switch cfg.ExtractionMethod {
	case store.ExtractionMethodBrightest:
		return []lights.Color{brightestInCell(img, bounds, cfg.WhiteBias)}
	case store.ExtractionMethodSaturated:
		return []lights.Color{mostSaturatedInCell(img, bounds, cfg.WhiteBias)}
	case store.ExtractionMethodVivid:
		return []lights.Color{vividDominantColor(img, cfg.WhiteBias)}
	case store.ExtractionMethodDiverse:
		// Diverse is a multi-color concept; fall back to vivid for single.
		return []lights.Color{vividDominantColor(img, cfg.WhiteBias)}
	default: // dominant
		return []lights.Color{dominantColor(img, cfg.WhiteBias)}
	}
}

// sampleStep is the pixel stride used when sampling an image.
// Every (sampleStep)th pixel is examined in both X and Y, giving ~1/16 of all pixels.
const sampleStep = 4

// rgbToHSB converts sRGB (each 0-1) to HSB (H: 0-360, S: 0-1, B: 0-1).
func rgbToHSB(r, g, b float64) (h, s, bv float64) {
	maxC := r
	if g > maxC {
		maxC = g
	}
	if b > maxC {
		maxC = b
	}
	minC := r
	if g < minC {
		minC = g
	}
	if b < minC {
		minC = b
	}

	bv = maxC
	if maxC == 0 {
		return 0, 0, 0
	}
	s = (maxC - minC) / maxC

	if maxC == minC {
		h = 0
		return
	}
	d := maxC - minC
	switch maxC {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h *= 60
	return
}

// colorDist returns the Euclidean distance between two quantized RGB triples.
func colorDist(a, b [3]uint8) float64 {
	dr := float64(int(a[0]) - int(b[0]))
	dg := float64(int(a[1]) - int(b[1]))
	db := float64(int(a[2]) - int(b[2]))
	// Fast integer-ish approximation — no sqrt needed for comparisons.
	return dr*dr + dg*dg + db*db
}

// luminance returns the perceptual luminance (ITU-R BT.709) of an RGB pixel.
func luminance(r, g, b uint8) float64 {
	return 0.2126*float64(r)/255 + 0.7152*float64(g)/255 + 0.0722*float64(b)/255
}

// saturation returns the HSV saturation (0-1) of an RGB pixel.
func saturation(r, g, b uint8) float64 {
	maxC := float64(r)
	if float64(g) > maxC {
		maxC = float64(g)
	}
	if float64(b) > maxC {
		maxC = float64(b)
	}
	minC := float64(r)
	if float64(g) < minC {
		minC = float64(g)
	}
	if float64(b) < minC {
		minC = float64(b)
	}
	if maxC == 0 {
		return 0
	}
	return (maxC - minC) / maxC
}

// shouldSkipWhiteBias returns true when white-bias filtering should exclude this pixel.
// Negative white bias rejects low-saturation (grayscale) pixels.
// Positive white bias rejects high-saturation pixels.
func shouldSkipWhiteBias(r, g, b uint8, whiteBias float64) bool {
	if whiteBias == 0 {
		return false
	}
	sat := saturation(r, g, b)
	if whiteBias < 0 {
		// Reject grayscale — skip pixels whose saturation is below the threshold.
		return sat < -whiteBias*0.5
	}
	// Positive bias: skip colorful pixels to favour white/grey.
	return sat > 1-(whiteBias*0.5)
}

// fallbackColor is returned when no suitable pixels are found.
var fallbackColor = lights.Color{H: 0, S: 0, B: 0.5}

// hsbColorDist returns a weighted perceptual distance between two HSB colours.
// Used inside the extract package for palette-level temporal matching.
func hsbColorDist(a, b lights.Color) float64 {
	dh := a.H - b.H
	for dh > 180 {
		dh -= 360
	}
	for dh < -180 {
		dh += 360
	}
	if dh < 0 {
		dh = -dh
	}
	hNorm := dh / 180.0
	dS := a.S - b.S
	dB := a.B - b.B
	return hNorm*hNorm*0.65 + dS*dS*0.20 + dB*dB*0.15
}

// lerpHSB linearly interpolates two HSB colours using shortest-path hue blending.
func lerpHSB(a, b lights.Color, t float64) lights.Color {
	dh := b.H - a.H
	for dh > 180 {
		dh -= 360
	}
	for dh < -180 {
		dh += 360
	}
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

// replaceBlackCells replaces colors with B < threshold using the average of others.
// Prevents a light from staying stuck on black when its screen region is dark.
func replaceBlackCells(colors []lights.Color, threshold float64) {
	for i, c := range colors {
		if c.B < threshold {
			colors[i] = averageOfOthers(colors, i)
		}
	}
}

// averageOfOthers returns the average H, S, B of all colors except the one at exclude.
// Skips near-black colors when computing the average. Uses circular mean for hue.
func averageOfOthers(colors []lights.Color, exclude int) lights.Color {
	if len(colors) <= 1 {
		return fallbackColor
	}
	var hSin, hCos, sSum, bSum float64
	count := 0
	for i, c := range colors {
		if i == exclude || c.B < 0.02 {
			continue
		}
		rad := c.H * math.Pi / 180
		hSin += math.Sin(rad)
		hCos += math.Cos(rad)
		sSum += c.S
		bSum += c.B
		count++
	}
	if count == 0 {
		return fallbackColor
	}
	h := math.Atan2(hSin/float64(count), hCos/float64(count)) * 180 / math.Pi
	if h < 0 {
		h += 360
	}
	return lights.Color{H: h, S: sSum / float64(count), B: bSum / float64(count)}
}
