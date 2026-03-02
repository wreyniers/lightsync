//go:build !windows

package capture

import (
	"fmt"
	"image"
)

type WindowCapturer struct{}

func newWindowCapturer(_ uint64) (*WindowCapturer, error) {
	return nil, errUnsupported
}

func (w *WindowCapturer) Capture() (image.Image, error) {
	return nil, errUnsupported
}

func (w *WindowCapturer) Close() {}

func CaptureThumbnail(_ uint64) (image.Image, error) {
	return nil, fmt.Errorf("window capture not supported on this platform")
}
