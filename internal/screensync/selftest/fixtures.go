package selftest

import "lightsync/internal/lights"

// Fixture is one entry in the built-in test set.
type Fixture struct {
	// ID is a stable slug used by the frontend (e.g. "lake-blue").
	ID string `json:"id"`
	// Label is the human-readable name shown in the UI.
	Label string `json:"label"`
	// Description is a one-line summary of what the image contains.
	Description string `json:"description"`
	// URL is the asset path served by the Wails dev server and production
	// build (everything under frontend/public is served at root).
	URL string `json:"url"`
	// Kind is "photo" or "synthetic" — used purely for UI grouping.
	Kind string `json:"kind"`
	// Expected maps ModeCase.ID to the expected colour output. When a mode
	// isn't present in the map the UI just shows the actual colour with no
	// pass/fail indicator.
	Expected map[string][]lights.Color `json:"expected,omitempty"`
}

// BuiltInFixtures returns the shipped fixture list. URLs point into
// frontend/public/testdata/, which Vite serves verbatim.
//
// Expected maps are populated from the output of extract/probe_test.go's
// TestProbeTestdata run against the raw fixture files. They're golden-file
// baselines — run the probe (LIGHTSYNC_PROBE=1 go test -run TestProbeTestdata
// -v ./internal/screensync/extract/...) after any algorithm change and
// paste the new values in here. The probe emits ready-to-paste Go literals.
func BuiltInFixtures() []Fixture {
	return []Fixture{
		// -------- Synthetics --------
		{
			ID:          "synth-solid-red",
			Label:       "Solid Red",
			Description: "RGB(230, 40, 40). Every mode should yield the same red — small variations between methods reflect the 32-step RGB quantisation the Dominant/Vivid extractors use.",
			URL:         "/testdata/synth_red.png",
			Kind:        "synthetic",
			Expected:    expectSolidRed(),
		},
		{
			ID:          "synth-solid-green",
			Label:       "Solid Green",
			Description: "RGB(40, 200, 60). Every mode should yield the same green.",
			URL:         "/testdata/synth_green.png",
			Kind:        "synthetic",
			Expected:    expectSolidGreen(),
		},
		{
			ID:          "synth-solid-blue",
			Label:       "Solid Blue",
			Description: "RGB(40, 90, 230). Every mode should yield the same blue.",
			URL:         "/testdata/synth_blue.png",
			Kind:        "synthetic",
			Expected:    expectSolidBlue(),
		},
		{
			ID:          "synth-quadrants",
			Label:       "Four Quadrants (R/G/B/Y)",
			Description: "Red (TL), Green (TR), Blue (BL), Yellow (BR). Verifies palette-mode surfaces all four families and spatial-mode preserves position.",
			URL:         "/testdata/synth_quadrants.png",
			Kind:        "synthetic",
			Expected:    expectQuadrants(),
		},

		// -------- Real photos --------
		{
			ID:          "photo-lake-blue",
			Label:       "Lake scene — blue dominant",
			Description: "Strongly blue-dominant photograph. All modes should yield H≈200–220°.",
			URL:         "/testdata/picsum_1015.jpg",
			Kind:        "photo",
			Expected:    expectLakeBlue(),
		},
		{
			ID:          "photo-ocean-warm",
			Label:       "Ocean with warm accents",
			Description: "Blue water with desaturated warm foreground patches.",
			URL:         "/testdata/picsum_29.jpg",
			Kind:        "photo",
			Expected:    expectOceanWarm(),
		},
		{
			ID:          "photo-warm-orange",
			Label:       "Warm outdoor — orange",
			Description: "Warm-toned scene with orange/brown dominance (H≈30°).",
			URL:         "/testdata/picsum_177.jpg",
			Kind:        "photo",
			Expected:    expectWarmOrange(),
		},
		{
			ID:          "photo-landscape-mixed",
			Label:       "Landscape — blue sky + warm ground",
			Description: "Mixed scene. Single modes pick blue sky; palette should surface blue AND warm ground tones.",
			URL:         "/testdata/picsum_103.jpg",
			Kind:        "photo",
			Expected:    expectLandscapeMixed(),
		},
		{
			ID:          "photo-olive-tan",
			Label:       "Tan pug — olive/tan dominant",
			Description: "Desaturated warm scene with olive-tan dominance (H≈60°).",
			URL:         "/testdata/picsum_1025.jpg",
			Kind:        "photo",
			Expected:    expectOliveTan(),
		},
		{
			ID:          "photo-violet",
			Label:       "Violet / lavender scene",
			Description: "Purple-dominant photograph. Colour modes should yield H≈255–285°.",
			URL:         "/testdata/picsum_152.jpg",
			Kind:        "photo",
			Expected:    expectViolet(),
		},
		{
			ID:          "photo-greyscale",
			Label:       "Greyscale photograph",
			Description: "Pure B&W. Every mode should yield near-zero saturation.",
			URL:         "/testdata/picsum_453.jpg",
			Kind:        "photo",
			Expected:    expectGreyscale(),
		},
	}
}

