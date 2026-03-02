package process

import (
	"math"
	"testing"
	"time"

	"lightsync/internal/lights"
)

func red() lights.Color   { return lights.Color{H: 0, S: 1, B: 1} }
func blue() lights.Color  { return lights.Color{H: 240, S: 1, B: 1} }
func green() lights.Color { return lights.Color{H: 120, S: 1, B: 1} }

const defaultDev = 0.15

// newTestSmoother returns a smoother whose clock is controlled by the caller.
// Advance the returned *time.Time to simulate elapsed time between frames.
func newTestSmoother() (*TemporalSmoother, *time.Time) {
	t := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	s := &TemporalSmoother{now: func() time.Time { return t }}
	return s, &t
}

// ─── First frame / scene cut / reset ────────────────────────────────────────

func TestTemporalSmoother_FirstFrame_PassThrough(t *testing.T) {
	s := NewTemporalSmoother()
	input := []lights.Color{red(), blue()}
	out := s.Smooth(input, false, 0.8, 0.8, defaultDev, 0.0, 1.0)

	for i, c := range out {
		if c.H != input[i].H || c.S != input[i].S || c.B != input[i].B {
			t.Errorf("slot %d: first frame should pass through unchanged, got H=%.1f S=%.2f B=%.2f", i, c.H, c.S, c.B)
		}
	}
}

func TestTemporalSmoother_SceneCut_SnapsImmediately(t *testing.T) {
	s, clock := newTestSmoother()

	s.Smooth([]lights.Color{red()}, false, 1.0, 1.0, defaultDev, 0.0, 1.0)
	*clock = clock.Add(50 * time.Millisecond)
	s.Smooth([]lights.Color{red()}, false, 1.0, 1.0, defaultDev, 0.0, 1.0)
	*clock = clock.Add(50 * time.Millisecond)

	out := s.Smooth([]lights.Color{blue()}, true, 1.0, 1.0, defaultDev, 0.0, 1.0)
	if math.Abs(out[0].H-240) > 0.1 {
		t.Errorf("scene cut should snap to blue (H=240), got H=%.1f", out[0].H)
	}
}

func TestTemporalSmoother_SceneCut_DoesNotBypassBrightnessRateLimit(t *testing.T) {
	s, clock := newTestSmoother()

	// Establish prior state at low brightness.
	s.Smooth([]lights.Color{{H: 0, S: 1, B: 0.20}}, false, 1.0, 1.0, defaultDev, 0.0, 1.0)
	*clock = clock.Add(50 * time.Millisecond)

	// Scene cut with a large brightness jump. Color should snap, brightness should not.
	out := s.Smooth([]lights.Color{{H: 240, S: 1, B: 0.90}}, true, 1.0, 1.0, defaultDev, 0.0, 1.0)
	if math.Abs(out[0].H-240) > 0.1 {
		t.Errorf("scene cut should snap hue to blue (H=240), got H=%.1f", out[0].H)
	}
	// At max smoothing, max rate is 0.10/s. Over 50ms, max step is 0.005.
	if out[0].B > 0.23 {
		t.Errorf("scene cut should not bypass brightness rate limit, got B=%.3f (expected <=0.23)", out[0].B)
	}
}

func TestTemporalSmoother_ZeroSmoothing_PassThrough(t *testing.T) {
	s, clock := newTestSmoother()

	s.Smooth([]lights.Color{red()}, false, 0, 0, defaultDev, 0.0, 1.0)
	*clock = clock.Add(50 * time.Millisecond)

	out := s.Smooth([]lights.Color{green()}, false, 0, 0, defaultDev, 0.0, 1.0)
	if math.Abs(out[0].H-120) > 0.1 {
		t.Errorf("zero smoothing should pass through hue, got H=%.1f", out[0].H)
	}
	if math.Abs(out[0].B-1.0) > 0.01 {
		t.Errorf("zero smoothing should pass through brightness, got B=%.3f", out[0].B)
	}
}

func TestTemporalSmoother_SlotCountChange_Resets(t *testing.T) {
	s := NewTemporalSmoother()

	s.Smooth([]lights.Color{red(), blue()}, false, 0.8, 0.8, defaultDev, 0.0, 1.0)

	out := s.Smooth([]lights.Color{green(), red(), blue()}, false, 0.8, 0.8, defaultDev, 0.0, 1.0)
	if len(out) != 3 {
		t.Errorf("expected 3 output colors, got %d", len(out))
	}
	if math.Abs(out[0].H-120) > 0.1 {
		t.Errorf("after slot count change, first color should be green (H=120), got H=%.1f", out[0].H)
	}
}

