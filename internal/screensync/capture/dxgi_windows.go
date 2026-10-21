//go:build windows

package capture

import (
	"errors"
	"fmt"
	"image"
	"log"
	"reflect"
	"runtime"
	"unsafe"

	"github.com/kirides/go-d3d/d3d11"
	"github.com/kirides/go-d3d/outputduplication"
	"github.com/kirides/go-d3d/win"
)

// dxgiCapturer uses DXGI Desktop Duplication to capture a monitor (or a
// sub-region of a monitor). Instead of copying the entire full-resolution
// frame to system RAM, it maps the GPU surface and samples only the pixels
// needed for the output directly from mapped memory.
type dxgiCapturer struct {
	device    *d3d11.ID3D11Device
	deviceCtx *d3d11.ID3D11DeviceContext
	ddup      *outputduplication.OutputDuplicator

	smallBuf *image.RGBA
	dstW     int
	dstH     int

	// Monitor bounds in screen coordinates.
	monLeft, monTop int
	monW, monH      int

	// needsSwizzle is true when DXGI handed us BGRA data (fallback path via
	// DuplicateOutput). When it's false, DuplicateOutput1 with RGBA succeeded
	// and the bytes are already in image.RGBA order — swapping would invert
	// red and blue. Read from the library's internal flag at construction.
	needsSwizzle bool

	threadLocked bool
}

// dxgiNeedsSwizzle reads the unexported needsSwizzle field from the
// outputduplication library via reflection + unsafe. The library doesn't
// expose this information any other way, and silently guessing is worse:
// on Win10+ we'd inadvertently swap red↔blue on every pixel (the original
// DXGI "fix" in this file did exactly that, producing magenta-dominant
// output regardless of screen content).
func dxgiNeedsSwizzle(dup *outputduplication.OutputDuplicator) bool {
	v := reflect.ValueOf(dup).Elem().FieldByName("needsSwizzle")
	if !v.IsValid() || v.Kind() != reflect.Bool {
		// Library upgrade changed the field name — bail out safely.
		// DuplicateOutput1 + RGBA is the happy path on every Win10/11 build,
		// so "assume no swizzle" matches the majority case.
		return false
	}
	return *(*bool)(unsafe.Pointer(v.UnsafeAddr()))
}

func newDXGICapturer(monitorIndex int, dstW, dstH int) (*dxgiCapturer, error) {
	runtime.LockOSThread()

	if win.IsValidDpiAwarenessContext(win.DpiAwarenessContextPerMonitorAwareV2) {
		win.SetThreadDpiAwarenessContext(win.DpiAwarenessContextPerMonitorAwareV2)
	}

	device, deviceCtx, err := d3d11.NewD3D11Device()
	if err != nil {
		runtime.UnlockOSThread()
		return nil, fmt.Errorf("D3D11 device creation failed: %w", err)
	}

	ddup, err := outputduplication.NewIDXGIOutputDuplication(device, deviceCtx, uint(monitorIndex))
	if err != nil {
		deviceCtx.Release()
		device.Release()
		runtime.UnlockOSThread()
		return nil, fmt.Errorf("DXGI output duplication failed: %w", err)
	}

	bounds, err := ddup.GetBounds()
	if err != nil {
		ddup.Release()
		deviceCtx.Release()
		device.Release()
		runtime.UnlockOSThread()
		return nil, fmt.Errorf("DXGI GetBounds failed: %w", err)
	}

	swizzle := dxgiNeedsSwizzle(ddup)
	log.Printf("[capture] DXGI initialised for monitor %d (format=%s)",
		monitorIndex, map[bool]string{true: "BGRA, swizzle on", false: "RGBA, no swizzle"}[swizzle])

	return &dxgiCapturer{
		device:       device,
		deviceCtx:    deviceCtx,
		ddup:         ddup,
		smallBuf:     image.NewRGBA(image.Rect(0, 0, dstW, dstH)),
		dstW:         dstW,
		dstH:         dstH,
		monLeft:      bounds.Min.X,
		monTop:       bounds.Min.Y,
		monW:         bounds.Dx(),
		monH:         bounds.Dy(),
		needsSwizzle: swizzle,
		threadLocked: true,
	}, nil
}

