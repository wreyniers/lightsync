# LightSync — Development Guide

This guide covers everything needed to set up a local development environment, understand the project conventions, and contribute changes.

---

## Table of Contents

- [Prerequisites](#prerequisites)
- [First-Time Setup](#first-time-setup)
- [Development Workflow](#development-workflow)
  - [Running the App](#running-the-app)
  - [Hot Reload](#hot-reload)
  - [Browser Dev Mode](#browser-dev-mode)
- [Project Conventions](#project-conventions)
  - [Go Backend](#go-backend)
  - [TypeScript Frontend](#typescript-frontend)
- [Adding a New Light Brand](#adding-a-new-light-brand)
- [Adding a New Wails Method](#adding-a-new-wails-method)
- [Modifying the Store / Data Model](#modifying-the-store--data-model)
- [Building](#building)
- [Colour-Extraction Self-Test](#colour-extraction-self-test)
  - [Self-test calibration (screen sync)](#self-test-calibration-screen-sync)
- [Troubleshooting](#troubleshooting)

---

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.24+ | https://go.dev/dl/ |
| Node.js | 18+ | https://nodejs.org/ |
| npm | 9+ | Bundled with Node |
| Wails CLI | v2 (latest) | `go install github.com/wailsapp/wails/v2/cmd/wails@latest` |
| Git | any | https://git-scm.com/ |

Verify your Wails installation:

```powershell
wails doctor
```

This checks that all required dependencies (WebView2 on Windows, Xcode on macOS) are present.

---

## First-Time Setup

```powershell
# Clone the repository
git clone <repo-url>
cd lightsync

# Install Go dependencies
go mod download

# Install frontend dependencies
cd frontend
npm install
cd ..
```

---

## Development Workflow

### Running the App

```powershell
wails dev
```

This starts:

1. A Vite dev server for the React frontend (with HMR).
2. A Wails dev host that wraps the Go backend and renders the frontend in a native window.

The app window opens automatically. Any change to a `.go` file triggers a Go rebuild and window reload. Any change to a frontend file triggers Vite's HMR without a full reload.

### Hot Reload

| Change type | Behaviour |
|-------------|-----------|
| React component (`.tsx`) | Instant HMR — no reload required |
| CSS / Tailwind class | Instant HMR |
| Go source file | Full recompile + window reload (~2–5 s) |
| `wails.json` | Restart `wails dev` |

### Browser Dev Mode

While `wails dev` is running, the frontend is also accessible at:

```
http://localhost:34115
```

Open this in any browser to use DevTools. Go methods are callable via the `window.go` namespace, for example:

```javascript
window.go.main.App.GetDevices().then(console.log)
```

Note that features requiring native OS access (webcam detection, system tray) only work inside the Wails window, not the browser.

---

## Project Conventions

### Go Backend

**Package structure** follows domain boundaries:

```
internal/lights/    — device abstraction and brand controllers
internal/discovery/ — network scanning
internal/scenes/    — scene logic
internal/store/     — persistence
internal/webcam/    — OS camera integration
```

**Naming:**
- Exported functions on `App` that are bound to Wails follow the pattern `VerbNoun` (e.g. `GetDevices`, `SetLightState`, `ActivateScene`).
- Internal helpers use standard Go conventions (unexported, lower camelCase).

**Error handling:**
- All network operations must use a `context.WithTimeout` — never an unbounded context.
- Errors are returned to the frontend as Go `error` values, which Wails serialises to a rejected Promise.
- Log unexpected internal errors with `runtime.LogErrorf` / `runtime.LogWarningf`; do not log user-facing errors redundantly.

**Concurrency:**
- The `Store` uses `sync.RWMutex` — always acquire the appropriate lock.
- The `WebcamMonitor` goroutine is the only background goroutine. New goroutines must respect the application context so they shut down cleanly.

### TypeScript Frontend

**Imports** use the `@/` alias for `src/`:

```typescript
import { Device } from "@/lib/types"
import { useLightStore } from "@/hooks/useLightStore"
```

**Component files** are co-located with their siblings under `components/`. Reusable primitives (no business logic) go in `components/ui/`.

**State management:** All shared state lives in `useLightStore`. Components should not call Wails methods directly for state that affects other parts of the UI — route through the store instead. One-off calls (e.g. triggering a scan) may call Wails methods directly.

**Styling:** Use Tailwind CSS utility classes. Avoid inline `style` props except for dynamic values that cannot be expressed with utilities (e.g. computed HSL colors from user-chosen hues).

**Types:** All shared data types are defined in `lib/types.ts` and must be kept in sync with the Go types in `internal/lights/types.go` and `internal/store/store.go`.

---

## Adding a New Light Brand

1. **Create a controller file** at `internal/lights/<brand>.go`. Implement the `Controller` interface:

```go
type Controller interface {
    Discover(ctx context.Context) ([]Device, error)
    SetState(ctx context.Context, deviceID string, state DeviceState) error
    TurnOn(ctx context.Context, deviceID string) error
    TurnOff(ctx context.Context, deviceID string) error
    Close() error
}
```

2. **Add the brand constant** to `internal/lights/types.go`:

```go
BrandMyBrand Brand = "mybrand"
```

3. **Register the controller** in `app.go` inside `startup()`:

```go
myCtrl := lights.NewMyBrandController()
a.lightManager.RegisterController(myCtrl)
```

4. **Add discovery phases** to `internal/discovery/scanner.go` if the brand needs its own scan step.

5. **Update `docs/api.md`** and the supported devices table in `README.md`.

---

## Adding a New Wails Method

1. Add a public method to the `App` struct in `app.go`:

```go
func (a *App) MyNewMethod(param string) (string, error) {
    // implementation
}
```

2. Run `wails dev` (or `wails generate module`) — Wails auto-generates the TypeScript binding.

3. The method is immediately callable from the frontend:

```typescript
import { MyNewMethod } from "@wailsapp/runtime"
// or via the auto-generated path:
import { MyNewMethod } from "../wailsjs/go/main/App"

const result = await MyNewMethod("value")
```

4. Add the method to `docs/api.md` with its signature and description.

---

## Modifying the Store / Data Model

The persistent config is a flat JSON file — there is no migration system. When changing the `Config`, `Scene`, `Settings`, or `HueBridge` structs:

1. Make the Go struct change in `internal/store/store.go`.
2. Add `omitempty` to new optional fields so existing config files remain valid (missing fields default to zero values).
3. Mirror any changes to `frontend/src/lib/types.ts`.
4. If removing or renaming a field, check all call sites in both Go and TypeScript.
5. Document any breaking change in the commit message — users upgrading may need to manually edit their `config.json`.

---

## Building

### Development build (debug info, no optimisations)

```powershell
wails build -debug
```

Output: `build/bin/lightsync.exe`

### Production build

```powershell
wails build
```

The React frontend is compiled by Vite and embedded into the Go binary via `//go:embed`. No separate web server or assets directory is needed at runtime.

### Windows installer (requires [NSIS](https://nsis.sourceforge.io/))

```powershell
wails build -nsis
```

Output: `build/bin/lightsync-installer.exe`

### macOS app bundle

```bash
wails build
```

Output: `build/bin/LightSync.app`

---

## Colour-Extraction Self-Test

The screen-sync pipeline has three complementary layers of regression
protection. Knowing which layer to reach for saves a lot of debugging time
when a colour looks wrong on the bulb.

| Layer | Where it lives | What it catches |
|---|---|---|
| **Unit/integration tests** | `internal/screensync/extract/pipeline_test.go` | Synthetic fixtures (solids, quadrant) with tight tolerances + real photos with loose tolerances. Runs under normal `go test ./...`. Catches channel swaps, magenta drift, HSL↔HSB mismatches. |
| **Probe** | `internal/screensync/extract/probe_test.go` | Not an assertion — prints the HSB triples the production pipeline extracts from every `testdata/` image across every mode. Used to regenerate baselines for the Self-Test harness. Gated on `LIGHTSYNC_PROBE=1`. |
| **In-app Self-Test** | `internal/screensync/selftest/` + the **Self-Test** tab in the running app | Drives the full real pipeline end-to-end — DXGI capture of a fullscreen fixture popup, the same Extract / Adjust / Smooth / Assign / Handoff chain the engine uses, convergence tracking, and a copy-pasteable diagnostic blob. Catches anything the unit tests miss (DWM compositing, DPI, actual UDP send paths). |

### Regenerating the Self-Test baselines (the probe)

The baselines in `internal/screensync/selftest/fixtures.go` are golden-file
snapshots of what the extractor produces for each `testdata/` image. They
need to be refreshed any time the extraction algorithm, saturation boost,
brightness adjustments, or mode matrix changes — otherwise the in-app
Self-Test will flag false "drifting" verdicts.

**PowerShell (Windows):**

```powershell
$env:LIGHTSYNC_PROBE="1"
go test -run TestProbeTestdata -v ./internal/screensync/extract/...
$env:LIGHTSYNC_PROBE=""
```

**bash / zsh (macOS / Linux):**

```bash
LIGHTSYNC_PROBE=1 go test -run TestProbeTestdata -v ./internal/screensync/extract/...
```

At the end of the output, the probe emits a block starting with
`========== PASTE BELOW INTO fixtures.go ==========`. Each fixture's
`map[string][]lights.Color{…}` block is pre-keyed by fixture ID and mode
slug; drop each one into the corresponding `expect<Name>()` helper in
`internal/screensync/selftest/fixtures.go`, replacing the previous literal.

The probe is deliberately skipped under plain `go test ./...` so CI stays
fast and the baselines never drift silently between manual refreshes.

### Adding or swapping fixtures

1. **Photo fixtures** — drop a `.jpg` or `.png` into both
   `internal/screensync/extract/testdata/` (for the probe) **and**
   `frontend/public/testdata/` (for the app to serve to the fixture popup).
   Most photos should live in both places; only the synthetics diverge
   because they're generated, not downloaded.
2. **Synthetic fixtures** — edit `scripts/gen-synth-fixtures.go` and rerun
   `go run scripts/gen-synth-fixtures.go`. This writes PNGs into
   `frontend/public/testdata/`; copy the new files over to
   `internal/screensync/extract/testdata/` as well.
3. **Register the fixture** in `selftest.BuiltInFixtures()` with an ID slug,
   label, description, URL, kind, and an `expect<Name>()` helper.
4. **Run the probe** (section above) and paste the emitted literal into
   your new helper.
5. **Add a `fixtureIDFromName` case** in `probe_test.go` so future probe
   runs key the paste-block to your fixture ID.

### Running the in-app Self-Test

1. Launch the production build (`wails3 task windows:build:production`,
   then `bin\lightsync.exe`) so the DXGI capture path matches what real
   users run.
2. Click the **Self-Test** tab (flask icon) in the left nav.
3. Pick a fixture — a fullscreen popup appears with just that image and an
   "ESC to stop" hint. Capture runs at 2 Hz through the full engine-
   equivalent pipeline.
4. Watch the results grid. Every row shows a three-stripe swatch per slot:
   **expected / target / active**. Active slides into target during the
   handoff ramp; the ring goes green and `locked · Xs` appears once active
   matches expected for two consecutive frames.
5. If any cell doesn't lock within 10 s, the header grows a **Copy
   Diagnostics** button. Clicking it dumps a markdown + JSON blob onto the
   clipboard with the exact HSB triples, per-slot deltas, and a one-word
   verdict per cell (`locked`, `wrong-hue`, `hue-drift`, `sat-drift`,
   `brightness-drift`, `drifting`, `no-baseline`, or `empty-output`).

Paste that blob into any bug report or PR description to communicate
exactly what the pipeline produced versus what was expected.

### Self-test calibration (screen sync)

After **Run Full Suite** completes, the Self-Test page offers **Screen sync
calibration** for any scene whose trigger is `screen_sync`.

1. Pick the target scene from the dropdown.
2. Click **Calibrate**. The app runs a bounded coordinate search over a
   **subset** of built-in fixtures (fast synthetics-first set), reusing the
   same live DXGI + pipeline path as the suite. Each evaluation scores the
   run with a **balanced** cost (accuracy, lock quality, stability) — lower
   is better. Nothing is written to disk during the search.
3. When the run finishes, review **baseline vs candidate** metrics and the
   list of **changed knobs** only.
4. **Apply to scene** persists the candidate `ScreenSyncConfig` on that scene.
   **Rollback last apply** restores the config snapshot from immediately before
   the last successful apply for that same scene (one level).
5. **Copy report JSON** grabs the full `CalibrationReport` for bug reports or
   tuning notes.

Regenerate JS bindings after changing exported Go app methods:

```powershell
wails3 generate bindings
```

---

## Troubleshooting

### `wails doctor` reports missing WebView2

Install the [Microsoft Edge WebView2 Runtime](https://developer.microsoft.com/en-us/microsoft-edge/webview2/). It ships by default on Windows 11 and most Windows 10 installations.

### Wails dev fails with "port already in use"

The Vite dev server defaults to port 5173. Kill any conflicting process or change the port in `frontend/vite.config.ts`.

### Lights are not discovered

- Ensure your computer and the lights are on the **same subnet**.
- Check your firewall — the app needs outbound UDP on ports 56700 (LIFX), and inbound UDP for mDNS (5353) and Govee LAN.
- Elgato: try a manual subnet probe by clicking Scan again; it falls back to HTTP probing the entire `/24` subnet.
- Hue: the bridge must be paired first (Settings tab). See [Philips Hue Setup](../README.md#philips-hue-setup).

### App opens a second instance

On Windows, the single-instance mutex in `singleinstance_windows.go` prevents this. If you see two windows during development, run `wails dev` only once.

### Config becomes corrupt

Delete or rename the config file and restart the app to start fresh:

- Windows: `%APPDATA%\lightsync\config.json`
- macOS: `~/Library/Application Support/lightsync/config.json`

### Frontend changes are not reflected after a Go rebuild

Force a full reload inside the Wails window with **Ctrl+Shift+R** (Windows) or **Cmd+Shift+R** (macOS).
