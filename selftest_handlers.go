package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"lightsync/internal/screensync/capture"
	"lightsync/internal/screensync/selftest"
	"lightsync/internal/store"
)

// ------------------ Wails-bound methods ------------------

// ListSelfTestFixtures returns the built-in fixture catalogue for the UI.
func (a *App) ListSelfTestFixtures() []selftest.Fixture {
	return selftest.BuiltInFixtures()
}

// ListSelfTestModes returns the full mode matrix the self-test runs per
// frame (~13 cells: single, multi-palette, multi-spatial with every
// extraction method the app exposes).
func (a *App) ListSelfTestModes() []selftest.ModeCase {
	return selftest.AllModes(4)
}

// StartSelfTest opens the fullscreen fixture popup if needed, emits the
// selected fixture's image URL to it, and starts (or re-seats) a 1 Hz
// capture-extract-emit loop. Switching fixtures builds a fresh Session so
// smoother/handoff state resets cleanly between tests.
func (a *App) StartSelfTest(fixtureID string) error {
	fx := selftest.FixtureByID(fixtureID)
	if fx == nil {
		return fmt.Errorf("unknown self-test fixture %q", fixtureID)
	}

	a.selfTestMu.Lock()

	a.ensureSelfTestPopup()

	// Swap fixtures: cancel old loop (if any), drop session, start fresh.
	if a.selfTestCancel != nil {
		a.selfTestCancel()
		a.selfTestCancel = nil
	}
	a.selfTestSession = selftest.NewSession(fx.ID, selftest.AllModes(4), fx.Expected)

	ctx, cancel := context.WithCancel(context.Background())
	a.selfTestCancel = cancel
	a.selfTestMu.Unlock()

	app := application.Get()
	app.Event.Emit("selftest:state", map[string]interface{}{
		"running": true,
		"fixture": fx.ID,
	})

	// Handoff to the loop — it handles page-ready + image-loaded
	// handshakes before emitting fixture-url so the event can't land on
	// an unmounted React handler.
	go a.runSelfTestLoop(ctx, *fx)
	return nil
}

// GetSelfTestDiagnostic builds a copy-pasteable diagnostic blob that
// captures the current session's expected/target/active state plus deltas.
// Returns empty string when no session is active.
func (a *App) GetSelfTestDiagnostic() (string, error) {
	a.selfTestMu.Lock()
	session := a.selfTestSession
	a.selfTestMu.Unlock()
	if session == nil {
		return "", fmt.Errorf("no active self-test session")
	}
	d := session.BuildDiagnostic()
	return d.AsText(), nil
}

// StopSelfTest halts the capture loop and closes the fixture popup.
func (a *App) StopSelfTest() {
	a.selfTestMu.Lock()
	cancel := a.selfTestCancel
	a.selfTestCancel = nil
	win := a.selfTestWindow
	a.selfTestWindow = nil
	a.selfTestSession = nil
	a.selfTestMu.Unlock()

	if cancel != nil {
		cancel()
	}
	if win != nil {
		win.Close()
	}

	if app := application.Get(); app != nil {
		app.Event.Emit("selftest:state", map[string]interface{}{
			"running": false,
		})
	}
}

// StartSelfTestSuite walks every built-in fixture, runs the full mode
// matrix against each for enough frames to settle, and emits a single
// "selftest:suite-complete" event with the aggregated results. Progress
// events are emitted as each fixture advances.
func (a *App) StartSelfTestSuite() error {
	a.selfTestMu.Lock()
	// Cancel any running per-fixture loop first; the suite owns the popup
	// and the session field for its duration.
	if a.selfTestCancel != nil {
		a.selfTestCancel()
		a.selfTestCancel = nil
	}
	a.selfTestSession = nil
	a.selfTestSuiteResult = nil

	a.ensureSelfTestPopup()

	ctx, cancel := context.WithCancel(context.Background())
	a.selfTestCancel = cancel
	a.selfTestMu.Unlock()

	if app := application.Get(); app != nil {
		app.Event.Emit("selftest:state", map[string]interface{}{
			"running": true,
			"mode":    "suite",
		})
	}

	go a.runSelfTestSuite(ctx)
	return nil
}

