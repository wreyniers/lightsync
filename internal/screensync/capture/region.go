package capture

import (
	"fmt"
	"image"

	"lightsync/internal/store"
)

// RegionCapturer captures a user-defined rectangular region of the screen.
type RegionCapturer struct {
	region store.CaptureRect
	scaler scaledCapturer
}

// NewRegionCapturer creates a capturer for the given screen region.
func NewRegionCapturer(region store.CaptureRect) (*RegionCapturer, error) {
	if region.Width <= 0 || region.Height <= 0 {
		return nil, fmt.Errorf("region must have positive width and height (got %dx%d)", region.Width, region.Height)
	}
	return &RegionCapturer{region: region}, nil
}

func (r *RegionCapturer) Capture() (image.Image, error) {
	srcW, srcH := r.region.Width, r.region.Height

	dstW := captureTargetWidth
	dstH := captureTargetWidth * srcH / srcW
	if dstH < 1 {
		dstH = 1
	}

	if r.scaler != nil {
		img, err := r.scaler.capture(r.region.X, r.region.Y, srcW, srcH)
		if err == nil {
			return img, nil
		}
		r.scaler.close()
		r.scaler = nil
	}

	scaler, err := newPlatformScaler(dstW, dstH)
	if err != nil {
		img, err := captureScaled(r.region.X, r.region.Y, srcW, srcH, dstW, dstH)
		if err != nil {
			return nil, fmt.Errorf("region capture failed: %w", err)
		}
		return img, nil
	}
	r.scaler = scaler

	img, err := r.scaler.capture(r.region.X, r.region.Y, srcW, srcH)
	if err != nil {
		return nil, fmt.Errorf("region capture failed: %w", err)
	}
	return img, nil
}

func (r *RegionCapturer) Close() {
	if r.scaler != nil {
		r.scaler.close()
		r.scaler = nil
	}
}

// UpdateRegion hot-swaps the capture region without recreating the capturer.
func (r *RegionCapturer) UpdateRegion(region store.CaptureRect) {
	r.region = region
}
