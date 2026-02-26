package webcam

import (
	"log"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// isCameraOn checks the Windows CapabilityAccessManager consent store
// to determine if any camera device is currently in use.
//
// It checks both HKCU and HKLM, and looks under the top-level webcam key
// (for packaged/UWP apps) as well as the NonPackaged subkey (for desktop apps).
func isCameraOn() bool {
	paths := []struct {
		root registry.Key
		path string
	}{
		{registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\CapabilityAccessManager\ConsentStore\webcam`},
		{registry.LOCAL_MACHINE, `Software\Microsoft\Windows\CurrentVersion\CapabilityAccessManager\ConsentStore\webcam`},
	}

	for _, p := range paths {
		if checkConsentKey(p.root, p.path) {
			return true
		}
	}
	return false
}

func checkConsentKey(root registry.Key, basePath string) bool {
	key, err := registry.OpenKey(root, basePath, registry.QUERY_VALUE|registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return false
	}
	defer key.Close()

	subKeys, err := key.ReadSubKeyNames(-1)
	if err != nil {
		return false
	}

	for _, sub := range subKeys {
		subPath := basePath + `\` + sub

		if strings.EqualFold(sub, "NonPackaged") {
			if checkNonPackaged(root, subPath) {
				return true
			}
			continue
		}

		// Packaged app entry - check directly
		if checkDeviceEntry(root, subPath) {
			return true
		}
	}
	return false
}

func checkNonPackaged(root registry.Key, nonPkgPath string) bool {
	key, err := registry.OpenKey(root, nonPkgPath, registry.QUERY_VALUE|registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return false
	}
	defer key.Close()

	subKeys, err := key.ReadSubKeyNames(-1)
	if err != nil {
		return false
	}

	for _, sub := range subKeys {
		if checkDeviceEntry(root, nonPkgPath+`\`+sub) {
			return true
		}
	}
	return false
}

// checkDeviceEntry returns true if LastUsedTimeStop == 0, meaning the device is in use.
func checkDeviceEntry(root registry.Key, path string) bool {
	key, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()

	stop, _, err := key.GetIntegerValue("LastUsedTimeStop")
	if err != nil {
		return false
	}

	if stop == 0 {
		// Verify there's also a non-zero start time to confirm real usage
		start, _, err := key.GetIntegerValue("LastUsedTimeStart")
		if err != nil {
			return false
		}
		if start > 0 {
			log.Printf("[webcam] Active camera entry: %s (start=%d, stop=%d)", path, start, stop)
			return true
		}
	}
	return false
}
