package extract_test

// End-to-end tests for the colour-extraction pipeline.
//
// These tests drive the SAME functions the engine uses in production
// (extract.New(cfg).Extract + process.ApplyAdjustments) — no reimplementation,
// no cloning. If these tests pass, the app is extracting colours correctly
// for the exercised scenarios; if they fail, screen sync is broken in a way
// the user would see.
//
// Two categories of test:
//
//   * Synthetic — images generated on the fly with pixel-perfect control.
//     Expected HSB is computed from first principles, tolerances are tight.
//
//   * Real     — JPEGs in testdata/ downloaded from picsum.photos. Tolerances
//     are intentionally loose (broad hue/saturation/brightness windows) to
//     catch gross regressions (channel swaps, saturation-to-magenta drift,
//     HSL↔HSB mismatches) without being brittle to minor algorithm tweaks.

import (
	"fmt"
	"image"
	"math"
	"path/filepath"
	"testing"

	"lightsync/internal/lights"
	"lightsync/internal/store"
)

// -------- helpers: image generators --------

func solidImage(w, h int, r, g, b uint8) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i+0] = r
		img.Pix[i+1] = g
		img.Pix[i+2] = b
		img.Pix[i+3] = 255
	}
	return img
}

// halfSplit places leftRGB on the left half and rightRGB on the right half.
func halfSplit(w, h int, left, right [3]uint8) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	half := w / 2
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			off := img.PixOffset(x, y)
			c := left
			if x >= half {
				c = right
			}
			img.Pix[off+0] = c[0]
			img.Pix[off+1] = c[1]
			img.Pix[off+2] = c[2]
			img.Pix[off+3] = 255
		}
	}
	return img
}

// fourQuadrants writes four colours in TL/TR/BL/BR order.
func fourQuadrants(w, h int, colors [4][3]uint8) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	halfW, halfH := w/2, h/2
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := 0
			if x >= halfW {
				idx |= 1
			}
			if y >= halfH {
				idx |= 2
			}
			c := colors[idx]
			off := img.PixOffset(x, y)
			img.Pix[off+0] = c[0]
			img.Pix[off+1] = c[1]
			img.Pix[off+2] = c[2]
			img.Pix[off+3] = 255
		}
	}
	return img
}

// -------- helpers: config builders --------

func singleCfg(method store.ExtractionMethod) store.ScreenSyncConfig {
	cfg := store.ScreenSyncConfig{
		ColorMode:        store.ColorModeSingle,
		ExtractionMethod: method,
	}
	store.NormalizeScreenSyncConfig(&cfg)
	return cfg
}

func multiPaletteCfg(n int, sub store.ExtractionMethod) store.ScreenSyncConfig {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("dev%d", i)
	}
	cfg := store.ScreenSyncConfig{
		ColorMode:          store.ColorModeMulti,
		MultiColorApproach: store.MultiColorScenePalette,
		SubMethod:          sub,
		DeviceIDs:          ids,
	}
	store.NormalizeScreenSyncConfig(&cfg)
	return cfg
}

func multiSpatialCfg(n int, sub store.ExtractionMethod) store.ScreenSyncConfig {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("dev%d", i)
	}
	cfg := store.ScreenSyncConfig{
		ColorMode:          store.ColorModeMulti,
		MultiColorApproach: store.MultiColorSpatialGrid,
		SubMethod:          sub,
		DeviceIDs:          ids,
	}
	store.NormalizeScreenSyncConfig(&cfg)
	return cfg
}

// -------- expectation primitive --------

// expect describes a window of valid HSB values for a single result slot.
// Hue wraps: if hueLo > hueHi, the accepted window spans the 360°/0° seam.
type expect struct {
	label string
	// hueLo and hueHi are inclusive (degrees, 0–360). Use -1 to disable check.
	hueLo, hueHi float64
	minS, maxS   float64
	minB, maxB   float64
}

