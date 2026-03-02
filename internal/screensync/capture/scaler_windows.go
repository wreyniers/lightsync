//go:build windows

package capture

func newPlatformScaler(dstW, dstH int) (scaledCapturer, error) {
	return newGDIScaler(dstW, dstH)
}
