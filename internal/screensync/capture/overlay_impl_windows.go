//go:build windows

package capture

import (
	"syscall"
	"unsafe"
)

// Win32 constants used by the overlay window.
const (
	wsExLayered    = 0x00080000
	wsExTopmost    = 0x00000008
	wsExTransparent = 0x00000020
	wsExToolWindow = 0x00000080
	wsPopup        = 0x80000000
	wsVisible      = 0x10000000

	lwaAlpha = 0x00000002
	lwaColorkey = 0x00000001

	wmLbuttondown = 0x0201
	wmLbuttonup   = 0x0202
	wmMousemove   = 0x0200
	wmKeydown     = 0x0100
	wmDestroy     = 0x0002
	wmPaint       = 0x000F
	wmErasebkgnd  = 0x0014

	vkEscape = 0x1B

	ropSrcCopy = 0x00CC0020

	// GDI pen / brush styles
	psNull  = 5
	bsNull  = 1
	bsSolid = 0

	// SystemMetrics
	smXvirtualscreen  = 76
	smYvirtualscreen  = 77
	smCxvirtualscreen = 78
	smCyvirtualscreen = 79
)

var (
	procRegisterClassExW      = user32dll.NewProc("RegisterClassExW")
	procCreateWindowExW       = user32dll.NewProc("CreateWindowExW")
	procShowWindow            = user32dll.NewProc("ShowWindow")
	procUpdateWindow          = user32dll.NewProc("UpdateWindow")
	procGetMessageW           = user32dll.NewProc("GetMessageW")
	procTranslateMessage      = user32dll.NewProc("TranslateMessage")
	procDispatchMessageW      = user32dll.NewProc("DispatchMessageW")
	procSetLayeredWindowAttributes = user32dll.NewProc("SetLayeredWindowAttributes")
	procInvalidateRect        = user32dll.NewProc("InvalidateRect")
	procBeginPaint            = user32dll.NewProc("BeginPaint")
	procEndPaint              = user32dll.NewProc("EndPaint")
	procGetSystemMetrics      = user32dll.NewProc("GetSystemMetrics")
	procDestroyWindow         = user32dll.NewProc("DestroyWindow")
	procPostQuitMessage       = user32dll.NewProc("PostQuitMessage")
	procDefWindowProcW        = user32dll.NewProc("DefWindowProcW")
	procGetCursorPos          = user32dll.NewProc("GetCursorPos")
	procSetCursor             = user32dll.NewProc("SetCursor")
	procLoadCursorW           = user32dll.NewProc("LoadCursorW")
	procGetModuleHandleW      = kernel32dll.NewProc("GetModuleHandleW")
	procPostMessageW          = user32dll.NewProc("PostMessageW")

	procCreateSolidBrush = gdi32dll.NewProc("CreateSolidBrush")
	procCreatePen        = gdi32dll.NewProc("CreatePen")
	procSelectObjectGDI  = gdi32dll.NewProc("SelectObject")
	procRectangle        = gdi32dll.NewProc("Rectangle")
	procFillRect         = gdi32dll.NewProc("FillRect")
)

// WNDCLASSEXW mirrors the Win32 WNDCLASSEXW struct.
type wndClassExW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

type point struct{ X, Y int32 }
type msgW struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

type paintStruct struct {
	Hdc         uintptr
	FErase      int32
	RcPaint     rect
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}

// overlayState holds the mutable state for the running overlay window.
var overlayState struct {
	dragging    bool
	startX      int32
	startY      int32
	curX        int32
	curY        int32
	hwnd        uintptr
	resultCh    chan OverlayResult
	cancelled   bool
}