// capture grabs the full monitor and downscales to dstW x dstH.
func (d *dxgiCapturer) capture() (*image.RGBA, error) {
	return d.captureRect(d.monLeft, d.monTop, d.monW, d.monH)
}

// captureRect grabs the full monitor but only downscales the given screen-
// coordinate rectangle into the output buffer.
func (d *dxgiCapturer) captureRect(screenX, screenY, w, h int) (*image.RGBA, error) {
	unmap, mappedRect, size, err := d.ddup.Snapshot(0)
	if err != nil {
		if errors.Is(err, outputduplication.ErrNoImageYet) {
			return d.smallBuf, nil
		}
		return nil, err
	}

	texW := int(size.X)
	texH := int(size.Y)
	pitch := int(mappedRect.Pitch)
	dataSize := pitch * texH
	data := unsafe.Slice((*byte)(mappedRect.PBits), dataSize)

	// Convert screen coordinates to texture-local coordinates.
	cropX := screenX - d.monLeft
	cropY := screenY - d.monTop
	cropW := w
	cropH := h

	// Clamp to texture bounds.
	if cropX < 0 {
		cropW += cropX
		cropX = 0
	}
	if cropY < 0 {
		cropH += cropY
		cropY = 0
	}
	if cropX+cropW > texW {
		cropW = texW - cropX
	}
	if cropY+cropH > texH {
		cropH = texH - cropY
	}
	if cropW <= 0 || cropH <= 0 {
		unmap()
		return d.smallBuf, nil
	}

	downsampleCropFromMapped(data, pitch, cropX, cropY, cropW, cropH, d.smallBuf, d.dstW, d.dstH, d.needsSwizzle)

	unmap()
	return d.smallBuf, nil
}

func (d *dxgiCapturer) close() {
	if d.ddup != nil {
		d.ddup.Release()
		d.ddup = nil
	}
	if d.deviceCtx != nil {
		d.deviceCtx.Release()
		d.deviceCtx = nil
	}
	if d.device != nil {
		d.device.Release()
		d.device = nil
	}
	if d.threadLocked {
		runtime.UnlockOSThread()
		d.threadLocked = false
	}
}

// downsampleCropFromMapped performs a nearest-neighbor downscale of a
// sub-rectangle within the mapped GPU surface into the destination RGBA image.
//
// On modern Win10/Win11 the outputduplication library uses DuplicateOutput1
// with DXGI_FORMAT_R8G8B8A8_UNORM, so source bytes are already [R, G, B, A]
// and we copy them straight. On older systems (or when D3D11 debug layers
// force the fallback path), the library uses DuplicateOutput which returns
// [B, G, R, A] and swizzle == true — in that case we swap channels 0 and 2
// as we copy so image.RGBA.Pix ends up correct.
func downsampleCropFromMapped(src []byte, pitch, cropX, cropY, cropW, cropH int, dst *image.RGBA, dstW, dstH int, swizzle bool) {
	dstPix := dst.Pix
	dstStride := dst.Stride

	if swizzle {
		for dy := 0; dy < dstH; dy++ {
			sy := cropY + (dy*cropH+cropH/2)/dstH
			srcRowOff := sy * pitch
			dstRowOff := dy * dstStride

			for dx := 0; dx < dstW; dx++ {
				sx := cropX + (dx*cropW+cropW/2)/dstW
				srcOff := srcRowOff + sx*4
				dstOff := dstRowOff + dx*4

				dstPix[dstOff] = src[srcOff+2]
				dstPix[dstOff+1] = src[srcOff+1]
				dstPix[dstOff+2] = src[srcOff]
				dstPix[dstOff+3] = 255
			}
		}
		return
	}

	for dy := 0; dy < dstH; dy++ {
		sy := cropY + (dy*cropH+cropH/2)/dstH
		srcRowOff := sy * pitch
		dstRowOff := dy * dstStride

		for dx := 0; dx < dstW; dx++ {
			sx := cropX + (dx*cropW+cropW/2)/dstW
			srcOff := srcRowOff + sx*4
			dstOff := dstRowOff + dx*4

			dstPix[dstOff] = src[srcOff]
			dstPix[dstOff+1] = src[srcOff+1]
			dstPix[dstOff+2] = src[srcOff+2]
			dstPix[dstOff+3] = 255
		}
	}
}
