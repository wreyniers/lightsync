//go:build !windows

package capture

import "fmt"

func newPlatformScaler(dstW, dstH int) (scaledCapturer, error) {
	return nil, fmt.Errorf("no reusable scaler on this platform")
}
