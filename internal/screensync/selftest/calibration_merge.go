package selftest

import "lightsync/internal/store"

// OverlayCalibrationFields copies tunable colour/temporal fields from src
// onto dst, preserving dst's colour mode / extraction method / device layout.
func OverlayCalibrationFields(dst *store.ScreenSyncConfig, src store.ScreenSyncConfig) {
	dst.SaturationBoost = src.SaturationBoost
	dst.WhiteBias = src.WhiteBias
	dst.BrightnessMultiplier = src.BrightnessMultiplier
	dst.PaletteStability = src.PaletteStability
	dst.ColorSmoothing = src.ColorSmoothing
	dst.BrightnessSmoothing = src.BrightnessSmoothing
	dst.AssignmentHandoffMs = src.AssignmentHandoffMs
	dst.ColorTuning = src.ColorTuning
	store.NormalizeScreenSyncConfig(dst)
}

// AllModesWithCalibrationOverlay returns AllModes(deviceCount) with each
// cell's config overlaid by non-zero fields from base (when base is non-nil).
func AllModesWithCalibrationOverlay(deviceCount int, base *store.ScreenSyncConfig) []ModeCase {
	modes := AllModes(deviceCount)
	if base == nil {
		return modes
	}
	for i := range modes {
		OverlayCalibrationFields(&modes[i].Config, *base)
	}
	return modes
}

// CalibrationSearchFixtures returns the first four built-in fixtures
// (synthetics) for faster calibration search iterations.
func CalibrationSearchFixtures() []Fixture {
	all := BuiltInFixtures()
	const n = 4
	if len(all) <= n {
		out := make([]Fixture, len(all))
		copy(out, all)
		return out
	}
	out := make([]Fixture, n)
	copy(out, all[:n])
	return out
}
