//go:build !windows

package capture

import (
	"image"
	"image/draw"

	"github.com/kbinani/screenshot"
	xdraw "golang.org/x/image/draw"
)

// captureScaled grabs the screen region and scales it to dstW×dstH.
// On non-Windows platforms we fall back to a software resize of the full-res
// screenshot since the StretchBlt fast-path is Windows-only.
func captureScaled(srcX, srcY, srcW, srcH, dstW, dstH int) (*image.RGBA, error) {
	full, err := screenshot.CaptureRect(image.Rect(srcX, srcY, srcX+srcW, srcY+srcH))
	if err != nil {
		return nil, err
	}

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	xdraw.BiLinear.Scale(dst, dst.Bounds(), full, full.Bounds(), draw.Src, nil)
	return dst, nil
}