func TestTemporalSmoother_Reset(t *testing.T) {
	s, clock := newTestSmoother()
	s.Smooth([]lights.Color{red()}, false, 0.8, 0.8, defaultDev, 0.0, 1.0)
	*clock = clock.Add(50 * time.Millisecond)
	s.Smooth([]lights.Color{red()}, false, 0.8, 0.8, defaultDev, 0.0, 1.0)

	s.Reset()

	*clock = clock.Add(50 * time.Millisecond)
	out := s.Smooth([]lights.Color{blue()}, false, 0.8, 0.8, defaultDev, 0.0, 1.0)
	if math.Abs(out[0].H-240) > 0.1 {
		t.Errorf("after reset, first frame should pass through blue (H=240), got H=%.1f", out[0].H)
	}
}

// ─── Color (H+S) smoothing (EMA, unchanged) ────────────────────────────────

func TestTemporalSmoother_MaxSmoothing_DampsSmallChanges(t *testing.T) {
	s, clock := newTestSmoother()

	base := lights.Color{H: 100, S: 0.8, B: 0.7}
	s.Smooth([]lights.Color{base}, false, 1.0, 1.0, defaultDev, 0.0, 1.0)
	*clock = clock.Add(50 * time.Millisecond)

	shifted := lights.Color{H: 103, S: 0.82, B: 0.72}
	out := s.Smooth([]lights.Color{shifted}, false, 1.0, 1.0, defaultDev, 0.0, 1.0)

	hDelta := math.Abs(out[0].H - base.H)
	if hDelta > 2.0 {
		t.Errorf("max smoothing should damp small hue shift (3°→~0.1°), got delta=%.2f°", hDelta)
	}
}

func TestTemporalSmoother_ColorConvergesOverTime(t *testing.T) {
	s, clock := newTestSmoother()

	start := lights.Color{H: 0, S: 0.5, B: 0.5}
	target := lights.Color{H: 30, S: 0.7, B: 0.5}

	s.Smooth([]lights.Color{start}, false, 0.5, 0.5, defaultDev, 0.0, 1.0)

	var last lights.Color
	for i := 0; i < 200; i++ {
		*clock = clock.Add(50 * time.Millisecond)
		out := s.Smooth([]lights.Color{target}, false, 0.5, 0.5, defaultDev, 0.0, 1.0)
		last = out[0]
	}

	if math.Abs(last.H-target.H) > 1.0 {
		t.Errorf("should converge to target hue (30), got H=%.1f", last.H)
	}
	if math.Abs(last.S-target.S) > 0.05 {
		t.Errorf("should converge to target saturation (0.7), got S=%.3f", last.S)
	}
}

func TestAdaptiveAlpha_ZeroSmoothing(t *testing.T) {
	a := adaptiveAlpha(0, 0.5)
	if a != 1.0 {
		t.Errorf("zero smoothing should yield alpha=1.0, got %f", a)
	}
}

func TestAdaptiveAlpha_MaxSmoothing_SmallDelta(t *testing.T) {
	a := adaptiveAlpha(1.0, 0.001)
	if a > 0.10 {
		t.Errorf("max smoothing + tiny delta should yield very small alpha, got %f", a)
	}
}

func TestAdaptiveAlpha_MaxSmoothing_LargeDelta(t *testing.T) {
	a := adaptiveAlpha(1.0, 1.0)
	if a < 0.90 {
		t.Errorf("max smoothing + large delta should still yield high alpha (responsive), got %f", a)
	}
}

func TestAdaptiveAlpha_MidSmoothing_IsPerceptible(t *testing.T) {
	a := adaptiveAlpha(0.5, 0.01)
	if a > 0.20 {
		t.Errorf("mid-range smoothing + small delta should yield alpha < 0.20 for visible effect, got %f", a)
	}
}

func TestAdaptiveAlpha_MidSmoothing_ModerateDelta(t *testing.T) {
	a := adaptiveAlpha(0.5, 0.05)
	if a > 0.25 {
		t.Errorf("mid-range smoothing + moderate delta should still damp noticeably, got alpha=%f", a)
	}
}

