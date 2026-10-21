package selftest

import (
	"image"
	"math"
	"sort"
	"sync"
	"time"

	"lightsync/internal/lights"
	"lightsync/internal/screensync/assign"
	"lightsync/internal/screensync/extract"
	"lightsync/internal/screensync/process"
	"lightsync/internal/store"
)

// The per-cell stateful pipeline. One instance per mode × fixture so the
// smoother's EMA, the assigner's tracking state and the handoff blender's
// ramps all converge realistically as frames come in — exactly like they do
// inside screensync.Engine.
type cellState struct {
	cfg       store.ScreenSyncConfig
	extractor extract.Extractor
	smoother  *process.TemporalSmoother
	assigner  assign.Assigner
	blender   *process.HandoffBlender

	// prevOutput is fed back into Assign() each frame so tracking-capable
	// assigners (flow_track etc.) have continuity.
	prevOutput map[string]lights.Color

	// lastTarget / lastActive are the most recent target (pre-handoff) and
	// active (post-handoff) colour lists, stored so the UI can read them
	// without re-running Step().
	lastTarget []lights.Color
	lastActive []lights.Color

	// lockedFrames counts consecutive frames whose active output has landed
	// within alignment tolerance of expected. Used to report "reached".
	lockedFrames int
	lockedAt     time.Time // zero until lockedFrames first hits the threshold

	// activeHistory holds recent post-handoff active colours per frame for
	// calibration stability scoring (ring buffer, max stabilityRingCap).
	activeHistory [][]lights.Color
}

const stabilityRingCap = 8

// StepResult is what Session.Step() produces for one mode cell on one frame.
type StepResult struct {
	ModeID    string         `json:"modeId"`
	Target    []lights.Color `json:"target"`
	Active    []lights.Color `json:"active"`
	// Expected is mirrored from the fixture for convenience so the frontend
	// doesn't have to zip two maps together.
	Expected []lights.Color `json:"expected,omitempty"`
	// Locked is true once Active has been within alignment tolerance of
	// Expected for lockThreshold consecutive frames.
	Locked bool `json:"locked"`
	// LockedSinceMs is how long the lock has been held (0 when not locked).
	LockedSinceMs int64 `json:"lockedSinceMs"`
}

// alignment tolerances (match the SelfTestPage UI defaults).
const (
	alignHueDeg    = 15.0
	alignSatAbs    = 0.15
	alignBrightAbs = 0.15
	// lockThreshold — a cell has "reached" target once its active side
	// satisfies alignment for this many consecutive frames in a row. At 1 Hz
	// that's 2 seconds of stable match, enough to rule out a transient
	// passthrough during a fade.
	lockThreshold = 2
)

// Session holds one stateful pipeline per mode cell. Create it once per
// fixture, call Step() for each captured frame, reset when the fixture
// changes.
type Session struct {
	mu        sync.Mutex
	fixtureID string
	expected  map[string][]lights.Color
	modes     []ModeCase
	cells     map[string]*cellState
	startedAt time.Time
	frameIdx  int
}

// NewSession returns a fresh session bound to a fixture. expected is the
// fixture's Expected map; nil for "unknown" custom fixtures.
func NewSession(fixtureID string, modes []ModeCase, expected map[string][]lights.Color) *Session {
	s := &Session{
		fixtureID: fixtureID,
		expected:  expected,
		modes:     modes,
		cells:     make(map[string]*cellState, len(modes)),
		startedAt: time.Now(),
	}
	for _, m := range modes {
		s.cells[m.ID] = &cellState{
			cfg:       m.Config,
			extractor: extract.New(m.Config),
			smoother:  process.NewTemporalSmoother(),
			assigner:  assign.New(m.Config),
			blender:   process.NewHandoffBlender(),
		}
	}
	return s
}

// FixtureID reports the fixture this session is bound to.
func (s *Session) FixtureID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fixtureID
}

// FrameIndex returns how many Step() calls have landed on this session.
func (s *Session) FrameIndex() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.frameIdx
}

// ElapsedMs returns wall-clock ms since NewSession.
func (s *Session) ElapsedMs() int64 {
	return time.Since(s.startedAt).Milliseconds()
}