// GetSelfTestSuiteDiagnostic returns the aggregated suite diagnostic blob
// from the most recent completed run. Empty string if no suite has
// finished since process start.
func (a *App) GetSelfTestSuiteDiagnostic() (string, error) {
	a.selfTestMu.Lock()
	result := a.selfTestSuiteResult
	a.selfTestMu.Unlock()
	if result == nil {
		return "", fmt.Errorf("no suite results available")
	}
	return result.AsText(), nil
}

// StartSelfTestCalibrate runs a bounded coordinate search over global
// Screen Sync knobs using the synthetic self-test subset, then stores a
// CalibrationReport for preview / apply. Requires self-test to be idle.
func (a *App) StartSelfTestCalibrate(sceneID string) error {
	if sceneID == "" {
		return fmt.Errorf("sceneID is required")
	}
	if _, err := a.sceneManager.GetScene(sceneID); err != nil {
		return err
	}

	a.selfTestMu.Lock()
	if a.selfTestCancel != nil {
		a.selfTestMu.Unlock()
		return fmt.Errorf("stop the self-test suite or fixture run before calibrating")
	}
	a.selfTestMu.Unlock()

	a.calibrateMu.Lock()
	if a.calibrateRunning {
		a.calibrateMu.Unlock()
		return fmt.Errorf("calibration already running")
	}
	a.calibrateRunning = true
	a.calibrateSceneID = sceneID
	a.calibrateReport = nil
	ctx, cancel := context.WithCancel(context.Background())
	a.calibrateCancel = cancel
	a.calibrateMu.Unlock()

	go a.runSelfTestCalibrate(ctx, sceneID)
	return nil
}

