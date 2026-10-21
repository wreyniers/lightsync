package selftest

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"lightsync/internal/lights"
	"lightsync/internal/store"
)

// Diagnostic is a self-contained, copy-pasteable record of the self-test
// run. It captures every piece of context needed to diagnose a colour
// mismatch offline: fixture, config, expected values, the captured active/
// target histories for every cell, and a human-readable delta summary.
type Diagnostic struct {
	GeneratedAt time.Time          `json:"generatedAt"`
	Fixture     string             `json:"fixture"`
	ElapsedMs   int64              `json:"elapsedMs"`
	FrameCount  int                `json:"frameCount"`
	Cells       []DiagnosticCell   `json:"cells"`
}

// DiagnosticCell captures the current end-state of one mode cell plus
// whichever alignment deltas exist against expected.
type DiagnosticCell struct {
	ModeID        string                 `json:"modeId"`
	ModeLabel     string                 `json:"modeLabel"`
	Config        store.ScreenSyncConfig `json:"config"`
	Expected      []lights.Color         `json:"expected,omitempty"`
	Target        []lights.Color         `json:"target"`
	Active        []lights.Color         `json:"active"`
	Locked        bool                   `json:"locked"`
	LockedSinceMs int64                  `json:"lockedSinceMs"`
	Delta         *DiagnosticDelta       `json:"delta,omitempty"`
	Verdict       string                 `json:"verdict"` // "locked", "drifting", "wrong-hue", ...
	// StabilityJitter is recent frame-to-frame active colour variance (0..~1).
	StabilityJitter float64 `json:"stabilityJitter,omitempty"`
}

// DiagnosticDelta summarises |active − expected| per colour, and the worst
// offender across the slots, so an LLM or human can scan one line per cell.
//
// When the mode permits permutation (scene palette) the matching reported
// here pairs each expected slot with the closest available active slot,
// not the slot at the same index. That's what makes "locked" cells show
// small deltas even when the palette rank shuffled.
type DiagnosticDelta struct {
	// PerSlot holds one entry per expected slot. Slot.Matched tells you
	// which active index was paired to that expected entry; it equals
	// Slot.Slot for ordered modes and can differ for palette.
	PerSlot     []SlotDelta `json:"perSlot"`
	WorstSlot   int         `json:"worstSlot"`
	WorstHue    float64     `json:"worstHueDeg"`
	WorstSat    float64     `json:"worstSatAbs"`
	WorstBright float64     `json:"worstBrightAbs"`
	// Permuted is true when any expected slot was paired with a non-same-
	// index active slot, indicating palette rank reshuffling. Harmless for
	// palette mode; a real bug if it ever appears in ordered modes.
	Permuted bool `json:"permuted,omitempty"`
}

// SlotDelta is a single expected-vs-active comparison.
type SlotDelta struct {
	// Slot is the expected index.
	Slot int `json:"slot"`
	// Matched is the active index this expected slot was paired with.
	// -1 means no suitable pair was available.
	Matched         int     `json:"matched"`
	HueDeg          float64 `json:"hueDeg"` // shortest-arc signed
	SatAbs          float64 `json:"satAbs"`
	BrightAbs       float64 `json:"brightAbs"`
	WithinTolerance bool    `json:"withinTolerance"`
}

// BuildDiagnostic snapshots the current state of the session. Safe to call
// at any point; normal use is when the user clicks "Copy Diagnostics"
// because something didn't lock in a reasonable time.
func (s *Session) BuildDiagnostic() Diagnostic {
	s.mu.Lock()
	defer s.mu.Unlock()

	d := Diagnostic{
		GeneratedAt: time.Now().UTC(),
		Fixture:     s.fixtureID,
		ElapsedMs:   time.Since(s.startedAt).Milliseconds(),
		FrameCount:  s.frameIdx,
	}

	// Keep cell order stable — sort by mode ID so the diagnostic reads the
	// same every time for the same fixture.
	ids := make([]string, 0, len(s.modes))
	for _, m := range s.modes {
		ids = append(ids, m.ID)
	}
	sort.Strings(ids)

	modeByID := make(map[string]ModeCase, len(s.modes))
	for _, m := range s.modes {
		modeByID[m.ID] = m
	}

	for _, id := range ids {
		cell := s.cells[id]
		if cell == nil {
			continue
		}
		m := modeByID[id]
		expected := cloneColors(s.expected[id])

		dc := DiagnosticCell{
			ModeID:          id,
			ModeLabel:       m.Label,
			Config:          cell.cfg,
			Expected:        expected,
			Target:          cloneColors(cell.lastTarget),
			Active:          cloneColors(cell.lastActive),
			Locked:          cell.lockedFrames >= lockThreshold,
			LockedSinceMs:   0,
			StabilityJitter: activeHistoryJitter(cell.activeHistory),
		}
		if !cell.lockedAt.IsZero() {
			dc.LockedSinceMs = time.Since(cell.lockedAt).Milliseconds()
		}
		if len(expected) > 0 && len(cell.lastActive) > 0 {
			delta := computeDelta(expected, cell.lastActive, cell.cfg)
			dc.Delta = &delta
		}
		dc.Verdict = verdict(dc)
		d.Cells = append(d.Cells, dc)
	}

	return d
}

