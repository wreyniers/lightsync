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
- **Multi-brand support** — control LIFX, Philips Hue, Elgato Key Light, and Govee devices from one interface
- **Scene editor** — define per-device states (power, brightness, color, color temperature) and save them as named scenes
- **Global color/temperature override** — apply a single color or Kelvin value to all devices in a scene at once
- **Auto-discovery** — finds lights on your local network via mDNS, SSDP, and subnet probing; no manual IP entry required
- **System tray** — runs minimized, accessible via tray icon with pause/resume control
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

1. Open LightSync and navigate to the **Lights** tab.
2. Click **Scan for Lights**. The app runs a multi-phase scan:
   - mDNS for Elgato devices
   - SSDP + N-UPnP cloud lookup for Hue bridges
   - UDP broadcast for LIFX and Govee
   - Subnet HTTP probe as a fallback for Elgato and Hue
3. Discovered devices appear grouped by brand. Each card shows the device name, current power state, brightness, and color.
4. You can control lights directly from the **Lights** tab — toggle power, adjust brightness, and change color or color temperature.

> **Philips Hue** requires pairing a bridge first. See [Philips Hue Setup](#philips-hue-setup).

### Creating Scenes

1. Go to the **Scenes** tab and click **New Scene**.
2. Give the scene a name and choose a [trigger](#triggers).
3. Configure each device: toggle it on/off, set brightness, choose a color or Kelvin temperature.
4. Optionally apply a **global color** or **global Kelvin** to all devices at once.
5. Click **Save**.

You can also **activate a scene manually** by clicking the play button next to it in the scene list.

### Triggers

Each scene can have one of three triggers:

| Trigger | When it activates |
|---------|------------------|
| `camera_on` | Automatically when the webcam is detected as in use |
| `camera_off` | Automatically when the webcam is no longer in use |
| `manual` | Only when you click the activate button in the UI |

Only one scene per trigger type is active at a time. Saving a new `camera_on` scene replaces the previous one.

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
┌─────────────────────────────────────────────────┐
│                  Wails Desktop App               │
│                                                  │
│  ┌──────────────────┐    ┌────────────────────┐  │
│  │  React Frontend  │    │    Go Backend       │  │
│  │  (TypeScript)    │◄──►│    (app.go)         │  │
│  │                  │    │                     │  │
│  │  Lights tab      │    │  ┌───────────────┐  │  │
│  │  Scenes tab      │    │  │ LightManager  │  │  │
│  │  Settings tab    │    │  │  LIFX / Hue   │  │  │
│  │                  │    │  │  Elgato/Govee │  │  │
│  │  useLightStore   │    │  └───────────────┘  │  │
│  │  (optimistic UI) │    │                     │  │
│  └──────────────────┘    │  ┌───────────────┐  │  │
│                          │  │ SceneManager  │  │  │
│   Events (Wails IPC):    │  └───────────────┘  │  │
│   camera:state ──────────►                     │  │
│   scene:active ◄─────────  ┌───────────────┐  │  │
│   scan:progress ◄────────  │ WebcamMonitor │  │  │
│   monitoring:state ◄─────  └───────────────┘  │  │
│                          │                     │  │
│                          │  ┌───────────────┐  │  │
│                          │  │    Store      │  │  │
│                          │  │ (config.json) │  │  │
│                          │  └───────────────┘  │  │
│                          └────────────────────┘  │
└─────────────────────────────────────────────────┘
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
│   │   ├── hue.go             # Philips Hue HTTP controller
│   │   ├── elgato.go          # Elgato Key Light HTTP controller
│   │   └── govee.go           # Govee LAN controller
│   ├── discovery/
│   │   └── scanner.go         # Multi-protocol network scanner
│   ├── scenes/
│   │   └── manager.go         # Scene CRUD, trigger handling
│   ├── store/
│   │   └── store.go           # JSON persistence layer
│   └── webcam/
│       ├── monitor.go         # Polling loop, state-change events
│       ├── camera_windows.go  # Windows registry detection
│       └── camera_darwin.go   # macOS AVFoundation detection
│
├── frontend/
│   └── src/
│       ├── App.tsx            # Root component, tab routing
│       ├── components/
│       │   ├── Layout.tsx     # Sidebar, status bar
│       │   ├── Lights.tsx     # Device list, scan UI
│       │   ├── Scenes.tsx     # Scene list and editor
│       │   ├── Settings.tsx   # App preferences, Hue pairing
│       │   └── ui/            # Reusable UI primitives (ColorPanel, ColorPicker, LightCard, etc.)
│       ├── hooks/
│       │   ├── useLightStore.ts  # External store with optimistic updates
│       │   └── useWails.ts       # Wails event subscription helper
│       └── lib/
│           ├── types.ts       # TypeScript type definitions
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
# Production build (creates ./build/bin/lightsync.exe on Windows)
wails build

# Build with installer (Windows, requires NSIS)
wails build -nsis
```

The build embeds the compiled React frontend into the Go binary — no separate web server is needed at runtime.

---

## Documentation

| Document | Description |
|----------|-------------|
| [`docs/architecture.md`](docs/architecture.md) | System design, component interactions, data flow, event reference |
| [`docs/api.md`](docs/api.md) | All Wails-exposed Go methods callable from the frontend |
| [`docs/development.md`](docs/development.md) | Local setup, tooling, conventions, and contribution guide |
