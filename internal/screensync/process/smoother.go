package process

import (
	"math"
	"sync"
	"time"

	"lightsync/internal/lights"
)

// TemporalSmoother applies smoothing to a stream of extracted color frames.
//
// Color (H+S) uses an adaptive EMA per slot: small deltas are heavily damped
// while large deltas pass through responsively.
//
// Brightness operates at the frame level: the mean brightness across all zones
// is averaged over a sliding time window and rate-limited to a maximum change
// per second. Each light's output brightness is the smoothed frame average plus
// its per-light deviation (clamped to ±maxBrightnessDeviation). This prevents
// individual zone jitter from causing visible flicker while preserving spatial
// variation between lights.
//
// Scene cuts snap all values instantly.
type TemporalSmoother struct {
	mu    sync.Mutex
	slots []smoothedSlot
	now   func() time.Time

	// Frame-level brightness state.
	bSmoothed float64
	bRing     []bSample
	bTime     time.Time
	bInit     bool
}

type smoothedSlot struct {
	H, S   float64
	bOut   float64 // previous rate-limited output brightness
	active bool
}

type bSample struct {
	t time.Time
	b float64
}

// NewTemporalSmoother returns a smoother with no state. The first call to
// Smooth initialises slots from the incoming colors.
func NewTemporalSmoother() *TemporalSmoother {
	return &TemporalSmoother{now: time.Now}
}

// Reset discards all state so the next frame is treated as the first.
func (t *TemporalSmoother) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.slots = nil
	t.bInit = false
	t.bRing = nil
}

// Smooth returns a temporally-smoothed version of colors. When isCut is true
// all slots snap to the new values immediately. colorSmoothing supports an
// extended range [0, 2] where values above 1 provide ultra-smooth transitions.
// brightnessSmoothing remains in [0, 1].
func (t *TemporalSmoother) Smooth(
	colors []lights.Color,
	isCut bool,
	colorSmoothing float64,
	brightnessSmoothing float64,
	brightnessMaxDeviation float64,
	brightnessFloor float64,
	brightnessCeiling float64,
) []lights.Color {
	t.mu.Lock()
	defer t.mu.Unlock()

	n := len(colors)
	if n == 0 {
		return colors
	}

	now := t.now()

	// Compute frame average brightness.
	var sumB float64
	for _, c := range colors {
		sumB += c.B
	}
	frameAvgB := sumB / float64(n)

	// First frame or slot count changed: initialise from input.
	if len(t.slots) != n || !t.bInit {
		t.slots = make([]smoothedSlot, n)
		for i, c := range colors {
			t.slots[i] = smoothedSlot{H: c.H, S: c.S, bOut: c.B, active: true}
		}
		t.bSmoothed = frameAvgB
		t.bRing = []bSample{{t: now, b: frameAvgB}}
		t.bTime = now
		t.bInit = true
		return cloneColors(colors)
	}

	// Scene cut: snap color immediately, but keep brightness on the rate-limited
	// path. This avoids large brightness jumps from false-positive cut detection.
	if isCut {
		for i, c := range colors {
			t.slots[i].H = c.H
			t.slots[i].S = c.S
			t.slots[i].active = true
		}
	}

	if colorSmoothing < 0 {
		colorSmoothing = 0
	}
	if colorSmoothing > 2 {
		colorSmoothing = 2
	}
	brightnessSmoothing = clamp01(brightnessSmoothing)
	if brightnessMaxDeviation < 0.01 {
		brightnessMaxDeviation = 0.01
	}
	if brightnessMaxDeviation > 1.0 {
		brightnessMaxDeviation = 1.0
	}
	brightnessFloor = clamp01(brightnessFloor)
	brightnessCeiling = clamp01(brightnessCeiling)
	if brightnessCeiling-brightnessFloor < 0.05 {
		brightnessCeiling = brightnessFloor + 0.05
	}
	bRange := brightnessCeiling - brightnessFloor

	// ── Compute dt and maxStep (used by both frame-level and per-light limiters).
	dt := now.Sub(t.bTime).Seconds()
	if dt <= 0 {
		dt = 1.0 / 60.0
	}
	maxStep := brightnessMaxRate(brightnessSmoothing) * dt

	// ── Frame-level brightness: windowed average + slew-rate limiter ─────
	if brightnessSmoothing <= 0 {
		t.bSmoothed = frameAvgB
		t.bRing = t.bRing[:0]
		t.bTime = now
	} else {
		t.bRing = append(t.bRing, bSample{t: now, b: frameAvgB})

		windowDur := brightnessWindow(brightnessSmoothing)
		cutoff := now.Add(-windowDur)
		trimIdx := 0
		for trimIdx < len(t.bRing) && t.bRing[trimIdx].t.Before(cutoff) {
			trimIdx++
		}
		if trimIdx > 0 {
			copy(t.bRing, t.bRing[trimIdx:])
			t.bRing = t.bRing[:len(t.bRing)-trimIdx]
		}

		var sum float64
		for _, sample := range t.bRing {
			sum += sample.b
		}
		windowedAvg := sum / float64(len(t.bRing))

		diff := windowedAvg - t.bSmoothed
		if diff > maxStep {
			t.bSmoothed += maxStep
		} else if diff < -maxStep {
			t.bSmoothed -= maxStep
		} else {
			t.bSmoothed = windowedAvg
		}
		t.bTime = now
	}

	// ── Per-slot: color EMA + brightness deviation ──────────────────────
	result := make([]lights.Color, n)
	for i, c := range colors {
		s := &t.slots[i]

		// -- Color (H + S) --
		colorDelta := hsbColorDelta(s.H, s.S, c.H, c.S)
		cAlpha := adaptiveAlpha(colorSmoothing, colorDelta)
		s.H, s.S = lerpHS(s.H, s.S, c.H, c.S, cAlpha)

		// -- Brightness: compute target, then rate-limit per-light output --
		var outB float64
		if brightnessSmoothing <= 0 {
			outB = c.B
			s.bOut = c.B
		} else {
			dev := c.B - frameAvgB
			if dev > brightnessMaxDeviation {
				dev = brightnessMaxDeviation
			} else if dev < -brightnessMaxDeviation {
				dev = -brightnessMaxDeviation
			}
			target := t.bSmoothed + dev
			if target < 0 {
				target = 0
			}
			if target > 1 {
				target = 1
			}

			// Per-light rate limit: the absolute guarantee that no light's
			// brightness can change faster than maxStep per frame.
			diff := target - s.bOut
			if diff > maxStep {
				s.bOut += maxStep
			} else if diff < -maxStep {
				s.bOut -= maxStep
			} else {
				s.bOut = target
			}
			outB = s.bOut
		}

		// -- Brightness range compression: remap [0,1] → [floor,ceiling] --
		outB = brightnessFloor + outB*bRange

		result[i] = lights.Color{H: s.H, S: s.S, B: outB}
	}
	return result
}

