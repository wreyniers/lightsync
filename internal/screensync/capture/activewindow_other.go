//go:build !windows

package capture

import "image"

type ActiveWindowCapturer struct{}

func newActiveWindowCapturer() (*ActiveWindowCapturer, error) {
	return nil, errUnsupported
}

func (a *ActiveWindowCapturer) Capture() (image.Image, error) {
	return nil, errUnsupported
}

func (a *ActiveWindowCapturer) Close() {}

func GetForegroundWindowHWND() uint64 { return 0 }