// FixtureByID returns the named fixture, or nil if not found. Used by the
// app layer to validate incoming fixture IDs from the frontend.
func FixtureByID(id string) *Fixture {
	for i, f := range BuiltInFixtures() {
		if f.ID == id {
			list := BuiltInFixtures()
			return &list[i]
		}
	}
	return nil
}

// ---------------------------------------------------------------------
// Baselines — extracted verbatim from extract/probe_test.go output.
// ---------------------------------------------------------------------

func expectSolidRed() map[string][]lights.Color {
	return map[string][]lights.Color{
		"single-dominant":   {{H: 0, S: 0.86, B: 0.88}},
		"single-vivid":      {{H: 0, S: 0.86, B: 0.88}},
		"single-saturated":  {{H: 0, S: 0.83, B: 0.90}},
		"single-brightest":  {{H: 0, S: 0.83, B: 0.90}},
		"palette-dominant":  {{H: 0, S: 0.86, B: 0.88}, {H: 0, S: 0.86, B: 0.88}, {H: 0, S: 0.86, B: 0.88}, {H: 0, S: 0.86, B: 0.88}},
		"palette-vivid":     {{H: 0, S: 0.86, B: 0.88}, {H: 0, S: 0.86, B: 0.88}, {H: 0, S: 0.86, B: 0.88}, {H: 0, S: 0.86, B: 0.88}},
		"palette-saturated": {{H: 0, S: 0.86, B: 0.88}, {H: 0, S: 0.86, B: 0.88}, {H: 0, S: 0.86, B: 0.88}, {H: 0, S: 0.86, B: 0.88}},
		"palette-brightest": {{H: 0, S: 0.86, B: 0.88}, {H: 0, S: 0.86, B: 0.88}, {H: 0, S: 0.86, B: 0.88}, {H: 0, S: 0.86, B: 0.88}},
		"palette-diverse":   {{H: 0, S: 0.86, B: 0.88}, {H: 0, S: 0.86, B: 0.88}, {H: 0, S: 0.86, B: 0.88}, {H: 0, S: 0.86, B: 0.88}},
		"spatial-dominant":  {{H: 0, S: 0.86, B: 0.88}, {H: 0, S: 0.86, B: 0.88}, {H: 0, S: 0.86, B: 0.88}, {H: 0, S: 0.86, B: 0.88}},
		"spatial-vivid":     {{H: 0, S: 0.86, B: 0.88}, {H: 0, S: 0.86, B: 0.88}, {H: 0, S: 0.86, B: 0.88}, {H: 0, S: 0.86, B: 0.88}},
		"spatial-saturated": {{H: 0, S: 0.83, B: 0.90}, {H: 0, S: 0.83, B: 0.90}, {H: 0, S: 0.83, B: 0.90}, {H: 0, S: 0.83, B: 0.90}},
		"spatial-brightest": {{H: 0, S: 0.83, B: 0.90}, {H: 0, S: 0.83, B: 0.90}, {H: 0, S: 0.83, B: 0.90}, {H: 0, S: 0.83, B: 0.90}},
	}
}

