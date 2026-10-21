package selftest

import (
	"testing"
	"time"

	"lightsync/internal/lights"
	"lightsync/internal/store"
)

func TestScoreSuiteRun_LowerWhenMoreLocked(t *testing.T) {
	locked := &SuiteRun{
		LockedCells: 10,
		TotalCells:  10,
		Fixtures: []FixtureRunResult{{
			Diagnostic: Diagnostic{
				Cells: []DiagnosticCell{
					{
						Expected: []lights.Color{{H: 0, S: 1, B: 1}},
						Locked:   true,
						Delta: &DiagnosticDelta{
							WorstHue: 0, WorstSat: 0, WorstBright: 0,
						},
						StabilityJitter: 0.01,
					},
				},
			},
		}},
	}
	unlocked := &SuiteRun{
		LockedCells: 5,
		TotalCells:  10,
		Fixtures: []FixtureRunResult{{
			Diagnostic: Diagnostic{
				Cells: []DiagnosticCell{
					{
						Expected: []lights.Color{{H: 0, S: 1, B: 1}},
						Locked:   false,
						Delta: &DiagnosticDelta{
							WorstHue: 30, WorstSat: 0.2, WorstBright: 0.1,
						},
						StabilityJitter: 0.05,
					},
				},
			},
		}},
	}
	s1 := ScoreSuiteRun(locked).Total
	s2 := ScoreSuiteRun(unlocked).Total
	if s1 >= s2 {
		t.Fatalf("expected fully locked run to score lower than partial; got %v vs %v", s1, s2)
	}
}

func TestClampCalibrationFields_RespectsBounds(t *testing.T) {
	cfg := store.ScreenSyncConfig{
		SaturationBoost:       5,
		WhiteBias:             9,
		BrightnessMultiplier: 0.01,
		PaletteStability:      0,
		ColorSmoothing:        3,
		BrightnessSmoothing:   -1,
		AssignmentHandoffMs:   10_000,
		ColorTuning: store.ColorScienceTuning{
			BrightestPaletteChromaBias:  9,
			BrightestRankMinSaturation: 9,
			PaletteSlotBlend:           0.01,
		},
	}
	out := clampCalibrationFields(cfg)
	if out.SaturationBoost > 1.5 || out.SaturationBoost < 0.8 {
		t.Fatalf("SaturationBoost out of bounds: %v", out.SaturationBoost)
	}
	if out.WhiteBias > 0.25 || out.WhiteBias < -0.25 {
		t.Fatalf("WhiteBias out of bounds: %v", out.WhiteBias)
	}
	if out.AssignmentHandoffMs < 150 || out.AssignmentHandoffMs > 900 {
		t.Fatalf("AssignmentHandoffMs out of bounds: %d", out.AssignmentHandoffMs)
	}
	if out.ColorTuning.BrightestPaletteChromaBias < 0.04 || out.ColorTuning.BrightestPaletteChromaBias > 0.28 {
		t.Fatalf("BrightestPaletteChromaBias out of bounds: %v", out.ColorTuning.BrightestPaletteChromaBias)
	}
	if out.ColorTuning.BrightestRankMinSaturation < 0.04 || out.ColorTuning.BrightestRankMinSaturation > 0.22 {
		t.Fatalf("BrightestRankMinSaturation out of bounds: %v", out.ColorTuning.BrightestRankMinSaturation)
	}
	if out.ColorTuning.PaletteSlotBlend < 0.15 || out.ColorTuning.PaletteSlotBlend > 1.0 {
		t.Fatalf("PaletteSlotBlend out of bounds: %v", out.ColorTuning.PaletteSlotBlend)
	}
}

func TestRunCoordinateSearch_NoEvalPanic(t *testing.T) {
	start := store.DefaultScreenSyncConfig()
	n := 0
	_, err := RunCoordinateSearch(start, func(cfg store.ScreenSyncConfig) (*SuiteRun, error) {
		n++
		// Minimal synthetic suite run for scoring.
		return &SuiteRun{
			StartedAt:    time.Now(),
			FinishedAt:   time.Now(),
			FixtureCount: 1,
			LockedCells:  1,
			TotalCells:   1,
			Fixtures: []FixtureRunResult{{
				Diagnostic: Diagnostic{
					Cells: []DiagnosticCell{
						{
							Expected: []lights.Color{{H: 0, S: 0.5, B: 0.5}},
							Locked:   true,
							Delta: &DiagnosticDelta{
								WorstHue: 0, WorstSat: 0.01, WorstBright: 0.01,
							},
						},
					},
				},
			}},
		}, nil
	}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Fatalf("expected at least one evaluation")
	}
}
