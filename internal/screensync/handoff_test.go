package screensync

import (
	"testing"
	"time"

	"lightsync/internal/lights"
)

func TestHandoffBlend_FirstFramePassThrough(t *testing.T) {
	b := newColorHandoffBlender()
	in := map[string]lights.Color{
		"a": {H: 30, S: 1, B: 1},
	}

	out := b.Blend(in, 400)
	if out["a"].H != 30 {
		t.Fatalf("expected first frame pass-through hue=30, got %.2f", out["a"].H)
	}
}

func TestHandoffBlend_TransitionsGradually(t *testing.T) {
	b := newColorHandoffBlender()
	_ = b.Blend(map[string]lights.Color{"a": {H: 30, S: 1, B: 1}}, 300)

	_ = b.Blend(map[string]lights.Color{"a": {H: 120, S: 1, B: 1}}, 300)
	time.Sleep(120 * time.Millisecond)
	out1 := b.Blend(map[string]lights.Color{"a": {H: 120, S: 1, B: 1}}, 300)
	if out1["a"].H <= 30 || out1["a"].H >= 120 {
		t.Fatalf("expected intermediate hue after target change, got %.2f", out1["a"].H)
	}

	time.Sleep(350 * time.Millisecond)
	out2 := b.Blend(map[string]lights.Color{"a": {H: 120, S: 1, B: 1}}, 300)
	if out2["a"].H < 117 || out2["a"].H > 123 {
		t.Fatalf("expected convergence near 120 after handoff duration, got %.2f", out2["a"].H)
	}
}

func TestHandoffBlend_DisabledByZeroMs(t *testing.T) {
	b := newColorHandoffBlender()
	_ = b.Blend(map[string]lights.Color{"a": {H: 30, S: 1, B: 1}}, 250)

	out := b.Blend(map[string]lights.Color{"a": {H: 210, S: 1, B: 1}}, 0)
	if out["a"].H != 210 {
		t.Fatalf("expected pass-through when disabled, got %.2f", out["a"].H)
	}
}