func expectSolidGreen() map[string][]lights.Color {
	return map[string][]lights.Color{
		"single-dominant":   {{H: 120, S: 0.83, B: 0.75}},
		"single-vivid":      {{H: 120, S: 0.83, B: 0.75}},
		"single-saturated":  {{H: 128, S: 0.80, B: 0.78}},
		"single-brightest":  {{H: 128, S: 0.80, B: 0.78}},
		"palette-dominant":  {{H: 120, S: 0.83, B: 0.75}, {H: 120, S: 0.83, B: 0.75}, {H: 120, S: 0.83, B: 0.75}, {H: 120, S: 0.83, B: 0.75}},
		"palette-vivid":     {{H: 120, S: 0.83, B: 0.75}, {H: 120, S: 0.83, B: 0.75}, {H: 120, S: 0.83, B: 0.75}, {H: 120, S: 0.83, B: 0.75}},
		"palette-saturated": {{H: 120, S: 0.83, B: 0.75}, {H: 120, S: 0.83, B: 0.75}, {H: 120, S: 0.83, B: 0.75}, {H: 120, S: 0.83, B: 0.75}},
		"palette-brightest": {{H: 120, S: 0.83, B: 0.75}, {H: 120, S: 0.83, B: 0.75}, {H: 120, S: 0.83, B: 0.75}, {H: 120, S: 0.83, B: 0.75}},
		"palette-diverse":   {{H: 120, S: 0.83, B: 0.75}, {H: 120, S: 0.83, B: 0.75}, {H: 120, S: 0.83, B: 0.75}, {H: 120, S: 0.83, B: 0.75}},
		"spatial-dominant":  {{H: 120, S: 0.83, B: 0.75}, {H: 120, S: 0.83, B: 0.75}, {H: 120, S: 0.83, B: 0.75}, {H: 120, S: 0.83, B: 0.75}},
		"spatial-vivid":     {{H: 120, S: 0.83, B: 0.75}, {H: 120, S: 0.83, B: 0.75}, {H: 120, S: 0.83, B: 0.75}, {H: 120, S: 0.83, B: 0.75}},
		"spatial-saturated": {{H: 128, S: 0.80, B: 0.78}, {H: 128, S: 0.80, B: 0.78}, {H: 128, S: 0.80, B: 0.78}, {H: 128, S: 0.80, B: 0.78}},
		"spatial-brightest": {{H: 128, S: 0.80, B: 0.78}, {H: 128, S: 0.80, B: 0.78}, {H: 128, S: 0.80, B: 0.78}, {H: 128, S: 0.80, B: 0.78}},
	}
}

func expectSolidBlue() map[string][]lights.Color {
	return map[string][]lights.Color{
		"single-dominant":   {{H: 230, S: 0.86, B: 0.88}},
		"single-vivid":      {{H: 230, S: 0.86, B: 0.88}},
		"single-saturated":  {{H: 224, S: 0.83, B: 0.90}},
		"single-brightest":  {{H: 224, S: 0.83, B: 0.90}},
		"palette-dominant":  {{H: 230, S: 0.86, B: 0.88}, {H: 230, S: 0.86, B: 0.88}, {H: 230, S: 0.86, B: 0.88}, {H: 230, S: 0.86, B: 0.88}},
		"palette-vivid":     {{H: 230, S: 0.86, B: 0.88}, {H: 230, S: 0.86, B: 0.88}, {H: 230, S: 0.86, B: 0.88}, {H: 230, S: 0.86, B: 0.88}},
		"palette-saturated": {{H: 230, S: 0.86, B: 0.88}, {H: 230, S: 0.86, B: 0.88}, {H: 230, S: 0.86, B: 0.88}, {H: 230, S: 0.86, B: 0.88}},
		"palette-brightest": {{H: 230, S: 0.86, B: 0.88}, {H: 230, S: 0.86, B: 0.88}, {H: 230, S: 0.86, B: 0.88}, {H: 230, S: 0.86, B: 0.88}},
		"palette-diverse":   {{H: 230, S: 0.86, B: 0.88}, {H: 230, S: 0.86, B: 0.88}, {H: 230, S: 0.86, B: 0.88}, {H: 230, S: 0.86, B: 0.88}},
		"spatial-dominant":  {{H: 230, S: 0.86, B: 0.88}, {H: 230, S: 0.86, B: 0.88}, {H: 230, S: 0.86, B: 0.88}, {H: 230, S: 0.86, B: 0.88}},
		"spatial-vivid":     {{H: 230, S: 0.86, B: 0.88}, {H: 230, S: 0.86, B: 0.88}, {H: 230, S: 0.86, B: 0.88}, {H: 230, S: 0.86, B: 0.88}},
		"spatial-saturated": {{H: 224, S: 0.83, B: 0.90}, {H: 224, S: 0.83, B: 0.90}, {H: 224, S: 0.83, B: 0.90}, {H: 224, S: 0.83, B: 0.90}},
		"spatial-brightest": {{H: 224, S: 0.83, B: 0.90}, {H: 224, S: 0.83, B: 0.90}, {H: 224, S: 0.83, B: 0.90}, {H: 224, S: 0.83, B: 0.90}},
	}
}