func (e expect) matches(c lights.Color) bool {
	if c.S < e.minS-1e-6 || c.S > e.maxS+1e-6 {
		return false
	}
	if c.B < e.minB-1e-6 || c.B > e.maxB+1e-6 {
		return false
	}
	if e.hueLo < 0 && e.hueHi < 0 {
		return true // hue not constrained (greyscale acceptable)
	}
	// If saturation is essentially zero the hue is meaningless — treat as match.
	if c.S < 0.05 {
		return true
	}
	if e.hueLo <= e.hueHi {
		return c.H >= e.hueLo-1e-6 && c.H <= e.hueHi+1e-6
	}
	// Wrap-around window (e.g. hueLo=350, hueHi=20 accepts red).
	return c.H >= e.hueLo-1e-6 || c.H <= e.hueHi+1e-6
}

func (e expect) String() string {
	hue := "any"
	if e.hueLo >= 0 || e.hueHi >= 0 {
		hue = fmt.Sprintf("H∈[%d°..%d°]", int(e.hueLo), int(e.hueHi))
	}
	return fmt.Sprintf("%s %s S∈[%.2f..%.2f] B∈[%.2f..%.2f]",
		e.label, hue, e.minS, e.maxS, e.minB, e.maxB)
}

func formatHSB(c lights.Color) string {
	return fmt.Sprintf("H:%3d° S:%3d%% B:%3d%%",
		int(math.Round(c.H)),
		int(math.Round(c.S*100)),
		int(math.Round(c.B*100)))
}

// assertSingle asserts that the first (and only) colour matches e.
func assertSingle(t *testing.T, colors []lights.Color, e expect) {
	t.Helper()
	if len(colors) != 1 {
		t.Fatalf("expected 1 colour, got %d", len(colors))
	}
	if !e.matches(colors[0]) {
		t.Errorf("single-colour mismatch\n  want %s\n  got  %s",
			e, formatHSB(colors[0]))
	}
}

// assertContainsAll asserts that every expectation is satisfied by at least
// one colour in the result, allowing order-independent matching. Used for
// scene-palette output where ordering is weight-based and not fully stable.
func assertContainsAll(t *testing.T, colors []lights.Color, expects []expect) {
	t.Helper()
	used := make([]bool, len(colors))
	for _, e := range expects {
		found := false
		for i, c := range colors {
			if used[i] {
				continue
			}
			if e.matches(c) {
				used[i] = true
				found = true
				break
			}
		}
		if !found {
			var got string
			for _, c := range colors {
				got += "\n    " + formatHSB(c)
			}
			t.Errorf("palette missing expected entry\n  want %s\n  got %s", e, got)
		}
	}
}

// assertAtIndex asserts that colors[idx] matches e. Used for spatial mode where
// position determines slot.
func assertAtIndex(t *testing.T, colors []lights.Color, idx int, e expect) {
	t.Helper()
	if idx < 0 || idx >= len(colors) {
		t.Fatalf("index %d out of range (len=%d)", idx, len(colors))
	}
	if !e.matches(colors[idx]) {
		t.Errorf("spatial slot %d mismatch\n  want %s\n  got  %s",
			idx, e, formatHSB(colors[idx]))
	}
}

// =================================================================
//                      SYNTHETIC IMAGE TESTS
// =================================================================

func TestSingleColor_SolidRed(t *testing.T) {
	img := solidImage(128, 128, 230, 40, 40)

	// True HSB for RGB(230,40,40): H=0°, S≈0.826, B≈0.902.
	// SaturationBoost default 1.2 pushes S past 1.0 → clamped to 1.0.
	red := expect{
		label: "red",
		hueLo: 350, hueHi: 10,
		minS: 0.80, maxS: 1.0,
		minB: 0.80, maxB: 1.0,
	}

	for _, m := range []store.ExtractionMethod{
		store.ExtractionMethodDominant,
		store.ExtractionMethodVivid,
		store.ExtractionMethodSaturated,
		store.ExtractionMethodBrightest,
	} {
		t.Run(string(m), func(t *testing.T) {
			got := runPipeline(singleCfg(m), img)
			assertSingle(t, got, red)
		})
	}
}

