package selftest

import (
	"lightsync/internal/store"
)

const (
	DefaultCalibrationMaxEvaluations = 64
)

// CoordinateSearchResult is the outcome of a bounded coordinate search.
type CoordinateSearchResult struct {
	BestConfig   store.ScreenSyncConfig `json:"bestConfig"`
	BestRun      *SuiteRun              `json:"bestRun"`
	BaselineRun  *SuiteRun              `json:"baselineRun,omitempty"`
	BestMetrics  SuiteCalibrationMetrics `json:"bestMetrics"`
	StartMetrics SuiteCalibrationMetrics `json:"startMetrics"`
	EvalCount    int                     `json:"evalCount"`
}

// RunCoordinateSearch performs coordinate descent on calibration knobs.
// eval must return a fresh suite run for cfg (caller runs capture suite).
func RunCoordinateSearch(
	start store.ScreenSyncConfig,
	eval func(cfg store.ScreenSyncConfig) (*SuiteRun, error),
	maxEval int,
) (CoordinateSearchResult, error) {
	if maxEval <= 0 {
		maxEval = DefaultCalibrationMaxEvaluations
	}
	var out CoordinateSearchResult
	cur := clampCalibrationFields(CloneScreenSyncConfig(start))
	run, err := eval(cur)
	if err != nil {
		return out, err
	}
	out.EvalCount++
	out.BaselineRun = run
	out.BestRun = run
	out.BestMetrics = ScoreSuiteRun(run)
	out.StartMetrics = out.BestMetrics
	out.BestConfig = cur
	bestScore := out.BestMetrics.Total

	steps := []struct {
		satBoost, whiteBias, brightMul, palStab, colSmooth, brightSmooth float64
		handoffMs       int
		chromaBias, rankMinSat, slotBlend float64
	}{
		{0.06, 0.05, 0.05, 0.12, 0.12, 0.12, 75, 0.02, 0.012, 0.06},
		{0.03, 0.025, 0.025, 0.06, 0.06, 0.06, 50, 0.012, 0.008, 0.04},
		{0.015, 0.012, 0.012, 0.03, 0.03, 0.03, 25, 0.006, 0.004, 0.02},
	}

	for _, st := range steps {
		improved := true
		for improved && out.EvalCount < maxEval {
			improved = false

			try := func(modify func(*store.ScreenSyncConfig)) bool {
				if out.EvalCount >= maxEval {
					return false
				}
				cand := clampCalibrationFields(CloneScreenSyncConfig(cur))
				modify(&cand)
				if configsEqualCalibration(cand, cur) {
					return false
				}
				rn, e := eval(cand)
				if e != nil {
					return false
				}
				out.EvalCount++
				sc := ScoreSuiteRun(rn).Total
				if sc < bestScore {
					bestScore = sc
					cur = cand
					out.BestRun = rn
					out.BestConfig = cand
					out.BestMetrics = ScoreSuiteRun(rn)
					return true
				}
				return false
			}

			// SaturationBoost
			if try(func(c *store.ScreenSyncConfig) { c.SaturationBoost += st.satBoost }) {
				improved = true
			}
			if out.EvalCount < maxEval && try(func(c *store.ScreenSyncConfig) { c.SaturationBoost -= st.satBoost }) {
				improved = true
			}
			// WhiteBias
			if try(func(c *store.ScreenSyncConfig) { c.WhiteBias += st.whiteBias }) {
				improved = true
			}
			if out.EvalCount < maxEval && try(func(c *store.ScreenSyncConfig) { c.WhiteBias -= st.whiteBias }) {
				improved = true
			}
			// BrightnessMultiplier
			if try(func(c *store.ScreenSyncConfig) { c.BrightnessMultiplier += st.brightMul }) {
				improved = true
			}
			if out.EvalCount < maxEval && try(func(c *store.ScreenSyncConfig) { c.BrightnessMultiplier -= st.brightMul }) {
				improved = true
			}
			// PaletteStability
			if try(func(c *store.ScreenSyncConfig) { c.PaletteStability += st.palStab }) {
				improved = true
			}
			if out.EvalCount < maxEval && try(func(c *store.ScreenSyncConfig) { c.PaletteStability -= st.palStab }) {
				improved = true
			}
			// ColorSmoothing
			if try(func(c *store.ScreenSyncConfig) { c.ColorSmoothing += st.colSmooth }) {
				improved = true
			}
			if out.EvalCount < maxEval && try(func(c *store.ScreenSyncConfig) { c.ColorSmoothing -= st.colSmooth }) {
				improved = true
			}
			// BrightnessSmoothing
			if try(func(c *store.ScreenSyncConfig) { c.BrightnessSmoothing += st.brightSmooth }) {
				improved = true
			}
			if out.EvalCount < maxEval && try(func(c *store.ScreenSyncConfig) { c.BrightnessSmoothing -= st.brightSmooth }) {
				improved = true
			}
			// AssignmentHandoffMs
			if try(func(c *store.ScreenSyncConfig) { c.AssignmentHandoffMs += st.handoffMs }) {
				improved = true
			}
			if out.EvalCount < maxEval && try(func(c *store.ScreenSyncConfig) { c.AssignmentHandoffMs -= st.handoffMs }) {
				improved = true
			}
			// Color science (scene-palette brightest heuristics + slot blend)
			if try(func(c *store.ScreenSyncConfig) { c.ColorTuning.BrightestPaletteChromaBias += st.chromaBias }) {
				improved = true
			}
			if out.EvalCount < maxEval && try(func(c *store.ScreenSyncConfig) { c.ColorTuning.BrightestPaletteChromaBias -= st.chromaBias }) {
				improved = true
			}
			if try(func(c *store.ScreenSyncConfig) { c.ColorTuning.BrightestRankMinSaturation += st.rankMinSat }) {
				improved = true
			}
			if out.EvalCount < maxEval && try(func(c *store.ScreenSyncConfig) { c.ColorTuning.BrightestRankMinSaturation -= st.rankMinSat }) {
				improved = true
			}
			if try(func(c *store.ScreenSyncConfig) { c.ColorTuning.PaletteSlotBlend += st.slotBlend }) {
				improved = true
			}
			if out.EvalCount < maxEval && try(func(c *store.ScreenSyncConfig) { c.ColorTuning.PaletteSlotBlend -= st.slotBlend }) {
				improved = true
			}
		}
	}

	out.BestConfig = cur
	return out, nil
}

