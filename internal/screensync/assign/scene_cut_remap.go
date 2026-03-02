package assign

import (
	"sync"
	"time"

	"lightsync/internal/lights"
	"lightsync/internal/store"
)

// sceneCutRemapAssigner uses FlowTrack trajectory matching during steady-state
// playback. When a scene cut is detected it immediately runs a fresh global
// Hungarian remap (no stickiness bias), then holds that assignment for
// SceneCutRemapHoldMs before handing back to trajectory tracking.
//
// The hold period uses the current frame's colours with the frozen assignment
// indices, so the output still follows live content while the assignment is
// locked — avoiding the "stale frozen colours" problem of the old hybrid.
type sceneCutRemapAssigner struct {
	mu               sync.Mutex
	flowTrack        *flowTrackAssigner
	frozenAssignment []int
	frozenUntil      time.Time
}

func newSceneCutRemap() *sceneCutRemapAssigner {
	return &sceneCutRemapAssigner{
		flowTrack: newFlowTrack(),
	}
}

func (s *sceneCutRemapAssigner) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flowTrack.Reset()
	s.frozenAssignment = nil
	s.frozenUntil = time.Time{}
}

func (s *sceneCutRemapAssigner) Assign(
	colors []lights.Color,
	deviceIDs []string,
	current map[string]lights.Color,
	cfg store.ScreenSyncConfig,
	isCut bool,
) map[string]lights.Color {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(deviceIDs) == 0 || len(colors) == 0 {
		return map[string]lights.Color{}
	}

	now := time.Now()

	if isCut {
		nc := len(colors)
		nd := len(deviceIDs)

		// Build cost using the current transition output as device references.
		costs := make([][]float64, nc)
		for i, c := range colors {
			costs[i] = make([]float64, nd)
			for j, id := range deviceIDs {
				ref := current[id]
				costs[i][j] = colorDist(c, ref)
			}
		}

		assignment := hungarianSolve(costs, nc, nd)
		result := applyAssignment(assignment, colors, deviceIDs)

		// Sync FlowTrack so it is warm when the freeze expires.
		s.flowTrack.InitFromAssignment(colors, deviceIDs, assignment)

		holdMs := cfg.SceneCutRemapHoldMs
		if holdMs > 0 {
			s.frozenAssignment = assignment
			s.frozenUntil = now.Add(time.Duration(holdMs) * time.Millisecond)
		} else {
			s.frozenAssignment = nil
		}
		return result
	}

	// Within freeze: apply frozen assignment to CURRENT colours (live content,
	// locked device-to-color-slot mapping).
	if s.frozenAssignment != nil && now.Before(s.frozenUntil) {
		result := applyAssignment(s.frozenAssignment, colors, deviceIDs)
		// Keep FlowTrack trajectories warm during the freeze.
		alpha := cfg.FlowTrackEmaAlpha
		if alpha <= 0 || alpha > 1 {
			alpha = 0.25
		}
		s.flowTrack.mu.Lock()
		s.flowTrack.evolveTrajectories(colors, deviceIDs, s.frozenAssignment, alpha)
		s.flowTrack.mu.Unlock()
		return result
	}

	// Normal frame: delegate to FlowTrack.
	return s.flowTrack.Assign(colors, deviceIDs, current, cfg, false)
}
