//go:build windows

package capture

import (
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procEnumWindows      = user32dll.NewProc("EnumWindows")
	procIsWindowVisible  = user32dll.NewProc("IsWindowVisible")
	procGetWindowTextW   = user32dll.NewProc("GetWindowTextW")
	procGetWindowTextLen = user32dll.NewProc("GetWindowTextLengthW")
)

// EnumWindows lists all visible top-level windows with a non-empty title.
func EnumWindows() []WindowInfo {
	var windows []WindowInfo

	cb := syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		// Skip invisible windows.
		vis, _, _ := procIsWindowVisible.Call(hwnd)
		if vis == 0 {
			return 1
		}
		// Skip windows with no title.
		length, _, _ := procGetWindowTextLen.Call(hwnd)
		if length == 0 {
			return 1
		}
		buf := make([]uint16, length+1)
		procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), length+1)
		title := syscall.UTF16ToString(buf)
		if strings.TrimSpace(title) == "" {
			return 1
		}

		exeName := exeNameForHWND(hwnd)
		windows = append(windows, WindowInfo{
			HWND:    uint64(hwnd),
			Title:   title,
			ExeName: exeName,
		})
		return 1 // continue enumeration
	})

	procEnumWindows.Call(cb, 0)
	return windows
}

func exeNameForHWND(hwnd uintptr) string {
	var pid uint32
	procGetWindowThreadProcessId := user32dll.NewProc("GetWindowThreadProcessId")
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return ""
	}

	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(handle)

	buf := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(handle, 0, &buf[0], &size); err != nil {
		return ""
	}
	return filepath.Base(syscall.UTF16ToString(buf[:size]))
}