// StopSelfTestCalibrate cancels an in-progress calibration run.
func (a *App) StopSelfTestCalibrate() {
	a.calibrateMu.Lock()
	cancel := a.calibrateCancel
	a.calibrateMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// IsSelfTestCalibrateRunning reports whether calibration is active.
func (a *App) IsSelfTestCalibrateRunning() bool {
	a.calibrateMu.Lock()
	defer a.calibrateMu.Unlock()
	return a.calibrateRunning
}

// GetSelfTestCalibrateReportJSON returns the last calibration report as JSON.
func (a *App) GetSelfTestCalibrateReportJSON() (string, error) {
	a.calibrateMu.Lock()
	rep := a.calibrateReport
	a.calibrateMu.Unlock()
	if rep == nil {
		return "", fmt.Errorf("no calibration report yet")
	}
	b, err := json.Marshal(rep)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// SelfTestCalibrateApply persists the candidate ScreenSyncConfig from the
// last calibration report onto the scene. Saves the previous config for
// rollback.
func (a *App) SelfTestCalibrateApply(sceneID string) error {
	a.calibrateMu.Lock()
	rep := a.calibrateReport
	a.calibrateMu.Unlock()
	if rep == nil || rep.Error != "" {
		return fmt.Errorf("no successful calibration report to apply")
	}
	if rep.SceneID != sceneID {
		return fmt.Errorf("report is for scene %q, not %q", rep.SceneID, sceneID)
	}
	scene, err := a.sceneManager.GetScene(sceneID)
	if err != nil {
		return err
	}
	if scene.Trigger != "screen_sync" {
		return fmt.Errorf("scene %q is not a screen_sync scene", sceneID)
	}
	prev := store.DefaultScreenSyncConfig()
	if scene.ScreenSync != nil {
		prev = selftest.CloneScreenSyncConfig(*scene.ScreenSync)
	}
	a.calibrateMu.Lock()
	if a.calibrateRollback == nil {
		a.calibrateRollback = make(map[string]store.ScreenSyncConfig)
	}
	a.calibrateRollback[sceneID] = prev
	a.calibrateMu.Unlock()

	cfg := selftest.CloneScreenSyncConfig(rep.CandidateConfig)
	return a.UpdateScreenSyncConfig(sceneID, cfg)
}

// SelfTestCalibrateRollback restores the ScreenSyncConfig saved at the last
// successful SelfTestCalibrateApply for this scene.
func (a *App) SelfTestCalibrateRollback(sceneID string) error {
	a.calibrateMu.Lock()
	if a.calibrateRollback == nil {
		a.calibrateMu.Unlock()
		return fmt.Errorf("nothing to roll back")
	}
	prev, ok := a.calibrateRollback[sceneID]
	a.calibrateMu.Unlock()
	if !ok {
		return fmt.Errorf("no rollback snapshot for scene %q", sceneID)
	}
	if err := a.UpdateScreenSyncConfig(sceneID, prev); err != nil {
		return err
	}
	a.calibrateMu.Lock()
	delete(a.calibrateRollback, sceneID)
	a.calibrateMu.Unlock()
	return nil
}

func (a *App) runSelfTestCalibrate(ctx context.Context, sceneID string) {
	app := application.Get()
	report := &selftest.CalibrationReport{
		SceneID:   sceneID,
		StartedAt: time.Now(),
	}

	defer a.forceCloseSelfTestFixtureWindow()

	defer func() {
		report.FinishedAt = time.Now()
		a.calibrateMu.Lock()
		a.calibrateRunning = false
		a.calibrateCancel = nil
		a.calibrateReport = report
		a.calibrateMu.Unlock()
		if app != nil {
			app.Event.Emit("selftest:calibrate-state", map[string]interface{}{
				"running": false,
				"error":   report.Error,
			})
			app.Event.Emit("selftest:calibrate-complete", report)
		}
	}()

	if app != nil {
		app.Event.Emit("selftest:calibrate-state", map[string]interface{}{
			"running": true,
			"sceneId": sceneID,
		})
	}

	scene, err := a.sceneManager.GetScene(sceneID)
	if err != nil {
		report.Error = err.Error()
		return
	}
	if scene.Trigger != "screen_sync" {
		report.Error = fmt.Sprintf("scene %q is not a screen_sync scene", sceneID)
		return
	}

	base := store.EffectiveScreenSyncConfig(scene.ScreenSync, a.store.GetScreenSyncPortableDefaults())
	base = selftest.CloneScreenSyncConfig(base)
	report.BaselineConfig = base

	a.selfTestMu.Lock()
	a.ensureSelfTestPopup()
	a.selfTestMu.Unlock()

	capturer, err := a.newSelfTestCapturer()
	if err != nil {
		report.Error = fmt.Sprintf("capturer: %v", err)
		return
	}
	defer capturer.Close()

	a.waitForSelfTestPageReady(ctx, time.Duration(suitePageReadyTimeoutMs)*time.Millisecond)
	if ctx.Err() != nil {
		report.Error = ctx.Err().Error()
		return
	}

	subset := selfTestExecParams{
		Fixtures:          selftest.CalibrationSearchFixtures(),
		Modes:             selftest.AllModesWithCalibrationOverlay(4, &base),
		MinDwellMs:        1200,
		MaxDwellMs:        4000,
		StepIntervalMs:    suiteStepIntervalMs,
		LeavePopupOpen:    true,
		WriteSuiteResult:  false,
		EmitSuiteResults:  false,
		EmitSuiteProgress: false,
	}

	searchRes, searchErr := selftest.RunCoordinateSearch(base, func(cfg store.ScreenSyncConfig) (*selftest.SuiteRun, error) {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// Use Get() per eval: this goroutine may run before the app singleton
		// is visible on the first line of runSelfTestCalibrate in some builds.
		if appNow := application.Get(); appNow != nil {
			appNow.Event.Emit("selftest:calibrate-progress", map[string]interface{}{
				"phase": "eval",
			})
		}
		modes := selftest.AllModesWithCalibrationOverlay(4, &cfg)
		t0 := time.Now()
		p := subset
		p.Modes = modes
		return a.executeSelfTestSuiteCore(ctx, t0, p, capturer)
	}, selftest.DefaultCalibrationMaxEvaluations)

	if searchErr != nil {
		report.Error = searchErr.Error()
		return
	}
	report.CandidateConfig = searchRes.BestConfig
	report.BaselineMetrics = searchRes.StartMetrics
	report.CandidateMetrics = searchRes.BestMetrics
	report.SubsetBaselineRun = searchRes.BaselineRun
	report.SubsetCandidateRun = searchRes.BestRun
	report.SearchEvalCount = searchRes.EvalCount
}

// forceCloseSelfTestFixtureWindow closes the self-test popup if present.
func (a *App) forceCloseSelfTestFixtureWindow() {
	a.selfTestMu.Lock()
	win := a.selfTestWindow
	a.selfTestWindow = nil
	a.selfTestMu.Unlock()
	if win != nil {
		win.Close()
	}
}

// ------------------ internals ------------------

// selfTestFixtureRect is the fixed screen rectangle the Self-Test popup
// occupies AND the region the capturer reads, in PHYSICAL pixels.
//
// Wails's X/Y/Width/Height in WebviewWindowOptions are DIPs (scaled by
// monitor DPI), while DXGI reads physical pixels — so constructor-time
// positioning can diverge from the capture rect on any non-100% display.
// ensureSelfTestPopup uses SetPhysicalBounds() after creation to pin the
// window to this exact rectangle regardless of DPI, and runSelfTestLoop
// feeds the same rect into CaptureModeRegion. Both sides read physical
// pixels from the same origin, so they can't drift.
//
// If you ever change the fixture asset dimensions, update these values,
// then regenerate the baselines via TestProbeTestdata.
const (
	selfTestFixtureX = 0
	selfTestFixtureY = 0
	selfTestFixtureW = 640
	selfTestFixtureH = 480
	// selfTestCaptureInsetPx trims the popup's outer physical bounds before
	// creating the capture region. On Windows, frameless windows can still have
	// an invisible border/shadow area in outer bounds; including that leaks
	// desktop pixels into the self-test and disproportionately breaks
	// brightest/palette modes.
	selfTestCaptureInsetPx = 8
)

func selfTestFixtureBounds() application.Rect {
	return application.Rect{
		X:      selfTestFixtureX,
		Y:      selfTestFixtureY,
		Width:  selfTestFixtureW,
		Height: selfTestFixtureH,
	}
}

// ensureSelfTestPopup creates the borderless fixture window at the known
// fixed rect if one isn't already open. Caller must hold selfTestMu.
//
// When a new window is created, we also arm the page-ready handshake:
// the frontend's SelfTestFixturePage emits `selftest:page-ready` once it
// has mounted and subscribed to fixture-url events. Callers can then use
// waitForSelfTestPageReady before emitting the first fixture URL so the
// emission isn't lost to the mount race.
func (a *App) ensureSelfTestPopup() {
	if a.selfTestWindow != nil {
		a.selfTestWindow.Show()
		a.selfTestWindow.Focus()
		return
	}

	app := application.Get()

	// Arm the handshake BEFORE creating the window so we can't miss the
	// page-ready emission on extremely fast React mounts.
	ready := make(chan struct{})
	a.selfTestPageReady = ready
	a.selfTestPopupGen++
	popupGen := a.selfTestPopupGen
	offReady := app.Event.On("selftest:page-ready", func(*application.CustomEvent) {
		select {
		case <-ready:
			// Already closed — ignore duplicate emissions.
		default:
			close(ready)
		}
	})

	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:          "self-test-fixture",
		Title:         "Self-Test Fixture",
		URL:           "/#self-test-fixture",
		Frameless:     true,
		AlwaysOnTop:   true,
		DisableResize: true,
		Windows: application.WindowsWindow{
			Theme:                             application.Dark,
			DisableFramelessWindowDecorations: true,
		},
	})

	// Pin the window to exact physical pixel coordinates AFTER creation —
	// SetPhysicalBounds bypasses DPI scaling so the window origin matches
	// the DXGI capture origin regardless of monitor DPI. Constructor-time
	// X/Y/W/H are DIPs and would place the window somewhere else on
	// non-100% displays.
	win.SetPhysicalBounds(selfTestFixtureBounds())

	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		offReady()
		a.selfTestMu.Lock()
		// Ignore closes from a previous popup instance — otherwise a late
		// WindowClosing from calibration teardown can wipe the new page-ready
		// channel or cancel a suite that already replaced this window.
		if a.selfTestPopupGen != popupGen {
			a.selfTestMu.Unlock()
			return
		}
		a.selfTestWindow = nil
		a.selfTestPageReady = nil
		cancel := a.selfTestCancel
		a.selfTestCancel = nil
		a.selfTestMu.Unlock()
		if cancel != nil {
			cancel()
		}
		if appNow := application.Get(); appNow != nil {
			appNow.Event.Emit("selftest:state", map[string]interface{}{
				"running": false,
			})
		}
	})

	a.selfTestWindow = win
}