func expectQuadrants() map[string][]lights.Color {
	return map[string][]lights.Color{
		"single-dominant":   {{H: 0, S: 0.86, B: 0.88}},
		"single-vivid":      {{H: 50, S: 0.86, B: 0.88}},
		"single-saturated":  {{H: 54, S: 0.83, B: 0.94}},
		"single-brightest":  {{H: 54, S: 0.83, B: 0.94}},
		"palette-dominant":  {{H: 230, S: 0.86, B: 0.88}, {H: 120, S: 0.83, B: 0.75}, {H: 0, S: 0.86, B: 0.88}, {H: 50, S: 0.86, B: 0.88}},
		"palette-vivid":     {{H: 50, S: 0.86, B: 0.88}, {H: 230, S: 0.86, B: 0.88}, {H: 0, S: 0.86, B: 0.88}, {H: 120, S: 0.83, B: 0.75}},
		"palette-saturated": {{H: 50, S: 0.86, B: 0.88}, {H: 230, S: 0.86, B: 0.88}, {H: 0, S: 0.86, B: 0.88}, {H: 120, S: 0.83, B: 0.75}},
		"palette-brightest": {{H: 50, S: 0.86, B: 0.88}, {H: 120, S: 0.83, B: 0.75}, {H: 230, S: 0.86, B: 0.88}, {H: 0, S: 0.86, B: 0.88}},
		"palette-diverse":   {{H: 230, S: 0.86, B: 0.88}, {H: 50, S: 0.86, B: 0.88}, {H: 120, S: 0.83, B: 0.75}, {H: 0, S: 0.86, B: 0.88}},
		"spatial-dominant":  {{H: 0, S: 0.86, B: 0.88}, {H: 120, S: 0.83, B: 0.75}, {H: 230, S: 0.86, B: 0.88}, {H: 50, S: 0.86, B: 0.88}},
		"spatial-vivid":     {{H: 0, S: 0.86, B: 0.88}, {H: 120, S: 0.83, B: 0.75}, {H: 230, S: 0.86, B: 0.88}, {H: 50, S: 0.86, B: 0.88}},
		"spatial-saturated": {{H: 0, S: 0.83, B: 0.90}, {H: 128, S: 0.80, B: 0.78}, {H: 224, S: 0.83, B: 0.90}, {H: 54, S: 0.83, B: 0.94}},
		"spatial-brightest": {{H: 0, S: 0.83, B: 0.90}, {H: 128, S: 0.80, B: 0.78}, {H: 224, S: 0.83, B: 0.90}, {H: 54, S: 0.83, B: 0.94}},
	}
}

