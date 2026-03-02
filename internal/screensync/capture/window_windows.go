//go:build windows

package capture

import (
	"fmt"
	"image"
	"syscall"
	"unsafe"
)

var (
	user32dll   = syscall.NewLazyDLL("user32.dll")
	gdi32dll    = syscall.NewLazyDLL("gdi32.dll")
	kernel32dll = syscall.NewLazyDLL("kernel32.dll")

	procGetWindowDC            = user32dll.NewProc("GetWindowDC")
	procGetDC                  = user32dll.NewProc("GetDC")
	procReleaseDC              = user32dll.NewProc("ReleaseDC")
	procGetClientRect          = user32dll.NewProc("GetClientRect")
	procCreateCompatibleDC     = gdi32dll.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = gdi32dll.NewProc("CreateCompatibleBitmap")
	procSelectObject           = gdi32dll.NewProc("SelectObject")
	procBitBlt                 = gdi32dll.NewProc("BitBlt")
	procStretchBlt             = gdi32dll.NewProc("StretchBlt")
	procSetStretchBltMode      = gdi32dll.NewProc("SetStretchBltMode")
	procDeleteObject           = gdi32dll.NewProc("DeleteObject")
	procDeleteDC               = gdi32dll.NewProc("DeleteDC")
	procGetDIBits              = gdi32dll.NewProc("GetDIBits")
)

const (
	bitmapInfoHeaderSize = 40
	biBitCount           = 32
	biCompression        = 0 // BI_RGB
	srcCopy              = 0x00CC0020
	pwRenderfullcontent  = 2
	colorOnColor         = 3 // COLORONCOLOR — fast, drops intermediate pixels
	halftoneMode         = 4 // HALFTONE — slow bilinear, kept for reference
	dibRGBColors         = 0
)

type rect struct {
	Left, Top, Right, Bottom int32
}

type bitmapInfoHeader struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

type bitmapInfo struct {
	BmiHeader bitmapInfoHeader
	BmiColors [1]uint32
}

var (
	procClientToScreen  = user32dll.NewProc("ClientToScreen")
	procMonitorFromWindow = user32dll.NewProc("MonitorFromWindow")
	procGetMonitorInfoW   = user32dll.NewProc("GetMonitorInfoW")
)

const monitorDefaultToNearest = 2

type screenPoint struct{ X, Y int32 }

type monitorInfoEx struct {
	CbSize    uint32
	RcMonitor rect
	RcWork    rect
	DwFlags   uint32
	SzDevice  [32]uint16
}

// WindowCapturer captures a specific window via DXGI Desktop Duplication,
// cropping the monitor capture to the window's client area. Falls back to
// GDI BitBlt if DXGI is unavailable.
type WindowCapturer struct {
	hwnd       uintptr
	dxgi       *dxgiCapturer
	dxgiFailed bool
}

func newWindowCapturer(hwnd uint64) (*WindowCapturer, error) {
	if hwnd == 0 {
		return nil, fmt.Errorf("invalid window handle")
	}
	return &WindowCapturer{hwnd: uintptr(hwnd)}, nil
}

func (w *WindowCapturer) Capture() (image.Image, error) {
	// Get the window's client area in screen coordinates.
	screenX, screenY, clientW, clientH, err := windowClientScreenRect(w.hwnd)
	if err != nil || clientW <= 0 || clientH <= 0 {
		return captureHWND(w.hwnd)
	}

	dstW := captureTargetWidth
	dstH := captureTargetWidth * clientH / clientW
	if dstH < 1 {
		dstH = 1
	}

	// Try DXGI.
	if w.dxgi != nil {
		// Resize output buffer if window aspect ratio changed.
		if w.dxgi.dstW != dstW || w.dxgi.dstH != dstH {
			w.dxgi.smallBuf = image.NewRGBA(image.Rect(0, 0, dstW, dstH))
			w.dxgi.dstW = dstW
			w.dxgi.dstH = dstH
		}
		img, err := w.dxgi.captureRect(screenX, screenY, clientW, clientH)
		if err == nil {
			return img, nil
		}
		w.dxgi.close()
		w.dxgi = nil
	}

	if !w.dxgiFailed {
		monIdx := monitorIndexForWindow(w.hwnd)
		dxgi, err := newDXGICapturer(monIdx, dstW, dstH)
		if err != nil {
			w.dxgiFailed = true
		} else {
			w.dxgi = dxgi
			img, err := w.dxgi.captureRect(screenX, screenY, clientW, clientH)
			if err == nil {
				return img, nil
			}
			w.dxgi.close()
			w.dxgi = nil
			w.dxgiFailed = true
		}
	}

	// GDI fallback.
	return captureHWND(w.hwnd)
}