// waitForSelfTestPageReady blocks until the fixture popup's page has
// signalled it is mounted and subscribed to fixture-url events, the
// timeout expires, or ctx is cancelled. Safe to call after the popup has
// already been ready — the channel stays closed for the window's
// lifetime.
func (a *App) waitForSelfTestPageReady(ctx context.Context, timeout time.Duration) {
	a.selfTestMu.Lock()
	ch := a.selfTestPageReady
	a.selfTestMu.Unlock()
	if ch == nil {
		return
	}
	select {
	case <-ch:
	case <-time.After(timeout):
		log.Printf("[selftest] page-ready timeout after %v — proceeding", timeout)
	case <-ctx.Done():
	}
}

// waitForSelfTestImageLoaded blocks until the frontend emits
// `selftest:image-loaded` carrying the given fixture id — i.e. the <img>
// has finished loading AND decoded the new bitmap so DWM is about to
// composite it — or the timeout expires. Prevents the capture loop from
// sampling a half-painted window on fixture swaps.
func (a *App) waitForSelfTestImageLoaded(ctx context.Context, fixtureID string, timeout time.Duration) {
	app := application.Get()
	if app == nil {
		return
	}
	loaded := make(chan struct{}, 1)
	off := app.Event.On("selftest:image-loaded", func(e *application.CustomEvent) {
		if e == nil {
			return
		}
		id := extractFixtureID(e.Data)
		if id != fixtureID {
			return
		}
		select {
		case loaded <- struct{}{}:
		default:
		}
	})
	defer off()
	select {
	case <-loaded:
	case <-time.After(timeout):
		log.Printf("[selftest] image-loaded timeout for %s after %v — proceeding", fixtureID, timeout)
	case <-ctx.Done():
	}
}

