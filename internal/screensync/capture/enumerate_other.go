//go:build !windows

package capture

// EnumWindows returns an empty list on non-Windows platforms.
func EnumWindows() []WindowInfo { return nil }
