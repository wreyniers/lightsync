# LightSync

A desktop application for automatically controlling smart lights based on webcam activity. When your camera turns on, your lighting scene activates. When it turns off, your lights revert. Built with [Wails](https://wails.io) (Go + React/TypeScript).

---

## Table of Contents

- [Features](#features)
- [Supported Devices](#supported-devices)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Usage Guide](#usage-guide)
  - [Discovering Lights](#discovering-lights)
  - [Creating Scenes](#creating-scenes)
  - [Triggers](#triggers)
  - [Settings](#settings)
  - [Philips Hue Setup](#philips-hue-setup)
- [System Tray](#system-tray)
- [Configuration File](#configuration-file)
- [Architecture Overview](#architecture-overview)
- [Project Structure](#project-structure)
- [Development](#development)
- [Building](#building)
- [Documentation](#documentation)

---

## Features

- **Webcam-triggered automation** — scenes activate automatically when your camera turns on or off
- **Screen Sync** — continuously captures the screen, extracts colors, and drives your lights in real time; supports monitor, region, window, and active-window capture modes
- **Multi-brand support** — control LIFX, Philips Hue, Elgato Key Light, and Govee devices from one interface
- **Scene editor** — define per-device states (power, brightness, color, color temperature) and save them as named scenes
- **Global color/temperature override** — apply a single color or Kelvin value to all devices in a scene at once
- **Auto-discovery** — finds lights on your local network via mDNS, SSDP, and subnet probing; no manual IP entry required
- **System tray** — runs minimized, accessible via tray icon with pause/resume control; close button minimizes to tray, "Exit" quits
- **Single-instance enforcement** — prevents duplicate app instances from running simultaneously
- **Persistent config** — device list, scenes, and settings survive restarts

---

## Supported Devices

| Brand | Discovery Method | Protocol |
|-------|-----------------|----------|
| LIFX | UDP broadcast | LIFX LAN |
| Philips Hue | SSDP + N-UPnP cloud + subnet probe | HTTP/HTTPS (Hue API v2) |
| Elgato Key Light | mDNS (`_elg._tcp`) + subnet probe | HTTP REST |
| Govee | LAN broadcast | Govee LAN API |

---

## Prerequisites

### To run the built application

- **Windows 10/11** or **macOS 12+**
- Smart lights on the same local network

### To build from source

- [Go](https://go.dev/dl/) 1.24+
- [Node.js](https://nodejs.org/) 18+ with npm
- [Wails CLI](https://wails.io/docs/gettingstarted/installation) v2: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

---

## Installation

Download the latest release for your platform from the [Releases](../../releases) page and run the installer.

Or build from source — see the [Development](#development) section.

---

## Usage Guide

### Discovering Lights

1. Open LightSync and navigate to the **Settings** tab.
2. Click **Scan Network**. The app runs a multi-phase scan:
   - mDNS for Elgato devices
   - SSDP + N-UPnP cloud lookup for Hue bridges
   - UDP broadcast for LIFX and Govee
   - Subnet HTTP probe as a fallback for Elgato and Hue
3. Discovered devices appear grouped by brand. Each card shows the device name, current power state, brightness, and color.
4. You can control lights directly from the **Lights** tab — toggle power, adjust brightness, and change color or color temperature.

**Discovery fails on Windows?** mDNS and UDP broadcast can be blocked by Windows Firewall. When you first run LightSync, allow it through the firewall if prompted. If discovery finds no devices:
- **Elgato**: Use the "Add Elgato Key Light by IP" field in Settings (e.g. `192.168.4.73`) to add your device manually.
- **LIFX / others**: Ensure the app is allowed for private networks in Windows Defender Firewall.

> **Philips Hue** requires pairing a bridge first. See [Philips Hue Setup](#philips-hue-setup).

### Creating Scenes

1. Go to the **Scenes** tab and click **New Scene**.
2. Give the scene a name and choose a [trigger](#triggers).
3. Configure each device: toggle it on/off, set brightness, choose a color or Kelvin temperature.
4. Optionally apply a **global color** or **global Kelvin** to all devices at once.
5. Click **Save**.

You can also **activate a scene manually** by clicking the play button next to it in the scene list.

### Triggers

Each scene can have one of the following triggers:

| Trigger | When it activates |
|---------|------------------|
| `camera_on` | Automatically when the webcam is detected as in use |
| `camera_off` | Automatically when the webcam is no longer in use |
| `manual` | Only when you click the activate button in the UI |
| `screen_sync` | Continuous screen capture; the engine drives lights in real time |

Only one scene per trigger type is active at a time (except `screen_sync`, which allows multiple scenes — each with its own capture and device configuration — but only one runs at once).

### Screen Sync

Screen Sync scenes continuously capture the screen and drives your lights in real time:

1. Create a new scene and set the trigger to **Screen Sync**.
2. Select a **capture source**: full monitor, a custom region drawn on screen, a specific window, or the active window.
3. Choose a **color mode**: single color (all lights match the dominant color) or multi-color (lights follow distinct screen zones via spatial grid or scene palette).
4. Assign the lights that should follow the screen under **Devices**.
5. Tune **brightness**, **smoothing**, **scene-cut detection**, and the **assignment strategy** in the advanced tabs.
6. Click the **play** button in the sidebar or scene list to start. A live color preview and performance stats appear while running.

### Settings

Navigate to the **Settings** tab to configure:

| Setting | Description | Default |
|---------|-------------|---------|
| Poll interval | How often the app checks webcam state (250 ms – 5000 ms) | 1000 ms |
| Start minimized | Launch directly to the system tray on startup | Off |
| Launch at login | Start LightSync automatically when you log in | Off |

### Philips Hue Setup

1. Go to **Settings → Hue Bridges**.
2. Click **Discover Bridges** — the app searches via SSDP and the Hue N-UPnP cloud service.
3. Select a bridge and click **Pair**. You have 30 seconds to press the **physical link button** on the bridge.
4. Once paired, the bridge is saved and Hue lights appear during device discovery.

---

## System Tray

LightSync minimizes to the system tray when you close the window. Right-click the tray icon for options:

- **Show** — bring the main window back
- **Pause / Resume monitoring** — temporarily disable webcam-triggered automations
- **Quit** — exit the application

---

## Configuration File

Settings, devices, and scenes are stored as a JSON file at:

| Platform | Path |
|----------|------|
| Windows | `%APPDATA%\lightsync\config.json` |
| macOS | `~/Library/Application Support/lightsync/config.json` |

The file is human-readable and can be edited manually, though the app overwrites it on any change.

---

## Architecture Overview

```
┌──────────────────────────────────────────────────────┐
│                   Wails Desktop App                   │
│                                                       │
│  ┌───────────────────┐    ┌─────────────────────────┐ │
│  │  React Frontend   │    │      Go Backend          │ │
│  │  (TypeScript)     │◄──►│      (app.go)            │ │
│  │                   │    │                          │ │
│  │  Lights tab       │    │  ┌────────────────────┐  │ │
│  │  Scenes tab       │    │  │   LightManager     │  │ │
│  │  Settings tab     │    │  │  LIFX / Hue        │  │ │
│  │                   │    │  │  Elgato / Govee    │  │ │
│  │  useLightStore    │    │  └────────────────────┘  │ │
│  │  (optimistic UI)  │    │                          │ │
│  └───────────────────┘    │  ┌────────────────────┐  │ │
│                           │  │   SceneManager     │  │ │
│   Events (Wails IPC):     │  └────────────────────┘  │ │
│   camera:state ───────────►                          │ │
│   scene:active ◄──────────  ┌────────────────────┐  │ │
│   scan:progress ◄─────────  │  WebcamMonitor     │  │ │
│   monitoring:state ◄──────  └────────────────────┘  │ │
│   screensync:colors ◄─────                          │ │
│   screensync:stats ◄──────  ┌────────────────────┐  │ │
│   screensync:state ◄──────  │  ScreenSync Engine │  │ │
│                           │  │  capture → extract │  │ │
│                           │  │  → process → send  │  │ │
│                           │  └────────────────────┘  │ │
│                           │                          │ │
│                           │  ┌────────────────────┐  │ │
│                           │  │      Store         │  │ │
│                           │  │  (config.json)     │  │ │
│                           │  └────────────────────┘  │ │
│                           └─────────────────────────┘ │
└──────────────────────────────────────────────────────┘
```

See [`docs/architecture.md`](docs/architecture.md) for the full architecture deep-dive.

---

## Project Structure

```
lightsync/
├── main.go                    # Wails entry point, window config
├── app.go                     # App struct, all Wails-bound methods
├── tray.go                    # System tray setup
├── singleinstance_windows.go  # Single-instance mutex (Windows)
│
├── internal/
│   ├── lights/
│   │   ├── types.go           # Device, DeviceState, Color, Brand
│   │   ├── controller.go      # Controller interface
│   │   ├── manager.go         # Routes calls to brand controllers
│   │   ├── lifx.go            # LIFX LAN UDP controller
│   │   ├── hue.go             # Philips Hue HTTP controller (HTTP/2, connection pooling)
│   │   ├── elgato.go          # Elgato Key Light HTTP controller
│   │   └── govee.go           # Govee LAN controller (single-packet color updates)
│   ├── discovery/
│   │   └── scanner.go         # Multi-protocol network scanner
│   ├── scenes/
│   │   └── manager.go         # Scene CRUD, trigger handling
│   ├── screensync/            # Screen Sync engine
│   │   ├── engine.go          # Orchestrates capture → extract → process → assign → send
│   │   ├── handoff.go         # Per-device color crossfade on assignment changes
│   │   ├── stats.go           # Per-second performance metrics
│   │   ├── assign/            # Color-to-device assignment strategies
│   │   │   ├── assigner.go    # Assigner interface + factory
│   │   │   ├── flow_track.go  # EMA trajectory + Hungarian solve
│   │   │   ├── identity_lock.go  # Anchor-based stable assignment
│   │   │   ├── scene_cut_remap.go # Full remap on scene cuts
│   │   │   ├── zone_dominant.go  # Permanent positional binding
│   │   │   ├── distance.go    # Color distance helpers
│   │   │   └── solver.go      # Jonker-Volgenant Hungarian algorithm
│   │   ├── capture/           # Screen capture backends
│   │   │   ├── capture.go     # Capturer interface + factory
│   │   │   ├── dxgi_windows.go   # DXGI desktop duplication (Windows)
│   │   │   ├── gdi_windows.go    # GDI BitBlt fallback (Windows)
│   │   │   ├── window_windows.go # Window enumeration + capture
│   │   │   ├── overlay_windows.go # Region-select full-screen overlay
│   │   │   ├── monitor.go     # MonitorInfo, GetMonitors
│   │   │   └── region.go      # CaptureRect helpers
│   │   ├── extract/           # Color extraction algorithms
│   │   │   ├── extract.go     # DispatchExtractor, single/multi dispatch
│   │   │   ├── dominant.go    # Dominant / brightest / saturated / vivid
│   │   │   ├── palette.go     # Scene palette (histogram accumulation)
│   │   │   └── spatial.go     # Spatial grid (per-zone extraction)
│   │   └── process/           # Post-extraction processing
│   │       ├── adjustments.go # Hue correction, saturation boost, brightness clamp
│   │       ├── scenechange.go # Scene-cut detection (brightness + hue jump)
│   │       └── smoother.go    # Adaptive EMA color + windowed brightness smoothing
│   ├── store/
│   │   ├── store.go           # JSON persistence layer
│   │   └── screensync_types.go # ScreenSyncConfig and enums
│   └── webcam/
│       ├── monitor.go         # Polling loop, state-change events
│       ├── camera_windows.go  # Windows registry detection
│       └── camera_darwin.go   # macOS AVFoundation detection
│
├── frontend/
│   └── src/
│       ├── App.tsx            # Root component, tab routing + popup mode
│       ├── components/
│       │   ├── Layout.tsx          # Sidebar with scene play/stop, last-scene restore
│       │   ├── Lights.tsx          # Device list, scan UI
│       │   ├── LightsPopupPage.tsx # Detached lights popup window
│       │   ├── Scenes.tsx          # Scene list and editor
│       │   ├── SceneRow.tsx        # Expandable scene card with Screen Sync preview
│       │   ├── Settings.tsx        # App preferences, Hue pairing
│       │   ├── CloseConfirmDialog.tsx  # Minimize vs quit dialog
│       │   ├── screensync/         # Screen Sync editor components
│       │   │   ├── ScreenSyncEditor.tsx    # Tab container
│       │   │   ├── CaptureTab.tsx          # Source selection
│       │   │   ├── ColorsTab.tsx           # Extraction settings
│       │   │   ├── BrightnessTab.tsx       # Brightness mode + range
│       │   │   ├── TransitionsTab.tsx      # Smoothing + assignment engine
│       │   │   ├── DevicesTab.tsx          # Device assignment
│       │   │   ├── SetupWizard.tsx         # First-run setup flow
│       │   │   ├── ScreenSyncSidebarWidget.tsx  # Live stats in sidebar
│       │   │   ├── ColorPreview.tsx        # Live color swatches
│       │   │   ├── MonitorSelector.tsx     # Monitor grid picker
│       │   │   ├── SpatialGridPreview.tsx  # Device-to-zone mapping preview
│       │   │   └── WindowPicker.tsx        # Window thumbnail list
│       │   └── ui/            # Reusable UI primitives
│       ├── hooks/
│       │   └── useLightStore.ts  # External store with optimistic updates + Screen Sync bridge
│       └── lib/
│           ├── types.ts       # TypeScript type definitions (incl. ScreenSyncConfig)
│           └── utils.ts       # Color conversion, Kelvin → CSS
│
├── build/                     # Build assets (icons, manifests)
├── docs/                      # Extended documentation
│   ├── architecture.md
│   ├── api.md
│   └── development.md
├── go.mod
├── wails.json
└── README.md
```

---

## Development

```powershell
# Install Wails CLI (first time only)
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Install frontend dependencies
cd frontend
npm install
cd ..

# Start dev server (hot reload for both Go and React)
wails dev
```

The dev server exposes a browser-accessible version of the frontend at `http://localhost:34115`. Go methods are callable from the browser devtools console via the `window.go` namespace.

See [`docs/development.md`](docs/development.md) for a full local setup guide, including environment details and troubleshooting.

---

## Building

```powershell
# Production build (Wails v3 — creates bin/lightsync.exe and copies to build/bin/)
.\dev build
```

Or manually:

```powershell
wails3 task build -p PRODUCTION=true
```

The build embeds the compiled React frontend into the Go binary — no separate web server is needed at runtime.

---

## Documentation

| Document | Description |
|----------|-------------|
| [`docs/architecture.md`](docs/architecture.md) | System design, component interactions, data flow, event reference |
| [`docs/api.md`](docs/api.md) | All Wails-exposed Go methods callable from the frontend |
| [`docs/development.md`](docs/development.md) | Local setup, tooling, conventions, and contribution guide |
