package store

// CaptureMode defines how the screen is captured.
type CaptureMode string

const (
	CaptureModeMonitor      CaptureMode = "monitor"
	CaptureModeRegion       CaptureMode = "region"
	CaptureModeWindow       CaptureMode = "window"
	CaptureModeActiveWindow CaptureMode = "active_window"
)

// ColorMode controls whether to extract one or multiple colors.
type ColorMode string

const (
	ColorModeSingle ColorMode = "single"
	ColorModeMulti  ColorMode = "multi"
)

// ExtractionMethod selects which algorithm picks the representative color.
type ExtractionMethod string

const (
	ExtractionMethodDominant  ExtractionMethod = "dominant"
	ExtractionMethodBrightest ExtractionMethod = "brightest"
	ExtractionMethodSaturated ExtractionMethod = "saturated"
	ExtractionMethodDiverse   ExtractionMethod = "diverse"
	// ExtractionMethodVivid weights each pixel's vote by saturation² so colorful
	// regions dominate over dull backgrounds without losing area awareness.
	// Best sub-method for spatial_grid when the subject is more saturated than
	// its background (flowers, games, movies).
	ExtractionMethodVivid ExtractionMethod = "vivid"
)

// MultiColorApproach selects the multi-color strategy.
type MultiColorApproach string

const (
	MultiColorSpatialGrid  MultiColorApproach = "spatial_grid"
	MultiColorScenePalette MultiColorApproach = "scene_palette"
)

// BrightnessMode clamps the output brightness to a range.
type BrightnessMode string

const (
	BrightnessModeDynamic    BrightnessMode = "fully_dynamic"
	BrightnessModeDark       BrightnessMode = "dark"
	BrightnessModeMedium     BrightnessMode = "medium"
	BrightnessModeBright     BrightnessMode = "bright"
	BrightnessModeFullBright BrightnessMode = "full_bright"
)

// SpeedPreset bundles FPS target, update interval, and transition duration.
type SpeedPreset string

const (
	SpeedPresetVerySlow SpeedPreset = "very_slow"
	SpeedPresetSlow     SpeedPreset = "slow"
	SpeedPresetMedium   SpeedPreset = "medium"
	SpeedPresetFast     SpeedPreset = "fast"
	SpeedPresetRealtime SpeedPreset = "realtime"
)

// SceneCutMode controls whether scene-cut detection is active.
type SceneCutMode string

const (
	SceneCutModeOn  SceneCutMode = "on"
	SceneCutModeOff SceneCutMode = "off"
)

// AssignmentStrategy selects how extracted colors are mapped to devices.
type AssignmentStrategy string

const (
	// AssignmentStrategyIdentityLock anchors each device to a stable color and
	// only allows reassignment when the color has shifted significantly.
	AssignmentStrategyIdentityLock AssignmentStrategy = "identity_lock"
	// AssignmentStrategyFlowTrack matches colors to per-device EMA trajectories.
	// Adapts smoothly to gradual content changes without sudden swaps.
	AssignmentStrategyFlowTrack AssignmentStrategy = "flow_track"
	// AssignmentStrategySceneCutRemap uses trajectory matching during steady
	// state and performs a fresh global remap on detected scene cuts.
	AssignmentStrategySceneCutRemap AssignmentStrategy = "scene_cut_remap"
	// AssignmentStrategyZoneDominant maps color[i] to device[i] permanently.
	// Maximum stability — never reassigns devices.
	AssignmentStrategyZoneDominant AssignmentStrategy = "zone_dominant"
)

