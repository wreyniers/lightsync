package extract

import (
	"image"
	"sort"

	"lightsync/internal/lights"
	"lightsync/internal/store"
)

// vividDominantColor is the same family-grouping algorithm as dominantColor, but
// each pixel's count contribution is weighted by its saturation squared.
// Highly colourful pixels dominate over desaturated backgrounds while still
// respecting area — a small yellow stamen cannot beat a large pink petal field.
func vividDominantColor(img image.Image, whiteBias float64) lights.Color {
	bounds := img.Bounds()
	counts := make(map[[3]uint8]float64, 128)

	for y := bounds.Min.Y; y < bounds.Max.Y; y += sampleStep {
		for x := bounds.Min.X; x < bounds.Max.X; x += sampleStep {
			c := img.At(x, y)
			r32, g32, b32, _ := c.RGBA()
			r, g, b := uint8(r32>>8), uint8(g32>>8), uint8(b32>>8)

			if shouldSkipWhiteBias(r, g, b, whiteBias) {
				continue
			}

			qr := r / 32 * 32
			qg := g / 32 * 32
			qb := b / 32 * 32

			sat := saturation(r, g, b)
			// Weight by sat × brightness (HSB chroma proxy).
			// A dark saturated green (high S, low B) loses to a light vivid pink
			// (moderate S, high B), matching human perception of colour prominence.
			// Floor of 0.02 ensures dark/grey zones still produce a result.
			maxC := float64(r)
			if float64(g) > maxC {
				maxC = float64(g)
			}
			if float64(b) > maxC {
				maxC = float64(b)
			}
			bright := maxC / 255
			weight := sat*bright + 0.02
			counts[[3]uint8{qr, qg, qb}] += weight
		}
	}

	if len(counts) == 0 {
		return fallbackColor
	}

	type cc struct {
		color [3]uint8
		count float64
	}
	sorted := make([]cc, 0, len(counts))
	for c, n := range counts {
		sorted = append(sorted, cc{c, n})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].count > sorted[j].count })

	const familyThresholdSq = 3600.0
	type family struct {
		representative [3]uint8
		totalCount     float64
		bestColor      [3]uint8
		bestCount      float64
	}
	families := make([]family, 0, 16)

	for _, item := range sorted {
		placed := false
		for i := range families {
			if colorDist(item.color, families[i].representative) < familyThresholdSq {
				families[i].totalCount += item.count
				if item.count > families[i].bestCount {
					families[i].bestCount = item.count
					families[i].bestColor = item.color
				}
				placed = true
				break
			}
		}
		if !placed {
			families = append(families, family{
				representative: item.color,
				totalCount:     item.count,
				bestColor:      item.color,
				bestCount:      item.count,
			})
		}
	}

	best := families[0]
	for _, f := range families[1:] {
		if f.totalCount > best.totalCount {
			best = f
		}
	}

	r := float64(best.bestColor[0]) / 255
	g := float64(best.bestColor[1]) / 255
	b := float64(best.bestColor[2]) / 255
	h, s, bv := rgbToHSB(r, g, b)
	return lights.Color{H: h, S: s, B: bv}
}

// DominantExtractor extracts the most visually dominant color using weighted
// frequency analysis with perceptual family grouping.
//
// Algorithm:
//  1. Sample pixels at sampleStep intervals.
//  2. Quantize each pixel to a 32-step RGB bucket.
//  3. Count occurrences per bucket.
//  4. Group nearby buckets into perceptual "families" (Euclidean distance threshold).
//  5. Return the representative color of the family with the highest total pixel count.
//
// This ensures a dark-green forest with a small bright-orange element returns
// green (dominant by area) rather than orange (dominant by brightness).
type DominantExtractor struct{}

func (d *DominantExtractor) Extract(img image.Image, cfg store.ScreenSyncConfig) []lights.Color {
	color := dominantColor(img, cfg.WhiteBias)
	return []lights.Color{color}
}

func dominantColor(img image.Image, whiteBias float64) lights.Color {
	bounds := img.Bounds()

	// Count quantized colors.
	counts := make(map[[3]uint8]int, 128)
	for y := bounds.Min.Y; y < bounds.Max.Y; y += sampleStep {
		for x := bounds.Min.X; x < bounds.Max.X; x += sampleStep {
			c := img.At(x, y)
			r32, g32, b32, _ := c.RGBA()
			r, g, b := uint8(r32>>8), uint8(g32>>8), uint8(b32>>8)

			if shouldSkipWhiteBias(r, g, b, whiteBias) {
				continue
			}

			// Quantize to 32-value steps.
			qr := r / 32 * 32
			qg := g / 32 * 32
			qb := b / 32 * 32
			counts[[3]uint8{qr, qg, qb}]++
		}
	}

	if len(counts) == 0 {
		return fallbackColor
	}

	// Sort by count descending.
	type cc struct {
		color [3]uint8
		count int
	}
	sorted := make([]cc, 0, len(counts))
	for c, n := range counts {
		sorted = append(sorted, cc{c, n})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].count > sorted[j].count })

	// Perceptual family grouping.
	// Two quantized buckets belong to the same family when their squared RGB
	// distance is below familyThresholdSq (≈ 60² in Euclidean terms = 3600).
	const familyThresholdSq = 3600.0

	type family struct {
		representative [3]uint8
		totalCount     int
		bestColor      [3]uint8
		bestCount      int
	}
	families := make([]family, 0, 16)

	for _, item := range sorted {
		placed := false
		for i := range families {
			if colorDist(item.color, families[i].representative) < familyThresholdSq {
				families[i].totalCount += item.count
				if item.count > families[i].bestCount {
					families[i].bestCount = item.count
					families[i].bestColor = item.color
				}
				placed = true
				break
			}
		}
		if !placed {
			families = append(families, family{
				representative: item.color,
				totalCount:     item.count,
				bestColor:      item.color,
				bestCount:      item.count,
			})
		}
	}

	// Pick the family with the highest total pixel area.
	best := families[0]
	for _, f := range families[1:] {
		if f.totalCount > best.totalCount {
			best = f
		}
	}

	r := float64(best.bestColor[0]) / 255
	g := float64(best.bestColor[1]) / 255
	b := float64(best.bestColor[2]) / 255
	h, s, bv := rgbToHSB(r, g, b)
	return lights.Color{H: h, S: s, B: bv}
}
