package selftest

import (
	"time"

	"lightsync/internal/store"
)

// CalibrationReport is the outcome of a self-test calibration run (subset
// search + optional verification). Safe to JSON-serialise for the UI.
type CalibrationReport struct {
	SceneID   string    `json:"sceneId"`
	StartedAt time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`
	Error     string    `json:"error,omitempty"`

	BaselineConfig   store.ScreenSyncConfig `json:"baselineConfig"`
	CandidateConfig  store.ScreenSyncConfig `json:"candidateConfig"`
	BaselineMetrics  SuiteCalibrationMetrics `json:"baselineMetrics"`
	CandidateMetrics SuiteCalibrationMetrics `json:"candidateMetrics"`

	SubsetBaselineRun   *SuiteRun `json:"subsetBaselineRun,omitempty"`
	SubsetCandidateRun  *SuiteRun `json:"subsetCandidateRun,omitempty"`
	SearchEvalCount     int       `json:"searchEvalCount"`
}