// Progress returns (lockedCount, cellsWithExpected). Used by the suite
// orchestrator to decide when a fixture has settled enough to move on.
func (s *Session) Progress() (locked, total int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.modes {
		exp := s.expected[m.ID]
		if len(exp) == 0 {
			continue
		}
		total++
		cell := s.cells[m.ID]
		if cell != nil && cell.lockedFrames >= lockThreshold {
			locked++
		}
	}
	return locked, total
}

// AllExpectedLocked reports whether every cell that has a baseline has
// reached its lock threshold. Cells with no baseline (custom fixtures,
// modes we haven't calibrated yet) are ignored.
func (s *Session) AllExpectedLocked() bool {
	locked, total := s.Progress()
	return total > 0 && locked == total
}

// Step runs one captured frame through every cell's pipeline and returns
// per-mode target + active colours.
func (s *Session) Step(img image.Image) []StepResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.frameIdx++

	out := make([]StepResult, 0, len(s.modes))
	for _, m := range s.modes {
		cell := s.cells[m.ID]
		target, active := runPipeline(cell, img)
		cell.pushActiveHistory(active)

		expected := cloneColors(s.expected[m.ID])
		locked, lockedSinceMs := updateLock(cell, active, expected)

		out = append(out, StepResult{
			ModeID:        m.ID,
			Target:        target,
			Active:        active,
			Expected:      expected,
			Locked:        locked,
			LockedSinceMs: lockedSinceMs,
		})
	}
	return out
}

// runPipeline is the per-frame sequence that mirrors engine.run():
// Extract → ApplyAdjustments → Smooth → Assign → HandoffBlend.
// Returns (target, active): target is pre-handoff, active is post-handoff.
func runPipeline(c *cellState, img image.Image) (target, active []lights.Color) {
	raw := c.extractor.Extract(img, c.cfg)
	adjusted := process.ApplyAdjustments(raw, c.cfg)

	// Smoother. We pass isCut=false — the Self-Test never emulates scene
	// cuts; it wants the full smoothing behaviour.
	smoothed := c.smoother.Smooth(
		adjusted,
		false,
		c.cfg.ColorSmoothing,
		c.cfg.BrightnessSmoothing,
		c.cfg.BrightnessMaxDeviation,
		c.cfg.BrightnessFloor,
		c.cfg.BrightnessCeiling,
	)

	// Assign — map palette/spatial colours onto device IDs.
	current := c.prevOutput
	if current == nil {
		current = map[string]lights.Color{}
	}
	assigned := c.assigner.Assign(smoothed, c.cfg.DeviceIDs, current, c.cfg, false)

	// Handoff blender — this is where the "fade between colours" lives.
	blended := c.blender.Blend(assigned, c.cfg.AssignmentHandoffMs)
	c.prevOutput = blended

	target = flatten(assigned, c.cfg.DeviceIDs)
	active = flatten(blended, c.cfg.DeviceIDs)

	// Store for future non-stepping reads (diagnostics dump).
	c.lastTarget = target
	c.lastActive = active
	return target, active
}

// updateLock inspects whether this frame's active output satisfies the
// alignment tolerance against expected, and updates the consecutive-frame
// lock counter accordingly.
func updateLock(cell *cellState, active, expected []lights.Color) (locked bool, lockedSinceMs int64) {
	if len(expected) == 0 || len(active) == 0 {
		cell.lockedFrames = 0
		cell.lockedAt = time.Time{}
		return false, 0
	}

	if _, ok := MatchColors(expected, active, cell.cfg); ok {
		cell.lockedFrames++
		if cell.lockedFrames == lockThreshold {
			cell.lockedAt = time.Now()
		}
	} else {
		cell.lockedFrames = 0
		cell.lockedAt = time.Time{}
	}

	if cell.lockedFrames >= lockThreshold {
		return true, time.Since(cell.lockedAt).Milliseconds()
	}
	return false, 0
}