func expectLakeBlue() map[string][]lights.Color {
	return map[string][]lights.Color{
		"single-dominant":   {{H: 210, S: 1.00, B: 0.50}},
		"single-vivid":      {{H: 210, S: 1.00, B: 0.50}},
		"single-saturated":  {{H: 199, S: 1.00, B: 0.99}},
		"single-brightest":  {{H: 60, S: 0.05, B: 1.00}},
		"palette-dominant":  {{H: 210, S: 1.00, B: 0.50}, {H: 210, S: 0.57, B: 0.88}, {H: 210, S: 0.29, B: 0.88}, {H: 0, S: 0.00, B: 0.63}},
		"palette-vivid":     {{H: 210, S: 1.00, B: 0.50}, {H: 210, S: 0.57, B: 0.88}, {H: 210, S: 1.00, B: 0.75}, {H: 197, S: 1.00, B: 0.88}},
		"palette-saturated": {{H: 210, S: 1.00, B: 0.50}, {H: 210, S: 0.57, B: 0.88}, {H: 210, S: 1.00, B: 0.75}, {H: 197, S: 1.00, B: 0.88}},
		"palette-brightest": {{H: 210, S: 0.57, B: 0.88}, {H: 210, S: 0.29, B: 0.88}, {H: 60, S: 0.17, B: 0.75}, {H: 210, S: 1.00, B: 0.50}},
		"palette-diverse":   {{H: 210, S: 1.00, B: 0.50}, {H: 0, S: 0.00, B: 0.88}, {H: 36, S: 1.00, B: 0.63}, {H: 192, S: 0.71, B: 0.88}},
		"spatial-dominant":  {{H: 210, S: 0.57, B: 0.88}, {H: 220, S: 0.43, B: 0.88}, {H: 0, S: 0.00, B: 0.75}, {H: 210, S: 1.00, B: 0.50}},
		"spatial-vivid":     {{H: 210, S: 0.57, B: 0.88}, {H: 220, S: 0.43, B: 0.88}, {H: 60, S: 0.17, B: 0.75}, {H: 210, S: 1.00, B: 0.50}},
		"spatial-saturated": {{H: 199, S: 1.00, B: 0.99}, {H: 199, S: 1.00, B: 0.93}, {H: 196, S: 1.00, B: 0.82}, {H: 202, S: 1.00, B: 0.84}},
		"spatial-brightest": {{H: 185, S: 0.23, B: 1.00}, {H: 186, S: 0.24, B: 1.00}, {H: 60, S: 0.05, B: 1.00}, {H: 94, S: 0.03, B: 1.00}},
	}
}

func expectOceanWarm() map[string][]lights.Color {
	return map[string][]lights.Color{
		"single-dominant":   {{H: 0, S: 0.00, B: 0.63}},
		"single-vivid":      {{H: 200, S: 0.75, B: 0.50}},
		"single-saturated":  {{H: 120, S: 1.00, B: 0.01}},
		"single-brightest":  {{H: 60, S: 0.01, B: 1.00}},
		"palette-dominant":  {{H: 0, S: 0.00, B: 0.63}, {H: 200, S: 0.75, B: 0.50}, {H: 0, S: 0.00, B: 0.25}, {H: 342, S: 0.25, B: 0.46}},
		"palette-vivid":     {{H: 200, S: 0.75, B: 0.50}, {H: 0, S: 0.00, B: 0.13}, {H: 0, S: 0.00, B: 0.63}, {H: 0, S: 0.00, B: 0.38}},
		"palette-saturated": {{H: 200, S: 0.75, B: 0.50}, {H: 342, S: 0.25, B: 0.42}, {H: 0, S: 0.00, B: 0.25}, {H: 0, S: 0.00, B: 0.50}},
		"palette-brightest": {{H: 0, S: 0.00, B: 0.63}, {H: 200, S: 0.75, B: 0.50}, {H: 0, S: 0.00, B: 0.38}, {H: 0, S: 0.00, B: 0.88}},
		"palette-diverse":   {{H: 0, S: 0.00, B: 0.63}, {H: 346, S: 0.44, B: 0.50}, {H: 195, S: 1.00, B: 0.50}, {H: 0, S: 0.33, B: 0.38}},
		"spatial-dominant":  {{H: 0, S: 0.00, B: 0.63}, {H: 200, S: 0.75, B: 0.50}, {H: 0, S: 0.00, B: 0.63}, {H: 0, S: 0.00, B: 0.13}},
		"spatial-vivid":     {{H: 200, S: 1.00, B: 0.38}, {H: 200, S: 0.75, B: 0.50}, {H: 0, S: 0.00, B: 0.63}, {H: 240, S: 1.00, B: 0.13}},
		"spatial-saturated": {{H: 203, S: 0.84, B: 0.61}, {H: 203, S: 0.84, B: 0.61}, {H: 203, S: 0.84, B: 0.61}, {H: 203, S: 0.84, B: 0.61}},
		"spatial-brightest": {{H: 100, S: 0.01, B: 1.00}, {H: 180, S: 0.03, B: 1.00}, {H: 60, S: 0.01, B: 1.00}, {H: 160, S: 0.01, B: 1.00}},
	}
}

