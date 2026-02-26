package webcam

import (
	"os/exec"
	"strings"
)

// isCameraOn uses ioreg to check if any camera device is in use on macOS.
func isCameraOn() bool {
	out, err := exec.Command("bash", "-c",
		`log stream --predicate 'subsystem == "com.apple.camera"' --timeout 1 2>/dev/null | grep -c "Activate" || true`).
		CombinedOutput()
	if err != nil {
		// Fallback: check via ioreg for VDC assistant
		out2, err2 := exec.Command("bash", "-c",
			`ioreg -l | grep -c '"VDC_SelfSight" = Yes' || echo 0`).
			CombinedOutput()
		if err2 != nil {
			return false
		}
		return strings.TrimSpace(string(out2)) != "0"
	}
	return strings.TrimSpace(string(out)) != "0"
}