func TestSmoothstep(t *testing.T) {
	if v := smoothstep(-1, 0, 1); v != 0 {
		t.Errorf("smoothstep below edge0 should be 0, got %f", v)
	}
	if v := smoothstep(2, 0, 1); v != 1 {
		t.Errorf("smoothstep above edge1 should be 1, got %f", v)
	}
	if v := smoothstep(0.5, 0, 1); math.Abs(v-0.5) > 0.01 {
		t.Errorf("smoothstep at midpoint should be ~0.5, got %f", v)
	}
}

// ─── Brightness helper functions ────────────────────────────────────────────

func TestBrightnessWindow(t *testing.T) {
	w0 := brightnessWindow(0)
	if w0 != 100*time.Millisecond {
		t.Errorf("smoothing=0 should give 100ms window, got %v", w0)
	}
	w1 := brightnessWindow(1.0)
	if w1 != 3000*time.Millisecond {
		t.Errorf("smoothing=1 should give 3s window, got %v", w1)
	}
	if brightnessWindow(0.5) <= w0 || brightnessWindow(0.5) >= w1 {
		t.Error("mid-range window should be between min and max")
	}
}

func TestBrightnessMaxRate(t *testing.T) {
	if !math.IsInf(brightnessMaxRate(0), 1) {
		t.Errorf("smoothing=0 should give infinite rate, got %f", brightnessMaxRate(0))
	}
	r1 := brightnessMaxRate(1.0)
	if math.Abs(r1-0.10) > 0.01 {
		t.Errorf("smoothing=1 should give ~0.10/s, got %f", r1)
	}
	r50 := brightnessMaxRate(0.5)
	if math.Abs(r50-1.0) > 0.05 {
		t.Errorf("smoothing=0.5 should give ~1.0/s, got %f", r50)
	}
}

// ─── Frame-level brightness smoothing ───────────────────────────────────────

func TestTemporalSmoother_BrightnessRampsAtControlledSpeed(t *testing.T) {
	s, clock := newTestSmoother()

	s.Smooth([]lights.Color{{H: 0, S: 1, B: 0.0}}, false, 0, 0.5, defaultDev, 0.0, 1.0)

	for i := 0; i < 11; i++ {
		*clock = clock.Add(50 * time.Millisecond)
		s.Smooth([]lights.Color{{H: 0, S: 1, B: 1.0}}, false, 0, 0.5, defaultDev, 0.0, 1.0)
	}

	*clock = clock.Add(50 * time.Millisecond)
	out := s.Smooth([]lights.Color{{H: 0, S: 1, B: 1.0}}, false, 0, 0.5, defaultDev, 0.0, 1.0)

	if out[0].B < 0.30 || out[0].B > 0.75 {
		t.Errorf("brightness should ramp at ~1.0/s, got B=%.3f after 0.6s (expected 0.30–0.75)", out[0].B)
	}
}

func TestTemporalSmoother_BrightnessRampsMonotonically(t *testing.T) {
	s, clock := newTestSmoother()

	s.Smooth([]lights.Color{{H: 0, S: 1, B: 0.0}}, false, 0, 0.75, defaultDev, 0.0, 1.0)

	var prev float64
	for i := 0; i < 200; i++ {
		*clock = clock.Add(50 * time.Millisecond)
		out := s.Smooth([]lights.Color{{H: 0, S: 1, B: 1.0}}, false, 0, 0.75, defaultDev, 0.0, 1.0)
		if out[0].B < prev-0.001 {
			t.Fatalf("brightness should ramp monotonically upward, but frame %d went from %.4f to %.4f", i, prev, out[0].B)
		}
		prev = out[0].B
	}

	if prev < 0.95 {
		t.Errorf("brightness should reach target after 10s at smoothing=0.75, got %.3f", prev)
	}
}

func TestTemporalSmoother_BrightnessMaxSmoothing_VerySlowRamp(t *testing.T) {
	s, clock := newTestSmoother()

	s.Smooth([]lights.Color{{H: 0, S: 1, B: 0.0}}, false, 0, 1.0, defaultDev, 0.0, 1.0)
	*clock = clock.Add(50 * time.Millisecond)

	out := s.Smooth([]lights.Color{{H: 0, S: 1, B: 1.0}}, false, 0, 1.0, defaultDev, 0.0, 1.0)

	if out[0].B > 0.02 {
		t.Errorf("max smoothing should ramp very slowly, got B=%.4f after 1 frame (expected <0.02)", out[0].B)
	}
}