func (w *WindowCapturer) Close() {
	if w.dxgi != nil {
		w.dxgi.close()
		w.dxgi = nil
	}
}

// windowClientScreenRect returns the client area of hwnd in screen coordinates.
func windowClientScreenRect(hwnd uintptr) (x, y, w, h int, err error) {
	var cr rect
	ret, _, e := procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&cr)))
	if ret == 0 {
		return 0, 0, 0, 0, fmt.Errorf("GetClientRect: %w", e)
	}

	var pt screenPoint
	ret, _, e = procClientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&pt)))
	if ret == 0 {
		return 0, 0, 0, 0, fmt.Errorf("ClientToScreen: %w", e)
	}

	return int(pt.X), int(pt.Y), int(cr.Right - cr.Left), int(cr.Bottom - cr.Top), nil
}

// monitorIndexForWindow returns the screenshot library monitor index for the
// monitor that contains most of the given window.
func monitorIndexForWindow(hwnd uintptr) int {
	hMon, _, _ := procMonitorFromWindow.Call(hwnd, monitorDefaultToNearest)
	if hMon == 0 {
		return 0
	}

	var mi monitorInfoEx
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	ret, _, _ := procGetMonitorInfoW.Call(hMon, uintptr(unsafe.Pointer(&mi)))
	if ret == 0 {
		return 0
	}

	monLeft := int(mi.RcMonitor.Left)
	monTop := int(mi.RcMonitor.Top)

	// Match against the screenshot library's display list.
	monitors := GetMonitors()
	for _, m := range monitors {
		if m.X == monLeft && m.Y == monTop {
			return m.Index
		}
	}
	return 0
}

func captureHWND(hwnd uintptr) (image.Image, error) {
	// Get the client area dimensions.
	var r rect
	ret, _, err := procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	if ret == 0 {
		return nil, fmt.Errorf("GetClientRect failed: %w", err)
	}
	width := int(r.Right - r.Left)
	height := int(r.Bottom - r.Top)
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("window has zero dimensions")
	}

	// Get the window's device context.
	srcDC, _, _ := procGetWindowDC.Call(hwnd)
	if srcDC == 0 {
		return nil, fmt.Errorf("GetWindowDC failed")
	}
	defer procReleaseDC.Call(hwnd, srcDC)

	// Create a compatible DC and bitmap.
	memDC, _, _ := procCreateCompatibleDC.Call(srcDC)
	if memDC == 0 {
		return nil, fmt.Errorf("CreateCompatibleDC failed")
	}
	defer procDeleteDC.Call(memDC)

	hBmp, _, _ := procCreateCompatibleBitmap.Call(srcDC, uintptr(width), uintptr(height))
	if hBmp == 0 {
		return nil, fmt.Errorf("CreateCompatibleBitmap failed")
	}
	defer procDeleteObject.Call(hBmp)

	procSelectObject.Call(memDC, hBmp)

	// BitBlt the window contents into our bitmap.
	const srccopy = 0x00CC0020
	ret, _, err = procBitBlt.Call(memDC, 0, 0, uintptr(width), uintptr(height), srcDC, 0, 0, srccopy)
	if ret == 0 {
		return nil, fmt.Errorf("BitBlt failed: %w", err)
	}

	// Extract pixel data via GetDIBits.
	bi := bitmapInfo{
		BmiHeader: bitmapInfoHeader{
			BiSize:        bitmapInfoHeaderSize,
			BiWidth:       int32(width),
			BiHeight:      -int32(height), // top-down
			BiPlanes:      1,
			BiBitCount:    32,
			BiCompression: biCompression,
		},
	}

	buf := make([]byte, width*height*4)
	ret, _, err = procGetDIBits.Call(
		memDC,
		hBmp,
		0, uintptr(height),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&bi)),
		0, // DIB_RGB_COLORS
	)
	if ret == 0 {
		return nil, fmt.Errorf("GetDIBits failed: %w", err)
	}

	// Build image.RGBA (GetDIBits returns BGRA on Windows).
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < width*height; i++ {
		b := buf[i*4+0]
		g := buf[i*4+1]
		r := buf[i*4+2]
		img.Pix[i*4+0] = r
		img.Pix[i*4+1] = g
		img.Pix[i*4+2] = b
		img.Pix[i*4+3] = 255
	}
	return img, nil
}

