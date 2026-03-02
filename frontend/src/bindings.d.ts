/**
 * Declaration merge for Wails v3 generated bindings.
 * The bindings are .js files; this provides TypeScript compatibility.
 */
declare module "@bindings" {
  export const App: {
    ActivateScene(id: string): Promise<void>;
    AddElgatoByIP(ip: string): Promise<any>;
    AddHueBridge(ip: string, username: string): Promise<void>;
    CheckCameraNow(): Promise<boolean>;
    CloneScene(id: string): Promise<any>;
    CreateScene(req: any): Promise<any>;
    DeactivateScene(): Promise<void>;
    DeleteScene(id: string): Promise<void>;
    DiscoverHueBridges(): Promise<any[]>;
    DiscoverLights(): Promise<{ devices: any[] }>;
    GetActiveScene(): Promise<string>;
    GetCameraState(): Promise<boolean>;
    GetCapturePreview(): Promise<string>;
    GetDefaultScreenSyncConfig(): Promise<any>;
    GetDevices(): Promise<any[]>;
    GetHueBridges(): Promise<any[]>;
    GetLastSceneID(): Promise<string>;
    GetLightState(deviceID: string): Promise<any>;
    GetMonitors(): Promise<any[]>;
    GetScene(id: string): Promise<any>;
    GetScenes(): Promise<any[]>;
    GetScreenSyncState(): Promise<{ running: boolean; sceneId?: string }>;
    GetSettings(): Promise<any>;
    GetWindowThumbnail(hwnd: number): Promise<string>;
    GetWindows(): Promise<any[]>;
    IsMonitoringEnabled(): Promise<boolean>;
    OpenConfigFile(): Promise<void>;
    OpenLightsPopup(): Promise<void>;
    PairHueBridge(ip: string): Promise<{ success: boolean; username?: string; error?: string }>;
    QuitApp(): Promise<void>;
    RemoveDevice(deviceID: string): Promise<void>;
    RemoveHueBridge(id: string): Promise<void>;
    SetDeviceRoom(deviceID: string, room: string): Promise<void>;
    SetLightState(deviceID: string, state: any): Promise<void>;
    SetMonitoringEnabled(enabled: boolean): Promise<void>;
    StartRegionSelect(): Promise<void>;
    StopScreenSync(): Promise<void>;
    TurnOffLight(deviceID: string): Promise<void>;
    TurnOnLight(deviceID: string): Promise<void>;
    UpdateScreenSyncConfig(sceneID: string, cfg: any): Promise<void>;
    UpdateScene(scene: any): Promise<void>;
    UpdateSettings(settings: any): Promise<void>;
  };
  export const CreateSceneRequest: new (src?: unknown) => unknown;
  export const DiscoverResult: new (src?: unknown) => unknown;
  export const PairResult: new (src?: unknown) => unknown;
  export const ScreenSyncState: new (src?: unknown) => unknown;
}

declare module "@bindings/internal/lights/models.js" {
  export class Color {
    constructor(src?: { h?: number; s?: number; b?: number });
    h: number;
    s: number;
    b: number;
  }
  export class DeviceState {
    constructor(src?: { on?: boolean; brightness?: number; kelvin?: number; color?: unknown });
  }
  export class Device {
    constructor(src?: unknown);
  }
  export const Brand: Record<string, string>;
}

declare module "@bindings/internal/store/models.js" {
  export class Scene {
    constructor(src?: any);
    id?: string;
    name?: string;
    trigger?: string;
    devices?: Record<string, any>;
    screenSync?: any;
    globalColor?: any;
    globalKelvin?: number;
  }
  export class Settings {
    constructor(src?: { pollIntervalMs?: number; startMinimized?: boolean; launchAtLogin?: boolean });
  }
  export class HueBridge {
    constructor(src?: unknown);
  }
}