func TestSingleColor_SolidGreen(t *testing.T) {
	img := solidImage(128, 128, 40, 200, 60)

	green := expect{
		label: "green",
		hueLo: 115, hueHi: 140,
		minS: 0.80, maxS: 1.0,
		minB: 0.70, maxB: 1.0,
	}
	for _, m := range []store.ExtractionMethod{
		store.ExtractionMethodDominant,
		store.ExtractionMethodVivid,
		store.ExtractionMethodSaturated,
		store.ExtractionMethodBrightest,
	} {
		t.Run(string(m), func(t *testing.T) {
			got := runPipeline(singleCfg(m), img)
			assertSingle(t, got, green)
		})
	}
}

func TestSingleColor_SolidBlue(t *testing.T) {
	img := solidImage(128, 128, 40, 90, 230)

	blue := expect{
		label: "blue",
		hueLo: 210, hueHi: 230,
		minS: 0.80, maxS: 1.0,
		minB: 0.80, maxB: 1.0,
	}
	for _, m := range []store.ExtractionMethod{
		store.ExtractionMethodDominant,
		store.ExtractionMethodVivid,
		store.ExtractionMethodSaturated,
		store.ExtractionMethodBrightest,
	} {
		t.Run(string(m), func(t *testing.T) {
			got := runPipeline(singleCfg(m), img)
			assertSingle(t, got, blue)
		})
	}
}

func TestSingleColor_GreyscaleIsDesaturated(t *testing.T) {
	img := solidImage(128, 128, 128, 128, 128)
	got := runPipeline(singleCfg(store.ExtractionMethodDominant), img)
	if len(got) != 1 {
		t.Fatalf("want 1 colour, got %d", len(got))
	}
	if got[0].S > 0.10 {
		t.Errorf("greyscale should extract S≈0, got %s", formatHSB(got[0]))
	}
	if got[0].B < 0.30 || got[0].B > 0.70 {
		t.Errorf("greyscale mid-brightness expected ~0.5, got B=%.2f", got[0].B)
	}
}

// Area-dominance test: a 70/30 split of blue/red should extract BLUE under
// the Dominant method. The previous DXGI R↔B swap bug would invert this.
func TestSingleColor_DominantPrefersLargerArea(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	// 0..70 = blue, 70..100 = red.
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			off := img.PixOffset(x, y)
			if x < 70 {
				img.Pix[off+0], img.Pix[off+1], img.Pix[off+2] = 40, 90, 230
			} else {
				img.Pix[off+0], img.Pix[off+1], img.Pix[off+2] = 230, 40, 40
			}
			img.Pix[off+3] = 255
		}
	}
	got := runPipeline(singleCfg(store.ExtractionMethodDominant), img)
	assertSingle(t, got, expect{
		label: "blue (70% area)",
		hueLo: 200, hueHi: 240,
		minS:  0.60, maxS: 1.0,
		minB:  0.60, maxB: 1.0,
	})
}

func TestMultiPalette_FourQuadrantsYieldFourFamilies(t *testing.T) {
	// TL=red, TR=green, BL=blue, BR=yellow.
	img := fourQuadrants(200, 200, [4][3]uint8{
		{230, 40, 40},   // TL red
		{40, 200, 60},   // TR green
		{40, 90, 230},   // BL blue
		{240, 220, 40},  // BR yellow
	})
	got := runPipeline(multiPaletteCfg(4, store.ExtractionMethodDominant), img)
	assertContainsAll(t, got, []expect{
		{label: "red",    hueLo: 350, hueHi: 10,  minS: 0.6, maxS: 1.0, minB: 0.5, maxB: 1.0},
		{label: "green",  hueLo: 105, hueHi: 145, minS: 0.6, maxS: 1.0, minB: 0.5, maxB: 1.0},
		{label: "blue",   hueLo: 210, hueHi: 235, minS: 0.6, maxS: 1.0, minB: 0.5, maxB: 1.0},
		{label: "yellow", hueLo: 45,  hueHi: 70,  minS: 0.6, maxS: 1.0, minB: 0.5, maxB: 1.0},
	})
}

