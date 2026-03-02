//go:build windows

package capture

// OverlayResult is sent back after the user draws a region selection.
type OverlayResult struct {
	Cancelled bool
	Region    struct {
		X, Y, Width, Height int
	}
}

// StartRegionOverlay opens a full-screen transparent Win32 window that lets the
// user click-drag to select a rectangular region. It blocks until the user
// finishes or cancels, then returns the result.
//
// Implementation note: we spawn a native Win32 layered window (WS_EX_LAYERED +
// WS_EX_TOPMOST) that covers all monitors. The window is transparent except for
// a semi-transparent dark overlay and a bright selection rectangle drawn with GDI.
func StartRegionOverlay() OverlayResult {
	return runOverlay()
}
