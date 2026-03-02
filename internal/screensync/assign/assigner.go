package assign

import (
	"lightsync/internal/lights"
	"lightsync/internal/store"
)

// Assigner maps a set of extracted colors to device IDs.
type Assigner interface {
	// Assign returns a device-ID → color mapping for one frame.
	// colors:    extracted colors (1 in single mode; N in multi mode).
	// deviceIDs: the ordered list of devices in the scene.
	// current:   last emitted output color per device.
	// cfg:       live scene configuration.
	// isCut:     true when a scene change was detected this frame.
	Assign(
		colors []lights.Color,
		deviceIDs []string,
		current map[string]lights.Color,
		cfg store.ScreenSyncConfig,
		isCut bool,
	) map[string]lights.Color

	// Reset clears internal state (called on engine Start).
	Reset()
}

// New returns the Assigner selected by cfg.AssignmentStrategy.
func New(cfg store.ScreenSyncConfig) Assigner {
	switch cfg.AssignmentStrategy {
	case store.AssignmentStrategyIdentityLock:
		return newIdentityLock()
	case store.AssignmentStrategySceneCutRemap:
		return newSceneCutRemap()
	case store.AssignmentStrategyZoneDominant:
		return newZoneDominant()
	default: // flow_track or empty
		return newFlowTrack()
	}
}
