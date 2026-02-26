//go:build windows

package main

import (
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32         = syscall.NewLazyDLL("user32.dll")
	procMessageBox = user32.NewProc("MessageBoxW")
)

func ensureSingleInstance() {
	name, _ := windows.UTF16PtrFromString("Global\\LightSyncSingleInstance")
	_, err := windows.CreateMutex(nil, false, name)
	if err == windows.ERROR_ALREADY_EXISTS {
		text, _ := windows.UTF16PtrFromString("LightSync is already running. Check your system tray.")
		caption, _ := windows.UTF16PtrFromString("LightSync")
		const mbIconInformation = 0x00000040
		procMessageBox.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(caption)), mbIconInformation)
		os.Exit(0)
	}
}
