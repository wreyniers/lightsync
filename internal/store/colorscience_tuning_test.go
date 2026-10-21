package store

import "testing"

func TestColorScienceTuning_Defaults(t *testing.T) {
	var cfg ScreenSyncConfig
	if g, w := cfg.BrightestPaletteChromaBias(), 0.12; g != w {
		t.Fatalf("default chroma bias: got %v want %v", g, w)
	}
	if g, w := cfg.BrightestRankMinSaturation(), 0.08; g != w {
		t.Fatalf("default rank min sat: got %v want %v", g, w)
	}
	if g, w := cfg.PaletteSlotBlendAlpha(), 0.5; g != w {
		t.Fatalf("default slot blend: got %v want %v", g, w)
	}
}

func TestColorScienceTuning_Clamped(t *testing.T) {
	cfg := ScreenSyncConfig{
		ColorTuning: ColorScienceTuning{
			BrightestPaletteChromaBias: 99,
			BrightestRankMinSaturation: 99,
			PaletteSlotBlend:           99,
		},
	}
	if v := cfg.BrightestPaletteChromaBias(); v > 0.45 || v < 0.02 {
		t.Fatalf("chroma bias clamp: %v", v)
	}
	if v := cfg.BrightestRankMinSaturation(); v > 0.35 || v < 0.02 {
		t.Fatalf("rank min sat clamp: %v", v)
	}
	if v := cfg.PaletteSlotBlendAlpha(); v > 1.0 || v < 0.05 {
		t.Fatalf("slot blend clamp: %v", v)
	}
}
