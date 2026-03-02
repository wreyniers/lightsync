package assign

import (
	"sync"
	"time"

	"lightsync/internal/lights"
	"lightsync/internal/store"
)

// flowTrackAssigner maintains an EMA colour trajectory per device and runs a
// Hungarian solve against those trajectories on each solve interval.
// Between solves the cached assignment is re-applied to the current frame's
// colours so device outputs still follow live content without re-solving.
//
// This differs fundamentally from the old Hungarian assigner: the matching
// target is a smoothed trajectory (stable over time) rather than the last
// transition-stack output (noisy, drifts every frame). No stickiness penalty
// is needed because the stable trajectory already acts as a natural inertia.
type flowTrackAssigner struct {
	mu             sync.Mutex
	trajectories   map[string]lights.Color // per-device EMA colour trajectory
	lastAssignment []int                   // lastAssignment[colorIdx] = deviceIdx
	lastSolveAt    time.Time
}

func newFlowTrack() *flowTrackAssigner {
	return &flowTrackAssigner{
		trajectories: make(map[string]lights.Color),
	}
}

func (f *flowTrackAssigner) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.trajectories = make(map[string]lights.Color)
	f.lastAssignment = nil
	f.lastSolveAt = time.Time{}
}

func (f *flowTrackAssigner) Assign(
	colors []lights.Color,
	deviceIDs []string,
	_ map[string]lights.Color,
	cfg store.ScreenSyncConfig,
	isCut bool,
) map[string]lights.Color {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(deviceIDs) == 0 || len(colors) == 0 {
		return map[string]lights.Color{}
	}

	if cfg.ColorMode == store.ColorModeSingle || len(colors) == 1 {
		result := make(map[string]lights.Color, len(deviceIDs))
		for _, id := range deviceIDs {
			result[id] = colors[0]
			f.trajectories[id] = colors[0]
		}
		return result
	}

	// On cut or first frame, reset to positional and restart trajectories.
	if isCut || len(f.trajectories) == 0 {
		return f.resetPositional(colors, deviceIDs)
	}

	// Bootstrap trajectories for any devices added after first frame.
	for i, id := range deviceIDs {
		if _, ok := f.trajectories[id]; !ok {
			c := colors[0]
			if i < len(colors) {
				c = colors[i]
			}
			f.trajectories[id] = c
		}
	}

	solveIntervalMs := cfg.FlowTrackSolveIntervalMs
	if solveIntervalMs <= 0 {
		solveIntervalMs = 33
	}

	now := time.Now()
	needsSolve := f.lastAssignment == nil ||
		now.Sub(f.lastSolveAt) >= time.Duration(solveIntervalMs)*time.Millisecond

	alpha := cfg.FlowTrackEmaAlpha
	if alpha <= 0 || alpha > 1 {
		alpha = 0.25
	}

	if !needsSolve {
		// Re-apply the cached assignment to current frame colours so the device
		// output tracks live content between solves.
		result := applyAssignment(f.lastAssignment, colors, deviceIDs)
		f.evolveTrajectories(colors, deviceIDs, f.lastAssignment, alpha)
		return result
	}

	nc := len(colors)
	nd := len(deviceIDs)

	// Build cost: distance from each incoming colour to each device trajectory.
	costs := make([][]float64, nc)
	for i, c := range colors {
		costs[i] = make([]float64, nd)
		for j, id := range deviceIDs {
			costs[i][j] = colorDist(c, f.trajectories[id])
		}
	}

	f.lastAssignment = hungarianSolve(costs, nc, nd)
	f.lastSolveAt = now

	result := applyAssignment(f.lastAssignment, colors, deviceIDs)
	f.evolveTrajectories(colors, deviceIDs, f.lastAssignment, alpha)
	return result
}

// evolveTrajectories advances each device trajectory toward its assigned colour.
func (f *flowTrackAssigner) evolveTrajectories(
	colors []lights.Color,
	deviceIDs []string,
	assignment []int,
	alpha float64,
) {
	for colorIdx, devIdx := range assignment {
		if colorIdx < len(colors) && devIdx < len(deviceIDs) {
			id := deviceIDs[devIdx]
			f.trajectories[id] = lerpColor(f.trajectories[id], colors[colorIdx], alpha)
		}
	}
}

// InitFromAssignment syncs trajectories to a given assignment result and
// records it as the current cached solve. Used by SceneCutRemap to hand off
// state after a cut remap.
func (f *flowTrackAssigner) InitFromAssignment(colors []lights.Color, deviceIDs []string, assignment []int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for colorIdx, devIdx := range assignment {
		if colorIdx < len(colors) && devIdx < len(deviceIDs) {
			id := deviceIDs[devIdx]
			f.trajectories[id] = colors[colorIdx]
		}
	}
	f.lastAssignment = assignment
	f.lastSolveAt = time.Now()
}

func (f *flowTrackAssigner) resetPositional(colors []lights.Color, deviceIDs []string) map[string]lights.Color {
	result := make(map[string]lights.Color, len(deviceIDs))

	// Build positional assignment: assignment[colorIdx] = colorIdx (identity).
	// Only covers colors, not devices — applyAssignment handles the excess.
	assignment := make([]int, len(colors))
	for i := range assignment {
		assignment[i] = i
	}

	for i, id := range deviceIDs {
		c := colors[0]
		if i < len(colors) {
			c = colors[i]
		}
		result[id] = c
		f.trajectories[id] = c
	}
	f.lastAssignment = assignment
	f.lastSolveAt = time.Now()
	return result
}
