package extract

import (
	"image"
	"sort"
	"sync"

	"lightsync/internal/lights"
	"lightsync/internal/store"
)

// PaletteExtractor extracts the N most dominant colours from the entire frame.
//
// Stability pipeline (three layers):
//
//  1. Histogram accumulation – pixel weights are summed into a rolling
//     histogram with exponential decay.
//       - PaletteStability 0.0–1.0: histDecay = stability × 0.95
//       - PaletteStability 1.0–2.0: histDecay = 0.95 + (stability-1.0)×0.04
//     (up to 0.99 for ultra-stable palette hold)
//     A colour family only becomes dominant once it has consistently appeared
//     across several frames; single-frame noise cannot hijack a slot.
//
//  2a. Canonical slot order (brightest / saturated / vivid only) — after
//     histogram selection, colours are sorted so slot 0 is the strongest by
//     that metric (saturated-first by S; vivid by S×B; brightest by B×S so
//     near-greys do not rank above pale chromatic highlights). That keeps multi-
//     light semantics stable and matches how golden baselines are authored.
//
//  2b. Temporal blend — dominant & diverse still match each previous slot to
//     the nearest unused fresh colour (reduces visible slot swaps). Brightest,
//     saturated, and vivid instead lerp slot i toward fresh[i] after canonical
//     sort so rank order does not drift from greedy reassignment.
//
//  3. Light EMA blend — each slot is lerped 50 % toward the matched / ranked
//     fresh colour, smoothing fine residual jitter without adding perceptible lag.
type PaletteExtractor struct {
	mu         sync.Mutex
	accumHist  map[[3]uint8]float64
	prevColors []lights.Color
}