// CaptureRect defines a rectangular screen region.
type CaptureRect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// ScreenSyncConfig is the complete per-scene Screen Sync configuration.
// All fields have JSON tags so they round-trip through the store unchanged.
type ScreenSyncConfig struct {
	// Capture
	CaptureMode  CaptureMode `json:"captureMode"`
	MonitorIndex int         `json:"monitorIndex"`
	Region       CaptureRect `json:"region"`
	WindowHWND   uint64      `json:"windowHwnd,omitempty"`
	WindowTitle  string      `json:"windowTitle,omitempty"`

	// Devices assigned to this screen sync scene.
	DeviceIDs []string `json:"deviceIds"`

	// Color extraction
	ColorMode          ColorMode          `json:"colorMode"`
	ExtractionMethod   ExtractionMethod   `json:"extractionMethod"`
	MultiColorApproach MultiColorApproach `json:"multiColorApproach"`
	SubMethod          ExtractionMethod   `json:"subMethod"`

	// PaletteStability controls how strongly the scene-palette histogram is
	// accumulated across frames before colours are selected.
	// 0.0 = fully fresh each frame (jittery); 1.0 = stable; 2.0 = ultra-stable.
	// 0.0–1.0 uses histDecay = paletteStability × 0.95.
	// 1.0–2.0 extends hold time up to histDecay=0.99 for extra stability.
	// Only affects MultiColorScenePalette mode.
	PaletteStability float64 `json:"paletteStability"` // 0.0–2.0

	// Color processing
	SaturationBoost float64 `json:"saturationBoost"` // 0.0–2.0; 1.0 = no change
	WhiteBias       float64 `json:"whiteBias"`       // -1.0–+1.0

	// Brightness
	BrightnessMode       BrightnessMode `json:"brightnessMode"`
	BrightnessMultiplier float64        `json:"brightnessMultiplier"` // 0.1–10.0

	// Performance preset
	SpeedPreset SpeedPreset `json:"speedPreset"`

	// ── Color Assignment Engine ───────────────────────────────────────────────

	// AssignmentStrategy selects the algorithm that maps extracted colors to devices.
	AssignmentStrategy AssignmentStrategy `json:"assignmentStrategy"`

	// IdentityLock: fraction of max distance a color must shift before the
	// device anchor is updated. Lower = more stable; higher = adapts faster.
	IdentityLockBreachThreshold float64 `json:"identityLockBreachThreshold"` // 0.10–0.80

	// FlowTrack: EMA blend factor applied to per-device trajectories each frame.
	// Lower = smoother tracking; higher = faster response to content changes.
	FlowTrackEmaAlpha float64 `json:"flowTrackEmaAlpha"` // 0.05–1.0
	// FlowTrackSolveIntervalMs is the minimum time between full Hungarian solves.
	FlowTrackSolveIntervalMs int `json:"flowTrackSolveIntervalMs"` // 16–500

	// SceneCutRemap: ms to hold the post-cut assignment before resuming normal tracking.
	SceneCutRemapHoldMs int `json:"sceneCutRemapHoldMs"` // 0–2000

	// ── Temporal Smoothing ───────────────────────────────────────────────────

	// ColorSmoothing controls how much hue and saturation changes are damped
	// between frames. 0 = pass-through, 1 = heavy smoothing, 2 = ultra smooth.
	ColorSmoothing float64 `json:"colorSmoothing"` // 0.0–2.0

	// AssignmentHandoffMs applies post-assignment per-device color crossfading.
	// Softens visible color-slot swaps by ramping from old to new mapped color.
	AssignmentHandoffMs int `json:"assignmentHandoffMs"` // 0–3000

	// BrightnessSmoothing controls how much brightness changes are damped.
	// Independent from color so users can suppress brightness flicker while
	// keeping color responsive. 0 = pass-through, 1 = maximum smoothing.
	BrightnessSmoothing float64 `json:"brightnessSmoothing"` // 0.0–1.0

	// BrightnessMaxDeviation is the maximum amount any single light's
	// brightness can deviate from the smoothed frame average. Keeps spatial
	// variation between lights while preventing per-zone jitter from
	// causing visible flicker. 0.01 = very tight, 1.0 = unrestricted.
	BrightnessMaxDeviation float64 `json:"brightnessMaxDeviation"` // 0.01–1.0

	// SceneCutSensitivity controls how aggressively scene changes are detected.
	// On a detected cut the temporal smoother resets instantly. 0 = rarely
	// detect (only very dramatic changes), 1 = very sensitive.
	SceneCutSensitivity float64      `json:"sceneCutSensitivity"` // 0.0–1.0
	SceneCutMode        SceneCutMode `json:"sceneCutMode"`

	// ── Brightness Range Compressor ──────────────────────────────────────────

	// BrightnessFloor is the minimum output brightness after smoothing.
	// The full extracted range [0, 1] is linearly remapped into
	// [BrightnessFloor, BrightnessCeiling]. 0 = allow fully dark.
	BrightnessFloor float64 `json:"brightnessFloor"` // 0.0–1.0

	// BrightnessCeiling is the maximum output brightness after smoothing.
	// 1.0 = allow full brightness.
	BrightnessCeiling float64 `json:"brightnessCeiling"` // 0.0–1.0
}

