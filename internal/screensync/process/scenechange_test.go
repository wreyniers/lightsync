package process

import (
	"testing"

	"lightsync/internal/lights"
)

func TestSceneChangeDetector_FirstFrame_NoCut(t *testing.T) {
	d := NewSceneChangeDetector()
	colors := []lights.Color{{H: 0, S: 1, B: 1}}
	isCut, _ := d.Check(colors, 0.5)
	if isCut {
		t.Error("first frame should never be a cut")
	}
}

func TestSceneChangeDetector_IdenticalFrames_NoCut(t *testing.T) {
	d := NewSceneChangeDetector()
	colors := []lights.Color{{H: 100, S: 0.5, B: 0.7}}

	for i := 0; i < 40; i++ {
		isCut, _ := d.Check(colors, 0.5)
		if i > 0 && isCut {
			t.Errorf("frame %d: identical frames should not trigger a cut", i)
		}
	}
}

func TestSceneChangeDetector_DramaticChange_DetectsCut(t *testing.T) {
	d := NewSceneChangeDetector()
	redFrame := []lights.Color{{H: 0, S: 1, B: 1}}
	blueFrame := []lights.Color{{H: 240, S: 1, B: 1}}

	for i := 0; i < 10; i++ {
		d.Check(redFrame, 0.5)
	}

	isCut, _ := d.Check(blueFrame, 0.5)
	if !isCut {
		t.Error("dramatic red→blue change should be detected as a scene cut")
	}
}

func TestSceneChangeDetector_BrightnessJump_DetectsCut(t *testing.T) {
	d := NewSceneChangeDetector()
	dark := []lights.Color{{H: 200, S: 0.5, B: 0.1}}
	bright := []lights.Color{{H: 200, S: 0.5, B: 0.9}}

	for i := 0; i < 10; i++ {
		d.Check(dark, 0.5)
	}

	isCut, reasons := d.Check(bright, 0.5)
	if !isCut {
		t.Error("large brightness jump should be detected as a cut")
	}
	if !reasons.Brightness {
		t.Error("expected Brightness reason to be set")
	}
}

func TestSceneChangeDetector_Debounce(t *testing.T) {
	d := NewSceneChangeDetector()
	a := []lights.Color{{H: 0, S: 1, B: 1}}
	b := []lights.Color{{H: 240, S: 1, B: 1}}

	for i := 0; i < 10; i++ {
		d.Check(a, 0.5)
	}

	isCut, _ := d.Check(b, 0.5)
	if !isCut {
		t.Fatal("expected first cut to trigger")
	}

	isCut, _ = d.Check(a, 0.5)
	if isCut {
		t.Error("cut within debounce window should be suppressed")
	}
}

func TestSceneChangeDetector_MultiColor_AggregatesAll(t *testing.T) {
	d := NewSceneChangeDetector()
	frameA := []lights.Color{
		{H: 0, S: 1, B: 1},
		{H: 120, S: 1, B: 1},
		{H: 240, S: 1, B: 1},
	}
	frameB := []lights.Color{
		{H: 180, S: 1, B: 1},
		{H: 300, S: 1, B: 1},
		{H: 60, S: 1, B: 1},
	}

	for i := 0; i < 10; i++ {
		d.Check(frameA, 0.5)
	}

	isCut, _ := d.Check(frameB, 0.5)
	if !isCut {
		t.Error("all colors changing dramatically should trigger a cut")
	}
}

func TestSceneChangeDetector_SensitivityAffectsDetection(t *testing.T) {
	a := []lights.Color{{H: 0, S: 1, B: 1}}
	// Moderate shift — not dramatic
	b := []lights.Color{{H: 40, S: 0.8, B: 0.85}}

	check := func(sensitivity float64) bool {
		d := NewSceneChangeDetector()
		for i := 0; i < 10; i++ {
			d.Check(a, sensitivity)
		}
		isCut, _ := d.Check(b, sensitivity)
		return isCut
	}

	highSens := check(1.0)
	lowSens := check(0.0)

	if lowSens && !highSens {
		t.Error("higher sensitivity should be at least as likely to detect cuts as lower sensitivity")
	}
}

func TestAggregateDistance_Identical(t *testing.T) {
	colors := []lights.Color{{H: 100, S: 0.5, B: 0.7}}
	c, b := aggregateDistance(colors, colors)
	if c != 0 || b != 0 {
		t.Errorf("identical colors should have zero distance, got color=%f bright=%f", c, b)
	}
}

func TestAggregateDistance_OppositeHues(t *testing.T) {
	a := []lights.Color{{H: 0, S: 1, B: 1}}
	b := []lights.Color{{H: 180, S: 1, B: 1}}
	c, br := aggregateDistance(a, b)
	if c < 0.7 {
		t.Errorf("opposite hues should have high color distance, got %f", c)
	}
	if br > 0.01 {
		t.Errorf("same brightness should have near-zero brightness distance, got %f", br)
	}
}

func TestRingBuf_Stats(t *testing.T) {
	var r ringBuf
	r.push(1)
	r.push(2)
	r.push(3)

	mean, std := r.stats()
	if mean < 1.99 || mean > 2.01 {
		t.Errorf("expected mean ≈ 2.0, got %f", mean)
	}
	if std < 0.75 || std > 0.90 {
		t.Errorf("expected stddev ≈ 0.816, got %f", std)
	}
}

func TestRingBuf_Wraps(t *testing.T) {
	var r ringBuf
	for i := 0; i < ringSize+5; i++ {
		r.push(float64(i))
	}
	if r.count != ringSize {
		t.Errorf("expected count capped at %d, got %d", ringSize, r.count)
	}
}