// extractFixtureID defensively pulls the "id" field out of whatever shape
// Wails v3 hands us in CustomEvent.Data. Frontend sends `{id: "..."}` via
// Events.Emit; depending on runtime plumbing this can land as
// map[string]any, []any wrapping it, or (in some paths) a raw string.
func extractFixtureID(data any) string {
	switch v := data.(type) {
	case nil:
		return ""
	case string:
		return v
	case map[string]any:
		if s, ok := v["id"].(string); ok {
			return s
		}
	case []any:
		if len(v) > 0 {
			return extractFixtureID(v[0])
		}
	}
	return ""
}

// newSelfTestCapturer returns a region capturer pinned to the popup's
// actual physical-pixel rect. Shared by the single-fixture loop and the
// suite runner.
//
// The rect comes from Wails's PhysicalBounds() — the authoritative source
// on where the popup really is in the DXGI coordinate space. We fall back
// to the configured rect if the window isn't available (shouldn't happen
// in normal flow; belt-and-braces for error paths).
func (a *App) newSelfTestCapturer() (capture.Capturer, error) {
	rect := a.selfTestPopupRect()
	log.Printf("[selftest] capture region: X=%d Y=%d W=%d H=%d",
		rect.X, rect.Y, rect.Width, rect.Height)
	return capture.NewCapturer(store.ScreenSyncConfig{
		CaptureMode: store.CaptureModeRegion,
		Region:      rect,
	})
}