// brightnessWindow returns the duration of the sliding window used to average
// incoming frame brightness readings before the rate limiter sees them.
//
//   - smoothing=0   → 100ms  (nearly raw input)
//   - smoothing=0.5 → ~1.55s
//   - smoothing=1.0 → 3s     (heavy averaging)
func brightnessWindow(smoothing float64) time.Duration {
	ms := 100.0 + smoothing*2900.0
	return time.Duration(ms) * time.Millisecond
}

// brightnessMaxRate returns the maximum brightness change per second allowed
// by the slew-rate limiter. The slider maps exponentially so that mid-range
// values are already perceptibly smooth.
//
//   - smoothing=0    → +Inf   (no limit, pass-through)
//   - smoothing=0.25 → ~3.2/s
//   - smoothing=0.50 → ~1.0/s (full range in ~1s)
//   - smoothing=0.75 → ~0.32/s
//   - smoothing=1.0  → ~0.10/s (full range in ~10s)
func brightnessMaxRate(smoothing float64) float64 {
	if smoothing <= 0 {
		return math.Inf(1)
	}
	return 10.0 * math.Pow(0.01, smoothing)
}

// adaptiveAlpha converts a user smoothing slider (0–1) and the current change
// magnitude into an EMA blend factor (alpha). The soft-knee smoothstep ensures
// small deltas get heavy smoothing while large deltas pass through.
//
//   - smoothing=0 → always alpha=1 (pass-through, no smoothing)
//   - smoothing=0.5 → baseAlpha≈0.14 (visible smoothing, ~500ms convergence at 20fps)
//   - smoothing=1.0 → baseAlpha≈0.02 (very heavy, ~1.5s convergence at 20fps)
//   - smoothing=2.0 → baseAlpha≈0.0003 (ultra smooth, very gradual)
//
// The exponential curve ensures that even moderate slider values produce
// perceptible damping; a linear mapping made mid-range values too fast.
func adaptiveAlpha(smoothing, delta float64) float64 {
	if smoothing <= 0 {
		return 1.0
	}
	if smoothing > 2 {
		smoothing = 2
	}

	baseAlpha := math.Exp(-smoothing * 4.0)

	knee := 0.20 + 0.30*smoothing // 0.20 – 0.50

	factor := smoothstep(delta, knee*0.3, knee*1.5)
	return baseAlpha + (1.0-baseAlpha)*factor
}

// smoothstep performs Hermite interpolation between 0 and 1.
func smoothstep(x, edge0, edge1 float64) float64 {
	t := (x - edge0) / (edge1 - edge0)
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return t * t * (3 - 2*t)
}

// hsbColorDelta returns a perceptual distance between two (H, S) pairs,
// normalised roughly to [0, 1].
func hsbColorDelta(h1, s1, h2, s2 float64) float64 {
	dh := h2 - h1
	for dh > 180 {
		dh -= 360
	}
	for dh < -180 {
		dh += 360
	}
	hNorm := math.Abs(dh) / 180.0
	ds := math.Abs(s2 - s1)
	return math.Sqrt(0.65*hNorm*hNorm + 0.35*ds*ds)
}

// lerpHS interpolates hue (shortest-path) and saturation by t.
func lerpHS(h1, s1, h2, s2, t float64) (float64, float64) {
	dh := h2 - h1
	for dh > 180 {
		dh -= 360
	}
	for dh < -180 {
		dh += 360
	}
	h := h1 + dh*t
	for h < 0 {
		h += 360
	}
	for h >= 360 {
		h -= 360
	}
	s := s1 + (s2-s1)*t
	return h, s
}

func cloneColors(c []lights.Color) []lights.Color {
	out := make([]lights.Color, len(c))
	copy(out, c)
	return out
}
