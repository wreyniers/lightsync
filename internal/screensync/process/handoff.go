package process

import (
	"math"
	"sync"
	"time"

	"lightsync/internal/lights"
)

// HandoffBlender applies a short per-device ramp after assignment changes so
// colour-slot swaps fade smoothly instead of snapping. The engine runs it
// after Assign and before SetState; the Self-Test harness uses it to
// reproduce the engine's temporal behaviour one frame at a time.
type HandoffBlender struct {
	mu     sync.Mutex
	states map[string]handoffState
}

type handoffState struct {
	from     lights.Color
	to       lights.Color
	started  time.Time
	duration time.Duration
}

// NewHandoffBlender returns an empty blender.
func NewHandoffBlender() *HandoffBlender {
	return &HandoffBlender{states: make(map[string]handoffState)}
}

// Reset discards all state — useful on engine Start or a fixture change.
func (b *HandoffBlender) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.states = make(map[string]handoffState)
}

// Snap seeds the blender with settled states so the next Blend call starts
// from the current visible output. Used on scene cuts to preserve instant
// snaps.
func (b *HandoffBlender) Snap(colors map[string]lights.Color) {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	b.states = make(map[string]handoffState, len(colors))
	for id, c := range colors {
		b.states[id] = handoffState{
			from:     c,
			to:       c,
			started:  now,
			duration: 0,
		}
	}
}

// Blend applies per-device colour handoff ramping toward target.
// handoffMs <= 0 disables blending for this frame.
func (b *HandoffBlender) Blend(target map[string]lights.Color, handoffMs int) map[string]lights.Color {
	b.mu.Lock()
	defer b.mu.Unlock()

	out := make(map[string]lights.Color, len(target))
	now := time.Now()

	if handoffMs <= 0 {
		b.states = make(map[string]handoffState, len(target))
		for id, c := range target {
			out[id] = c
			b.states[id] = handoffState{from: c, to: c, started: now}
		}
		return out
	}

	if handoffMs > 3000 {
		handoffMs = 3000
	}
	duration := time.Duration(handoffMs) * time.Millisecond

	nextStates := make(map[string]handoffState, len(target))
	for id, nextTarget := range target {
		st, ok := b.states[id]
		if !ok {
			st = handoffState{
				from:     nextTarget,
				to:       nextTarget,
				started:  now,
				duration: duration,
			}
			out[id] = nextTarget
			nextStates[id] = st
			continue
		}

		// If target changed meaningfully, restart the handoff from the currently
		// visible blended value so transitions stay continuous.
		if handoffHSBDelta(st.to, nextTarget) > 0.01 {
			current := handoffValueAt(st, now)
			st = handoffState{
				from:     current,
				to:       nextTarget,
				started:  now,
				duration: duration,
			}
		} else {
			st.duration = duration
		}

		value := handoffValueAt(st, now)
		out[id] = value

		// Collapse settled states to keep precision stable.
		if handoffProgress(st, now) >= 1.0 {
			st.from = st.to
		}
		nextStates[id] = st
	}

	b.states = nextStates
	return out
}

func handoffValueAt(st handoffState, now time.Time) lights.Color {
	p := handoffProgress(st, now)
	if p <= 0 {
		return st.from
	}
	if p >= 1 {
		return st.to
	}
	// Ease-in-out for less robotic linear fades.
	t := p * p * (3.0 - 2.0*p)
	return lerpHandoffHSB(st.from, st.to, t)
}

func handoffProgress(st handoffState, now time.Time) float64 {
	if st.duration <= 0 {
		return 1
	}
	p := now.Sub(st.started).Seconds() / st.duration.Seconds()
	if p < 0 {
		return 0
	}
	if p > 1 {
		return 1
	}
	return p
}

func lerpHandoffHSB(a, b lights.Color, t float64) lights.Color {
	dh := b.H - a.H
	for dh > 180 {
		dh -= 360
	}
	for dh < -180 {
		dh += 360
	}

	h := a.H + dh*t
	for h < 0 {
		h += 360
	}
	for h >= 360 {
		h -= 360
	}

	return lights.Color{
		H: h,
		S: a.S + (b.S-a.S)*t,
		B: a.B + (b.B-a.B)*t,
	}
}

func handoffHSBDelta(a, b lights.Color) float64 {
	dh := b.H - a.H
	for dh > 180 {
		dh -= 360
	}
	for dh < -180 {
		dh += 360
	}
	hNorm := math.Abs(dh) / 180.0
	ds := math.Abs(b.S - a.S)
	db := math.Abs(b.B - a.B)
	return math.Sqrt(0.65*hNorm*hNorm + 0.20*ds*ds + 0.15*db*db)
}