// DefaultScreenSyncConfig returns a ScreenSyncConfig with sensible defaults.
func DefaultScreenSyncConfig() ScreenSyncConfig {
	cfg := ScreenSyncConfig{
		CaptureMode:                 CaptureModeMonitor,
		MonitorIndex:                0,
		DeviceIDs:                   []string{},
		ColorMode:                   ColorModeSingle,
		ExtractionMethod:            ExtractionMethodVivid,
		MultiColorApproach:          MultiColorSpatialGrid,
		SubMethod:                   ExtractionMethodVivid,
		PaletteStability:            0.75,
		SaturationBoost:             1.2,
		WhiteBias:                   0,
		BrightnessMode:              BrightnessModeDynamic,
		BrightnessMultiplier:        1.0,
		SpeedPreset:                 SpeedPresetMedium,
		AssignmentStrategy:          AssignmentStrategyFlowTrack,
		IdentityLockBreachThreshold: 0.30,
		FlowTrackEmaAlpha:           0.25,
		FlowTrackSolveIntervalMs:    33,
		SceneCutRemapHoldMs:         500,
		ColorSmoothing:              0.5,
		AssignmentHandoffMs:         400,
		BrightnessSmoothing:         0.5,
		BrightnessMaxDeviation:      0.15,
		SceneCutSensitivity:         0.5,
		SceneCutMode:                SceneCutModeOn,
		BrightnessFloor:             0.0,
		BrightnessCeiling:           1.0,
	}
	return cfg
}

// NormalizeScreenSyncConfig fills in zero-value fields for configs loaded from
// older JSON that predate newer Screen Sync settings.
func NormalizeScreenSyncConfig(cfg *ScreenSyncConfig) {
	if cfg.SpeedPreset == "" {
		cfg.SpeedPreset = SpeedPresetMedium
	}
	if cfg.BrightnessMode == "" {
		cfg.BrightnessMode = BrightnessModeDynamic
	}
	if cfg.BrightnessMultiplier == 0 {
		cfg.BrightnessMultiplier = 1.0
	}
	if cfg.SaturationBoost == 0 {
		cfg.SaturationBoost = 1.0
	}
	if cfg.ExtractionMethod == "" {
		cfg.ExtractionMethod = ExtractionMethodVivid
	}
	if cfg.MultiColorApproach == "" {
		cfg.MultiColorApproach = MultiColorSpatialGrid
	}
	if cfg.SubMethod == "" {
		cfg.SubMethod = ExtractionMethodVivid
	}
	if cfg.PaletteStability == 0 {
		cfg.PaletteStability = 0.75
	}
	if cfg.ColorMode == "" {
		cfg.ColorMode = ColorModeSingle
	}
	if cfg.CaptureMode == "" {
		cfg.CaptureMode = CaptureModeMonitor
	}
	if cfg.SceneCutMode == "" {
		cfg.SceneCutMode = SceneCutModeOn
	}
	if cfg.SceneCutMode != SceneCutModeOn && cfg.SceneCutMode != SceneCutModeOff {
		cfg.SceneCutMode = SceneCutModeOn
	}
	if cfg.AssignmentStrategy == "" {
		cfg.AssignmentStrategy = AssignmentStrategyFlowTrack
	}
	if cfg.IdentityLockBreachThreshold == 0 {
		cfg.IdentityLockBreachThreshold = 0.30
	}
	if cfg.FlowTrackEmaAlpha == 0 {
		cfg.FlowTrackEmaAlpha = 0.25
	}
	if cfg.FlowTrackSolveIntervalMs == 0 {
		cfg.FlowTrackSolveIntervalMs = 33
	}
	if cfg.AssignmentHandoffMs < 0 {
		cfg.AssignmentHandoffMs = 0
	}
	if cfg.AssignmentHandoffMs > 3000 {
		cfg.AssignmentHandoffMs = 3000
	}
	if cfg.SceneCutRemapHoldMs == 0 && cfg.AssignmentStrategy == AssignmentStrategySceneCutRemap {
		cfg.SceneCutRemapHoldMs = 500
	}

	if cfg.BrightnessMaxDeviation == 0 {
		cfg.BrightnessMaxDeviation = 0.15
	}

	// BrightnessCeiling defaults to 1.0 for old configs that lack the field.
	if cfg.BrightnessCeiling == 0 && cfg.BrightnessFloor == 0 {
		cfg.BrightnessCeiling = 1.0
	}
	if cfg.BrightnessCeiling-cfg.BrightnessFloor < 0.05 {
		cfg.BrightnessCeiling = cfg.BrightnessFloor + 0.05
		if cfg.BrightnessCeiling > 1.0 {
			cfg.BrightnessCeiling = 1.0
			cfg.BrightnessFloor = 0.95
		}
	}
}
