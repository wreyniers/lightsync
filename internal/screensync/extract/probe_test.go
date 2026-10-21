package extract_test

// This file holds a diagnostic probe, not an assertion. Gated on the
// LIGHTSYNC_PROBE environment variable so it doesn't run during normal
// `go test ./...` invocations.
//
// PowerShell:
//
//   $env:LIGHTSYNC_PROBE="1"
//   go test -run TestProbeTestdata -v ./internal/screensync/extract/...
//   $env:LIGHTSYNC_PROBE=""
//
// bash / zsh:
//
//   LIGHTSYNC_PROBE=1 go test -run TestProbeTestdata -v ./internal/screensync/extract/...
//
// The probe prints the HSB triples the production pipeline extracts from
// every image in testdata/ across the full 13-cell mode matrix (single × 4,
// palette × 5, spatial × 4). The trailing "========== PASTE BELOW INTO
// fixtures.go ==========" block is ready-to-paste Go source that refreshes
// the baselines used by the in-app Self-Test harness.
//
// Full workflow is documented in docs/development.md → "Colour-Extraction
// Self-Test".

import (
	"fmt"
	"image"
	// Register JPEG/PNG decoders so image.Decode works on testdata.
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"lightsync/internal/lights"
	"lightsync/internal/screensync/extract"
	"lightsync/internal/screensync/process"
	"lightsync/internal/store"
)

// probeEntry is one row of the probe output: a mode ID (the same slug the
// Self-Test emits) plus its resolved config.
type probeEntry struct {
	modeID string
	label  string
	cfg    store.ScreenSyncConfig
}

// probeConfigs enumerates the full mode matrix the Self-Test harness
// exposes: single × 4 methods + palette × 5 sub-methods + spatial × 4
// sub-methods = 13 cells, with slug IDs that exactly match
// selftest.AllModes so the probe output can be pasted into fixtures.go
// verbatim.
func probeConfigs() []probeEntry {
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

	var out []probeEntry

	for _, m := range singleMethods {
		cfg := store.ScreenSyncConfig{
			ColorMode:        store.ColorModeSingle,
			ExtractionMethod: m,
			DeviceIDs:        []string{"__selftest_single"},
		}
		store.NormalizeScreenSyncConfig(&cfg)
		out = append(out, probeEntry{
			modeID: "single-" + string(m),
			label:  "single/" + string(m),
			cfg:    cfg,
		})
	}
	for _, m := range paletteSub {
		cfg := store.ScreenSyncConfig{
			ColorMode:          store.ColorModeMulti,
			MultiColorApproach: store.MultiColorScenePalette,
			SubMethod:          m,
			DeviceIDs:          []string{"a", "b", "c", "d"},
		}
		store.NormalizeScreenSyncConfig(&cfg)
		out = append(out, probeEntry{
			modeID: "palette-" + string(m),
			label:  "palette/" + string(m),
			cfg:    cfg,
		})
	}
	for _, m := range spatialSub {
		cfg := store.ScreenSyncConfig{
			ColorMode:          store.ColorModeMulti,
			MultiColorApproach: store.MultiColorSpatialGrid,
			SubMethod:          m,
			DeviceIDs:          []string{"a", "b", "c", "d"},
		}
		store.NormalizeScreenSyncConfig(&cfg)
		out = append(out, probeEntry{
			modeID: "spatial-" + string(m),
			label:  "spatial/" + string(m),
			cfg:    cfg,
		})
	}
	return out
}

func loadImage(tb testing.TB, path string) image.Image {
	tb.Helper()
	f, err := os.Open(path)
	if err != nil {
		tb.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		tb.Fatalf("decode %s: %v", path, err)
	}
	return img
}

// runPipeline mirrors engine.run(): Extract → ApplyAdjustments. Smoother and
// handoff are temporal and meaningless on single frames, so they are skipped.
func runPipeline(cfg store.ScreenSyncConfig, img image.Image) []lights.Color {
	ex := extract.New(cfg)
	raw := ex.Extract(img, cfg)
	return process.ApplyAdjustments(raw, cfg)
}

