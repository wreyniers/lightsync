package extract

import (
	"image"
	"image/color"

	"lightsync/internal/lights"
	"lightsync/internal/store"
)

// SpatialExtractor divides the frame into a grid and extracts one color per cell.
// The number of cells matches the number of assigned devices.
type SpatialExtractor struct{}

func (s *SpatialExtractor) Extract(img image.Image, cfg store.ScreenSyncConfig) []lights.Color {
	n := len(cfg.DeviceIDs)
	if n == 0 {
		n = 1
	}
	return spatialColors(img, n, cfg.SubMethod, cfg.WhiteBias)
}

func spatialColors(img image.Image, n int, method store.ExtractionMethod, whiteBias float64) []lights.Color {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	// Arrange n cells in a roughly square grid.
	cols, rows := gridDims(n)

	cellW := w / cols
	cellH := h / rows

	colors := make([]lights.Color, 0, n)
	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			if len(colors) >= n {
				break
			}
			x0 := bounds.Min.X + col*cellW
			y0 := bounds.Min.Y + row*cellH
			x1 := x0 + cellW
			y1 := y0 + cellH
			if col == cols-1 {
				x1 = bounds.Max.X
			}
			if row == rows-1 {
				y1 = bounds.Max.Y
			}

			sub := image.Rect(x0, y0, x1, y1)
			c := extractCell(img, sub, method, whiteBias)
			colors = append(colors, c)
		}
	}

	// Pad to exactly n if grid produced fewer.
	for len(colors) < n {
		colors = append(colors, fallbackColor)
	}

	// Replace black/near-black cells with the average of others, so no light stays stuck off.
	replaceBlackCells(colors, 0.05)
	return colors
}

func extractCell(img image.Image, bounds image.Rectangle, method store.ExtractionMethod, whiteBias float64) lights.Color {
	switch method {
	case store.ExtractionMethodBrightest:
		return brightestInCell(img, bounds, whiteBias)
	case store.ExtractionMethodSaturated:
		return mostSaturatedInCell(img, bounds, whiteBias)
	case store.ExtractionMethodVivid:
		return vividDominantColor(subImage{img, bounds}, whiteBias)
	case store.ExtractionMethodDiverse:
		// Diverse doesn't make sense per-cell; fall back to vivid.
		return vividDominantColor(subImage{img, bounds}, whiteBias)
	default: // dominant
		return dominantInCell(img, bounds, whiteBias)
	}
}

func dominantInCell(img image.Image, bounds image.Rectangle, whiteBias float64) lights.Color {
	return dominantColor(subImage{img, bounds}, whiteBias)
}

func brightestInCell(img image.Image, bounds image.Rectangle, whiteBias float64) lights.Color {
	var bestR, bestG, bestB uint8
	bestLum := -1.0
	for y := bounds.Min.Y; y < bounds.Max.Y; y += sampleStep {
		for x := bounds.Min.X; x < bounds.Max.X; x += sampleStep {
			c := img.At(x, y)
			r32, g32, b32, _ := c.RGBA()
			r, g, b := uint8(r32>>8), uint8(g32>>8), uint8(b32>>8)
			if shouldSkipWhiteBias(r, g, b, whiteBias) {
				continue
			}
			if lum := luminance(r, g, b); lum > bestLum {
				bestLum = lum
				bestR, bestG, bestB = r, g, b
			}
		}
	}
	if bestLum < 0 {
		return fallbackColor
	}
	h, s, bv := rgbToHSB(float64(bestR)/255, float64(bestG)/255, float64(bestB)/255)
	return lights.Color{H: h, S: s, B: bv}
}

func mostSaturatedInCell(img image.Image, bounds image.Rectangle, whiteBias float64) lights.Color {
	var bestR, bestG, bestB uint8
	bestSat := -1.0
	for y := bounds.Min.Y; y < bounds.Max.Y; y += sampleStep {
		for x := bounds.Min.X; x < bounds.Max.X; x += sampleStep {
			c := img.At(x, y)
			r32, g32, b32, _ := c.RGBA()
			r, g, b := uint8(r32>>8), uint8(g32>>8), uint8(b32>>8)
			if shouldSkipWhiteBias(r, g, b, whiteBias) {
				continue
			}
			if sat := saturation(r, g, b); sat > bestSat {
				bestSat = sat
				bestR, bestG, bestB = r, g, b
			}
		}
	}
	if bestSat < 0 {
		return fallbackColor
	}
	h, s, bv := rgbToHSB(float64(bestR)/255, float64(bestG)/255, float64(bestB)/255)
	return lights.Color{H: h, S: s, B: bv}
}

// gridDims returns (cols, rows) for a grid that holds at least n cells,
// arranged as squarely as possible.
func gridDims(n int) (cols, rows int) {
	switch {
	case n <= 1:
		return 1, 1
	case n <= 2:
		return 2, 1
	case n <= 4:
		return 2, 2
	case n <= 6:
		return 3, 2
	case n <= 9:
		return 3, 3
	default:
		return 4, (n + 3) / 4
	}
}

// subImage wraps an image with a restricted bounds rectangle.
type subImage struct {
	image.Image
	bounds image.Rectangle
}

func (s subImage) Bounds() image.Rectangle      { return s.bounds }
func (s subImage) At(x, y int) color.Color       { return s.Image.At(x, y) }
