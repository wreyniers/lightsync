//go:build windows

package capture

import (
	"fmt"
	"image"
	"syscall"
	"unsafe"
)

var (
	procGetForegroundWindow = user32dll.NewProc("GetForegroundWindow")
)

// ActiveWindowCapturer captures whichever window currently has foreground focus
// via DXGI Desktop Duplication, cropping to the window's client area.
type ActiveWindowCapturer struct {
	lastHWND uintptr
	dxgi     *dxgiCapturer
	lastMon  int // monitor index the DXGI capturer was created for
}

func newActiveWindowCapturer() (*ActiveWindowCapturer, error) {
	return &ActiveWindowCapturer{lastMon: -1}, nil
}

func (a *ActiveWindowCapturer) Capture() (image.Image, error) {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return nil, fmt.Errorf("no foreground window")
	}
	a.lastHWND = hwnd

	screenX, screenY, clientW, clientH, err := windowClientScreenRect(hwnd)
	if err != nil || clientW <= 0 || clientH <= 0 {
		return captureHWND(hwnd)
	}

	dstW := captureTargetWidth
	dstH := captureTargetWidth * clientH / clientW
	if dstH < 1 {
		dstH = 1
	}

	monIdx := monitorIndexForWindow(hwnd)

	// Recreate DXGI capturer if the window moved to a different monitor.
	if a.dxgi != nil && monIdx != a.lastMon {
		a.dxgi.close()
		a.dxgi = nil
	}

	if a.dxgi != nil {
		if a.dxgi.dstW != dstW || a.dxgi.dstH != dstH {
			a.dxgi.smallBuf = image.NewRGBA(image.Rect(0, 0, dstW, dstH))
			a.dxgi.dstW = dstW
			a.dxgi.dstH = dstH
		}
		img, err := a.dxgi.captureRect(screenX, screenY, clientW, clientH)
		if err == nil {
			return img, nil
		}
		a.dxgi.close()
		a.dxgi = nil
	}

	dxgi, err := newDXGICapturer(monIdx, dstW, dstH)
	if err != nil {
		return captureHWND(hwnd)
	}
	a.dxgi = dxgi
	a.lastMon = monIdx

	img, err := a.dxgi.captureRect(screenX, screenY, clientW, clientH)
	if err != nil {
		return captureHWND(hwnd)
	}
	return img, nil
}

func (a *ActiveWindowCapturer) Close() {
	if a.dxgi != nil {
		a.dxgi.close()
		a.dxgi = nil
	}
}

func (a *ActiveWindowCapturer) LastHWND() uintptr { return a.lastHWND }

func GetForegroundWindowHWND() uint64 {
	hwnd, _, _ := procGetForegroundWindow.Call()
	return uint64(hwnd)
}

func GetWindowTitle(hwnd uintptr) string {
	procGetWindowText := user32dll.NewProc("GetWindowTextW")
	buf := make([]uint16, 256)
	procGetWindowText.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), 256)
	return syscall.UTF16ToString(buf)
}
