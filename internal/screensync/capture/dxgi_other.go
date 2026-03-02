//go:build !windows

package capture

import (
	"fmt"
	"image"
)

type dxgiCapturer struct{}

func newDXGICapturer(monitorIndex int, dstW, dstH int) (*dxgiCapturer, error) {
	return nil, fmt.Errorf("DXGI capture is only available on Windows")
}

func (d *dxgiCapturer) capture() (*image.RGBA, error) {
	return nil, fmt.Errorf("DXGI capture is only available on Windows")
}

func (d *dxgiCapturer) close() {}
