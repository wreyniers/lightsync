//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const (
	runKeyPath   = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValueName = "LightSync"
)

// applyLaunchAtLogin adds or removes the per-user Run entry for this exe.
// The entry points at whichever exe is currently running, so moving the app
// and launching it once keeps login startup working.
func applyLaunchAtLogin(enabled bool) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open Run key: %w", err)
	}
	defer key.Close()

	if !enabled {
		if err := key.DeleteValue(runValueName); err != nil && err != registry.ErrNotExist {
			return fmt.Errorf("delete Run value: %w", err)
		}
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return key.SetStringValue(runValueName, `"`+exe+`"`)
}