// selfTestPopupRect returns the popup's current physical-pixel rectangle
// so the capture region follows the window wherever Wails actually placed
// it (DPI rounding, multi-monitor offsets, etc.). Falls back to the
// configured default.
func (a *App) selfTestPopupRect() store.CaptureRect {
	a.selfTestMu.Lock()
	win := a.selfTestWindow
	a.selfTestMu.Unlock()
	if win == nil {
		return store.CaptureRect{
			X: selfTestFixtureX, Y: selfTestFixtureY,
			Width: selfTestFixtureW, Height: selfTestFixtureH,
		}
	}
	b := win.PhysicalBounds()
	if b.Width <= 0 || b.Height <= 0 {
		return store.CaptureRect{
			X: selfTestFixtureX, Y: selfTestFixtureY,
			Width: selfTestFixtureW, Height: selfTestFixtureH,
		}
	}
	rect := store.CaptureRect{X: b.X, Y: b.Y, Width: b.Width, Height: b.Height}
	return insetCaptureRect(rect, selfTestCaptureInsetPx)
}

func insetCaptureRect(r store.CaptureRect, inset int) store.CaptureRect {
	if inset <= 0 {
		return r
	}
	w := r.Width - (2 * inset)
	h := r.Height - (2 * inset)
	// Keep a sane minimum in case window managers report unexpectedly small
	// bounds; in that case, use the original rect instead of producing an
	// invalid capture region.
	if w < 64 || h < 64 {
		return r
	}
	return store.CaptureRect{
		X:      r.X + inset,
		Y:      r.Y + inset,
		Width:  w,
		Height: h,
	}
}

// runSelfTestLoop captures the primary monitor at ~2 Hz (fast enough to
// animate the handoff ramp clearly without overwhelming the UI), pushes
// every captured frame through the session's stateful pipeline, and emits
// the per-cell target+active results.
//
// Before the first capture we perform the two-phase handshake:
//   1. Wait for the fixture page to mount + subscribe (`page-ready`).
//   2. Emit fixture-url, then wait for the <img> onLoad acknowledgement
//      (`image-loaded`).
// This replaces the old fixed 600 ms warmup, which was too short for a
// cold-boot WebView2 + React mount and silently dropped the first URL
// emission — causing the first fixture of a suite run to capture only
// the empty "Waiting for fixture…" placeholder.
func (a *App) runSelfTestLoop(ctx context.Context, fx selftest.Fixture) {
	a.waitForSelfTestPageReady(ctx, 5*time.Second)
	if ctx.Err() != nil {
		return
	}

	app := application.Get()
	if app != nil {
		app.Event.Emit("selftest:fixture-url", map[string]string{
			"id":  fx.ID,
			"url": fx.URL,
		})
	}

	a.waitForSelfTestImageLoaded(ctx, fx.ID, 3*time.Second)
	if ctx.Err() != nil {
		return
	}
	// Tiny paint-settle after the browser reports the image decoded.
	// DWM composites on the next vsync; 100 ms covers a 60 Hz frame with
	// margin without being user-visible.
	select {
	case <-time.After(100 * time.Millisecond):
	case <-ctx.Done():
		return
	}

	capturer, err := a.newSelfTestCapturer()
	if err != nil {
		log.Printf("[selftest] capturer init failed: %v", err)
		return
	}
	defer capturer.Close()

	// 500 ms = 2 Hz. Fast enough for the handoff ramp (max 3 s) to show ~6
	// visible intermediate frames. 1 Hz felt too coarse for observing the
	// fade.
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	first := make(chan struct{}, 1)
	first <- struct{}{}

	for {
		select {
		case <-ctx.Done():
			return
		case <-first:
		case <-ticker.C:
		}

		img, err := capturer.Capture()
		if err != nil {
			log.Printf("[selftest] capture failed: %v", err)
			continue
		}

		a.selfTestMu.Lock()
		session := a.selfTestSession
		a.selfTestMu.Unlock()
		if session == nil {
			return
		}

		cells := session.Step(img)

		if app := application.Get(); app != nil {
			app.Event.Emit("selftest:results", map[string]interface{}{
				"fixture":    session.FixtureID(),
				"cells":      cells,
				"at":         time.Now().UnixMilli(),
				"elapsedMs":  session.ElapsedMs(),
				"frameIndex": session.FrameIndex(),
			})
		}
	}
}

// ------------------ suite orchestrator ------------------

