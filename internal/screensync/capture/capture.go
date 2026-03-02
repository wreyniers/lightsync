// Package capture provides screen capture implementations for the Screen Sync engine.
package capture

import (
	"fmt"
	"image"

	"lightsync/internal/store"
)

// Capturer captures a single frame from a screen source.
type Capturer interface {
	// Capture returns the current frame as an image. The caller should not
	// modify the returned image concurrently with the next Capture call.
	Capture() (image.Image, error)
	// Close releases any OS resources held by the capturer.
	Close()
}

// MonitorInfo describes a display known to the OS.
type MonitorInfo struct {
	Index     int    `json:"index"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	IsPrimary bool   `json:"isPrimary"`
	Name      string `json:"name"`
}

// WindowInfo describes a visible application window.
type WindowInfo struct {
	HWND    uint64 `json:"hwnd"`
	Title   string `json:"title"`
	ExeName string `json:"exeName"`
}

// NewCapturer creates the appropriate Capturer for the given config.
func NewCapturer(cfg store.ScreenSyncConfig) (Capturer, error) {
	switch cfg.CaptureMode {
	case store.CaptureModeMonitor:
		return NewMonitorCapturer(cfg.MonitorIndex)
	case store.CaptureModeRegion:
		return NewRegionCapturer(cfg.Region)
	case store.CaptureModeWindow:
		return newWindowCapturer(cfg.WindowHWND)
	case store.CaptureModeActiveWindow:
		return newActiveWindowCapturer()
	default:
		return NewMonitorCapturer(0)
	}
}

// downsampleFactor is the fraction of pixels sampled during capture.
// 0.25 reduces a 1920×1080 image to 480×270, sufficient for color extraction.
const downsampleFactor = 0.25

// errUnsupported is returned when a capture mode is not available on this OS.
var errUnsupported = fmt.Errorf("capture mode not supported on this platform")
