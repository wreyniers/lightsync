package assign

import (
	"sync"

	"lightsync/internal/lights"
	"lightsync/internal/store"
)

// identityLockAssigner anchors each device to a stable color reference.
// Each frame the globally-optimal assignment to those anchors is solved.
// Anchors only advance when a color has shifted substantially, preventing
// micro-fluctuations from drifting the cost matrix and flipping assignments.
// On a scene cut the anchors are reset to the fresh positional colours.
type identityLockAssigner struct {
	mu      sync.Mutex
	anchors map[string]lights.Color
}

func newIdentityLock() *identityLockAssigner {
	return &identityLockAssigner{anchors: make(map[string]lights.Color)}
}

func (a *identityLockAssigner) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.anchors = make(map[string]lights.Color)
}

func (a *identityLockAssigner) Assign(
	colors []lights.Color,
	deviceIDs []string,
	_ map[string]lights.Color,
	cfg store.ScreenSyncConfig,
	isCut bool,
) map[string]lights.Color {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(deviceIDs) == 0 || len(colors) == 0 {
		return map[string]lights.Color{}
	}

	if cfg.ColorMode == store.ColorModeSingle || len(colors) == 1 {
		result := make(map[string]lights.Color, len(deviceIDs))
		for _, id := range deviceIDs {
			result[id] = colors[0]
			a.anchors[id] = colors[0]
		}
		return result
	}

	// On a scene cut or first call, reset anchors from positional colours.
	if isCut || len(a.anchors) == 0 {
		return a.resetPositional(colors, deviceIDs)
	}

	// Bootstrap any devices added after the first frame.
	for i, id := range deviceIDs {
		if _, ok := a.anchors[id]; !ok {
			c := colors[0]
			if i < len(colors) {
				c = colors[i]
			}
			a.anchors[id] = c
		}
	}

	nc := len(colors)
	nd := len(deviceIDs)

	// Build cost matrix: distance from each incoming colour to each device anchor.
	costs := make([][]float64, nc)
	for i, c := range colors {
		costs[i] = make([]float64, nd)
		for j, id := range deviceIDs {
			costs[i][j] = colorDist(c, a.anchors[id])
		}
	}

	assignment := hungarianSolve(costs, nc, nd)
	result := applyAssignment(assignment, colors, deviceIDs)

	// Advance an anchor only when the assigned colour has shifted substantially.
	// This prevents smooth gradients from slowly rotating the cost matrix until
	// it flips — the most common cause of gradual colour swapping.
	breachThreshold := cfg.IdentityLockBreachThreshold
	if breachThreshold <= 0 {
		breachThreshold = 0.30
	}
	for colorIdx, devIdx := range assignment {
		if colorIdx < nc && devIdx < nd {
			id := deviceIDs[devIdx]
			c := colors[colorIdx]
			if colorDist(c, a.anchors[id]) > breachThreshold {
				a.anchors[id] = c
			}
		}
	}

	return result
}

func (a *identityLockAssigner) resetPositional(colors []lights.Color, deviceIDs []string) map[string]lights.Color {
	result := make(map[string]lights.Color, len(deviceIDs))
	for i, id := range deviceIDs {
		c := colors[0]
		if i < len(colors) {
			c = colors[i]
		}
		result[id] = c
		a.anchors[id] = c
	}
	return result
}
