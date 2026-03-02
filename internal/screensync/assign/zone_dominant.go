package assign

import (
	"lightsync/internal/lights"
	"lightsync/internal/store"
)

// zoneDominantAssigner maps color[i] permanently to device[i].
// When spatial_grid extraction is active, color[i] originates from a fixed
// screen zone, so each device is permanently bound to a fixed area of the
// screen — the most stable possible assignment strategy.
// No state, no reassignment, no configuration knobs.
type zoneDominantAssigner struct{}

func newZoneDominant() *zoneDominantAssigner { return &zoneDominantAssigner{} }

func (z *zoneDominantAssigner) Reset() {}

func (z *zoneDominantAssigner) Assign(
	colors []lights.Color,
	deviceIDs []string,
	_ map[string]lights.Color,
	cfg store.ScreenSyncConfig,
	_ bool,
) map[string]lights.Color {
	result := make(map[string]lights.Color, len(deviceIDs))
	single := cfg.ColorMode == store.ColorModeSingle || len(colors) == 1
	for i, id := range deviceIDs {
		if single {
			result[id] = colors[0]
		} else if i < len(colors) {
			result[id] = colors[i]
		} else {
			result[id] = colors[0]
		}
	}
	return result
}