func (p *PaletteExtractor) Extract(img image.Image, cfg store.ScreenSyncConfig) []lights.Color {
	n := len(cfg.DeviceIDs)
	if n == 0 {
		n = 1
	}

	stability := cfg.PaletteStability
	if stability < 0 {
		stability = 0
	}
	if stability > 2 {
		stability = 2
	}
	histDecay := stability * 0.95
	if stability > 1 {
		// Extended "ultra stable" range; keep adding memory without exceeding 1.0 decay.
		histDecay = 0.95 + (stability-1.0)*0.04
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// --- Layer 1: histogram accumulation ---
	freshHist := buildPaletteHistogram(img, cfg)

	if p.accumHist == nil || histDecay == 0 {
		p.accumHist = freshHist
	} else {
		// Decay existing buckets.
		for bucket := range p.accumHist {
			p.accumHist[bucket] *= histDecay
			if p.accumHist[bucket] < 0.001 {
				delete(p.accumHist, bucket)
			}
		}
		// Add fresh frame's contribution.
		for bucket, count := range freshHist {
			p.accumHist[bucket] += count
		}
	}

	// Extract N diverse colours from the stable accumulated histogram.
	fresh := selectFromHistogram(p.accumHist, n, cfg.SubMethod)
	canonicalPaletteSlotOrder(fresh, cfg)

	// --- Layer 2 & 3: ordering match + EMA blend ---
	blendAlpha := cfg.PaletteSlotBlendAlpha()

	if len(p.prevColors) != n {
		p.prevColors = make([]lights.Color, n)
		copy(p.prevColors, fresh)
		return fresh
	}

	blended := make([]lights.Color, n)
	switch cfg.SubMethod {
	case store.ExtractionMethodBrightest, store.ExtractionMethodSaturated, store.ExtractionMethodVivid:
		for i := 0; i < n; i++ {
			blended[i] = lerpHSB(p.prevColors[i], fresh[i], blendAlpha)
		}
	default:
		used := make([]bool, n)
		for prevIdx, prev := range p.prevColors {
			bestDist := -1.0
			bestFreshIdx := 0
			for freshIdx, f := range fresh {
				if used[freshIdx] {
					continue
				}
				d := hsbColorDist(f, prev)
				if bestDist < 0 || d < bestDist {
					bestDist = d
					bestFreshIdx = freshIdx
				}
			}
			used[bestFreshIdx] = true
			blended[prevIdx] = lerpHSB(prev, fresh[bestFreshIdx], blendAlpha)
		}
	}

	copy(p.prevColors, blended)
	return blended
}

// canonicalPaletteSlotOrder sorts palette slots so each index has a stable
// meaning for methods where rank is part of the product contract. Dominant and
// diverse keep discovery order (histogram weight chain / farthest-point walk).
func canonicalPaletteSlotOrder(cols []lights.Color, cfg store.ScreenSyncConfig) {
	if len(cols) <= 1 {
		return
	}
	switch cfg.SubMethod {
	case store.ExtractionMethodBrightest:
		// Rank by chroma-weighted brightness so slot 0..N-1 are "brightest
		// colourful" swatches; raw B alone promotes clouds over pale blues.
		minSat := cfg.BrightestRankMinSaturation()
		brightRank := func(c lights.Color) float64 {
			s := c.S
			if s < minSat {
				s = minSat
			}
			return c.B * s
		}
		sort.SliceStable(cols, func(i, j int) bool {
			ri, rj := brightRank(cols[i]), brightRank(cols[j])
			if ri != rj {
				return ri > rj
			}
			a, b := cols[i], cols[j]
			if a.B != b.B {
				return a.B > b.B
			}
			if a.S != b.S {
				return a.S > b.S
			}
			return a.H < b.H
		})
	case store.ExtractionMethodSaturated:
		sort.SliceStable(cols, func(i, j int) bool {
			a, b := cols[i], cols[j]
			if a.S != b.S {
				return a.S > b.S
			}
			if a.B != b.B {
				return a.B > b.B
			}
			return a.H < b.H
		})
	case store.ExtractionMethodVivid:
		sort.SliceStable(cols, func(i, j int) bool {
			a, b := cols[i], cols[j]
			va := a.S * a.B
			vb := b.S * b.B
			if va != vb {
				return va > vb
			}
			if a.S != b.S {
				return a.S > b.S
			}
			return a.H < b.H
		})
	default:
		// dominant, diverse: order from selectFromHistogram already meaningful.
	}
}

// buildPaletteHistogram samples the image and returns a weighted pixel count
// per quantised RGB bucket. Weighting depends on the extraction method:
//   - vivid:     sat × brightness  (perceptual chroma — light pinks beat dark greens)
//   - saturated: saturation weight
//   - brightest: luminance × (saturation + chromaBias) — chromaBias from
//     cfg.ColorTuning (default 0.12) so bright greys do not out-vote saturated sky
//   - dominant / diverse / other: uniform weight (1.0)
func buildPaletteHistogram(img image.Image, cfg store.ScreenSyncConfig) map[[3]uint8]float64 {
	method := cfg.SubMethod
	bounds := img.Bounds()
	hist := make(map[[3]uint8]float64, 256)

	for y := bounds.Min.Y; y < bounds.Max.Y; y += sampleStep {
		for x := bounds.Min.X; x < bounds.Max.X; x += sampleStep {
			c := img.At(x, y)
			r32, g32, b32, _ := c.RGBA()
			r, g, b := uint8(r32>>8), uint8(g32>>8), uint8(b32>>8)
			if shouldSkipWhiteBias(r, g, b, cfg.WhiteBias) {
				continue
			}
			qr := r / 32 * 32
			qg := g / 32 * 32
			qb := b / 32 * 32

			var w float64
			switch method {
			case store.ExtractionMethodVivid:
				sat := saturation(r, g, b)
				maxC := float64(r)
				if float64(g) > maxC {
					maxC = float64(g)
				}
				if float64(b) > maxC {
					maxC = float64(b)
				}
				bright := maxC / 255
				w = sat*bright + 0.02
			case store.ExtractionMethodSaturated:
				w = saturation(r, g, b) + 0.01
			case store.ExtractionMethodBrightest:
				sat := saturation(r, g, b)
				bias := cfg.BrightestPaletteChromaBias()
				w = (luminance(r, g, b) + 0.01) * (sat + bias)
			default: // dominant, diverse
				w = 1.0
			}

			hist[[3]uint8{qr, qg, qb}] += w
		}
	}
	return hist
}

// weightedBucket is a quantised RGB bucket with its accumulated weight.
type weightedBucket struct {
	color [3]uint8
	count float64
}

// selectFromHistogram picks n diverse colours from the accumulated histogram.
// For ExtractionMethodDiverse it uses greedy farthest-point selection;
// for all other methods it uses dominant-family selection.
func selectFromHistogram(hist map[[3]uint8]float64, n int, method store.ExtractionMethod) []lights.Color {
	all := make([]weightedBucket, 0, len(hist))
	total := 0.0
	for c, cnt := range hist {
		all = append(all, weightedBucket{c, cnt})
		total += cnt
	}
	// Stable sort with RGB tiebreaker so equal-count buckets always appear in
	// the same order (map iteration is random in Go).
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].count != all[j].count {
			return all[i].count > all[j].count
		}
		ci, cj := all[i].color, all[j].color
		if ci[0] != cj[0] {
			return ci[0] < cj[0]
		}
		if ci[1] != cj[1] {
			return ci[1] < cj[1]
		}
		return ci[2] < cj[2]
	})

	if len(all) == 0 {
		result := make([]lights.Color, n)
		for i := range result {
			result[i] = fallbackColor
		}
		return result
	}

	// Near-monochrome guard:
	// When one bucket dominates almost the entire weighted histogram, treat
	// the frame as effectively single-colour and duplicate that colour across
	// palette slots. This prevents tiny transient contamination (first-paint
	// artefacts, edge pixels) from being surfaced as bogus extra palette
	// colours in solid/near-solid content.
	const monochromeDominanceRatio = 0.92
	if total > 0 && all[0].count/total >= monochromeDominanceRatio {
		r := float64(all[0].color[0]) / 255
		g := float64(all[0].color[1]) / 255
		b := float64(all[0].color[2]) / 255
		h, s, bv := rgbToHSB(r, g, b)
		c := lights.Color{H: h, S: s, B: bv}
		result := make([]lights.Color, n)
		for i := range result {
			result[i] = c
		}
		return result
	}

	if method == store.ExtractionMethodDiverse {
		return diverseFromSorted(all, n)
	}
	return dominantFromSorted(all, n)
}