func TestTemporalSmoother_BrightnessInterruptsRamp(t *testing.T) {
	s, clock := newTestSmoother()

	s.Smooth([]lights.Color{{H: 0, S: 1, B: 0.3}}, false, 0, 0.5, defaultDev, 0.0, 1.0)

	for i := 0; i < 5; i++ {
		*clock = clock.Add(50 * time.Millisecond)
		s.Smooth([]lights.Color{{H: 0, S: 1, B: 0.8}}, false, 0, 0.5, defaultDev, 0.0, 1.0)
	}

	*clock = clock.Add(50 * time.Millisecond)
	mid := s.Smooth([]lights.Color{{H: 0, S: 1, B: 0.8}}, false, 0, 0.5, defaultDev, 0.0, 1.0)
	midB := mid[0].B

	for i := 0; i < 40; i++ {
		*clock = clock.Add(50 * time.Millisecond)
		s.Smooth([]lights.Color{{H: 0, S: 1, B: 0.35}}, false, 0, 0.5, defaultDev, 0.0, 1.0)
	}

	*clock = clock.Add(50 * time.Millisecond)
	final := s.Smooth([]lights.Color{{H: 0, S: 1, B: 0.35}}, false, 0, 0.5, defaultDev, 0.0, 1.0)

	if final[0].B >= midB {
		t.Errorf("brightness should have reversed direction, got final=%.3f >= mid=%.3f", final[0].B, midB)
	}
}

func TestTemporalSmoother_BrightnessConvergesOverTime(t *testing.T) {
	s, clock := newTestSmoother()

	start := lights.Color{H: 0, S: 0.5, B: 0.2}
	target := lights.Color{H: 0, S: 0.5, B: 0.8}

	s.Smooth([]lights.Color{start}, false, 0, 0.5, defaultDev, 0.0, 1.0)

	var last lights.Color
	for i := 0; i < 200; i++ {
		*clock = clock.Add(50 * time.Millisecond)
		out := s.Smooth([]lights.Color{target}, false, 0, 0.5, defaultDev, 0.0, 1.0)
		last = out[0]
	}

	if math.Abs(last.B-target.B) > 0.05 {
		t.Errorf("should converge to target brightness (0.8), got B=%.3f", last.B)
	}
}

// ─── Multi-light deviation clamping ─────────────────────────────────────────

func TestTemporalSmoother_MultiLight_DeviationClamped(t *testing.T) {
	s, clock := newTestSmoother()

	init := []lights.Color{
		{H: 0, S: 1, B: 0.5},
		{H: 120, S: 1, B: 0.5},
		{H: 240, S: 1, B: 0.5},
	}
	s.Smooth(init, false, 0, 0.5, defaultDev, 0.0, 1.0)

	spread := []lights.Color{
		{H: 0, S: 1, B: 0.1},
		{H: 120, S: 1, B: 0.5},
		{H: 240, S: 1, B: 0.9},
	}
	*clock = clock.Add(50 * time.Millisecond)
	out := s.Smooth(spread, false, 0, 0.5, defaultDev, 0.0, 1.0)

	for i, c := range out {
		diff := math.Abs(c.B - out[1].B)
		if diff > defaultDev+0.02 {
			t.Errorf("light %d brightness %.3f deviates too far from mid light %.3f (max dev=%.2f)",
				i, c.B, out[1].B, defaultDev)
		}
	}
}

func TestTemporalSmoother_MultiLight_TightDeviation(t *testing.T) {
	s, clock := newTestSmoother()

	init := []lights.Color{
		{H: 0, S: 1, B: 0.5},
		{H: 120, S: 1, B: 0.5},
	}
	s.Smooth(init, false, 0, 0.5, 0.01, 0.0, 1.0)

	spread := []lights.Color{
		{H: 0, S: 1, B: 0.2},
		{H: 120, S: 1, B: 0.8},
	}
	*clock = clock.Add(50 * time.Millisecond)
	out := s.Smooth(spread, false, 0, 0.5, 0.01, 0.0, 1.0)

	// At ±1% deviation, both lights should be within 0.02 of each other.
	diff := math.Abs(out[0].B - out[1].B)
	if diff > 0.03 {
		t.Errorf("tight deviation (1%%) should keep lights very close, got diff=%.4f", diff)
	}
}