func TestMultiSpatial_GridYieldsPositionalColors(t *testing.T) {
	// Spatial extractor returns row-major: TL, TR, BL, BR.
	img := fourQuadrants(200, 200, [4][3]uint8{
		{230, 40, 40},  // TL red
		{40, 200, 60},  // TR green
		{40, 90, 230},  // BL blue
		{240, 220, 40}, // BR yellow
	})
	got := runPipeline(multiSpatialCfg(4, store.ExtractionMethodDominant), img)
	if len(got) != 4 {
		t.Fatalf("want 4 zone colours, got %d", len(got))
	}
	assertAtIndex(t, got, 0, expect{label: "TL red",
		hueLo: 350, hueHi: 10, minS: 0.6, maxS: 1.0, minB: 0.5, maxB: 1.0})
	assertAtIndex(t, got, 1, expect{label: "TR green",
		hueLo: 105, hueHi: 145, minS: 0.6, maxS: 1.0, minB: 0.5, maxB: 1.0})
	assertAtIndex(t, got, 2, expect{label: "BL blue",
		hueLo: 210, hueHi: 235, minS: 0.6, maxS: 1.0, minB: 0.5, maxB: 1.0})
	assertAtIndex(t, got, 3, expect{label: "BR yellow",
		hueLo: 45, hueHi: 70, minS: 0.6, maxS: 1.0, minB: 0.5, maxB: 1.0})
}

// =================================================================
//                       REAL IMAGE TESTS
// =================================================================

// Each entry below is a real JPEG in testdata/ with a wide acceptance window
// that captures the frame's unambiguous colour character. The windows are
// intentionally loose so algorithm micro-tweaks don't break them — they
// catch systemic failures like R↔B inversion, pastel magenta drift, etc.

type realCase struct {
	file     string
	descr    string
	method   store.ExtractionMethod
	single   expect        // expected single-colour output for this method
	palette  []expect      // expected palette contents (multi scene_palette, 4 slots)
	spatial  []expect      // expected spatial grid colours (ordered TL,TR,BL,BR)
}

var realCases = []realCase{
	{
		file:   "picsum_1015.jpg",
		descr:  "lake / boat — strongly blue-dominant",
		method: store.ExtractionMethodVivid,
		single: expect{label: "blue",
			hueLo: 190, hueHi: 235, minS: 0.35, maxS: 1.0, minB: 0.30, maxB: 1.0},
		palette: []expect{
			{label: "blue", hueLo: 190, hueHi: 235, minS: 0.30, maxS: 1.0, minB: 0.30, maxB: 1.0},
		},
	},
	{
		file:   "picsum_29.jpg",
		descr:  "water with warm accents — blue overall",
		method: store.ExtractionMethodVivid,
		single: expect{label: "blue",
			hueLo: 180, hueHi: 235, minS: 0.40, maxS: 1.0, minB: 0.30, maxB: 1.0},
		palette: []expect{
			{label: "blue", hueLo: 180, hueHi: 235, minS: 0.30, maxS: 1.0, minB: 0.30, maxB: 1.0},
		},
	},
	{
		file:   "picsum_177.jpg",
		descr:  "warm outdoor scene — orange-dominant",
		method: store.ExtractionMethodVivid,
		single: expect{label: "warm orange/brown",
			hueLo: 15, hueHi: 50, minS: 0.40, maxS: 1.0, minB: 0.20, maxB: 1.0},
		palette: []expect{
			{label: "warm", hueLo: 15, hueHi: 50, minS: 0.30, maxS: 1.0, minB: 0.20, maxB: 1.0},
		},
	},
	{
		file:   "picsum_103.jpg",
		descr:  "landscape with blue sky + warm ground",
		method: store.ExtractionMethodVivid,
		single: expect{label: "blue sky",
			hueLo: 190, hueHi: 235, minS: 0.25, maxS: 1.0, minB: 0.40, maxB: 1.0},
		palette: []expect{
			{label: "blue", hueLo: 190, hueHi: 235, minS: 0.20, maxS: 1.0, minB: 0.30, maxB: 1.0},
			{label: "warm", hueLo: 15, hueHi: 55, minS: 0.20, maxS: 1.0, minB: 0.20, maxB: 1.0},
		},
	},
	{
		file:   "picsum_1025.jpg",
		descr:  "neutral tan scene — olive/yellow-green dominant",
		method: store.ExtractionMethodVivid,
		single: expect{label: "olive",
			hueLo: 40, hueHi: 90, minS: 0.20, maxS: 1.0, minB: 0.15, maxB: 0.60},
		palette: []expect{
			{label: "olive", hueLo: 40, hueHi: 90, minS: 0.20, maxS: 1.0, minB: 0.15, maxB: 0.70},
		},
	},
	{
		file:   "picsum_152.jpg",
		descr:  "violet-dominant scene",
		method: store.ExtractionMethodVivid,
		single: expect{label: "violet",
			hueLo: 250, hueHi: 290, minS: 0.20, maxS: 1.0, minB: 0.40, maxB: 1.0},
		palette: []expect{
			{label: "violet", hueLo: 250, hueHi: 290, minS: 0.20, maxS: 1.0, minB: 0.40, maxB: 1.0},
		},
	},
	{
		file:   "picsum_453.jpg",
		descr:  "pure greyscale photo — extraction must yield ~S=0",
		method: store.ExtractionMethodDominant,
		single: expect{label: "neutral grey",
			hueLo: -1, hueHi: -1, minS: 0.0, maxS: 0.10, minB: 0.0, maxB: 1.0},
		palette: []expect{
			{label: "grey", hueLo: -1, hueHi: -1, minS: 0.0, maxS: 0.15, minB: 0.0, maxB: 1.0},
		},
	},
}

