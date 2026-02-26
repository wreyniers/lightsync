# LightSync — Architecture

This document describes the internal design of LightSync: how components are organised, how data flows through the system, and the key design decisions behind each layer.

---

## Table of Contents

- [High-Level Overview](#high-level-overview)
- [Technology Stack](#technology-stack)
- [Backend (Go)](#backend-go)
  - [Entry Point](#entry-point)
  - [App struct (app.go)](#app-struct-appgo)
  - [Light Manager & Controllers](#light-manager--controllers)
  - [Discovery Scanner](#discovery-scanner)
  - [Scene Manager](#scene-manager)
  - [Webcam Monitor](#webcam-monitor)
  - [Persistent Store](#persistent-store)
  - [System Tray](#system-tray)
- [Frontend (React/TypeScript)](#frontend-reacttypescript)
  - [Component Tree](#component-tree)
  - [State Management](#state-management)
  - [Optimistic Updates](#optimistic-updates)
- [IPC — Wails Binding Layer](#ipc--wails-binding-layer)
  - [Method Calls (Frontend → Backend)](#method-calls-frontend--backend)
  - [Events (Backend → Frontend)](#events-backend--frontend)
- [Data Flow Diagrams](#data-flow-diagrams)
  - [Startup Sequence](#startup-sequence)
  - [Camera State Change](#camera-state-change)
  - [Device Discovery](#device-discovery)
  - [Manual Light Control](#manual-light-control)
- [Data Model](#data-model)
- [Persistence](#persistence)
- [Platform Differences](#platform-differences)
- [Key Design Decisions](#key-design-decisions)

---

## High-Level Overview

LightSync is a **Wails v2** desktop application — a thin shell around a Go backend with a React/TypeScript frontend rendered inside a native WebView. The two layers communicate via:

1. **Direct RPC calls** — TypeScript calls Go methods synchronously (Wails generates the bindings).
2. **One-way events** — Go emits named events that the frontend subscribes to.

```
┌──────────────────────────────────────────────────────────────────┐
│                        LightSync Process                          │
│                                                                    │
│   ┌─────────────────────────────┐                                 │
│   │        React UI (WebView)   │                                 │
│   │                             │                                 │
│   │   ┌──────────┐ ┌─────────┐  │   Wails RPC / Events          │
│   │   │  Lights  │ │ Scenes  │  │◄────────────────────────────┐  │
│   │   └──────────┘ └─────────┘  │                             │  │
│   │   ┌──────────┐ ┌─────────┐  │                             │  │
│   │   │Settings  │ │ Layout  │  │─────────────────────────────►│  │
│   │   └──────────┘ └─────────┘  │                             │  │
│   └─────────────────────────────┘                             │  │
│                                                                │  │
│   ┌────────────────────────────────────────────────────────┐  │  │
│   │                   Go Backend (app.go)                   │  │  │
│   │                                                         │◄─┘  │
│   │  LightManager ── LIFX / Hue / Elgato / Govee          │     │
│   │  SceneManager                                           │     │
│   │  WebcamMonitor (OS-level polling)                       │     │
│   │  Discovery Scanner                                      │     │
│   │  Store (config.json)                                    │     │
│   │  System Tray                                            │     │
│   └────────────────────────────────────────────────────────┘     │
└──────────────────────────────────────────────────────────────────┘
```

---

## Technology Stack

| Layer | Technology | Version |
|-------|-----------|---------|
| Runtime | Wails | v2 |
| Backend language | Go | 1.24+ |
| Frontend framework | React | 18.3 |
| Frontend language | TypeScript | 5.7 |
| Build tool (frontend) | Vite | 6.0 |
| Styling | Tailwind CSS | 4.0 |
| Icons | Lucide React | latest |
| Color picker | @jaames/iro | latest |
| LIFX protocol | go-lifx (LAN) | — |
| Hue protocol | openhue-go | — |
| mDNS | hashicorp/mdns | — |

---

## Backend (Go)

### Entry Point

`main.go` does three things:

1. Calls `ensureSingleInstance()` — on Windows this acquires a named mutex; a second launch exits immediately.
2. Creates the `App` struct.
3. Calls `wails.Run(...)` with window options (1100×720, fixed size, dark background, `HideWindowOnClose: true`) and binds the `App` struct so all its public methods are callable from TypeScript.

### App struct (app.go)

The `App` struct owns references to every backend subsystem:

```
App
├── ctx            context.Context  (Wails application context)
├── store          *store.Store
├── lightManager   *lights.Manager
├── sceneManager   *scenes.Manager
├── webcamMon      *webcam.Monitor
├── scanner        *discovery.Scanner
├── lifxCtrl       *lights.LIFXController
├── hueCtrl        *lights.HueController
├── elgatoCtrl     *lights.ElgatoController
└── goveeCtrl      *lights.GoveeController
```

`startup(ctx)` is called by Wails after the window is created. It:

1. Initialises the store (load config.json from disk).
2. Creates and registers all brand controllers with the light manager.
3. Re-adds any stored Hue bridges and pre-discovers their lights.
4. Restores the saved device list into the light manager.
5. Creates the scene manager and wires up the `scene:active` event emitter.
6. Creates the webcam monitor with the stored poll interval; wires up the `camera:state` event and scene trigger handler.
7. Starts the webcam monitor in a background goroutine.
8. Sets up the system tray.

### Light Manager & Controllers

```
lights.Controller (interface)
  ├── Discover(ctx) ([]Device, error)
  ├── SetState(ctx, id, DeviceState) error
  ├── TurnOn(ctx, id) error
  ├── TurnOff(ctx, id) error
  └── Close() error

lights.Manager
  ├── RegisterController(Controller)
  ├── GetDevices() []Device
  ├── SetDevices([]Device)
  ├── GetDeviceState(ctx, id) (DeviceState, error)
  ├── SetDeviceState(ctx, id, DeviceState) error
  ├── TurnOn(ctx, id) / TurnOff(ctx, id)
  └── Close()
```

The `Manager` maintains a registry of brand controllers. When a method like `SetDeviceState` is called it looks up the device's brand and delegates to the correct controller. This keeps brand-specific protocol details fully encapsulated.

#### Brand Controllers

| Controller | Discovery | Control |
|-----------|-----------|---------|
| `LIFXController` | UDP broadcast on port 56700 | LIFX LAN protocol |
| `HueController` | Via registered bridges (HTTP) | Hue API v2 over HTTPS |
| `ElgatoController` | mDNS `_elg._tcp` + HTTP probe | HTTP REST to port 9123 |
| `GoveeController` | UDP LAN discovery | Govee LAN JSON API |

### Discovery Scanner

`internal/discovery/scanner.go` runs a coordinated multi-phase scan:

```
ScanAll()
 ├── Phase 1: mDNS scan (Elgato _elg._tcp)
 ├── Phase 2: Elgato subnet HTTP probe (fallback)
 ├── Phase 3: Hue SSDP + N-UPnP cloud lookup
 ├── Phase 4: Hue subnet HTTP probe (fallback)
 ├── Phase 5: LIFX UDP controller discovery
 └── Phase 6: Govee UDP controller discovery
```

Progress callbacks emit `scan:progress` events to the frontend so the UI can display a live progress bar. Results are merged into the light manager's device list and saved to the store.

### Scene Manager

`internal/scenes/manager.go` handles:

- **CRUD** — create, read, update, delete scenes in the store.
- **Trigger uniqueness** — only one scene per trigger (`camera_on`, `camera_off`, `manual`). `CreateScene` and `UpdateScene` return an error if the trigger is already in use.
- **Activation** — emits `scene:active` with the full scene object immediately (so the UI can apply preset states optimistically), then iterates the scene's device map and calls `lightManager.SetDeviceState` for each entry. A 10-second context timeout guards against unresponsive devices.
- **Trigger routing** — `OnCameraStateChange(ctx, cameraOn bool)` scans all scenes for a matching trigger and activates the first match.
- **OnChange callback** — `OnChange(fn func(scene store.Scene))` receives the full scene object when a scene is activated, not just the scene ID.

### Webcam Monitor

`internal/webcam/monitor.go` is a polling loop:

```
Monitor
├── interval    time.Duration  (configurable, default 1 s)
├── enabled     bool           (can be paused from tray)
├── lastState   bool
└── onChange    func(bool)     (callback)
```

On each tick it calls the platform-specific `checkCamera()` function:

- **Windows** (`camera_windows.go`): reads the registry key `HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\CapabilityAccessManager\ConsentStore\webcam\NonPackaged` and inspects sub-key `LastUsedTimeStop` values. A zero stop-time means the camera is currently active.
- **macOS** (`camera_darwin.go`): uses AVFoundation to query running capture sessions.

When the boolean state changes, the callback fires: it emits `camera:state` and calls `sceneManager.OnCameraStateChange`.

### Persistent Store

`internal/store/store.go` wraps a single `Config` struct:

```go
Config {
  Devices    []lights.Device
  Scenes     []Scene
  Settings   Settings
  HueBridges []HueBridge
}
```

All reads use `sync.RWMutex` for concurrency safety. Every write atomically updates the in-memory config and flushes to `config.json` via `os.WriteFile`. The file is pretty-printed JSON for human readability.

Config file locations:

| Platform | Path |
|----------|------|
| Windows | `%APPDATA%\lightsync\config.json` |
| macOS | `~/Library/Application Support/lightsync/config.json` |
| Linux | `~/.config/lightsync/config.json` |

### System Tray

`tray.go` uses the Wails systray API to add three menu items: **Show**, **Pause/Resume monitoring**, and **Quit**. Monitoring state changes are propagated to the webcam monitor and emitted as `monitoring:state` events so the frontend toggle stays in sync.

---

## Frontend (React/TypeScript)

### Component Tree

```
App.tsx  (tab router: lights | scenes | settings)
└── Layout.tsx  (sidebar, camera status, active scene)
    ├── Lights.tsx
    │   └── ui/LightCard.tsx  (×N, one per device)
    │       ├── ui/ColorPicker.tsx
    │       ├── ui/TemperaturePicker.tsx
    │       └── ui/Slider.tsx
    ├── Scenes.tsx
    │   ├── Scene list (×N)
    │   └── Scene editor
    │       └── ui/LightCard.tsx  (×N, per-scene device config)
    └── Settings.tsx
        └── Hue bridge pairing UI
```

### State Management

State lives in `hooks/useLightStore.ts`, implemented as an **external store** consumed via React's `useSyncExternalStore`. This avoids prop drilling without a third-party state library.

The store holds:

```typescript
{
  devices:        Device[]
  deviceOn:       Record<string, boolean>
  brightness:     Record<string, number>   // 0–100 for UI
  kelvin:         Record<string, number>
  color:          Record<string, Color>
  activeScene:   Scene | null             // full scene object when active
  // cameraState and isMonitoring are fetched on demand / via events
}
```

State is populated when components call `refreshDevices`, `discoverLights`, and `hydrateActiveScene`, and kept in sync via Wails events: `camera:state`, `scene:active`, `monitoring:state`.

### Optimistic Updates

Controlling lights via HTTP can be slow (Elgato, Hue). To keep the UI feeling instant:

1. When the user changes a light state, the store immediately applies the change locally (**optimistic update**).
2. The actual Go call is debounced by **80 ms** — rapid slider drags batch into a single network call.
3. A **2-second user action grace period** starts on any user interaction. During this window, state updates from background polling do not overwrite local changes. A **4-second scene-applied grace period** similarly protects devices that were just updated by a scene activation from being overwritten by stale hardware responses.

```
User drags brightness slider
        │
        ▼
Store: apply optimistic update  ──► UI re-renders instantly
        │
        ▼  (80 ms debounce)
SetLightState(id, newState) called on Go backend
        │
        ▼
Network call to device
```

---

## IPC — Wails Binding Layer

### Method Calls (Frontend → Backend)

Wails auto-generates TypeScript bindings in `frontend/wailsjs/go/main/App.js`. All exported methods on `App` are callable as async functions.

Full reference: [`docs/api.md`](api.md)

### Events (Backend → Frontend)

| Event name | Payload type | Emitted when |
|-----------|-------------|--------------|
| `camera:state` | `boolean` | Webcam active/inactive state changes |
| `scene:active` | `Scene` object | A scene is activated (full scene; emitted before device states are applied) |
| `scan:progress` | `ScanProgress` object | During device discovery, one event per scan phase |
| `monitoring:state` | `boolean` | Monitoring is paused or resumed (e.g. from tray) |

`ScanProgress` shape:
```typescript
{
  phase:    string   // e.g. "elgato_mdns", "hue_ssdp"
  total:    number
  current:  number
  message:  string
}
```

---

## Data Flow Diagrams

### Startup Sequence

```
main()
  │
  ├─ ensureSingleInstance()
  │
  └─ wails.Run()
       │
       └─ app.startup(ctx)
            │
            ├─ store.New()           load config.json
            ├─ lights.NewManager()
            ├─ Register controllers  (LIFX, Hue, Elgato, Govee)
            ├─ Add stored Hue bridges + pre-discover Hue lights
            ├─ lightManager.SetDevices(storedDevices)
            ├─ discovery.NewScanner()
            ├─ scenes.NewManager()   wire OnChange → emit scene:active
            ├─ webcam.NewMonitor()   wire OnChange → emit camera:state
            │                                      → sceneManager.OnCameraStateChange
            ├─ go webcamMon.Start()  background goroutine
            └─ setupTray()
```

### Camera State Change

```
[OS webcam registry/API]
        │  (poll every N ms)
        ▼
webcam.Monitor.tick()
        │  state changed?
        ▼
OnChange callback
  ├─ runtime.EventsEmit("camera:state", cameraOn)
  │       │
  │       ▼
  │   Frontend: useLightStore receives event
  │             cameraState updated → UI re-renders
  │
  └─ sceneManager.OnCameraStateChange(ctx, cameraOn)
          │  find scene with matching trigger
          ▼
      sceneManager.ActivateScene(ctx, sceneID)
          │  iterate scene.Devices
          ▼
      lightManager.SetDeviceState(ctx, id, state)  [×N]
          │
          ▼
      BrandController.SetState(ctx, id, state)
          │
          ▼
      [Network call to physical device]
          │
          ▼
      runtime.EventsEmit("scene:active", scene)   // full scene object
          │
          ▼
      Frontend: activeScene set to scene → preset states applied optimistically → status bar re-renders
```

### Device Discovery

```
Frontend: user clicks "Scan"
        │
        ▼
App.DiscoverLights()
        │
        ├─ scanner.ScanAll(ctx, progressCallback)
        │     │
        │     ├─ Elgato mDNS       → emit scan:progress
        │     ├─ Elgato HTTP probe  → emit scan:progress
        │     ├─ Hue SSDP          → emit scan:progress
        │     ├─ Hue N-UPnP cloud  → emit scan:progress
        │     ├─ Hue HTTP probe     → emit scan:progress
        │     ├─ LIFX UDP           → emit scan:progress
        │     └─ Govee UDP          → emit scan:progress
        │
        ├─ store.SetDevices(updatedList)   persist to disk
        │
        └─ return DiscoverResult{Devices, Errors}
                │
                ▼
        Frontend: device list updated → Lights tab re-renders
```

### Manual Light Control

```
User toggles light in UI
        │
        ▼
useLightStore.setDeviceState(id, newState)
  ├─ Optimistic update in store  → immediate re-render
  └─ debounce 80 ms
          │
          ▼
      App.SetLightState(id, state)   [Wails RPC]
          │
          ▼
      lightManager.SetDeviceState(ctx, id, state)
          │
          ▼
      BrandController.SetState(...)
          │
          ▼
      Physical device updated
```

---

## Data Model

### Device

```typescript
{
  id:              string      // brand-specific unique identifier
  brand:           "lifx" | "hue" | "elgato" | "govee"
  name:            string
  model?:          string
  lastIp:          string
  lastSeen:        string      // ISO 8601 timestamp
  supportsColor:   boolean
  supportsKelvin:  boolean
  minKelvin?:      number      // 0 = unknown
  maxKelvin?:      number
  kelvinStep?:     number
  firmwareVersion?: string
}
```

### DeviceState

```typescript
{
  on:         boolean
  brightness: number        // 0.0 – 1.0
  color?:     { h: number, s: number, b: number }  // HSB, 0–360 / 0–1 / 0–1
  kelvin?:    number        // colour temperature in Kelvin
}
```

### Scene

```typescript
{
  id:            string
  name:          string
  trigger:       "camera_on" | "camera_off" | "manual"
  devices:       Record<deviceId, DeviceState>
  globalColor?:  { h, s, b }
  globalKelvin?: number
}
```

### Settings

```typescript
{
  pollIntervalMs: number   // webcam poll frequency (default 1000)
  startMinimized: boolean
  launchAtLogin:  boolean
}
```

---

## Persistence

Config is written as indented JSON to a single file. There is no database, migration system, or schema versioning — the format is intentionally simple. Unknown fields from newer versions are silently dropped by Go's `json.Unmarshal`. The file is rewritten atomically on every change (single `os.WriteFile` call).

---

## Platform Differences

| Feature | Windows | macOS |
|---------|---------|-------|
| Webcam detection | Registry (`CapabilityAccessManager`) | AVFoundation |
| Single instance | Named Win32 mutex (`singleinstance_windows.go`) | (not implemented) |
| Config path | `%APPDATA%\lightsync\` | `~/Library/Application Support/lightsync/` |
| Build artifact | `lightsync.exe` + optional NSIS installer | `LightSync.app` bundle |

---

## Key Design Decisions

### Fixed window size (1100×720)
The UI is designed for a specific layout. Disabling resize keeps the experience consistent and avoids responsive breakpoints at the cost of flexibility.

### No React state management library
`useSyncExternalStore` with a hand-rolled store provides enough capability for this app's scope without adding a dependency. The store is effectively a singleton with subscriber callbacks.

### Optimistic updates + grace period
Smart light HTTP APIs (especially Elgato) can take 200–800 ms to respond. Applying UI changes optimistically and debouncing writes gives a snappy feel without showing incorrect state when the server is slow.

### Controller interface for lights
Each brand has different discovery mechanisms and wire protocols. The `Controller` interface hides this completely — the `Manager` only ever calls `Discover`, `SetState`, `TurnOn`, `TurnOff`, and `Close`. Adding a new brand is adding a new file that implements the interface.

### Polling (not push) for webcam state
Most OS webcam APIs do not offer push notifications. Polling the registry/AVFoundation at a configurable interval (default 1 s) is the simplest reliable approach. The interval is tunable down to 250 ms for users who need faster reaction times.

### JSON persistence over embedded DB
Config data is small (dozens of devices, tens of scenes). A human-readable JSON file is simpler to debug, easier to back up, and requires no migration tooling. The trade-off is a full file rewrite on every change, which is acceptable at this data size.
