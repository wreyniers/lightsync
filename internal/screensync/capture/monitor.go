package capture

import (
	"fmt"
	"image"
	"log"

	"github.com/kbinani/screenshot"
)

// captureTargetWidth is the width (pixels) we down-sample to before extraction.
// Color extraction needs very few pixels; capturing at native 4K resolution and
// reading it back from the GPU via GDI takes ~180 ms. Scaling to 192 px wide
// transfers only ~83 KB instead of ~33 MB for a 4K display.
const captureTargetWidth = 192

// MonitorCapturer captures the full output of a single display.
// It tries DXGI Desktop Duplication first (fast, GPU-direct) and falls back to
// GDI StretchBlt if DXGI is unavailable (Windows 7, RDP, driver issues).
type MonitorCapturer struct {
	index  int
	dxgi   *dxgiCapturer   // nil when DXGI isn't available
	scaler scaledCapturer  // GDI fallback; nil until needed
	dxgiFailed bool        // true after DXGI init failed; skip retrying
}

// scaledCapturer is implemented by gdiScaler on Windows. On other platforms
// it stays nil and the fallback captureScaled function is used.
type scaledCapturer interface {
	capture(srcX, srcY, srcW, srcH int) (*image.RGBA, error)
	close()
}

// NewMonitorCapturer creates a capturer for the monitor at the given index.
// Index 0 is the primary monitor.
func NewMonitorCapturer(index int) (*MonitorCapturer, error) {
	n := screenshot.NumActiveDisplays()
	if n == 0 {
		return nil, fmt.Errorf("no active displays found")
	}
	if index < 0 || index >= n {
		index = 0
	}
	return &MonitorCapturer{index: index}, nil
}

func (m *MonitorCapturer) Capture() (image.Image, error) {
	n := screenshot.NumActiveDisplays()
	if m.index >= n {
		m.index = 0
	}
	bounds := screenshot.GetDisplayBounds(m.index)
	srcW, srcH := bounds.Dx(), bounds.Dy()
	dstW := captureTargetWidth
	dstH := captureTargetWidth * srcH / srcW
	if dstH < 1 {
		dstH = 1
	}

	// ── Fast path: DXGI Desktop Duplication ──────────────────────────────
	if m.dxgi != nil {
		img, err := m.dxgi.capture()
		if err == nil {
			return img, nil
		}
		// DXGI lost (resolution change, display disconnect, etc.).
		log.Printf("[capture] DXGI capture error, recreating: %v", err)
		m.dxgi.close()
		m.dxgi = nil
	}

	if !m.dxgiFailed {
		dxgi, err := newDXGICapturer(m.index, dstW, dstH)
		if err != nil {
			log.Printf("[capture] DXGI init failed, using GDI fallback: %v", err)
			m.dxgiFailed = true
		} else {
			m.dxgi = dxgi
			img, err := m.dxgi.capture()
			if err == nil {
				return img, nil
			}
			log.Printf("[capture] DXGI first capture failed: %v", err)
			m.dxgi.close()
			m.dxgi = nil
			m.dxgiFailed = true
		}
	}

	// ── GDI fallback ─────────────────────────────────────────────────────
	if m.scaler != nil {
		img, err := m.scaler.capture(bounds.Min.X, bounds.Min.Y, srcW, srcH)
		if err == nil {
			return img, nil
		}
		m.scaler.close()
		m.scaler = nil
	}

	scaler, err := newPlatformScaler(dstW, dstH)
	if err != nil {
		img, err := captureScaled(bounds.Min.X, bounds.Min.Y, srcW, srcH, dstW, dstH)
		if err != nil {
			return nil, fmt.Errorf("monitor capture failed: %w", err)
		}
		return img, nil
	}
	m.scaler = scaler

	img, err := m.scaler.capture(bounds.Min.X, bounds.Min.Y, srcW, srcH)
	if err != nil {
		return nil, fmt.Errorf("monitor capture failed: %w", err)
	}
	return img, nil
}

func (m *MonitorCapturer) Close() {
	if m.dxgi != nil {
		m.dxgi.close()
		m.dxgi = nil
	}
	if m.scaler != nil {
		m.scaler.close()
		m.scaler = nil
	}
}

// GetMonitors returns info about all currently active displays.
func GetMonitors() []MonitorInfo {
	n := screenshot.NumActiveDisplays()
	monitors := make([]MonitorInfo, 0, n)
	for i := 0; i < n; i++ {
		b := screenshot.GetDisplayBounds(i)
		monitors = append(monitors, MonitorInfo{
			Index:     i,
			X:         b.Min.X,
			Y:         b.Min.Y,
			Width:     b.Dx(),
			Height:    b.Dy(),
			IsPrimary: i == 0,
			Name:      fmt.Sprintf("Display %d", i+1),
		})
	}
	return monitors
}
