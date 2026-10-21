package extract

import (
	"image"
	"testing"
)

func tieQuadrantsImage() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 128, 128))
	halfW, halfH := 64, 64
	// TL red, TR green, BL blue, BR yellow.
	colors := [4][3]uint8{
		{230, 40, 40},
		{40, 200, 60},
		{40, 90, 230},
		{240, 220, 40},
	}
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
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

func uniqueResultCount(values []lightsColorKey) int {
	seen := make(map[lightsColorKey]struct{}, len(values))
	for _, v := range values {
		seen[v] = struct{}{}
	}
	return len(seen)
}

type lightsColorKey struct {
	H int
	S int
	B int
}

func toColorKey(h, s, b float64) lightsColorKey {
	return lightsColorKey{
		H: int(h*10 + 0.5),
		S: int(s*1000 + 0.5),
		B: int(b*1000 + 0.5),
	}
}

func TestDominantColor_IsDeterministicOnTies(t *testing.T) {
	img := tieQuadrantsImage()
	results := make([]lightsColorKey, 0, 128)
	for i := 0; i < 128; i++ {
		c := dominantColor(img, 0)
		results = append(results, toColorKey(c.H, c.S, c.B))
	}
	if got := uniqueResultCount(results); got != 1 {
		t.Fatalf("dominantColor is non-deterministic on tie-heavy frame: got %d unique outputs", got)
	}
}

func TestVividDominantColor_IsDeterministicOnTies(t *testing.T) {
	img := tieQuadrantsImage()
	results := make([]lightsColorKey, 0, 128)
	for i := 0; i < 128; i++ {
		c := vividDominantColor(img, 0)
		results = append(results, toColorKey(c.H, c.S, c.B))
	}
	if got := uniqueResultCount(results); got != 1 {
		t.Fatalf("vividDominantColor is non-deterministic on tie-heavy frame: got %d unique outputs", got)
	}
}
