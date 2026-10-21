// Package selftest provides an in-app test harness for the colour-extraction
// pipeline. It drives the same Extract + ApplyAdjustments functions the
// engine uses in production, exposing results for every colour-mode
// combination so the UI can display Expected-vs-Actual swatches for every
// cell of the mode matrix.
package selftest

import (
	"image"

	"lightsync/internal/lights"
	"lightsync/internal/screensync/extract"
	"lightsync/internal/screensync/process"
	"lightsync/internal/store"
)

// ModeCase is one cell of the mode matrix: a labelled ScreenSyncConfig that
// maps 1:1 to something the user can actually select in the app UI.
type ModeCase struct {
	// ID is a stable, slug-style identifier (e.g. "single-vivid",
	// "multi-palette-saturated"). Frontend uses this to key grid cells.
	ID string `json:"id"`
	// Label is the human-readable name shown in the UI.
	Label string `json:"label"`
	// Group is "single", "palette", or "spatial" — used by the UI to
	// cluster cells visually.
	Group string `json:"group"`
	// Config is the normalised config that produces this cell's output.
	Config store.ScreenSyncConfig `json:"config"`
}

// CellResult is the extracted colours for a single mode cell on a single
// captured frame, plus the expected colours (if known) for comparison.
type CellResult struct {
	ModeID   string         `json:"modeId"`
	Actual   []lights.Color `json:"actual"`
	Expected []lights.Color `json:"expected,omitempty"` // empty when unknown (e.g. custom fixtures)
}

// AllModes returns the full matrix of colour modes tested per fixture:
//
//   - Single-colour × {dominant, vivid, saturated, brightest}         → 4 cells
//   - Multi-palette × {dominant, vivid, saturated, brightest, diverse} → 5 cells
//   - Multi-spatial × {dominant, vivid, saturated, brightest}         → 4 cells
//
// 13 total. The matrix deliberately mirrors the options the user can pick in
// the capture configuration panel — not every conceivable combination.
func AllModes(deviceCount int) []ModeCase {
	if deviceCount < 1 {
		deviceCount = 4
	}
	ids := make([]string, deviceCount)
	for i := range ids {
		ids[i] = "__selftest_" + string(rune('a'+i))
	}

	singleMethods := []store.ExtractionMethod{
		store.ExtractionMethodDominant,
		store.ExtractionMethodVivid,
		store.ExtractionMethodSaturated,
		store.ExtractionMethodBrightest,
	}
	paletteSub := []store.ExtractionMethod{
		store.ExtractionMethodDominant,
		store.ExtractionMethodVivid,
		store.ExtractionMethodSaturated,
		store.ExtractionMethodBrightest,
		store.ExtractionMethodDiverse,
	}
	spatialSub := []store.ExtractionMethod{
		store.ExtractionMethodDominant,
		store.ExtractionMethodVivid,
		store.ExtractionMethodSaturated,
		store.ExtractionMethodBrightest,
	}

	var modes []ModeCase

	for _, m := range singleMethods {
		cfg := store.ScreenSyncConfig{
			ColorMode:        store.ColorModeSingle,
			ExtractionMethod: m,
			// Single-colour mode still needs at least one device ID —
			// the assigner maps the extracted colour onto every device in
			// this list, and without any IDs the output is an empty map
			// which flattens to nil. The real engine always has at least
			// one device in the scene; we mirror that here.
			DeviceIDs: []string{"__selftest_single"},
		}
		store.NormalizeScreenSyncConfig(&cfg)
		modes = append(modes, ModeCase{
			ID:     "single-" + string(m),
			Label:  "Single · " + methodLabel(m),
			Group:  "single",
			Config: cfg,
		})
	}

	for _, m := range paletteSub {
		cfg := store.ScreenSyncConfig{
			ColorMode:          store.ColorModeMulti,
			MultiColorApproach: store.MultiColorScenePalette,
			SubMethod:          m,
			DeviceIDs:          append([]string(nil), ids...),
		}
		store.NormalizeScreenSyncConfig(&cfg)
		modes = append(modes, ModeCase{
			ID:     "palette-" + string(m),
			Label:  "Palette · " + methodLabel(m),
			Group:  "palette",
			Config: cfg,
		})
	}

	for _, m := range spatialSub {
		cfg := store.ScreenSyncConfig{
			ColorMode:          store.ColorModeMulti,
			MultiColorApproach: store.MultiColorSpatialGrid,
			SubMethod:          m,
			DeviceIDs:          append([]string(nil), ids...),
		}
		store.NormalizeScreenSyncConfig(&cfg)
		modes = append(modes, ModeCase{
			ID:     "spatial-" + string(m),
			Label:  "Spatial · " + methodLabel(m),
			Group:  "spatial",
			Config: cfg,
		})
	}

	return modes
}

func methodLabel(m store.ExtractionMethod) string {
	switch m {
	case store.ExtractionMethodDominant:
		return "Dominant"
	case store.ExtractionMethodVivid:
		return "Vivid"
	case store.ExtractionMethodSaturated:
		return "Saturated"
	case store.ExtractionMethodBrightest:
		return "Brightest"
	case store.ExtractionMethodDiverse:
		return "Diverse"
	default:
		return string(m)
	}
}

// Analyze runs the real production pipeline — extract.New(cfg).Extract +
// process.ApplyAdjustments — once per mode against img. The returned slice
// has the same length and order as modes.
//
// Expected values are populated on the caller's side via LookupExpected.
func Analyze(img image.Image, modes []ModeCase) []CellResult {
	results := make([]CellResult, 0, len(modes))
	for _, m := range modes {
		ex := extract.New(m.Config)
		raw := ex.Extract(img, m.Config)
		adj := process.ApplyAdjustments(raw, m.Config)
		results = append(results, CellResult{
			ModeID: m.ID,
			Actual: adj,
		})
	}
	return results
}