// Per-fixture capture budget. A fixture is marked done the moment every
// expected cell has locked for the required consecutive-frame window (see
// selftest.Session.AllExpectedLocked), but we also cap at suiteMaxDwellMs
// so a single drifting cell can't stall the whole run.
//
// Note: the old fixed `suiteWarmupMs` sleep was removed in favour of the
// explicit image-loaded handshake (waitForSelfTestImageLoaded) — a fixed
// sleep can't cover a cold-boot React mount reliably, which is exactly
// why the first fixture's URL emission was being lost.
const (
	suitePaintSettleMs        = 100
	suiteImageLoadTimeoutMs   = 3000
	suitePageReadyTimeoutMs   = 5000
	suiteMinDwellMs           = 2000
	suiteMaxDwellMs           = 6000
	suiteStepIntervalMs       = 500
)

// selfTestExecParams configures one suite walk (fixtures, timing, events).
type selfTestExecParams struct {
	Fixtures           []selftest.Fixture
	Modes              []selftest.ModeCase
	MinDwellMs         int
	MaxDwellMs         int
	StepIntervalMs     int
	LeavePopupOpen     bool
	WriteSuiteResult   bool
	EmitSuiteResults   bool
	EmitSuiteProgress  bool
}

// executeSelfTestSuiteCore runs the fixture loop against an existing capturer.
// When LeavePopupOpen is false, the self-test popup is closed at the end.
func (a *App) executeSelfTestSuiteCore(ctx context.Context, suiteStarted time.Time, p selfTestExecParams, capturer capture.Capturer) (run *selftest.SuiteRun, err error) {
	// If we bail early (cancelled calibration, etc.), drop any partial session
	// so the next suite/fixture run does not inherit stale pipeline state.
	defer func() {
		if err != nil {
			a.selfTestMu.Lock()
			a.selfTestSession = nil
			a.selfTestMu.Unlock()
		}
	}()

	a.waitForSelfTestPageReady(ctx, time.Duration(suitePageReadyTimeoutMs)*time.Millisecond)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	run = &selftest.SuiteRun{
		StartedAt:    suiteStarted,
		FixtureCount: len(p.Fixtures),
	}

	for i, fx := range p.Fixtures {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if p.EmitSuiteProgress {
			a.emitSuiteProgress(i, len(p.Fixtures), fx, "warming", run.StartedAt)
		}

		// application.Get() must be called when emitting: the suite goroutine can
		// start before the app singleton exists (same as calibration eval emits).
		if appNow := application.Get(); appNow != nil {
			appNow.Event.Emit("selftest:fixture-url", map[string]string{
				"id":  fx.ID,
				"url": fx.URL,
			})
		}
		session := selftest.NewSession(fx.ID, p.Modes, fx.Expected)
		a.selfTestMu.Lock()
		a.selfTestSession = session
		a.selfTestMu.Unlock()

		a.waitForSelfTestImageLoaded(ctx, fx.ID, time.Duration(suiteImageLoadTimeoutMs)*time.Millisecond)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if !sleepCtx(ctx, time.Duration(suitePaintSettleMs)*time.Millisecond) {
			return nil, ctx.Err()
		}

		if p.EmitSuiteProgress {
			a.emitSuiteProgress(i, len(p.Fixtures), fx, "capturing", run.StartedAt)
		}

		fixtureStart := time.Now()
		stepInterval := time.Duration(p.StepIntervalMs) * time.Millisecond
		minDwell := time.Duration(p.MinDwellMs) * time.Millisecond
		maxDwell := time.Duration(p.MaxDwellMs) * time.Millisecond

		for {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}

			img, capErr := capturer.Capture()
			if capErr == nil {
				cells := session.Step(img)
				if p.EmitSuiteResults {
					if appNow := application.Get(); appNow != nil {
						appNow.Event.Emit("selftest:results", map[string]interface{}{
							"fixture":    session.FixtureID(),
							"cells":      cells,
							"at":         time.Now().UnixMilli(),
							"elapsedMs":  session.ElapsedMs(),
							"frameIndex": session.FrameIndex(),
						})
					}
				}
			}

			elapsed := time.Since(fixtureStart)
			if elapsed >= maxDwell {
				break
			}
			if elapsed >= minDwell && session.AllExpectedLocked() {
				break
			}
			if !sleepCtx(ctx, stepInterval) {
				return nil, ctx.Err()
			}
		}

		diag := session.BuildDiagnostic()
		locked, total := session.Progress()
		run.Fixtures = append(run.Fixtures, selftest.FixtureRunResult{
			FixtureID:    fx.ID,
			FixtureLabel: fx.Label,
			DurationMs:   time.Since(fixtureStart).Milliseconds(),
			LockedCells:  locked,
			TotalCells:   total,
			Diagnostic:   diag,
		})
		run.LockedCells += locked
		run.TotalCells += total

		if p.EmitSuiteProgress {
			a.emitSuiteProgress(i, len(p.Fixtures), fx, "settled", run.StartedAt)
		}
	}

	run.FinishedAt = time.Now()
	run.TotalMs = run.FinishedAt.Sub(run.StartedAt).Milliseconds()

	a.selfTestMu.Lock()
	a.selfTestSession = nil
	if p.WriteSuiteResult {
		a.selfTestSuiteResult = run
	}
	if !p.LeavePopupOpen {
		win := a.selfTestWindow
		a.selfTestWindow = nil
		a.selfTestMu.Unlock()
		if win != nil {
			win.Close()
		}
	} else {
		a.selfTestMu.Unlock()
	}

	return run, nil
}