func expectWarmOrange() map[string][]lights.Color {
	return map[string][]lights.Color{
		"single-dominant":   {{H: 30, S: 0.67, B: 0.38}},
		"single-vivid":      {{H: 30, S: 0.67, B: 0.38}},
		"single-saturated":  {{H: 33, S: 1.00, B: 0.18}},
		"single-brightest":  {{H: 60, S: 0.06, B: 1.00}},
		"palette-dominant":  {{H: 0, S: 0.00, B: 0.88}, {H: 60, S: 0.17, B: 0.75}, {H: 30, S: 0.67, B: 0.38}, {H: 60, S: 0.25, B: 0.50}},
		"palette-vivid":     {{H: 30, S: 0.67, B: 0.38}, {H: 0, S: 0.00, B: 0.88}, {H: 60, S: 0.17, B: 0.75}, {H: 60, S: 0.25, B: 0.50}},
		"palette-saturated": {{H: 30, S: 0.67, B: 0.38}, {H: 60, S: 1.00, B: 0.13}, {H: 60, S: 0.25, B: 0.50}, {H: 60, S: 0.17, B: 0.75}},
		"palette-brightest": {{H: 0, S: 0.00, B: 0.88}, {H: 60, S: 0.17, B: 0.75}, {H: 60, S: 0.25, B: 0.50}, {H: 30, S: 0.67, B: 0.38}},
		"palette-diverse":   {{H: 0, S: 0.00, B: 0.88}, {H: 15, S: 0.27, B: 0.59}, {H: 45, S: 0.80, B: 0.63}, {H: 0, S: 0.00, B: 0.25}},
		"spatial-dominant":  {{H: 0, S: 0.00, B: 0.88}, {H: 0, S: 0.00, B: 0.88}, {H: 60, S: 0.50, B: 0.25}, {H: 30, S: 0.67, B: 0.38}},
		"spatial-vivid":     {{H: 60, S: 0.17, B: 0.75}, {H: 0, S: 0.00, B: 0.88}, {H: 30, S: 0.67, B: 0.38}, {H: 30, S: 0.67, B: 0.38}},
		"spatial-saturated": {{H: 27, S: 0.63, B: 0.14}, {H: 32, S: 0.59, B: 0.42}, {H: 33, S: 1.00, B: 0.18}, {H: 41, S: 1.00, B: 0.20}},
		"spatial-brightest": {{H: 46, S: 0.09, B: 1.00}, {H: 60, S: 0.06, B: 1.00}, {H: 39, S: 0.33, B: 0.95}, {H: 42, S: 0.25, B: 0.99}},
	}
}

func expectLandscapeMixed() map[string][]lights.Color {
	return map[string][]lights.Color{
		"single-dominant":   {{H: 210, S: 0.40, B: 0.63}},
		"single-vivid":      {{H: 210, S: 0.40, B: 0.63}},
		"single-saturated":  {{H: 45, S: 1.00, B: 0.06}},
		"single-brightest":  {{H: 0, S: 0.00, B: 1.00}},
		"palette-dominant":  {{H: 210, S: 0.40, B: 0.63}, {H: 9, S: 0.33, B: 0.63}, {H: 0, S: 0.00, B: 0.63}, {H: 40, S: 0.60, B: 0.63}},
		"palette-vivid":     {{H: 210, S: 0.40, B: 0.63}, {H: 30, S: 0.57, B: 0.88}, {H: 40, S: 0.60, B: 0.63}, {H: 30, S: 1.00, B: 0.25}},
		"palette-saturated": {{H: 210, S: 0.40, B: 0.63}, {H: 40, S: 0.72, B: 0.46}, {H: 30, S: 1.00, B: 0.25}, {H: 40, S: 0.75, B: 0.50}},
		"palette-brightest": {{H: 210, S: 0.40, B: 0.63}, {H: 30, S: 0.57, B: 0.88}, {H: 0, S: 0.00, B: 0.63}, {H: 40, S: 0.60, B: 0.63}},
		"palette-diverse":   {{H: 210, S: 0.40, B: 0.63}, {H: 9, S: 0.42, B: 0.79}, {H: 40, S: 0.86, B: 0.88}, {H: 0, S: 0.00, B: 0.88}},
		"spatial-dominant":  {{H: 210, S: 0.33, B: 0.75}, {H: 210, S: 0.40, B: 0.63}, {H: 45, S: 0.57, B: 0.88}, {H: 45, S: 0.67, B: 0.75}},
		"spatial-vivid":     {{H: 220, S: 0.50, B: 0.75}, {H: 210, S: 0.40, B: 0.63}, {H: 45, S: 0.57, B: 0.88}, {H: 45, S: 0.67, B: 0.75}},
		"spatial-saturated": {{H: 28, S: 0.89, B: 0.24}, {H: 45, S: 1.00, B: 0.06}, {H: 28, S: 1.00, B: 0.13}, {H: 18, S: 1.00, B: 0.16}},
		"spatial-brightest": {{H: 60, S: 0.05, B: 1.00}, {H: 70, S: 0.05, B: 0.93}, {H: 60, S: 0.02, B: 1.00}, {H: 0, S: 0.00, B: 1.00}},
	}
}

