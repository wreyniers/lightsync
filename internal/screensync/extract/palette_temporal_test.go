package extract_test

import (
	"image"
	"testing"

	"lightsync/internal/screensync/extract"
	"lightsync/internal/screensync/process"
	"lightsync/internal/store"
)

// mostlySolidWithStripe creates a near-monochrome frame with a narrow stripe
// of a contaminant colour at the right edge.
func mostlySolidWithStripe(w, h int, base, stripe [3]uint8, stripeWidth int) *image.RGBA {
	img := solidImage(w, h, base[0], base[1], base[2])
	startX := w - stripeWidth
	if startX < 0 {
		startX = 0
	}
	for y := 0; y < h; y++ {
		for x := startX; x < w; x++ {
			off := img.PixOffset(x, y)
			img.Pix[off+0] = stripe[0]
			img.Pix[off+1] = stripe[1]
			img.Pix[off+2] = stripe[2]
			img.Pix[off+3] = 255
		}
	}
	return img
}

func TestPaletteNearMonochromeSuppressesContaminantSlots(t *testing.T) {
	cfg := multiPaletteCfg(4, store.ExtractionMethodDiverse)
	ex := extract.New(cfg)

	// First frame intentionally contains a small grey contaminant stripe,
	// simulating a one-frame paint artifact around an otherwise solid fixture.
	contaminated := mostlySolidWithStripe(
		640, 480,
		[3]uint8{230, 40, 40}, // dominant red
		[3]uint8{32, 32, 32},  // contaminant grey
		8,                     // ~1.25% of width
	)
	raw := ex.Extract(contaminated, cfg)
	_ = process.ApplyAdjustments(raw, cfg)

	// Subsequent clean frames should not keep propagating that contaminant
	// into palette slots.
	clean := solidImage(640, 480, 230, 40, 40)
	for i := 0; i < 3; i++ {
		raw = ex.Extract(clean, cfg)
		out := process.ApplyAdjustments(raw, cfg)
		if len(out) != 4 {
			t.Fatalf("expected 4 palette colors, got %d", len(out))
		}
		for idx, c := range out {
			if c.S < 0.7 || c.B < 0.7 {
				t.Fatalf("slot %d retained non-red contaminant: got H:%.1f S:%.3f B:%.3f", idx, c.H, c.S, c.B)
			}
		}
	}
}