func TestProbeTestdata(t *testing.T) {
	if os.Getenv("LIGHTSYNC_PROBE") == "" {
		t.Skip("set LIGHTSYNC_PROBE=1 to run the diagnostic probe")
	}

	files := collectFixtures(t)
	if len(files) == 0 {
		t.Skip("no testdata images found")
	}

	configs := probeConfigs()

	for _, fx := range files {
		img := loadImage(t, fx.path)
		t.Logf("=== %s (%dx%d) ===", fx.name, img.Bounds().Dx(), img.Bounds().Dy())
		for _, c := range configs {
			colors := runPipeline(c.cfg, img)
			t.Logf("  %-28s -> %s", c.label, formatColors(colors))
		}
	}

	// Emit a block of Go source ready to paste into fixtures.go. Keeping
	// this inline with the probe avoids a separate transcription step every
	// time baselines need refreshing after an algorithm change.
	t.Log("")
	t.Log("========== PASTE BELOW INTO fixtures.go ==========")
	for _, fx := range files {
		img := loadImage(t, fx.path)
		t.Logf("")
		t.Logf("// %s", fx.name)
		t.Logf("%s: map[string][]lights.Color{", fixtureIDFromName(fx.name))
		for _, c := range configs {
			colors := runPipeline(c.cfg, img)
			t.Logf("    %q: %s,", c.modeID, goLiteralForColors(colors))
		}
		t.Logf("},")
	}
	t.Log("========== END PASTE BLOCK ==========")
}

type fixtureFile struct {
	name string // basename, e.g. "picsum_1015.jpg"
	path string // full testdata/<name>
}

func collectFixtures(t *testing.T) []fixtureFile {
	t.Helper()
	var out []fixtureFile
	for _, pattern := range []string{"*.jpg", "*.png"} {
		matches, err := filepath.Glob(filepath.Join("testdata", pattern))
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range matches {
			out = append(out, fixtureFile{
				name: filepath.Base(p),
				path: p,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// fixtureIDFromName maps a testdata filename to the fixture ID slug used
// by fixtures.go, so the pasted block is keyed the way the rest of the
// code expects.
func fixtureIDFromName(basename string) string {
	switch basename {
	case "synth_red.png":
		return `"synth-solid-red"`
	case "synth_green.png":
		return `"synth-solid-green"`
	case "synth_blue.png":
		return `"synth-solid-blue"`
	case "synth_quadrants.png":
		return `"synth-quadrants"`
	case "picsum_1015.jpg":
		return `"photo-lake-blue"`
	case "picsum_29.jpg":
		return `"photo-ocean-warm"`
	case "picsum_177.jpg":
		return `"photo-warm-orange"`
	case "picsum_103.jpg":
		return `"photo-landscape-mixed"`
	case "picsum_1025.jpg":
		return `"photo-olive-tan"`
	case "picsum_152.jpg":
		return `"photo-violet"`
	case "picsum_453.jpg":
		return `"photo-greyscale"`
	default:
		return fmt.Sprintf("%q", basename)
	}
}

func formatColors(colors []lights.Color) string {
	if len(colors) == 0 {
		return "(none)"
	}
	out := "["
	for i, c := range colors {
		if i > 0 {
			out += " | "
		}
		out += fmt.Sprintf("H:%3d°  S:%3d%%  B:%3d%%",
			int(c.H+0.5), int(c.S*100+0.5), int(c.B*100+0.5))
	}
	return out + "]"
}

// goLiteralForColors emits a single-line slice literal suitable for pasting
// into fixtures.go. Values are rounded to keep diffs readable.
func goLiteralForColors(colors []lights.Color) string {
	if len(colors) == 0 {
		return "{}"
	}
	parts := make([]string, len(colors))
	for i, c := range colors {
		parts[i] = fmt.Sprintf("{H: %.0f, S: %.2f, B: %.2f}",
			roundTo(c.H, 1), c.S, c.B)
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func roundTo(v, step float64) float64 {
	return float64(int((v+step/2)/step)) * step
}
