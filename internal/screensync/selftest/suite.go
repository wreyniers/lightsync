package selftest

import (
	"fmt"
	"strings"
	"time"
)

// FixtureRunResult is the captured end-state for one fixture during a
// suite run. The embedded Diagnostic carries the full per-cell detail so
// the master export contains everything an offline reviewer needs.
type FixtureRunResult struct {
	FixtureID   string     `json:"fixtureId"`
	FixtureLabel string    `json:"fixtureLabel"`
	DurationMs  int64      `json:"durationMs"`
	LockedCells int        `json:"lockedCells"`
	TotalCells  int        `json:"totalCells"` // cells with a baseline
	Diagnostic  Diagnostic `json:"diagnostic"`
}

// SuiteRun is the final output of a full-suite test run.
type SuiteRun struct {
	StartedAt     time.Time          `json:"startedAt"`
	FinishedAt    time.Time          `json:"finishedAt"`
	TotalMs       int64              `json:"totalMs"`
	FixtureCount  int                `json:"fixtureCount"`
	LockedCells   int                `json:"lockedCells"`
	TotalCells    int                `json:"totalCells"`
	Fixtures      []FixtureRunResult `json:"fixtures"`
}

// SuiteProgress is emitted once per fixture-step so the UI can show "3 of
// 11 — photo-lake-blue — settling…" without having to poll.
type SuiteProgress struct {
	FixtureIndex int    `json:"fixtureIndex"` // 0-based, current fixture being processed
	TotalCount   int    `json:"totalCount"`
	FixtureID    string `json:"fixtureId"`
	FixtureLabel string `json:"fixtureLabel"`
	Phase        string `json:"phase"`        // "warming" | "capturing" | "settled"
	ElapsedMs    int64  `json:"elapsedMs"`    // since suite start
}

// AsText renders the suite as a copy-pasteable markdown + JSON blob,
// following the same shape as single-fixture Diagnostic.AsText so a
// downstream reader can parse either with the same logic.
func (r SuiteRun) AsText() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# LightSync Self-Test Suite Diagnostic\n")
	fmt.Fprintf(&b, "Generated : %s\n", r.StartedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "Duration  : %d ms across %d fixtures\n", r.TotalMs, r.FixtureCount)
	fmt.Fprintf(&b, "Cells     : %d / %d locked (%.0f%%)\n",
		r.LockedCells, r.TotalCells, percent(r.LockedCells, r.TotalCells))
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Per-fixture summary")
	fmt.Fprintln(&b, "```")
	fmt.Fprintf(&b, "%-26s %-8s %-8s %s\n", "fixture", "locked", "time", "failures")
	for _, fr := range r.Fixtures {
		failures := summariseFailures(fr.Diagnostic)
		fmt.Fprintf(&b, "%-26s %d/%-6d %6dms  %s\n",
			fr.FixtureID, fr.LockedCells, fr.TotalCells, fr.DurationMs, failures)
	}
	fmt.Fprintln(&b, "```")
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, "## Per-fixture detail")
	for _, fr := range r.Fixtures {
		fmt.Fprintf(&b, "\n### %s (%s)\n\n", fr.FixtureLabel, fr.FixtureID)
		fmt.Fprint(&b, fr.Diagnostic.AsText())
	}
	return b.String()
}

// summariseFailures returns a short "mode-id: verdict, mode-id: verdict"
// string for every cell in diag that isn't locked. Empty string when
// everything is locked.
func summariseFailures(diag Diagnostic) string {
	var parts []string
	for _, c := range diag.Cells {
		if c.Locked {
			continue
		}
		if len(c.Expected) == 0 {
			continue // no baseline — not a failure
		}
		parts = append(parts, fmt.Sprintf("%s:%s", c.ModeID, c.Verdict))
	}
	if len(parts) == 0 {
		return "all locked ✓"
	}
	return strings.Join(parts, ", ")
}

func percent(part, whole int) float64 {
	if whole == 0 {
		return 0
	}
	return 100.0 * float64(part) / float64(whole)
}
