export interface Device {
  id: string;
  brand: string;
  name: string;
  model?: string;
  lastIp: string;
  lastSeen: string;
  supportsColor: boolean;
  supportsKelvin: boolean;
  /** Device's minimum supported colour temperature in Kelvin. 0 = unknown. */
  minKelvin?: number;
  /** Device's maximum supported colour temperature in Kelvin. 0 = unknown. */
  maxKelvin?: number;
  /** Snap increment for the Kelvin slider (e.g. 50 for Elgato). 0/1 = no constraint. */
  kelvinStep?: number;
  /** Firmware/software version string reported by the device. */
  firmwareVersion?: string;
  /** User-assigned room label used for grouping (e.g. "Bedroom", "Office"). */
  room?: string;
}

export interface DeviceState {
  on: boolean;
  brightness: number;
  color?: Color;
  kelvin?: number;
}

export interface Color {
  h: number;
  s: number;
  b: number;
}

export type LightMode = "color" | "kelvin";

export const DEFAULT_KELVIN = 4000;
export const APP_VERSION = "1.0.0";

export interface Scene {
  id: string;
  name: string;
  trigger: string;
  devices: Record<string, DeviceState>;
  /** Persists the global color override so the editor can restore it on re-edit. */
  globalColor?: Color;
  /** Persists the global kelvin override so the editor can restore it on re-edit. */
  globalKelvin?: number;
  /** Present only for Screen Sync scenes (trigger === "screen_sync"). */
  screenSync?: ScreenSyncConfig;
}

export interface Settings {
  pollIntervalMs: number;
  startMinimized: boolean;
  launchAtLogin: boolean;
}

// ---- Screen Sync types ----

export type CaptureMode = "monitor" | "region" | "window" | "active_window";
export type ColorMode = "single" | "multi";
export type ExtractionMethod = "dominant" | "brightest" | "saturated" | "diverse" | "vivid";
export type MultiColorApproach = "spatial_grid" | "scene_palette";
export type BrightnessMode = "fully_dynamic" | "dark" | "medium" | "bright" | "full_bright";
export type SpeedPreset = "very_slow" | "slow" | "medium" | "fast" | "realtime";
export type AssignmentStrategy = "identity_lock" | "flow_track" | "scene_cut_remap" | "zone_dominant";
export type SceneCutMode = "on" | "off";

export interface CaptureRect {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface ScreenSyncConfig {
  captureMode: CaptureMode;
  monitorIndex: number;
  region: CaptureRect;
  windowHwnd?: number;
  windowTitle?: string;
  deviceIds: string[];
  colorMode: ColorMode;
  extractionMethod: ExtractionMethod;
  multiColorApproach: MultiColorApproach;
  subMethod: ExtractionMethod;
  /** Scene palette only. 0.0 = reactive, 1.0 = stable, 2.0 = ultra-stable hold. */
  paletteStability: number;
  saturationBoost: number;
  whiteBias: number;
  brightnessMode: BrightnessMode;
  brightnessMultiplier: number;
  speedPreset: SpeedPreset;

  // ── Color Assignment Engine ───────────────────────────────────────────────
  assignmentStrategy: AssignmentStrategy;

  // IdentityLock: fraction of max distance a colour must shift before the anchor updates.
  identityLockBreachThreshold: number;  // 0.10–0.80

  // FlowTrack: EMA blend factor for per-device trajectory tracking.
  flowTrackEmaAlpha: number;            // 0.05–1.0
  // FlowTrack: minimum ms between full Hungarian solves.
  flowTrackSolveIntervalMs: number;     // 16–500

  // SceneCutRemap: ms to hold the post-cut assignment before resuming tracking.
  sceneCutRemapHoldMs: number;          // 0–2000

  // ── Temporal Smoothing ─────────────────────────────────────────────────
  /** How much to smooth hue/saturation changes. 0 = off, 1 = heavy, 2 = ultra smooth. */
  colorSmoothing: number;               // 0.0–2.0
  /** Post-assignment crossfade duration. Softens color-slot swaps. */
  assignmentHandoffMs: number;          // 0–3000
  /** How much to smooth brightness changes. 0 = off, 1 = max. */
  brightnessSmoothing: number;          // 0.0–1.0
  /** Max per-light brightness deviation from the smoothed frame average. 0.01–1.0. */
  brightnessMaxDeviation: number;       // 0.01–1.0
  /** Scene-cut detection sensitivity. 0 = rarely detect, 1 = very sensitive. */
  sceneCutSensitivity: number;          // 0.0–1.0
  sceneCutMode: SceneCutMode;

  // ── Brightness Range Compressor ────────────────────────────────────────
  /** Minimum output brightness after smoothing. 0 = allow fully dark. */
  brightnessFloor: number;              // 0.0–1.0
  /** Maximum output brightness after smoothing. 1 = allow full bright. */
  brightnessCeiling: number;            // 0.0–1.0
}

export interface MonitorInfo {
  index: number;
  x: number;
  y: number;
  width: number;
  height: number;
  isPrimary: boolean;
  name: string;
}

export interface WindowInfo {
  hwnd: number;
  title: string;
  exeName: string;
}

export interface ScreenSyncStats {
  fps: number;
  targetFps: number;
  latencyMs: number;
  captureMs: number;
  processMs: number;
  sendMs: number;
  /** Device updates (SetState calls) per second. */
  updateRate?: number;
  /** Frames skipped because previous send was still in flight. */
  framesDropped?: number;
  /** Percent of attempted-send frames that were dropped. */
  framesDroppedPct?: number;
  sceneChange: boolean;
  /** Which threshold(s) triggered the scene cut. */
  cutReasonBrightness: boolean;
  cutReasonHue: boolean;
  colorChanged: boolean;
  assignmentRewired: boolean;
}

export const DEFAULT_SCREEN_SYNC_CONFIG: ScreenSyncConfig = {
  captureMode: "monitor",
  monitorIndex: 0,
  region: { x: 0, y: 0, width: 1920, height: 1080 },
  deviceIds: [],
  colorMode: "single",
  extractionMethod: "vivid",
  multiColorApproach: "spatial_grid",
  subMethod: "vivid",
  paletteStability: 0.75,
  saturationBoost: 1.2,
  whiteBias: 0,
  brightnessMode: "fully_dynamic",
  brightnessMultiplier: 1.0,
  speedPreset: "medium",

  // Color Assignment Engine
  assignmentStrategy: "flow_track",
  identityLockBreachThreshold: 0.30,
  flowTrackEmaAlpha: 0.25,
  flowTrackSolveIntervalMs: 33,
  sceneCutRemapHoldMs: 500,

  // Temporal Smoothing
  colorSmoothing: 0.5,
  assignmentHandoffMs: 400,
  brightnessSmoothing: 0.5,
  brightnessMaxDeviation: 0.15,
  sceneCutSensitivity: 0.5,
  sceneCutMode: "on",

  // Brightness Range Compressor
  brightnessFloor: 0.0,
  brightnessCeiling: 1.0,
};
