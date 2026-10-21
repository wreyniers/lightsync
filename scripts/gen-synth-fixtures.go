//go:build ignore

// Generates the synthetic test fixtures used by the in-app Self-Test mode.
// Run with:
//
//   go run scripts/gen-synth-fixtures.go
//
// Outputs four PNGs into frontend/public/testdata/. Idempotent — running it
// again just overwrites the same files bit-for-bit.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

// 640×480 matches the picsum photos so the Self-Test popup can show any
// fixture 1:1 with no DWM bilinear scaling and no boundary blending. The
// popup is also sized 640×480 and captured as a Region at (0,0,640,480),
// so every captured pixel is pure fixture content and the baselines in
// selftest/fixtures.go are reproducible on any machine regardless of
// monitor size, DPI, or taskbar position.
const (
	outDir = "frontend/public/testdata"
	width  = 640
	height = 480
)

func solid(r, g, b uint8) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	c := color.RGBA{R: r, G: g, B: b, A: 255}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func quadrants() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	halfW, halfH := width/2, height/2 // 320, 240
	colors := [4]color.RGBA{
		{R: 230, G: 40, B: 40, A: 255},  // TL red
		{R: 40, G: 200, B: 60, A: 255},  // TR green
		{R: 40, G: 90, B: 230, A: 255},  // BL blue
		{R: 240, G: 220, B: 40, A: 255}, // BR yellow
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := 0
			if x >= halfW {
				idx |= 1
			}
			if y >= halfH {
				idx |= 2
			}
			img.SetRGBA(x, y, colors[idx])
		}
	}
	return img
}

func write(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func main() {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fixtures := map[string]*image.RGBA{
		"synth_red.png":       solid(230, 40, 40),
		"synth_green.png":     solid(40, 200, 60),
		"synth_blue.png":      solid(40, 90, 230),
		"synth_quadrants.png": quadrants(),
	}
	for name, img := range fixtures {
		path := filepath.Join(outDir, name)
		if err := write(path, img); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", path, err)
			os.Exit(1)
		}
		fmt.Printf("wrote %s\n", path)
	}
}