// runSelfTestSuite walks every built-in fixture in order, captures enough
// frames per fixture to either (a) reach full-lock state or (b) hit the
// dwell cap, then emits a single selftest:suite-complete event with the
// aggregated report.
func (a *App) runSelfTestSuite(ctx context.Context) {
	app := application.Get()

	capturer, err := a.newSelfTestCapturer()
	if err != nil {
		log.Printf("[selftest] suite capturer init failed: %v", err)
		a.emitSuiteCancelled()
		return
	}
	defer capturer.Close()

	started := time.Now()
	run, err := a.executeSelfTestSuiteCore(ctx, started, selfTestExecParams{
		Fixtures:          selftest.BuiltInFixtures(),
		Modes:             selftest.AllModes(4),
		MinDwellMs:        suiteMinDwellMs,
		MaxDwellMs:        suiteMaxDwellMs,
		StepIntervalMs:    suiteStepIntervalMs,
		LeavePopupOpen:    false,
		WriteSuiteResult:  true,
		EmitSuiteResults:  true,
		EmitSuiteProgress: true,
	}, capturer)
	if err != nil || run == nil {
		if ctx.Err() != nil {
			a.emitSuiteCancelled()
		}
		return
	}

	a.selfTestMu.Lock()
	a.selfTestCancel = nil
	a.selfTestMu.Unlock()

	if app != nil {
		app.Event.Emit("selftest:suite-complete", run)
		app.Event.Emit("selftest:state", map[string]interface{}{
			"running": false,
		})
	}
}

func (a *App) emitSuiteProgress(idx, total int, fx selftest.Fixture, phase string, startedAt time.Time) {
	app := application.Get()
	if app == nil {
		return
	}
	app.Event.Emit("selftest:suite-progress", selftest.SuiteProgress{
		FixtureIndex: idx,
		TotalCount:   total,
		FixtureID:    fx.ID,
		FixtureLabel: fx.Label,
		Phase:        phase,
		ElapsedMs:    time.Since(startedAt).Milliseconds(),
	})
}

func (a *App) emitSuiteCancelled() {
	a.selfTestMu.Lock()
	a.selfTestCancel = nil
	a.selfTestSession = nil
	win := a.selfTestWindow
	a.selfTestWindow = nil
	a.selfTestMu.Unlock()
	if win != nil {
		win.Close()
	}
	if app := application.Get(); app != nil {
		app.Event.Emit("selftest:suite-cancelled", map[string]interface{}{})
		app.Event.Emit("selftest:state", map[string]interface{}{"running": false})
	}
}

// sleepCtx is a context-aware Sleep. Returns false when ctx was cancelled.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}
