//go:build !windows

package capture

// OverlayResult is sent back after the user draws a region selection.
type OverlayResult struct {
	Cancelled bool
	Region    struct {
		X, Y, Width, Height int
	}
}

// StartRegionOverlay is a no-op on non-Windows platforms.
func StartRegionOverlay() OverlayResult {
	return OverlayResult{Cancelled: true}
}