func TestTemporalSmoother_MultiLight_WideDeviation(t *testing.T) {
	s, clock := newTestSmoother()

	init := []lights.Color{
		{H: 0, S: 1, B: 0.5},
		{H: 120, S: 1, B: 0.5},
	}
	s.Smooth(init, false, 0, 0.5, 1.0, 0.0, 1.0)

	spread := []lights.Color{
		{H: 0, S: 1, B: 0.1},
		{H: 120, S: 1, B: 0.9},
	}

	// Per-light rate limiting means divergence is gradual. Run for 2s so
	// lights have time to separate. At rate=1.0/s over 2s, each light can
	// move up to 2.0 from its start — more than enough to reach the targets.
	var out []lights.Color
	for i := 0; i < 40; i++ {
		*clock = clock.Add(50 * time.Millisecond)
		out = s.Smooth(spread, false, 0, 0.5, 1.0, 0.0, 1.0)
	}

	diff := math.Abs(out[0].B - out[1].B)
	if diff < 0.3 {
		t.Errorf("wide deviation (100%%) should allow large spread over time, got diff=%.4f (expected >0.30)", diff)
	}
}

func TestTemporalSmoother_MultiLight_StaticScene_StableBrightness(t *testing.T) {
	s, clock := newTestSmoother()

	base := []lights.Color{
		{H: 30, S: 0.8, B: 0.60},
		{H: 200, S: 0.6, B: 0.60},
	}
	s.Smooth(base, false, 0.5, 0.8, defaultDev, 0.0, 1.0)

	var maxDelta float64
	prevB := 0.60

	for i := 0; i < 100; i++ {
		*clock = clock.Add(50 * time.Millisecond)
		jitter := 0.02 * math.Sin(float64(i)*0.7)
		frame := []lights.Color{
			{H: 30, S: 0.8, B: 0.60 + jitter},
			{H: 200, S: 0.6, B: 0.60 - jitter},
		}
		out := s.Smooth(frame, false, 0.5, 0.8, defaultDev, 0.0, 1.0)

		avgOut := (out[0].B + out[1].B) / 2.0
		delta := math.Abs(avgOut - prevB)
		if delta > maxDelta {
			maxDelta = delta
		}
		prevB = avgOut
	}

	if maxDelta > 0.02 {
		t.Errorf("static scene with ±0.02 jitter should produce very stable brightness, max frame-to-frame delta was %.4f", maxDelta)
	}
}

func TestTemporalSmoother_SingleLight_FrameAvg_IsLightItself(t *testing.T) {
	s, clock := newTestSmoother()

	s.Smooth([]lights.Color{{H: 0, S: 1, B: 0.3}}, false, 0, 0.5, defaultDev, 0.0, 1.0)

	*clock = clock.Add(1 * time.Second)
	out := s.Smooth([]lights.Color{{H: 0, S: 1, B: 0.8}}, false, 0, 0.5, defaultDev, 0.0, 1.0)

	if out[0].B < 0.35 || out[0].B > 0.85 {
		t.Errorf("single light after 1s at smoothing=0.5 should be ramping, got B=%.3f", out[0].B)
	}
}

func TestTemporalSmoother_IndependentColorAndBrightness(t *testing.T) {
	s, clock := newTestSmoother()

	base := lights.Color{H: 200, S: 0.5, B: 0.50}
	s.Smooth([]lights.Color{base}, false, 0, 1.0, defaultDev, 0.0, 1.0)
	*clock = clock.Add(50 * time.Millisecond)

	shifted := lights.Color{H: 220, S: 0.5, B: 0.90}
	out := s.Smooth([]lights.Color{shifted}, false, 0, 1.0, defaultDev, 0.0, 1.0)

	if math.Abs(out[0].H-220) > 1.0 {
		t.Errorf("expected hue to pass through (220), got H=%.1f", out[0].H)
	}

	if out[0].B > 0.52 {
		t.Errorf("brightness should be rate-limited at max smoothing, got B=%.3f (expected ~0.505)", out[0].B)
	}
}