// MatchColors compares active against expected and returns (matching, ok).
//
//   - matching[i] = the active-slot index that best pairs with expected[i].
//     -1 means no suitable pair exists for that expected slot.
//   - ok        = true when every expected slot has a match within tolerance.
//
// For scene-palette mode the match is permutation-aware: the ranking of
// palette slots is weight-based and slides around when two families have
// near-equal weight, so an out-of-order result is not a bug. For every
// other mode (single colour, spatial grid) the match is strictly index-
// aligned — spatial position carries meaning and permuting it would hide
// real position bugs.
func MatchColors(expected, active []lights.Color, cfg store.ScreenSyncConfig) ([]int, bool) {
	if len(expected) == 0 || len(active) == 0 {
		return nil, false
	}
	if allowPermutation(cfg) {
		return greedyPermutationMatch(expected, active)
	}
	return strictMatch(expected, active)
}

// allowPermutation returns true when the mode's slot ordering is unstable
// and a slot-permutation match is appropriate. Only scene-palette mode
// qualifies — single-colour is trivially ordered (one slot) and spatial-
// grid slot N always corresponds to zone N.
func allowPermutation(cfg store.ScreenSyncConfig) bool {
	return cfg.ColorMode == store.ColorModeMulti &&
		cfg.MultiColorApproach == store.MultiColorScenePalette
}

func strictMatch(expected, active []lights.Color) ([]int, bool) {
	n := len(expected)
	matching := make([]int, n)
	ok := len(active) == n
	for i := range matching {
		matching[i] = -1
		if i < len(active) {
			matching[i] = i
			if !oneAligned(expected[i], active[i]) {
				ok = false
			}
		}
	}
	return matching, ok
}

// greedyPermutationMatch pairs each expected entry with the unused active
// entry closest to it in perceptual distance, then reports ok=true when
// every matched pair satisfies the alignment tolerance.
//
// Greedy (not Hungarian-optimal) because N ≤ 5 in practice and the cost
// of a sub-optimal match in the rare near-degenerate case is a single
// missed lock, not a correctness hazard.
func greedyPermutationMatch(expected, active []lights.Color) ([]int, bool) {
	n := len(expected)
	if len(active) != n || n == 0 {
		return nil, false
	}
	// Optimal assignment for the small palettes we ship (N≤6). Greedy pairing
	// mis-assigns when several swatches are close in distance (e.g. multiple
	// blues) and falsely reports huge hue errors.
	if n <= 6 {
		return optimalPermutationMatch(expected, active)
	}
	matching := make([]int, n)
	used := make([]bool, len(active))
	for i := range matching {
		matching[i] = -1
	}
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		return minPerceptualDistance(expected[order[a]], active) <
			minPerceptualDistance(expected[order[b]], active)
	})
	ok := true
	for _, i := range order {
		best := -1
		bestDist := math.Inf(1)
		for j, a := range active {
			if used[j] {
				continue
			}
			d := perceptualDistance(expected[i], a)
			if d < bestDist {
				bestDist = d
				best = j
			}
		}
		if best < 0 {
			ok = false
			continue
		}
		matching[i] = best
		used[best] = true
		if !oneAligned(expected[i], active[best]) {
			ok = false
		}
	}
	return matching, ok
}

// optimalPermutationMatch assigns each expected[i] to a distinct active[j]
// minimising sum perceptualDistance. N! is tiny for self-test (4! = 24).
func optimalPermutationMatch(expected, active []lights.Color) ([]int, bool) {
	n := len(expected)
	matching := make([]int, n)
	best := make([]int, n)
	bestCost := math.Inf(1)

	cur := make([]int, n)
	var dfs func(pos int, used []bool, cost float64)
	dfs = func(pos int, used []bool, cost float64) {
		if cost >= bestCost {
			return
		}
		if pos == n {
			bestCost = cost
			copy(best, cur)
			return
		}
		for j := 0; j < n; j++ {
			if used[j] {
				continue
			}
			cur[pos] = j
			used[j] = true
			dfs(pos+1, used, cost+perceptualDistance(expected[pos], active[j]))
			used[j] = false
		}
	}
	dfs(0, make([]bool, n), 0)

	copy(matching, best)
	ok := true
	for i := 0; i < n; i++ {
		if !oneAligned(expected[i], active[matching[i]]) {
			ok = false
		}
	}
	return matching, ok
}