func runOverlay() OverlayResult {
	resultCh := make(chan OverlayResult, 1)
	overlayState.resultCh = resultCh
	overlayState.dragging = false
	overlayState.cancelled = false

	hInstance, _, _ := procGetModuleHandleW.Call(0)

	className, _ := syscall.UTF16PtrFromString("LightSyncOverlay")
	windowTitle, _ := syscall.UTF16PtrFromString("Select Region")

	wndProc := syscall.NewCallback(overlayWndProc)

	wc := wndClassExW{
		CbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		LpfnWndProc:   wndProc,
		HInstance:     hInstance,
		LpszClassName: className,
		Style:         0x0003, // CS_HREDRAW | CS_VREDRAW
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	// Cover the entire virtual desktop (all monitors).
	vx, _, _ := procGetSystemMetrics.Call(smXvirtualscreen)
	vy, _, _ := procGetSystemMetrics.Call(smYvirtualscreen)
	vw, _, _ := procGetSystemMetrics.Call(smCxvirtualscreen)
	vh, _, _ := procGetSystemMetrics.Call(smCyvirtualscreen)

	exStyle := uintptr(wsExLayered | wsExTopmost | wsExToolWindow)
	hwnd, _, _ := procCreateWindowExW.Call(
		exStyle,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowTitle)),
		wsPopup|wsVisible,
		vx, vy, vw, vh,
		0, 0, hInstance, 0,
	)

	if hwnd == 0 {
		return OverlayResult{Cancelled: true}
	}
	overlayState.hwnd = hwnd

	// Semi-transparent dark overlay (alpha=180/255 ≈ 70%).
	procSetLayeredWindowAttributes.Call(hwnd, 0, 180, lwaAlpha)

	// Change cursor to crosshair.
	crossCursor, _, _ := procLoadCursorW.Call(0, 32515) // IDC_CROSS
	procSetCursor.Call(crossCursor)

	procShowWindow.Call(hwnd, 5) // SW_SHOW
	procUpdateWindow.Call(hwnd)

	// Message loop.
	var msg msgW
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if ret == 0 || ret == ^uintptr(0) {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}

	return <-resultCh
}

func overlayWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch uint32(msg) {
	case wmKeydown:
		if wParam == vkEscape {
			overlayState.cancelled = true
			procDestroyWindow.Call(hwnd)
		}

	case wmLbuttondown:
		overlayState.dragging = true
		overlayState.startX = int32(lParam & 0xFFFF)
		overlayState.startY = int32((lParam >> 16) & 0xFFFF)
		overlayState.curX = overlayState.startX
		overlayState.curY = overlayState.startY

	case wmMousemove:
		if overlayState.dragging {
			overlayState.curX = int32(lParam & 0xFFFF)
			overlayState.curY = int32((lParam >> 16) & 0xFFFF)
			procInvalidateRect.Call(hwnd, 0, 1)
		}

	case wmLbuttonup:
		if overlayState.dragging {
			overlayState.dragging = false
			overlayState.cancelled = false
			procDestroyWindow.Call(hwnd)
		}

	case wmPaint:
		var ps paintStruct
		hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		if hdc != 0 && overlayState.dragging {
			// Draw a white selection rectangle.
			pen, _, _ := procCreatePen.Call(0, 2, 0x00FFFFFF) // white, 2px
			brush, _, _ := procCreateSolidBrush.Call(0x33FFFFFF)  // semi-white fill
			oldPen, _, _ := procSelectObjectGDI.Call(hdc, pen)
			oldBrush, _, _ := procSelectObjectGDI.Call(hdc, brush)

			x1, y1 := overlayState.startX, overlayState.startY
			x2, y2 := overlayState.curX, overlayState.curY
			if x2 < x1 { x1, x2 = x2, x1 }
			if y2 < y1 { y1, y2 = y2, y1 }
			procRectangle.Call(hdc, uintptr(x1), uintptr(y1), uintptr(x2), uintptr(y2))

			procSelectObjectGDI.Call(hdc, oldPen)
			procSelectObjectGDI.Call(hdc, oldBrush)
			procDeleteObject.Call(pen)
			procDeleteObject.Call(brush)
		}
		procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))

	case wmDestroy:
		var result OverlayResult
		if overlayState.cancelled {
			result.Cancelled = true
		} else {
			x1, y1 := overlayState.startX, overlayState.startY
			x2, y2 := overlayState.curX, overlayState.curY
			if x2 < x1 { x1, x2 = x2, x1 }
			if y2 < y1 { y1, y2 = y2, y1 }
			w := int(x2 - x1)
			h := int(y2 - y1)
			if w < 10 || h < 10 {
				result.Cancelled = true
			} else {
				result.Region.X = int(x1)
				result.Region.Y = int(y1)
				result.Region.Width = w
				result.Region.Height = h
			}
		}
		overlayState.resultCh <- result
		procPostQuitMessage.Call(0)
	}

	ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return ret
}