// dominantFromSorted greedily selects n colours that are spaced apart in RGB
// space, ranked by accumulated weight (most frequent/vivid families first).
func dominantFromSorted(all []weightedBucket, n int) []lights.Color {
	const diversityThresholdSq = 5000.0
	selected := make([]lights.Color, 0, n)
	selectedRaw := make([][3]uint8, 0, n)

	for _, item := range all {
		if len(selected) >= n {
			break
		}
		tooClose := false
		for _, prev := range selectedRaw {
			if colorDist(item.color, prev) < diversityThresholdSq {
				tooClose = true
				break
			}
		}
		if !tooClose {
			r := float64(item.color[0]) / 255
			g := float64(item.color[1]) / 255
			b := float64(item.color[2]) / 255
			h, s, bv := rgbToHSB(r, g, b)
			selected = append(selected, lights.Color{H: h, S: s, B: bv})
			selectedRaw = append(selectedRaw, item.color)
		}
	}
	for len(selected) < n {
		if len(selected) > 0 {
			selected = append(selected, selected[0])
		} else {
			selected = append(selected, fallbackColor)
		}
	}
	replaceBlackCells(selected, 0.05)
	return selected
}

// diverseFromSorted picks n colours that maximise pairwise perceptual distance
// using greedy farthest-point selection seeded by the highest-weight bucket.
func diverseFromSorted(all []weightedBucket, n int) []lights.Color {
	selected := [][3]uint8{all[0].color}
	for len(selected) < n && len(selected) < len(all) {
		var farthest [3]uint8
		farthestDist := -1.0
		for _, candidate := range all {
			minDist := -1.0
			for _, s := range selected {
				d := colorDist(candidate.color, s)
				if minDist < 0 || d < minDist {
					minDist = d
				}
			}
			if minDist > farthestDist {
				farthestDist = minDist
				farthest = candidate.color
			}
		}
		selected = append(selected, farthest)
	}
	result := make([]lights.Color, len(selected))
	for i, raw := range selected {
		r := float64(raw[0]) / 255
		g := float64(raw[1]) / 255
		b := float64(raw[2]) / 255
		h, s, bv := rgbToHSB(r, g, b)
		result[i] = lights.Color{H: h, S: s, B: bv}
	}
	for len(result) < n {
		result = append(result, result[0])
	}
	replaceBlackCells(result, 0.05)
	return result
}