func expectOliveTan() map[string][]lights.Color {
	return map[string][]lights.Color{
		"single-dominant":   {{H: 60, S: 0.50, B: 0.25}},
		"single-vivid":      {{H: 60, S: 0.50, B: 0.25}},
		"single-saturated":  {{H: 78, S: 1.00, B: 0.13}},
		"single-brightest":  {{H: 62, S: 0.11, B: 0.98}},
		"palette-dominant":  {{H: 60, S: 0.50, B: 0.25}, {H: 0, S: 0.00, B: 0.38}, {H: 0, S: 0.00, B: 0.75}, {H: 60, S: 0.20, B: 0.63}},
		"palette-vivid":     {{H: 60, S: 0.50, B: 0.25}, {H: 60, S: 0.25, B: 0.50}, {H: 0, S: 0.00, B: 0.75}, {H: 80, S: 0.75, B: 0.50}},
		"palette-saturated": {{H: 60, S: 0.50, B: 0.25}, {H: 60, S: 0.25, B: 0.50}, {H: 80, S: 0.75, B: 0.50}, {H: 0, S: 0.17, B: 0.75}},
		"palette-brightest": {{H: 60, S: 0.50, B: 0.25}, {H: 0, S: 0.00, B: 0.75}, {H: 60, S: 0.20, B: 0.63}, {H: 0, S: 0.00, B: 0.38}},
		"palette-diverse":   {{H: 60, S: 0.50, B: 0.25}, {H: 0, S: 0.00, B: 0.88}, {H: 0, S: 0.20, B: 0.63}, {H: 19, S: 0.23, B: 0.59}},
		"spatial-dominant":  {{H: 60, S: 0.50, B: 0.25}, {H: 60, S: 0.50, B: 0.25}, {H: 0, S: 0.00, B: 0.13}, {H: 0, S: 0.00, B: 0.13}},
		"spatial-vivid":     {{H: 60, S: 0.50, B: 0.25}, {H: 60, S: 0.50, B: 0.25}, {H: 0, S: 0.00, B: 0.13}, {H: 0, S: 0.00, B: 0.13}},
		"spatial-saturated": {{H: 71, S: 0.83, B: 0.52}, {H: 73, S: 0.73, B: 0.54}, {H: 78, S: 1.00, B: 0.13}, {H: 89, S: 0.76, B: 0.16}},
		"spatial-brightest": {{H: 62, S: 0.11, B: 0.98}, {H: 46, S: 0.11, B: 0.95}, {H: 71, S: 0.09, B: 0.96}, {H: 28, S: 0.05, B: 0.97}},
	}
}