func TestRealImages_SingleColor(t *testing.T) {
	for _, rc := range realCases {
		t.Run(rc.file, func(t *testing.T) {
			img := loadImage(t, filepath.Join("testdata", rc.file))
			got := runPipeline(singleCfg(rc.method), img)
			if len(got) != 1 {
				t.Fatalf("expected 1 colour, got %d", len(got))
			}
			if !rc.single.matches(got[0]) {
				t.Errorf("%s: %s\n  want %s\n  got  %s",
					rc.file, rc.descr, rc.single, formatHSB(got[0]))
			}
		})
	}
}

// =================================================================
//                    HARNESS SELF-CHECK
// =================================================================

// TestHarnessCatchesRedBlueSwap proves the expectation windows are tight
// enough to catch a real-world regression: the DXGI BGRA/RGBA channel-order
// bug that was swapping red and blue before extraction. A solid-red image
// with R and B channels inverted should NOT satisfy the "red" expectation.
//
// If this test ever starts passing, someone's widened the tolerances past
// the point where real bugs can sneak through.
func TestHarnessCatchesRedBlueSwap(t *testing.T) {
	red := expect{
		label: "red",
		hueLo: 350, hueHi: 10,
		minS: 0.80, maxS: 1.0,
		minB: 0.80, maxB: 1.0,
	}

	// Actually red — sanity check.
	realRed := solidImage(64, 64, 230, 40, 40)
	gotReal := runPipeline(singleCfg(store.ExtractionMethodDominant), realRed)
	if !red.matches(gotReal[0]) {
		t.Fatalf("harness broken: real red didn't match red expectation (got %s)",
			formatHSB(gotReal[0]))
	}

	// R↔B swapped (blue pretending to be red due to the old DXGI bug).
	fakeRed := solidImage(64, 64, 40, 40, 230)
	gotFake := runPipeline(singleCfg(store.ExtractionMethodDominant), fakeRed)
	if red.matches(gotFake[0]) {
		t.Errorf("harness failed to catch R↔B swap: 'red' expectation accepted %s",
			formatHSB(gotFake[0]))
	}
}

func TestRealImages_ScenePalette(t *testing.T) {
	for _, rc := range realCases {
		if len(rc.palette) == 0 {
			continue
		}
		t.Run(rc.file, func(t *testing.T) {
			img := loadImage(t, filepath.Join("testdata", rc.file))
			got := runPipeline(multiPaletteCfg(4, rc.method), img)
			if len(got) < len(rc.palette) {
				t.Fatalf("palette too short: got %d, want >= %d", len(got), len(rc.palette))
			}
			assertContainsAll(t, got, rc.palette)
		})
	}
}
