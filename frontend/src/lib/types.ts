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
}

export interface Settings {
  pollIntervalMs: number;
  startMinimized: boolean;
  launchAtLogin: boolean;
}
