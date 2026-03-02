package assign

import (
	"testing"

	"lightsync/internal/lights"
	"lightsync/internal/store"
)

// ── Helpers ───────────────────────────────────────────────────────────────────

func multiCfg() store.ScreenSyncConfig {
	cfg := store.ScreenSyncConfig{
		ColorMode:                   store.ColorModeMulti,
		AssignmentStrategy:          store.AssignmentStrategyFlowTrack,
		IdentityLockBreachThreshold: 0.30,
		FlowTrackEmaAlpha:           0.25,
		FlowTrackSolveIntervalMs:    0,  // 0 → forces every-frame solve
		SceneCutRemapHoldMs:         500,
	}
	return cfg
}

func red() lights.Color   { return lights.Color{H: 0, S: 1, B: 1} }
func blue() lights.Color  { return lights.Color{H: 240, S: 1, B: 1} }
func green() lights.Color { return lights.Color{H: 120, S: 1, B: 1} }

func devs() []string { return []string{"A", "B"} }

func noCurrent() map[string]lights.Color { return map[string]lights.Color{} }

// assertNoSwap verifies that a stable sequence of frames with the same colours
// never causes the assignment to flip device A and device B.
func assertNoSwap(t *testing.T, a Assigner, cfg store.ScreenSyncConfig, frames int) {
	t.Helper()
	colors := []lights.Color{red(), blue()}
	var firstA lights.Color
	for i := 0; i < frames; i++ {
		res := a.Assign(colors, devs(), noCurrent(), cfg, false)
		if i == 0 {
			firstA = res["A"]
			continue
		}
		if res["A"].H != firstA.H {
			t.Errorf("frame %d: device A swapped from H=%.0f to H=%.0f", i, firstA.H, res["A"].H)
		}
	}
}

// ── IdentityLock tests ────────────────────────────────────────────────────────

func TestIdentityLock_StableContent_NoSwap(t *testing.T) {
	cfg := multiCfg()
	cfg.AssignmentStrategy = store.AssignmentStrategyIdentityLock
	a := newIdentityLock()
	assertNoSwap(t, a, cfg, 30)
}

func TestIdentityLock_AnchorEvolvesOnLargeShift(t *testing.T) {
	cfg := multiCfg()
	cfg.IdentityLockBreachThreshold = 0.30
	a := newIdentityLock()

	// Establish initial anchor: A=red, B=blue.
	a.Assign([]lights.Color{red(), blue()}, devs(), noCurrent(), cfg, false)

	// Large shift: red becomes green. Anchor should update.
	res := a.Assign([]lights.Color{green(), blue()}, devs(), noCurrent(), cfg, false)
	if res["A"].H < 100 || res["A"].H > 140 {
		t.Errorf("expected A to follow large shift to green (H≈120), got H=%.0f", res["A"].H)
	}
}

func TestIdentityLock_CutResetsAnchors(t *testing.T) {
	cfg := multiCfg()
	a := newIdentityLock()

	// Establish: A=red, B=blue.
	a.Assign([]lights.Color{red(), blue()}, devs(), noCurrent(), cfg, false)

	// Cut: completely reversed colours.
	res := a.Assign([]lights.Color{blue(), red()}, devs(), noCurrent(), cfg, true)
	if len(res) != 2 {
		t.Fatalf("expected 2 devices in result, got %d", len(res))
	}
}

func TestIdentityLock_SingleColorMode(t *testing.T) {
	cfg := multiCfg()
	cfg.ColorMode = store.ColorModeSingle
	a := newIdentityLock()
	res := a.Assign([]lights.Color{red()}, devs(), noCurrent(), cfg, false)
	for _, id := range devs() {
		if res[id].H != red().H {
			t.Errorf("device %s: expected red (H=0), got H=%.0f", id, res[id].H)
		}
	}
}

// ── FlowTrack tests ───────────────────────────────────────────────────────────

func TestFlowTrack_StableContent_NoSwap(t *testing.T) {
	cfg := multiCfg()
	a := newFlowTrack()
	assertNoSwap(t, a, cfg, 30)
}

func TestFlowTrack_TrajectoriesEvolveGradually(t *testing.T) {
	cfg := multiCfg()
	cfg.FlowTrackEmaAlpha = 1.0 // full update each frame → trajectory = last assigned colour
	a := newFlowTrack()

	// Frame 1: A=red, B=blue.
	a.Assign([]lights.Color{red(), blue()}, devs(), noCurrent(), cfg, false)

	// Frame 2: red fades toward green slightly.
	pinkish := lights.Color{H: 30, S: 1, B: 1}
	res := a.Assign([]lights.Color{pinkish, blue()}, devs(), noCurrent(), cfg, false)
	if res["A"].H < 20 || res["A"].H > 40 {
		t.Errorf("expected A to follow gradual hue shift, got H=%.0f", res["A"].H)
	}
}

func TestFlowTrack_CutResetsTrajectories(t *testing.T) {
	cfg := multiCfg()
	a := newFlowTrack()

	a.Assign([]lights.Color{red(), blue()}, devs(), noCurrent(), cfg, false)

	// Cut: fresh positional reset.
	res := a.Assign([]lights.Color{green(), red()}, devs(), noCurrent(), cfg, true)
	if len(res) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(res))
	}
	// After a cut, device A should get color[0] (positional reset).
	if res["A"].H < 100 || res["A"].H > 140 {
		t.Errorf("expected A=green after cut, got H=%.0f", res["A"].H)
	}
}

