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

export interface Scene {
  id: string;
  name: string;
  trigger: string;
  devices: Record<string, DeviceState>;
}

export interface Settings {
  pollIntervalMs: number;
  startMinimized: boolean;
  launchAtLogin: boolean;
}

export interface DiscoverResult {
  devices: Device[];
  errors?: string[];
}

export interface CreateSceneRequest {
  name: string;
  trigger: string;
  devices: Record<string, DeviceState>;
}
