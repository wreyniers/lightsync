package selftest

import (
	"math"
)

// SuiteCalibrationMetrics aggregates interpretable components of the
// balanced calibration objective (lower Total is better).
type SuiteCalibrationMetrics struct {
	AccuracyCost  float64 `json:"accuracyCost"`
	LockCost      float64 `json:"lockCost"`
	StabilityCost float64 `json:"stabilityCost"`
	Total         float64 `json:"total"`
	LockedCells   int     `json:"lockedCells"`
	TotalCells    int     `json:"totalCells"`
}

const (
	weightAccuracy  = 0.55
	weightLock      = 0.30
	weightStability = 0.15
)

// ScoreSuiteRun computes a scalar cost from a completed suite (lower is better).
func ScoreSuiteRun(run *SuiteRun) SuiteCalibrationMetrics {
	if run == nil || len(run.Fixtures) == 0 {
		return SuiteCalibrationMetrics{Total: 1e6}
	}
	var accSum, stabSum float64
	var accN, stabN int
	locked := run.LockedCells
	total := run.TotalCells

	for _, fr := range run.Fixtures {
		for _, cell := range fr.Diagnostic.Cells {
			if len(cell.Expected) == 0 {
				continue
			}
			accN++
			if cell.Delta != nil {
				h := math.Abs(cell.Delta.WorstHue) / 180.0
				if h > 1 {
					h = 1
				}
				s := math.Abs(cell.Delta.WorstSat)
				b := math.Abs(cell.Delta.WorstBright)
				accSum += h + 0.5*s + 0.5*b
			} else {
				accSum += 1.0
			}
			if cell.StabilityJitter > 0 {
				stabSum += cell.StabilityJitter
				stabN++
			}
		}
	}
	accMean := 0.0
	if accN > 0 {
		accMean = accSum / float64(accN)
	}
	stabMean := 0.0
	if stabN > 0 {
		stabMean = stabSum / float64(stabN)
	}
	lockRatio := 0.0
	if total > 0 {
		lockRatio = float64(locked) / float64(total)
	}
	lockCost := (1.0 - lockRatio) + 0.15*meanLockedAgeNorm(run)

	totalScore := weightAccuracy*accMean + weightLock*lockCost + weightStability*stabMean
	return SuiteCalibrationMetrics{
		AccuracyCost:  accMean,
		LockCost:      lockCost,
		StabilityCost: stabMean,
		Total:         totalScore,
		LockedCells:   locked,
		TotalCells:    total,
	}
}

// meanLockedAgeNorm averages lockedSinceMs for locked cells, scaled to ~0..1.
func meanLockedAgeNorm(run *SuiteRun) float64 {
	var sum float64
	var n int
	for _, fr := range run.Fixtures {
		for _, cell := range fr.Diagnostic.Cells {
			if !cell.Locked || len(cell.Expected) == 0 {
				continue
			}
			sum += float64(cell.LockedSinceMs)
			n++
		}
	}
	if n == 0 {
		return 0
	}
	avg := sum / float64(n)
	// Typical lock age a few seconds; cap contribution.
	return math.Min(avg/5000.0, 1.0)
}