// perceptualDistance is a cheap perceptual proxy (hue-weighted Euclidean
// in HSB space). Used only by greedy matching — not a substitute for the
// alignment tolerances, which decide pass/fail.
func perceptualDistance(a, b lights.Color) float64 {
	// Ignore hue when either side is essentially grey — it would otherwise
	// pick a random pairing for neutrals.
	if a.S < 0.08 && b.S < 0.08 {
		ds := a.S - b.S
		db := a.B - b.B
		return ds*ds + db*db
	}
	dh := a.H - b.H
	if dh > 180 {
		dh -= 360
	}
	if dh < -180 {
		dh += 360
	}
	dhNorm := dh / 180.0
	ds := a.S - b.S
	db := a.B - b.B
	return dhNorm*dhNorm*0.65 + ds*ds*0.20 + db*db*0.15
}

func minPerceptualDistance(x lights.Color, pool []lights.Color) float64 {
	best := math.Inf(1)
	for _, p := range pool {
		if d := perceptualDistance(x, p); d < best {
			best = d
		}
	}
	return best
}

func oneAligned(a, b lights.Color) bool {
	// When both sides are near-grey the hue is meaningless.
	grey := a.S < 0.08 && b.S < 0.08
	if !grey {
		dh := math.Abs(a.H - b.H)
		if dh > 180 {
			dh = 360 - dh
		}
		if dh > alignHueDeg {
			return false
		}
	}
	return math.Abs(a.S-b.S) <= alignSatAbs && math.Abs(a.B-b.B) <= alignBrightAbs
}

func flatten(m map[string]lights.Color, ids []string) []lights.Color {
	out := make([]lights.Color, 0, len(ids))
	for _, id := range ids {
		if c, ok := m[id]; ok {
			out = append(out, c)
		}
	}
	return out
}

func cloneColors(in []lights.Color) []lights.Color {
	if len(in) == 0 {
		return nil
	}
	out := make([]lights.Color, len(in))
	copy(out, in)
	return out
}

func (c *cellState) pushActiveHistory(active []lights.Color) {
	if len(active) == 0 {
		return
	}
	snap := cloneColors(active)
	if len(c.activeHistory) < stabilityRingCap {
		c.activeHistory = append(c.activeHistory, snap)
		return
	}
	c.activeHistory = append(c.activeHistory[1:], snap)
}

// activeHistoryJitter returns a 0..~1 score: higher means more frame-to-frame
// variation in active output for this cell.
func activeHistoryJitter(h [][]lights.Color) float64 {
	if len(h) < 3 {
		return 0
	}
	nSlots := len(h[0])
	if nSlots == 0 {
		return 0
	}
	var sum float64
	count := 0
	for slot := 0; slot < nSlots; slot++ {
		var hs, ss, bs []float64
		for _, frame := range h {
			if slot >= len(frame) {
				continue
			}
			hs = append(hs, frame[slot].H)
			ss = append(ss, frame[slot].S)
			bs = append(bs, frame[slot].B)
		}
		if len(ss) < 3 {
			continue
		}
		sum += variance(ss) + variance(bs) + circularHueDispersion(hs)
		count++
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

func variance(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	var m float64
	for _, x := range xs {
		m += x
	}
	m /= float64(len(xs))
	var acc float64
	for _, x := range xs {
		d := x - m
		acc += d * d
	}
	return acc / float64(len(xs))
}

// circularHueDispersion returns 1−R where R is mean resultant length (0..1);
// 0 = identical hues, higher = more spread.
func circularHueDispersion(hs []float64) float64 {
	if len(hs) < 2 {
		return 0
	}
	var s, c float64
	for _, h := range hs {
		r := h * math.Pi / 180
		s += math.Sin(r)
		c += math.Cos(r)
	}
	n := float64(len(hs))
	R := math.Sqrt(s*s+c*c) / n
	if R > 1 {
		R = 1
	}
	return 1 - R
}