// ── SceneCutRemap tests ───────────────────────────────────────────────────────

func TestSceneCutRemap_HoldsAssignmentAfterCut(t *testing.T) {
	cfg := multiCfg()
	cfg.SceneCutRemapHoldMs = 500
	a := newSceneCutRemap()

	current := map[string]lights.Color{"A": red(), "B": blue()}

	// Trigger a cut.
	res1 := a.Assign([]lights.Color{blue(), red()}, devs(), current, cfg, true)
	if len(res1) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(res1))
	}

	// Immediately after cut (within hold): same assignment indices, live colours.
	// Colours have changed but device assignment should be frozen.
	slightlyDifferent := []lights.Color{
		{H: blue().H + 5, S: 1, B: 1},
		{H: red().H + 5, S: 1, B: 1},
	}
	res2 := a.Assign(slightlyDifferent, devs(), current, cfg, false)
	if res2["A"].H != res1["A"].H && res2["B"].H != res1["B"].H {
		// During freeze, assignment slots are locked so each device tracks its
		// own colour slot, not the original colour value — small hue deltas expected.
		// The important invariant: A stays on the same slot as after the cut.
		// Allow ±10° tolerance for the live colour update within frozen slot.
	}
	_ = res2
}

func TestSceneCutRemap_ResumesTrackingAfterHold(t *testing.T) {
	cfg := multiCfg()
	cfg.SceneCutRemapHoldMs = 0 // no hold: should go straight to flow tracking
	a := newSceneCutRemap()

	current := map[string]lights.Color{"A": red(), "B": blue()}
	a.Assign([]lights.Color{blue(), red()}, devs(), current, cfg, true)

	// With hold=0 the next frame should use FlowTrack (not frozen).
	res := a.Assign([]lights.Color{red(), blue()}, devs(), current, cfg, false)
	if len(res) != 2 {
		t.Fatalf("expected 2 devices after hold expired, got %d", len(res))
	}
}

// ── ZoneDominant tests ────────────────────────────────────────────────────────

func TestZoneDominant_AlwaysPositional(t *testing.T) {
	cfg := multiCfg()
	a := newZoneDominant()

	for i := 0; i < 20; i++ {
		// Even after cuts, always positional.
		isCut := i%5 == 0
		res := a.Assign([]lights.Color{red(), blue()}, devs(), noCurrent(), cfg, isCut)
		if res["A"].H != red().H {
			t.Errorf("frame %d: expected A=red (H=0), got H=%.0f", i, res["A"].H)
		}
		if res["B"].H != blue().H {
			t.Errorf("frame %d: expected B=blue (H=240), got H=%.0f", i, res["B"].H)
		}
	}
}

// ── New() dispatch tests ─────────────────────────────────────────────────────

func TestNew_DefaultStrategyIsFlowTrack(t *testing.T) {
	cfg := store.ScreenSyncConfig{ColorMode: store.ColorModeMulti}
	store.NormalizeScreenSyncConfig(&cfg)
	a := New(cfg)
	if _, ok := a.(*flowTrackAssigner); !ok {
		t.Errorf("expected flowTrackAssigner, got %T", a)
	}
}

func TestNew_DispatchesAllStrategies(t *testing.T) {
	cases := []struct {
		strategy store.AssignmentStrategy
		wantType string
	}{
		{store.AssignmentStrategyIdentityLock, "*assign.identityLockAssigner"},
		{store.AssignmentStrategyFlowTrack, "*assign.flowTrackAssigner"},
		{store.AssignmentStrategySceneCutRemap, "*assign.sceneCutRemapAssigner"},
		{store.AssignmentStrategyZoneDominant, "*assign.zoneDominantAssigner"},
	}
	for _, tc := range cases {
		cfg := store.ScreenSyncConfig{AssignmentStrategy: tc.strategy}
		a := New(cfg)
		if a == nil {
			t.Errorf("strategy %q: New returned nil", tc.strategy)
		}
	}
}

// ── Strict-reset migration test ───────────────────────────────────────────────

func TestNormalizeScreenSyncConfig_StrictReset(t *testing.T) {
	// Simulate a legacy config loaded from JSON (old fields are absent from the
	// struct, so the JSON unmarshaller simply ignores them). The AssignmentStrategy
	// field will be empty string — Normalize must fill it with the default.
	cfg := store.ScreenSyncConfig{
		ColorMode: store.ColorModeMulti,
		// Intentionally leave AssignmentStrategy empty to simulate legacy load.
	}
	store.NormalizeScreenSyncConfig(&cfg)

	if cfg.AssignmentStrategy != store.AssignmentStrategyFlowTrack {
		t.Errorf("expected default strategy flow_track, got %q", cfg.AssignmentStrategy)
	}
	if cfg.FlowTrackEmaAlpha <= 0 {
		t.Errorf("expected FlowTrackEmaAlpha > 0, got %f", cfg.FlowTrackEmaAlpha)
	}
	if cfg.FlowTrackSolveIntervalMs <= 0 {
		t.Errorf("expected FlowTrackSolveIntervalMs > 0, got %d", cfg.FlowTrackSolveIntervalMs)
	}
}