func expectViolet() map[string][]lights.Color {
	return map[string][]lights.Color{
		"single-dominant":   {{H: 0, S: 0.00, B: 0.13}},
		"single-vivid":      {{H: 270, S: 0.29, B: 0.88}},
		"single-saturated":  {{H: 274, S: 0.70, B: 0.38}},
		"single-brightest":  {{H: 270, S: 0.04, B: 0.96}},
		"palette-dominant":  {{H: 0, S: 0.00, B: 0.13}, {H: 270, S: 0.29, B: 0.88}, {H: 260, S: 0.60, B: 0.63}, {H: 280, S: 0.50, B: 0.75}},
		"palette-vivid":     {{H: 270, S: 0.29, B: 0.88}, {H: 260, S: 0.60, B: 0.63}, {H: 0, S: 0.00, B: 0.13}, {H: 280, S: 0.50, B: 0.75}},
		"palette-saturated": {{H: 270, S: 0.29, B: 0.88}, {H: 260, S: 0.60, B: 0.63}, {H: 0, S: 0.00, B: 0.13}, {H: 280, S: 0.50, B: 0.75}},
		"palette-brightest": {{H: 0, S: 0.00, B: 0.13}, {H: 270, S: 0.29, B: 0.88}, {H: 280, S: 0.50, B: 0.75}, {H: 260, S: 0.60, B: 0.63}},
		"palette-diverse":   {{H: 0, S: 0.00, B: 0.13}, {H: 0, S: 0.00, B: 0.88}, {H: 270, S: 0.67, B: 0.75}, {H: 45, S: 0.67, B: 0.75}},
		"spatial-dominant":  {{H: 0, S: 0.00, B: 0.13}, {H: 0, S: 0.00, B: 0.13}, {H: 0, S: 0.00, B: 0.13}, {H: 0, S: 0.00, B: 0.13}},
		"spatial-vivid":     {{H: 270, S: 0.29, B: 0.88}, {H: 270, S: 0.29, B: 0.88}, {H: 270, S: 0.29, B: 0.88}, {H: 260, S: 0.60, B: 0.63}},
		"spatial-saturated": {{H: 266, S: 0.66, B: 0.76}, {H: 274, S: 0.70, B: 0.38}, {H: 42, S: 0.66, B: 0.57}, {H: 23, S: 0.65, B: 0.42}},
		"spatial-brightest": {{H: 270, S: 0.04, B: 0.96}, {H: 287, S: 0.18, B: 0.99}, {H: 287, S: 0.13, B: 1.00}, {H: 40, S: 0.48, B: 0.96}},
	}
}

func expectGreyscale() map[string][]lights.Color {
	return map[string][]lights.Color{
		"single-dominant":   {{H: 0, S: 0.00, B: 0.00}},
		"single-vivid":      {{H: 0, S: 0.00, B: 0.00}},
		"single-saturated":  {{H: 0, S: 0.00, B: 0.02}},
		"single-brightest":  {{H: 0, S: 0.00, B: 1.00}},
		"palette-dominant":  {{H: 0, S: 0.00, B: 0.54}, {H: 0, S: 0.00, B: 0.25}, {H: 0, S: 0.00, B: 0.50}, {H: 0, S: 0.00, B: 0.88}},
		"palette-vivid":     {{H: 0, S: 0.00, B: 0.54}, {H: 0, S: 0.00, B: 0.25}, {H: 0, S: 0.00, B: 0.50}, {H: 0, S: 0.00, B: 0.88}},
		"palette-saturated": {{H: 0, S: 0.00, B: 0.54}, {H: 0, S: 0.00, B: 0.25}, {H: 0, S: 0.00, B: 0.50}, {H: 0, S: 0.00, B: 0.88}},
		"palette-brightest": {{H: 0, S: 0.00, B: 0.59}, {H: 0, S: 0.00, B: 0.25}, {H: 0, S: 0.00, B: 0.63}, {H: 0, S: 0.00, B: 0.88}},
		"palette-diverse":   {{H: 0, S: 0.00, B: 0.63}, {H: 0, S: 0.00, B: 0.88}, {H: 0, S: 0.00, B: 0.38}, {H: 0, S: 0.00, B: 0.63}},
		"spatial-dominant":  {{H: 0, S: 0.00, B: 0.50}, {H: 0, S: 0.00, B: 0.50}, {H: 0, S: 0.00, B: 0.50}, {H: 0, S: 0.00, B: 0.50}},
		"spatial-vivid":     {{H: 0, S: 0.00, B: 0.50}, {H: 0, S: 0.00, B: 0.50}, {H: 0, S: 0.00, B: 0.50}, {H: 0, S: 0.00, B: 0.50}},
		"spatial-saturated": {{H: 0, S: 0.00, B: 0.37}, {H: 0, S: 0.00, B: 0.24}, {H: 0, S: 0.00, B: 0.37}, {H: 0, S: 0.00, B: 0.51}},
		"spatial-brightest": {{H: 0, S: 0.00, B: 1.00}, {H: 0, S: 0.00, B: 1.00}, {H: 0, S: 0.00, B: 1.00}, {H: 0, S: 0.00, B: 1.00}},
	}
}