// AsText returns the diagnostic as a plain-text blob safe to paste into a
// chat window. It includes both a scannable summary and the full JSON so
// an LLM reader has both surfaces.
func (d Diagnostic) AsText() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# LightSync Self-Test Diagnostic\n")
	fmt.Fprintf(&b, "Generated : %s\n", d.GeneratedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "Fixture   : %s\n", d.Fixture)
	fmt.Fprintf(&b, "Elapsed   : %d ms across %d frames\n", d.ElapsedMs, d.FrameCount)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Summary (one row per mode cell)")
	fmt.Fprintln(&b, "```")
	fmt.Fprintf(&b, "%-32s %-12s %-14s %s\n", "mode", "verdict", "lock age", "worst slot delta")
	for _, c := range d.Cells {
		lock := "-"
		if c.Locked {
			lock = fmt.Sprintf("%dms", c.LockedSinceMs)
		}
		worst := "n/a"
		if c.Delta != nil {
			permTag := ""
			if c.Delta.Permuted {
				permTag = " (permuted)"
			}
			worst = fmt.Sprintf("#%d H:%+.1f° S:%+.2f B:%+.2f%s",
				c.Delta.WorstSlot, c.Delta.WorstHue, c.Delta.WorstSat, c.Delta.WorstBright, permTag)
		}
		fmt.Fprintf(&b, "%-32s %-12s %-14s %s\n", c.ModeID, c.Verdict, lock, worst)
	}
	fmt.Fprintln(&b, "```")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Raw data (JSON)")
	fmt.Fprintln(&b, "```json")
	enc, err := json.MarshalIndent(d, "", "  ")
	if err == nil {
		b.Write(enc)
	} else {
		fmt.Fprintf(&b, "(marshal error: %v)", err)
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "```")
	return b.String()
}

func computeDelta(expected, active []lights.Color, cfg store.ScreenSyncConfig) DiagnosticDelta {
	// Use the same matching rule the lock detector uses — pair expected
	// with closest active (palette) or same-index active (single/spatial).
	matching, _ := MatchColors(expected, active, cfg)

	d := DiagnosticDelta{PerSlot: make([]SlotDelta, 0, len(expected))}
	worstScore := -1.0

	for i, exp := range expected {
		matched := -1
		if i < len(matching) {
			matched = matching[i]
		}
		slot := SlotDelta{
			Slot:    i,
			Matched: matched,
		}
		if matched < 0 || matched >= len(active) {
			slot.WithinTolerance = false
			d.PerSlot = append(d.PerSlot, slot)
			continue
		}
		a := active[matched]

		dh := exp.H - a.H
		for dh > 180 {
			dh -= 360
		}
		for dh < -180 {
			dh += 360
		}
		hueAbs := dh
		if hueAbs < 0 {
			hueAbs = -hueAbs
		}
		ds := exp.S - a.S
		db := exp.B - a.B

		slot.HueDeg = dh
		slot.SatAbs = ds
		slot.BrightAbs = db
		// If either side is essentially grey, ignore hue for tolerance —
		// same rule as oneAligned.
		greyish := exp.S < 0.08 && a.S < 0.08
		slot.WithinTolerance = (greyish || hueAbs <= alignHueDeg) &&
			abs(ds) <= alignSatAbs && abs(db) <= alignBrightAbs

		if matched != i {
			d.Permuted = true
		}

		// Worst-offender tracking — perceptually weighted.
		huePenalty := hueAbs / 180
		if greyish {
			huePenalty = 0
		}
		score := huePenalty + abs(ds)*0.5 + abs(db)*0.5
		if score > worstScore {
			worstScore = score
			d.WorstSlot = i
			d.WorstHue = dh
			d.WorstSat = ds
			d.WorstBright = db
		}
		d.PerSlot = append(d.PerSlot, slot)
	}
	return d
}

// verdict assigns a one-word label so the summary table reads at a glance.
func verdict(c DiagnosticCell) string {
	if c.Locked {
		return "locked"
	}
	if len(c.Expected) == 0 {
		// No known-good baseline for this (fixture, mode) combo — can't
		// pass/fail, just report what was produced.
		return "no-baseline"
	}
	if len(c.Active) == 0 {
		// Baseline exists but extractor produced nothing. Usually means a
		// plumbing bug upstream of the extractor (e.g. empty DeviceIDs so
		// assigner outputs an empty map).
		return "empty-output"
	}
	if c.Delta == nil {
		return "unknown"
	}
	h := absN(c.Delta.WorstHue)
	s := abs(c.Delta.WorstSat)
	b := abs(c.Delta.WorstBright)
	switch {
	case h > 60:
		return "wrong-hue"
	case h > alignHueDeg && s < alignSatAbs && b < alignBrightAbs:
		return "hue-drift"
	case s > alignSatAbs && h < alignHueDeg:
		return "sat-drift"
	case b > alignBrightAbs && h < alignHueDeg:
		return "brightness-drift"
	default:
		return "drifting"
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
func absN(x float64) float64 { return abs(x) }
