package store

import (
	"math"
	"reflect"
)

// ScreenSyncPortableDefaults holds optional global tuning for Screen Sync.
// Only non-nil fields participate in merge; persisted in app config JSON.
// These are the same “portable” fields calibration can optimize (capture and
// device layout stay per-scene).
type ScreenSyncPortableDefaults struct {
	SaturationBoost      *float64 `json:"saturationBoost,omitempty"`
	WhiteBias            *float64 `json:"whiteBias,omitempty"`
	BrightnessMultiplier *float64 `json:"brightnessMultiplier,omitempty"`
	PaletteStability     *float64 `json:"paletteStability,omitempty"`
	ColorSmoothing       *float64 `json:"colorSmoothing,omitempty"`
	BrightnessSmoothing  *float64 `json:"brightnessSmoothing,omitempty"`
	AssignmentHandoffMs  *int     `json:"assignmentHandoffMs,omitempty"`
	ColorTuning          *ColorScienceTuning `json:"colorTuning,omitempty"`
}

// BuildPortableDefaultsFromScreenSync copies portable fields from src into
// a defaults object with pointers (for persistence after calibration).
func BuildPortableDefaultsFromScreenSync(src ScreenSyncConfig) *ScreenSyncPortableDefaults {
	p := &ScreenSyncPortableDefaults{}
	p.SaturationBoost = floatPtr(src.SaturationBoost)
	p.WhiteBias = floatPtr(src.WhiteBias)
	p.BrightnessMultiplier = floatPtr(src.BrightnessMultiplier)
	p.PaletteStability = floatPtr(src.PaletteStability)
	p.ColorSmoothing = floatPtr(src.ColorSmoothing)
	p.BrightnessSmoothing = floatPtr(src.BrightnessSmoothing)
	if src.AssignmentHandoffMs != 0 {
		v := src.AssignmentHandoffMs
		p.AssignmentHandoffMs = &v
	}
	ct := src.ColorTuning
	if ct.BrightestPaletteChromaBias != 0 || ct.BrightestRankMinSaturation != 0 || ct.PaletteSlotBlend != 0 {
		cpy := ct
		p.ColorTuning = &cpy
	}
	return p
}

func floatPtr(f float64) *float64 {
	v := f
	return &v
}

func approxEq(a, b float64) bool {
	return math.Abs(a-b) <= 1e-6
}

// ApplyAllPortable applies every non-nil field from p onto out.
func (p *ScreenSyncPortableDefaults) ApplyAllPortable(out *ScreenSyncConfig) {
	if p == nil {
		return
	}
	if p.SaturationBoost != nil {
		out.SaturationBoost = *p.SaturationBoost
	}
	if p.WhiteBias != nil {
		out.WhiteBias = *p.WhiteBias
	}
	if p.BrightnessMultiplier != nil {
		out.BrightnessMultiplier = *p.BrightnessMultiplier
	}
	if p.PaletteStability != nil {
		out.PaletteStability = *p.PaletteStability
	}
	if p.ColorSmoothing != nil {
		out.ColorSmoothing = *p.ColorSmoothing
	}
	if p.BrightnessSmoothing != nil {
		out.BrightnessSmoothing = *p.BrightnessSmoothing
	}
	if p.AssignmentHandoffMs != nil {
		out.AssignmentHandoffMs = *p.AssignmentHandoffMs
	}
	if p.ColorTuning != nil {
		out.ColorTuning = *p.ColorTuning
	}
}

// ApplyPortableWhereSceneMatchesFactory upgrades out when a portable field
// still matches factory defaults — so per-scene custom tuning is preserved.
func (p *ScreenSyncPortableDefaults) ApplyPortableWhereSceneMatchesFactory(out *ScreenSyncConfig, factory ScreenSyncConfig) {
	if p == nil {
		return
	}
	if p.SaturationBoost != nil && approxEq(out.SaturationBoost, factory.SaturationBoost) {
		out.SaturationBoost = *p.SaturationBoost
	}
	if p.WhiteBias != nil && approxEq(out.WhiteBias, factory.WhiteBias) {
		out.WhiteBias = *p.WhiteBias
	}
	if p.BrightnessMultiplier != nil && approxEq(out.BrightnessMultiplier, factory.BrightnessMultiplier) {
		out.BrightnessMultiplier = *p.BrightnessMultiplier
	}
	if p.PaletteStability != nil && approxEq(out.PaletteStability, factory.PaletteStability) {
		out.PaletteStability = *p.PaletteStability
	}
	if p.ColorSmoothing != nil && approxEq(out.ColorSmoothing, factory.ColorSmoothing) {
		out.ColorSmoothing = *p.ColorSmoothing
	}
	if p.BrightnessSmoothing != nil && approxEq(out.BrightnessSmoothing, factory.BrightnessSmoothing) {
		out.BrightnessSmoothing = *p.BrightnessSmoothing
	}
	if p.AssignmentHandoffMs != nil && out.AssignmentHandoffMs == factory.AssignmentHandoffMs {
		out.AssignmentHandoffMs = *p.AssignmentHandoffMs
	}
	if p.ColorTuning != nil && reflect.DeepEqual(out.ColorTuning, factory.ColorTuning) {
		out.ColorTuning = *p.ColorTuning
	}
}

// EffectiveScreenSyncConfig merges factory defaults, optional global portable
// defaults, and the per-scene config.
//
// When scene is nil (e.g. seeding a new editor), returns factory defaults with
// all saved portable fields applied.
//
// When scene is set, portable defaults apply only where the scene still matches
// factory defaults for that field, so explicit per-scene overrides win.
func EffectiveScreenSyncConfig(scene *ScreenSyncConfig, portable *ScreenSyncPortableDefaults) ScreenSyncConfig {
	factory := DefaultScreenSyncConfig()
	if scene == nil {
		out := factory
		if portable != nil {
			portable.ApplyAllPortable(&out)
		}
		NormalizeScreenSyncConfig(&out)
		return out
	}
	out := *scene
	if portable != nil {
		portable.ApplyPortableWhereSceneMatchesFactory(&out, factory)
	}
	NormalizeScreenSyncConfig(&out)
	return out
}
