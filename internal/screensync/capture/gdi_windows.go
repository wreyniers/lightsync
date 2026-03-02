//go:build windows

package capture

import (
	"fmt"
	"image"
	"unsafe"
)

// gdiScaler holds reusable GDI resources for StretchBlt-based screen capture.
// Allocating these once and reusing across frames eliminates 6 kernel
// transitions (GetDC, CreateCompatibleDC, CreateCompatibleBitmap,
// DeleteObject, DeleteDC, ReleaseDC) per frame that were costing ~30-40ms.
type gdiScaler struct {
	screenDC uintptr
	memDC    uintptr
	hBmp     uintptr
	buf      []byte
	dstW     int
	dstH     int
}

func newGDIScaler(dstW, dstH int) (*gdiScaler, error) {
	screenDC, _, _ := procGetDC.Call(0)
	if screenDC == 0 {
		return nil, fmt.Errorf("GetDC failed")
	}

	memDC, _, _ := procCreateCompatibleDC.Call(screenDC)
	if memDC == 0 {
		procReleaseDC.Call(0, screenDC)
		return nil, fmt.Errorf("CreateCompatibleDC failed")
	}

	hBmp, _, _ := procCreateCompatibleBitmap.Call(screenDC, uintptr(dstW), uintptr(dstH))
	if hBmp == 0 {
		procDeleteDC.Call(memDC)
		procReleaseDC.Call(0, screenDC)
		return nil, fmt.Errorf("CreateCompatibleBitmap failed")
	}

	procSelectObject.Call(memDC, hBmp)
	procSetStretchBltMode.Call(memDC, colorOnColor)

	return &gdiScaler{
		screenDC: screenDC,
		memDC:    memDC,
		hBmp:     hBmp,
		buf:      make([]byte, dstW*dstH*4),
		dstW:     dstW,
		dstH:     dstH,
	}, nil
}

func (g *gdiScaler) capture(srcX, srcY, srcW, srcH int) (*image.RGBA, error) {
	ret, _, _ := procStretchBlt.Call(
		g.memDC, 0, 0, uintptr(g.dstW), uintptr(g.dstH),
		g.screenDC, uintptr(srcX), uintptr(srcY), uintptr(srcW), uintptr(srcH),
		srcCopy,
	)
	if ret == 0 {
		return nil, fmt.Errorf("StretchBlt failed")
	}

	bi := bitmapInfo{
		BmiHeader: bitmapInfoHeader{
			BiSize:        bitmapInfoHeaderSize,
			BiWidth:       int32(g.dstW),
			BiHeight:      -int32(g.dstH),
			BiPlanes:      1,
			BiBitCount:    32,
			BiCompression: biCompression,
		},
	}

	ret, _, _ = procGetDIBits.Call(
		g.memDC, g.hBmp,
		0, uintptr(g.dstH),
		uintptr(unsafe.Pointer(&g.buf[0])),
		uintptr(unsafe.Pointer(&bi)),
		dibRGBColors,
	)
	if ret == 0 {
		return nil, fmt.Errorf("GetDIBits failed")
	}

	img := image.NewRGBA(image.Rect(0, 0, g.dstW, g.dstH))
	for i := 0; i < len(g.buf); i += 4 {
		img.Pix[i+0] = g.buf[i+2]
		img.Pix[i+1] = g.buf[i+1]
		img.Pix[i+2] = g.buf[i+0]
		img.Pix[i+3] = 0xFF
	}
	return img, nil
}

func (g *gdiScaler) close() {
	if g.hBmp != 0 {
		procDeleteObject.Call(g.hBmp)
	}
	if g.memDC != 0 {
		procDeleteDC.Call(g.memDC)
	}
	if g.screenDC != 0 {
		procReleaseDC.Call(0, g.screenDC)
	}
}
