# LightSync — Backend API Reference

All public methods on the `App` struct in `app.go` are automatically exposed to the React frontend by the Wails framework. They are callable as async TypeScript functions via auto-generated bindings in `frontend/wailsjs/go/main/App.js`.

Each method runs synchronously in Go but returns a `Promise` on the TypeScript side.

---

## Table of Contents

- [Types](#types)
- [Discovery](#discovery)
  - [DiscoverLights](#discoverlights)
  - [GetDevices](#getdevices)
- [Light Control](#light-control)
  - [SetLightState](#setlightstate)
  - [GetLightState](#getlightstate)
  - [TurnOnLight](#turnonlight)
  - [TurnOffLight](#turnofflight)
- [Scenes](#scenes)
  - [GetScenes](#getscenes)
  - [GetScene](#getscene)
  - [CreateScene](#createscene)
  - [UpdateScene](#updatescene)
  - [DeleteScene](#deletescene)
  - [ActivateScene](#activatescene)
  - [GetActiveScene](#getactivescene)
- [Webcam & Monitoring](#webcam--monitoring)
  - [GetCameraState](#getcamerastate)
  - [CheckCameraNow](#checkcameranow)
  - [SetMonitoringEnabled](#setmonitoringenabled)
  - [IsMonitoringEnabled](#ismonitoringenabled)
- [Settings](#settings)
  - [GetSettings](#getsettings)
  - [UpdateSettings](#updatesettings)
- [Philips Hue Bridges](#philips-hue-bridges)
  - [GetHueBridges](#gethuesbridges)
  - [AddHueBridge](#addhuebridge)
  - [DiscoverHueBridges](#discoverhuebridges)
  - [PairHueBridge](#pairhuebridge)
- [Events Reference](#events-reference)

---

## Types

These types are shared between Go and TypeScript. Go definitions live in `internal/lights/types.go` and `internal/store/store.go`; TypeScript mirrors are in `frontend/src/lib/types.ts`.

### `Device`

```typescript
interface Device {
  id:              string
  brand:           "lifx" | "hue" | "elgato" | "govee"
  name:            string
  model?:          string
  lastIp:          string
  lastSeen:        string        // ISO 8601
  supportsColor:   boolean
  supportsKelvin:  boolean
  minKelvin?:      number        // 0 means unknown
  maxKelvin?:      number
  kelvinStep?:     number
  firmwareVersion?: string
}
```

### `DeviceState`

```typescript
interface DeviceState {
  on:         boolean
  brightness: number             // 0.0 – 1.0
  color?:     Color              // present when supportsColor is true
  kelvin?:    number             // present when supportsKelvin is true
}
```

### `Color`

```typescript
interface Color {
  h: number   // hue, 0 – 360
  s: number   // saturation, 0 – 1
  b: number   // brightness/value, 0 – 1
}
```

### `Scene`

```typescript
interface Scene {
  id:            string
  name:          string
  trigger:       "camera_on" | "camera_off" | "manual"
  devices:       Record<string, DeviceState>   // keyed by device ID
  globalColor?:  Color
  globalKelvin?: number
}
```

### `Settings`

```typescript
interface Settings {
  pollIntervalMs: number    // valid range: 250 – 5000
  startMinimized: boolean
  launchAtLogin:  boolean
}
```

### `HueBridge`

```typescript
interface HueBridge {
  id:       string
  ip:       string
  username: string
}
```

---

## Discovery

### `DiscoverLights`

Runs a full multi-protocol network scan and returns all discovered devices. The scan has a **30-second timeout**.

While scanning, `scan:progress` events are emitted for each phase.

```typescript
function DiscoverLights(): Promise<DiscoverResult>

interface DiscoverResult {
  devices: Device[]
  errors?: string[]
}
```

Discovered devices are automatically merged into the light manager and persisted to `config.json`.

---

### `GetDevices`

Returns the current in-memory device list (populated from the last scan or the stored config).

```typescript
function GetDevices(): Promise<Device[]>
```

Does not perform any network calls.

---

## Light Control

All light control methods have a **5-second timeout**.

### `SetLightState`

Sets the full state of a device (power, brightness, color, temperature).

```typescript
function SetLightState(deviceID: string, state: DeviceState): Promise<void>
```

Partial updates are not supported — the entire `DeviceState` is sent each time. Pass the current values for fields you do not wish to change.

**Errors:** Returns a non-null error if the device is unreachable or the brand controller returns an error.

---

### `GetLightState`

Reads the current state directly from the physical device.

```typescript
function GetLightState(deviceID: string): Promise<DeviceState>
```

This is a live network call, not a cached value.

---

### `TurnOnLight`

Turns a device on at its current brightness and color settings.

```typescript
function TurnOnLight(deviceID: string): Promise<void>
```

---

### `TurnOffLight`

Turns a device off.

```typescript
function TurnOffLight(deviceID: string): Promise<void>
```

---

## Scenes

### `GetScenes`

Returns all saved scenes.

```typescript
function GetScenes(): Promise<Scene[]>
```

---

### `GetScene`

Returns a single scene by ID.

```typescript
function GetScene(id: string): Promise<Scene>
```

**Errors:** Returns an error if the scene is not found.

---

### `CreateScene`

Creates a new scene with the given configuration.

```typescript
function CreateScene(req: CreateSceneRequest): Promise<Scene>

interface CreateSceneRequest {
  name:          string
  trigger:       "camera_on" | "camera_off" | "manual"
  devices:       Record<string, DeviceState>
  globalColor?:  Color
  globalKelvin?: number
}
```

A UUID is auto-generated for the `id` field. The scene is immediately persisted.

**Errors:** Returns `trigger "X" is already used by another scene` if another scene already uses the given trigger (only one scene per trigger is allowed).

---

### `UpdateScene`

Replaces an existing scene. The `id` field must match an existing scene.

```typescript
function UpdateScene(scene: Scene): Promise<void>
```

**Errors:** Returns an error if no scene with the given ID exists, or if `trigger "X" is already used by another scene` (another scene other than the one being updated already uses that trigger).

---

### `DeleteScene`

Permanently removes a scene by ID.

```typescript
function DeleteScene(id: string): Promise<void>
```

If the deleted scene was active, the active scene ID is cleared and a `scene:active` event is emitted with an empty string.

---

### `ActivateScene`

Manually activates a scene, pushing its device states to all configured lights. Has a **10-second timeout** to accommodate multiple slow devices.

```typescript
function ActivateScene(id: string): Promise<void>
```

On success, emits a `scene:active` event with the scene ID.

---

### `GetActiveScene`

Returns the ID of the currently active scene, or an empty string if none is active.

```typescript
function GetActiveScene(): Promise<string>
```

---

## Webcam & Monitoring

### `GetCameraState`

Returns the last known camera state (`true` = camera in use).

```typescript
function GetCameraState(): Promise<boolean>
```

This is a cached value from the last poll, not a live OS check.

---

### `CheckCameraNow`

Forces an immediate OS-level camera state check outside of the normal polling cycle.

```typescript
function CheckCameraNow(): Promise<boolean>
```

---

### `SetMonitoringEnabled`

Pauses or resumes automatic webcam monitoring.

```typescript
function SetMonitoringEnabled(enabled: boolean): Promise<void>
```

When disabled, the webcam is no longer polled and scene triggers will not fire automatically. Emits a `monitoring:state` event.

---

### `IsMonitoringEnabled`

Returns whether automatic webcam monitoring is currently active.

```typescript
function IsMonitoringEnabled(): Promise<boolean>
```

---

## Settings

### `GetSettings`

Returns the current application settings.

```typescript
function GetSettings(): Promise<Settings>
```

---

### `UpdateSettings`

Saves new settings and immediately applies them (e.g. updating the webcam poll interval takes effect without a restart).

```typescript
function UpdateSettings(settings: Settings): Promise<void>
```

`pollIntervalMs` must be greater than 0. Values below 500 ms are rounded up to 500 ms internally.

---

## Philips Hue Bridges

### `GetHueBridges`

Returns the list of paired Hue bridges.

```typescript
function GetHueBridges(): Promise<HueBridge[]>
```

---

### `AddHueBridge`

Manually registers a Hue bridge by IP and username (API key). Use `PairHueBridge` to obtain the username interactively.

```typescript
function AddHueBridge(ip: string, username: string): Promise<void>
```

---

### `DiscoverHueBridges`

Scans for Hue bridges on the local network using SSDP and the Philips Hue N-UPnP cloud service. Has a **15-second timeout**.

```typescript
function DiscoverHueBridges(): Promise<DiscoveredHueBridge[]>

interface DiscoveredHueBridge {
  ip:   string
  name: string
}
```

---

### `PairHueBridge`

Initiates CLIP API pairing with a bridge at the given IP. The user must press the **physical link button** on the bridge before or within a few seconds of calling this method.

```typescript
function PairHueBridge(ip: string): Promise<PairResult>

interface PairResult {
  success:   boolean
  username:  string    // the API key; only present on success
  error?:    string    // human-readable error; present on failure
}
```

Common error values:

| `error` string | Meaning |
|---------------|---------|
| `"link button not pressed"` | The bridge link button was not pressed in time |
| `"cannot reach bridge: ..."` | Network error reaching the bridge |
| `"paired but failed to save: ..."` | Pairing succeeded but the credential could not be persisted |

On success, the bridge is automatically registered via `AddHueBridge`.

---

## Events Reference

The backend emits these Wails events. Subscribe in the frontend using `runtime.EventsOn` (or the `useWails` hook):

```typescript
import { EventsOn } from "@wailsapp/runtime"

EventsOn("camera:state", (cameraOn: boolean) => { ... })
```

| Event | Payload | Description |
|-------|---------|-------------|
| `camera:state` | `boolean` | Webcam became active (`true`) or inactive (`false`) |
| `scene:active` | `Scene` | Full scene object of the newly active scene (emitted immediately before device states are applied) |
| `scan:progress` | `ScanProgress` | Emitted at the start of each discovery phase |
| `monitoring:state` | `boolean` | Monitoring was enabled (`true`) or disabled (`false`) |

### `ScanProgress`

```typescript
interface ScanProgress {
  phase:    string    // e.g. "elgato_mdns", "hue_ssdp", "lifx_udp"
  total:    number    // total number of phases
  current:  number    // 1-based index of the current phase
  message:  string    // human-readable description
}
```