// CloneScreenSyncConfig returns a deep-enough copy for mutation (DeviceIDs slice).
func CloneScreenSyncConfig(src store.ScreenSyncConfig) store.ScreenSyncConfig {
	out := src
	if len(src.DeviceIDs) > 0 {
		out.DeviceIDs = append([]string(nil), src.DeviceIDs...)
	}
	return out
}

func clampCalibrationFields(cfg store.ScreenSyncConfig) store.ScreenSyncConfig {
	cfg.SaturationBoost = clampF(cfg.SaturationBoost, 0.8, 1.5)
	cfg.WhiteBias = clampF(cfg.WhiteBias, -0.25, 0.25)
	cfg.BrightnessMultiplier = clampF(cfg.BrightnessMultiplier, 0.85, 1.2)
	cfg.PaletteStability = clampF(cfg.PaletteStability, 0.4, 1.4)
	cfg.ColorSmoothing = clampF(cfg.ColorSmoothing, 0.2, 1.0)
	cfg.BrightnessSmoothing = clampF(cfg.BrightnessSmoothing, 0.2, 1.0)
	cfg.AssignmentHandoffMs = clampI(cfg.AssignmentHandoffMs, 150, 900)
	if cfg.ColorTuning.BrightestPaletteChromaBias != 0 {
		cfg.ColorTuning.BrightestPaletteChromaBias = clampF(cfg.ColorTuning.BrightestPaletteChromaBias, 0.04, 0.28)
	}
	if cfg.ColorTuning.BrightestRankMinSaturation != 0 {
		cfg.ColorTuning.BrightestRankMinSaturation = clampF(cfg.ColorTuning.BrightestRankMinSaturation, 0.04, 0.22)
	}
	if cfg.ColorTuning.PaletteSlotBlend != 0 {
		cfg.ColorTuning.PaletteSlotBlend = clampF(cfg.ColorTuning.PaletteSlotBlend, 0.15, 1.0)
	}
	store.NormalizeScreenSyncConfig(&cfg)
	return cfg
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampI(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func configsEqualCalibration(a, b store.ScreenSyncConfig) bool {
	return a.SaturationBoost == b.SaturationBoost &&
		a.WhiteBias == b.WhiteBias &&
		a.BrightnessMultiplier == b.BrightnessMultiplier &&
		a.PaletteStability == b.PaletteStability &&
		a.ColorSmoothing == b.ColorSmoothing &&
		a.BrightnessSmoothing == b.BrightnessSmoothing &&
		a.AssignmentHandoffMs == b.AssignmentHandoffMs &&
		a.ColorTuning == b.ColorTuning
}
