package store

// ColorScienceTuning holds optional extraction heuristics for scene-palette
// (and related) paths. Zero values mean "use engine defaults" — see accessors
// below. Persisted on ScreenSyncConfig so presets, JSON edits, and calibration
// can tune colour science without code changes.
type ColorScienceTuning struct {
	// BrightestPaletteChromaBias is added to per-pixel saturation before it
	// multiplies luminance in scene-palette "brightest" histogram weighting.
	// Higher pushes neutrals down vs colourful pixels. Default 0.12 when 0.
	BrightestPaletteChromaBias float64 `json:"brightestPaletteChromaBias,omitempty"`

	// BrightestRankMinSaturation is the minimum S used when ranking brightest
	// palette slots by B×S. Higher demotes near-greys in slot order. Default
	// 0.08 when 0.
	BrightestRankMinSaturation float64 `json:"brightestRankMinSaturation,omitempty"`

	// PaletteSlotBlend is the EMA alpha from the previous frame's slot colour
	// toward the fresh ranked colour (0–1). Default 0.5 when 0.
	PaletteSlotBlend float64 `json:"paletteSlotBlend,omitempty"`
}

// BrightestPaletteChromaBias returns the histogram chroma offset for brightest
// palette weighting, clamped to a safe range.
func (c ScreenSyncConfig) BrightestPaletteChromaBias() float64 {
	v := c.ColorTuning.BrightestPaletteChromaBias
	if v <= 0 {
		return 0.12
	}
	return clampScienceF(v, 0.02, 0.45)
}

// BrightestRankMinSaturation returns the floor on saturation for brightest
// slot ranking (B×S key).
func (c ScreenSyncConfig) BrightestRankMinSaturation() float64 {
	v := c.ColorTuning.BrightestRankMinSaturation
	if v <= 0 {
		return 0.08
	}
	return clampScienceF(v, 0.02, 0.35)
}

// PaletteSlotBlendAlpha returns temporal blend alpha for ranked palette slots.
func (c ScreenSyncConfig) PaletteSlotBlendAlpha() float64 {
	v := c.ColorTuning.PaletteSlotBlend
	if v <= 0 {
		return 0.5
	}
	return clampScienceF(v, 0.05, 1.0)
}

func clampScienceF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
