package process

import (
	"math"
	"sync"

	"lightsync/internal/lights"
)

const (
	// Ring buffer length for adaptive thresholds (~1 s at 30 fps).
	ringSize = 30

	// Minimum frames between two reported cuts.
	debounceCooldown = 5
)

// CutReasons records which condition(s) triggered a scene-cut detection.
type CutReasons struct {
	Brightness bool // aggregate brightness jump
	Hue        bool // aggregate hue/saturation jump
}

// SceneChangeDetector identifies scene cuts by comparing consecutive frames'
// extracted colors. This adds zero overhead because the colors are already
// computed by the extraction pipeline.
type SceneChangeDetector struct {
	mu         sync.Mutex
	prevColors []lights.Color
	init       bool

	// Rolling statistics for adaptive thresholds.
	colorRing      ringBuf
	brightnessRing ringBuf

	framesSinceCut int
}

// NewSceneChangeDetector creates a detector ready to use.
func NewSceneChangeDetector() *SceneChangeDetector {
	return &SceneChangeDetector{}
}

// Reset clears all internal state so the next frame is treated as the first.
func (d *SceneChangeDetector) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.prevColors = nil
	d.init = false
	d.colorRing = ringBuf{}
	d.brightnessRing = ringBuf{}
	d.framesSinceCut = 0
}

// Check compares colors with the previous frame's colors and reports whether a
// scene cut was detected. sensitivity (0–1) controls how easily a cut is
// triggered: 0 = very conservative, 1 = very sensitive.
func (d *SceneChangeDetector) Check(colors []lights.Color, sensitivity float64) (bool, CutReasons) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.init || len(d.prevColors) == 0 || len(colors) == 0 {
		d.prevColors = cloneColorsForDetector(colors)
		d.init = true
		d.framesSinceCut = debounceCooldown
		return false, CutReasons{}
	}

	colorDist, brightDist := aggregateDistance(d.prevColors, colors)
	d.prevColors = cloneColorsForDetector(colors)

	d.colorRing.push(colorDist)
	d.brightnessRing.push(brightDist)
	d.framesSinceCut++

	if d.framesSinceCut < debounceCooldown {
		return false, CutReasons{}
	}

	s := clamp01(sensitivity)

	// Adaptive k: sensitivity 0 → k=8.0 (very hard to trigger),
	// sensitivity 1 → k=1.0.
	k := 8.0 - 7.0*s

	reasons := CutReasons{}

	if d.colorRing.count >= 5 {
		mean, std := d.colorRing.stats()
		if colorDist > mean+k*std && colorDist > 0.01 {
			reasons.Hue = true
		}
	}

	if d.brightnessRing.count >= 5 {
		mean, std := d.brightnessRing.stats()
		if brightDist > mean+k*std && brightDist > 0.01 {
			reasons.Brightness = true
		}
	}

	// Absolute thresholds for dramatic cuts, scaled by sensitivity.
	// Low sensitivity now requires much larger frame-to-frame jumps.
	absColorThresh := 0.85 - 0.65*s  // 0.85 → 0.20
	absBrightThresh := 0.70 - 0.55*s // 0.70 → 0.15
	if colorDist > absColorThresh {
		reasons.Hue = true
	}
	if brightDist > absBrightThresh {
		reasons.Brightness = true
	}

	detected := reasons.Brightness || reasons.Hue
	if detected {
		d.framesSinceCut = 0
	}
	return detected, reasons
}

// aggregateDistance computes the average perceptual color distance (hue +
// saturation) and the average brightness distance across all paired slots.
func aggregateDistance(prev, curr []lights.Color) (colorDist, brightDist float64) {
	n := len(prev)
	if len(curr) < n {
		n = len(curr)
	}
	if n == 0 {
		return 0, 0
	}

	var cSum, bSum float64
	for i := 0; i < n; i++ {
		p, c := prev[i], curr[i]

		// Hue + saturation distance (normalised to ~0–1).
		dh := c.H - p.H
		for dh > 180 {
			dh -= 360
		}
		for dh < -180 {
			dh += 360
		}
		hNorm := math.Abs(dh) / 180.0
		ds := math.Abs(c.S - p.S)
		cSum += math.Sqrt(0.65*hNorm*hNorm + 0.35*ds*ds)

		// Brightness distance.
		bSum += math.Abs(c.B - p.B)
	}

	return cSum / float64(n), bSum / float64(n)
}

func cloneColorsForDetector(c []lights.Color) []lights.Color {
	out := make([]lights.Color, len(c))
	copy(out, c)
	return out
}

// ---------- ring buffer for rolling statistics ----------

type ringBuf struct {
	data  [ringSize]float64
	pos   int
	count int
}

func (r *ringBuf) push(v float64) {
	r.data[r.pos] = v
	r.pos = (r.pos + 1) % ringSize
	if r.count < ringSize {
		r.count++
	}
}

func (r *ringBuf) stats() (mean, stddev float64) {
	if r.count == 0 {
		return 0, 0
	}
	n := float64(r.count)
	var sum float64
	for i := 0; i < r.count; i++ {
		sum += r.data[i]
	}
	mean = sum / n
	var variance float64
	for i := 0; i < r.count; i++ {
		d := r.data[i] - mean
		variance += d * d
	}
	variance /= n
	stddev = math.Sqrt(variance)
	return
}

// ---------- helpers ----------

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