// CaptureThumbnail captures a downscaled thumbnail of a window (~200×150).
func CaptureThumbnail(hwnd uint64) (image.Image, error) {
	return captureHWND(uintptr(hwnd))
}

// captureScaled grabs the screen region (srcX, srcY, srcW×srcH) and scales
// it to dstW×dstH in a single GDI StretchBlt call. This is dramatically
// faster than a full-resolution BitBlt because the GPU→CPU readback only
// transfers dstW×dstH×4 bytes (e.g. 230 KB for 320×180) instead of the full
// native resolution (e.g. 33 MB for 4K).
func captureScaled(srcX, srcY, srcW, srcH, dstW, dstH int) (*image.RGBA, error) {
	// Acquire the desktop screen DC (hwnd=0 means entire virtual screen).
	screenDC, _, _ := procGetDC.Call(0)
	if screenDC == 0 {
		return nil, fmt.Errorf("GetDC failed")
	}
	defer procReleaseDC.Call(0, screenDC)

	memDC, _, _ := procCreateCompatibleDC.Call(screenDC)
	if memDC == 0 {
		return nil, fmt.Errorf("CreateCompatibleDC failed")
	}
	defer procDeleteDC.Call(memDC)

	hBmp, _, _ := procCreateCompatibleBitmap.Call(screenDC, uintptr(dstW), uintptr(dstH))
	if hBmp == 0 {
		return nil, fmt.Errorf("CreateCompatibleBitmap failed")
	}
	defer procDeleteObject.Call(hBmp)

	procSelectObject.Call(memDC, hBmp)

	// COLORONCOLOR drops intermediate rows/cols — much faster than HALFTONE
	// and perfectly sufficient for colour extraction.
	procSetStretchBltMode.Call(memDC, colorOnColor)

	ret, _, _ := procStretchBlt.Call(
		memDC, 0, 0, uintptr(dstW), uintptr(dstH),
		screenDC, uintptr(srcX), uintptr(srcY), uintptr(srcW), uintptr(srcH),
		srcCopy,
	)
	if ret == 0 {
		return nil, fmt.Errorf("StretchBlt failed")
	}

	bi := bitmapInfo{
		BmiHeader: bitmapInfoHeader{
			BiSize:        bitmapInfoHeaderSize,
			BiWidth:       int32(dstW),
			BiHeight:      -int32(dstH), // negative = top-down row order
			BiPlanes:      1,
			BiBitCount:    32,
			BiCompression: biCompression,
		},
	}

	buf := make([]byte, dstW*dstH*4)
	ret, _, _ = procGetDIBits.Call(
		memDC, hBmp,
		0, uintptr(dstH),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&bi)),
		dibRGBColors,
	)
	if ret == 0 {
		return nil, fmt.Errorf("GetDIBits failed")
	}

	// GDI returns pixels in BGRA order; convert to RGBA.
	img := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	for i := 0; i < len(buf); i += 4 {
		img.Pix[i+0] = buf[i+2] // R ← B
		img.Pix[i+1] = buf[i+1] // G
		img.Pix[i+2] = buf[i+0] // B ← R
		img.Pix[i+3] = 0xFF
	}
	return img, nil
}
