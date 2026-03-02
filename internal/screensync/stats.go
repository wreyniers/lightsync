package screensync

import (
	"sync"
	"time"

	"lightsync/internal/screensync/process"
	"lightsync/internal/store"
)

// Stats contains the performance metrics emitted every second.
type Stats struct {
	FPS                 float64 `json:"fps"`
	TargetFPS           int     `json:"targetFps"`
	LatencyMs           float64 `json:"latencyMs"`
	CaptureMs           float64 `json:"captureMs"` // avg time spent in screen capture
	ProcessMs           float64 `json:"processMs"` // avg time spent in extract+process
	SendMs              float64 `json:"sendMs"`    // avg time spent sending to lights
	UpdateRate          float64 `json:"updateRate"`   // device updates (SetState calls) per second
	FramesDropped       int     `json:"framesDropped"` // frames skipped (previous send in flight)
	FramesDroppedPct    float64 `json:"framesDroppedPct"`
	SceneChange         bool    `json:"sceneChange"`
	CutReasonBrightness bool    `json:"cutReasonBrightness"` // brightness jump ≥ 40% triggered the cut
	CutReasonHue        bool    `json:"cutReasonHue"`        // hue jump ≥ 80° triggered the cut
	ColorChanged        bool    `json:"colorChanged"`
	AssignmentRewired   bool    `json:"assignmentRewired"` // a color was reassigned to a different device
}

type statsCollector struct {
	mu                  sync.Mutex
	frameCount          int
	latencyTotal        time.Duration
	captureTotal        time.Duration
	processTotal        time.Duration
	sendTotal           time.Duration
	lastReset           time.Time
	sendCount           int   // frames where we actually sent to lights
	dropCount           int   // frames we wanted to send but skipped (prev send in flight)
	deviceUpdatesTotal  int64 // total SetState calls across all sends
	sceneChanged        bool
	cutReasonBrightness bool
	cutReasonHue        bool
	colorChanged        bool
	assignmentRewired   bool
}

func newStatsCollector() *statsCollector {
	return &statsCollector{lastReset: time.Now()}
}

func (s *statsCollector) recordFrame(latency, capture, process, send time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.frameCount++
	s.latencyTotal += latency
	s.captureTotal += capture
	s.processTotal += process
	s.sendTotal += send
}

// recordSceneChange marks that a scene cut was detected in the current window.
func (s *statsCollector) recordSceneChange() {
	s.mu.Lock()
	s.sceneChanged = true
	s.mu.Unlock()
}

// recordCutReasons stores which threshold(s) triggered the cut.
func (s *statsCollector) recordCutReasons(r process.CutReasons) {
	s.mu.Lock()
	if r.Brightness {
		s.cutReasonBrightness = true
	}
	if r.Hue {
		s.cutReasonHue = true
	}
	s.mu.Unlock()
}

// recordColorChange marks that the output color changed meaningfully this frame.
func (s *statsCollector) recordColorChange() {
	s.mu.Lock()
	s.colorChanged = true
	s.mu.Unlock()
}

// recordAssignmentRewired marks that at least one device was reassigned.
func (s *statsCollector) recordAssignmentRewired() {
	s.mu.Lock()
	s.assignmentRewired = true
	s.mu.Unlock()
}

// recordSend records a completed send and how many device updates it performed.
func (s *statsCollector) recordSend(deviceCount int) {
	s.mu.Lock()
	s.sendCount++
	s.deviceUpdatesTotal += int64(deviceCount)
	s.mu.Unlock()
}

// recordSendDuration adds to the total send time (used by per-brand pipelines).
func (s *statsCollector) recordSendDuration(d time.Duration) {
	s.mu.Lock()
	s.sendTotal += d
	s.mu.Unlock()
}

// recordDrop records a frame where we wanted to send but skipped (previous send in flight).
func (s *statsCollector) recordDrop() {
	s.mu.Lock()
	s.dropCount++
	s.mu.Unlock()
}

func (s *statsCollector) snapshot(preset store.SpeedPreset) Stats {
	s.mu.Lock()
	defer s.mu.Unlock()

	elapsed := time.Since(s.lastReset).Seconds()
	var fps, latMs, capMs, procMs, sendMs, updateRate, dropPct float64
	if elapsed > 0 {
		fps = float64(s.frameCount) / elapsed
		updateRate = float64(s.deviceUpdatesTotal) / elapsed
	}
	if s.frameCount > 0 {
		n := float64(s.frameCount)
		latMs = s.latencyTotal.Seconds() * 1000 / n
		capMs = s.captureTotal.Seconds() * 1000 / n
		procMs = s.processTotal.Seconds() * 1000 / n
		sendMs = s.sendTotal.Seconds() * 1000 / n
	}
	sendAndDrop := s.sendCount + s.dropCount
	if sendAndDrop > 0 {
		dropPct = 100 * float64(s.dropCount) / float64(sendAndDrop)
	}

	st := Stats{
		FPS:                 fps,
		TargetFPS:           targetFPS(preset),
		LatencyMs:           latMs,
		CaptureMs:           capMs,
		ProcessMs:           procMs,
		SendMs:              sendMs,
		UpdateRate:          updateRate,
		FramesDropped:       s.dropCount,
		FramesDroppedPct:    dropPct,
		SceneChange:         s.sceneChanged,
		CutReasonBrightness: s.cutReasonBrightness,
		CutReasonHue:        s.cutReasonHue,
		ColorChanged:        s.colorChanged,
		AssignmentRewired:   s.assignmentRewired,
	}
	// Reset per-window flags so next snapshot starts clean.
	s.sceneChanged = false
	s.cutReasonBrightness = false
	s.cutReasonHue = false
	s.colorChanged = false
	s.assignmentRewired = false
	s.sendCount = 0
	s.dropCount = 0
	s.deviceUpdatesTotal = 0
	return st
}

func (s *statsCollector) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.frameCount = 0
	s.latencyTotal = 0
	s.captureTotal = 0
	s.processTotal = 0
	s.sendTotal = 0
	s.sendCount = 0
	s.dropCount = 0
	s.deviceUpdatesTotal = 0
	s.lastReset = time.Now()
	// Flags are intentionally NOT reset here — they're reset in snapshot() so
	// events that arrive between snapshot() and reset() are not lost.
}

func targetFPS(preset store.SpeedPreset) int {
	switch preset {
	case store.SpeedPresetVerySlow:
		return 5
	case store.SpeedPresetSlow:
		return 10
	case store.SpeedPresetFast:
		return 30
	case store.SpeedPresetRealtime:
		return 60
	default:
		return 20
	}
}
