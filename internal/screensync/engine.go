// Package screensync is the Screen Sync engine that drives continuous screen
// capture, color extraction, and light synchronisation.
package screensync

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/image/draw"

	"lightsync/internal/lights"
	"lightsync/internal/screensync/assign"
	"lightsync/internal/screensync/capture"
	"lightsync/internal/screensync/extract"
	"lightsync/internal/screensync/process"
	"lightsync/internal/store"
)

// Engine orchestrates the full Screen Sync pipeline for a single active scene.
// It is safe to call Start/Stop/UpdateConfig concurrently.
//
// Pipeline per frame:
//
//	capture → extract → adjustments → scene-cut detect → temporal smooth
//	    → assign colors to devices → send to lights
type Engine struct {
	mu       sync.RWMutex
	config   store.ScreenSyncConfig
	lightMgr *lights.Manager

	// Event callbacks (set once before Start, not changed concurrently).
	onColors func([]lights.Color)
	onStats  func(Stats)
	onState  func(running bool)

	// Pipeline state (recreated on each Start).
	sceneChange *process.SceneChangeDetector
	smoother    *process.TemporalSmoother
	assigner    assign.Assigner
	handoff     *colorHandoffBlender
	stats       *statsCollector

	cancel  context.CancelFunc
	done    chan struct{} // closed when run() exits
	running bool

	// lastSent tracks the most recently transmitted color per device so we can
	// skip sends when the color hasn't changed visibly.
	lastSentMu sync.Mutex
	lastSent   map[string]lights.Color

	// Per-brand send slots so Hue throttling doesn't block LIFX/Govee/Elgato.
	// Each slot is a permit; acquiring it means we can send to that brand.
	brandSlots         map[lights.Brand]chan struct{}
	initBrandSlotsOnce sync.Once

	// Hue bridge rate limit: ~10 req/sec. We throttle Hue batches.
	hueLastSend   time.Time
	hueLastSendMu sync.Mutex

	// Preview frame: a JPEG snapshot of the captured image, updated at ~1 fps.
	// Only produced when the popout has recently called GetPreviewFrame.
	previewMu        sync.Mutex
	previewJPEG      []byte
	previewAt        time.Time
	previewRequested int32 // atomic: >0 means someone wants previews
}

func (e *Engine) ensureBrandSlots() {
	e.initBrandSlotsOnce.Do(func() {
		brands := []lights.Brand{lights.BrandLIFX, lights.BrandHue, lights.BrandElgato, lights.BrandGovee}
		e.brandSlots = make(map[lights.Brand]chan struct{}, len(brands))
		for _, b := range brands {
			ch := make(chan struct{}, 1)
			ch <- struct{}{} // initial permit
			e.brandSlots[b] = ch
		}
	})
}

// NewEngine creates an Engine that uses lm to apply light states.
func NewEngine(lm *lights.Manager) *Engine {
	return &Engine{
		lightMgr:    lm,
		sceneChange: process.NewSceneChangeDetector(),
		smoother:    process.NewTemporalSmoother(),
		handoff:     newColorHandoffBlender(),
		stats:       newStatsCollector(),
		lastSent:    make(map[string]lights.Color),
	}
}

// OnColors registers a callback invoked on every extracted color set.
func (e *Engine) OnColors(fn func([]lights.Color)) { e.onColors = fn }

// OnStats registers a callback invoked every second with performance metrics.
func (e *Engine) OnStats(fn func(Stats)) { e.onStats = fn }

// OnState registers a callback invoked when the engine starts or stops.
func (e *Engine) OnState(fn func(running bool)) { e.onState = fn }

// Start begins a new capture loop using the given config.
// If the engine is already running it is stopped first.
func (e *Engine) Start(cfg store.ScreenSyncConfig) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
	}

	store.NormalizeScreenSyncConfig(&cfg)
	e.config = cfg
	e.sceneChange.Reset()
	e.smoother.Reset()
	e.assigner = assign.New(cfg)
	e.handoff.Reset()
	e.stats.reset()
	e.lastSentMu.Lock()
	e.lastSent = make(map[string]lights.Color)
	e.lastSentMu.Unlock()
	e.hueLastSendMu.Lock()
	e.hueLastSend = time.Time{} // reset so first Hue send isn't rate-limited
	e.hueLastSendMu.Unlock()

	// Seed preview so the first few frames generate a thumbnail immediately
	// (e.g. after a capture-mode switch restarts the engine).
	atomic.StoreInt32(&e.previewRequested, 3)

	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	e.done = make(chan struct{})
	e.running = true

	go e.run(ctx)

	if e.onState != nil {
		e.onState(true)
	}
	return nil
}

// Stop halts the capture loop and waits for it to fully shut down (releasing
// DXGI resources etc.) before returning. Safe to call when not running.
func (e *Engine) Stop() {
	e.mu.Lock()
	cancel := e.cancel
	done := e.done
	e.cancel = nil
	e.running = false
	e.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	// Wait for run() to finish so DXGI resources are released before a
	// subsequent Start() tries to create new ones. Cap at 2s to avoid
	// blocking Wails' shutdown sequence (which needs the UI thread free).
	if done != nil {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	}

	// Clear stale preview so the popup doesn't show an old frame.
	e.previewMu.Lock()
	e.previewJPEG = nil
	e.previewAt = time.Time{}
	e.previewMu.Unlock()

	if e.onState != nil {
		e.onState(false)
	}
}

// IsRunning reports whether the engine is currently capturing.
func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

// GetPreviewFrame returns the most recent 1-fps JPEG preview of the captured
// image as a raw byte slice. Returns nil when no frame has been stored yet.
// Calling this signals that a consumer wants previews; the engine will produce
// them for the next few seconds. When nobody calls this, preview generation is
// skipped entirely — avoiding the bilinear downscale and JPEG encode cost.
func (e *Engine) GetPreviewFrame() []byte {
	// Signal that previews are wanted for the next 3 seconds (at ~1 fps that
	// means we'll produce about 3 frames before stopping again).
	atomic.StoreInt32(&e.previewRequested, 3)

	e.previewMu.Lock()
	defer e.previewMu.Unlock()
	if len(e.previewJPEG) == 0 {
		return nil
	}
	out := make([]byte, len(e.previewJPEG))
	copy(out, e.previewJPEG)
	return out
}

// maybeStorePreview downscales img to ≤320 px wide and JPEG-encodes it into
// the preview slot. Only runs when a consumer has recently called
// GetPreviewFrame AND at least 1 second has elapsed since the last update.
func (e *Engine) maybeStorePreview(img image.Image) {
	// Skip entirely when nobody is watching.
	remaining := atomic.LoadInt32(&e.previewRequested)
	if remaining <= 0 {
		return
	}

	e.previewMu.Lock()
	ready := time.Since(e.previewAt) >= time.Second
	e.previewMu.Unlock()
	if !ready {
		return
	}

	// Consume one request tick.
	atomic.AddInt32(&e.previewRequested, -1)

	const maxW = 320
	b := img.Bounds()
	srcW, srcH := b.Dx(), b.Dy()

	dstW, dstH := srcW, srcH
	if srcW > maxW {
		dstW = maxW
		dstH = srcH * maxW / srcW
	}
	if dstH < 1 {
		dstH = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	draw.BiLinear.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 70}); err != nil {
		return
	}

	e.previewMu.Lock()
	e.previewJPEG = buf.Bytes()
	e.previewAt = time.Now()
	e.previewMu.Unlock()
}

// UpdateConfig applies new settings to the running engine. If the capture mode
// or assignment model changed the loop is restarted; otherwise processing
// parameters are hot-reloaded.
func (e *Engine) UpdateConfig(cfg store.ScreenSyncConfig) {
	store.NormalizeScreenSyncConfig(&cfg)
	e.mu.Lock()
	oldMode := e.config.CaptureMode
	oldMonitor := e.config.MonitorIndex
	oldHWND := e.config.WindowHWND
	oldAssignStrategy := e.config.AssignmentStrategy
	wasRunning := e.running
	e.config = cfg
	e.mu.Unlock()

	captureChanged := oldMode != cfg.CaptureMode ||
		(cfg.CaptureMode == store.CaptureModeMonitor && oldMonitor != cfg.MonitorIndex) ||
		(cfg.CaptureMode == store.CaptureModeWindow && oldHWND != cfg.WindowHWND)

	assignStrategyChanged := oldAssignStrategy != cfg.AssignmentStrategy

	if (captureChanged || assignStrategyChanged) && wasRunning {
		e.Stop()
		_ = e.Start(cfg)
	}
}

func (e *Engine) getConfig() store.ScreenSyncConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.config
}

func (e *Engine) getAssigner() assign.Assigner {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.assigner
}

// run is the main goroutine. It creates a capturer, builds an extractor, and
// loops until ctx is cancelled.
func (e *Engine) run(ctx context.Context) {
	defer func() {
		e.mu.Lock()
		e.running = false
		done := e.done
		e.mu.Unlock()
		if done != nil {
			close(done)
		}
	}()

	cfg := e.getConfig()
	capturer, err := capture.NewCapturer(cfg)
	if err != nil {
		return
	}
	defer capturer.Close()

	// DispatchExtractor reads cfg dynamically each frame — no need to replace it.
	extractor := extract.New(cfg)

	// Calibration: run pipeline for 2s without sending so extractors, smoothers,
	// and assigners settle. Then fade brightness up over 1s.
	runStart := time.Now()
	const calibrateDuration = 2 * time.Second
	const fadeDuration = 8 * time.Second

	// Tracks the last emitted output for color-changed detection.
	var prevOutput map[string]lights.Color

	// Stats emission goroutine.
	statsTicker := time.NewTicker(time.Second)
	defer statsTicker.Stop()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-statsTicker.C:
				if e.onStats != nil {
					e.onStats(e.stats.snapshot(e.getConfig().SpeedPreset))
				}
				e.stats.reset()
			}
		}
	}()

	// Main capture loop.
	// The sleep happens at the END of each iteration for only the remaining
	// time in the interval (interval − elapsed). This gives frame time of
	// max(interval, workTime) instead of interval + workTime.
	for {
		cfg = e.getConfig()
		interval := speedPresetInterval(cfg.SpeedPreset)
		frameStart := time.Now()

		img, err := capturer.Capture()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(500 * time.Millisecond):
			}
			continue
		}
		captureEnd := time.Now()

		// Store a 1-fps preview thumbnail. Runs inline because the captured
		// image buffer may be reused by DXGI on the next frame.
		e.maybeStorePreview(img)

		// ── 1. Extract colors from the frame. ───────────────────────────────
		colors := extractor.Extract(img, cfg)

		// ── 2. Adjust saturation and brightness limits. ──────────────────────
		colors = process.ApplyAdjustments(colors, cfg)

		// ── 3. Scene-cut detection (compares extracted colors, zero overhead).
		isCut := false
		cutWhy := process.CutReasons{}
		if cfg.SceneCutMode != store.SceneCutModeOff {
			isCut, cutWhy = e.sceneChange.Check(colors, cfg.SceneCutSensitivity)
		}
		if isCut {
			e.stats.recordCutReasons(cutWhy)
			e.stats.recordSceneChange()
		}

		// ── 4. Temporal smoothing (adaptive EMA, resets on scene cut). ───────
		colors = e.smoother.Smooth(colors, isCut, cfg.ColorSmoothing, cfg.BrightnessSmoothing, cfg.BrightnessMaxDeviation, cfg.BrightnessFloor, cfg.BrightnessCeiling)

		// ── 5. Assign colors to devices. ────────────────────────────────────
		currentOutput := prevOutput
		if currentOutput == nil {
			currentOutput = map[string]lights.Color{}
		}
		assignedColors := e.getAssigner().Assign(colors, cfg.DeviceIDs, currentOutput, cfg, isCut)

		// Detect assignment rewiring (any device got a different color index).
		if assignmentRewired(currentOutput, assignedColors) {
			e.stats.recordAssignmentRewired()
		}

		// ── 5b. Post-assignment handoff blending (swap softening). ───────────
		deviceColors := assignedColors
		if isCut {
			// Preserve instant scene-cut snaps.
			e.handoff.Snap(assignedColors)
		} else {
			deviceColors = e.handoff.Blend(assignedColors, cfg.AssignmentHandoffMs)
		}

		// ── 6. Color-changed indicator. ──────────────────────────────────────
		if mapChangedMeaningfully(prevOutput, deviceColors) {
			e.stats.recordColorChange()
		}
		prevOutput = deviceColors

		// ── 6b. Calibration + startup fade. ───────────────────────────────────
		elapsedSinceStart := time.Since(runStart)
		var fade float64
		if elapsedSinceStart < calibrateDuration {
			fade = 0 // Don't send during calibration (skip send below)
		} else if elapsedSinceStart < calibrateDuration+fadeDuration {
			t := float64(elapsedSinceStart-calibrateDuration) / float64(fadeDuration)
			// Cubic Bezier ease-in-out (smoothstep): 3t² - 2t³
			fade = 3*t*t - 2*t*t*t
		} else {
			fade = 1
		}
		if fade > 0 {
			faded := make(map[string]lights.Color, len(deviceColors))
			for id, c := range deviceColors {
				faded[id] = lights.Color{H: c.H, S: c.S, B: c.B * fade}
			}
			deviceColors = faded
		}

		// ── 7. Send to lights (per-brand pipelines). Hue throttling doesn't block LIFX/etc.
		procEnd := time.Now()
		captureMs := captureEnd.Sub(frameStart)
		processMs := procEnd.Sub(captureEnd)
		e.sendDeviceColorsByBrand(ctx, deviceColors, fade > 0, procEnd.Sub(frameStart))
		e.stats.recordFrame(procEnd.Sub(frameStart), captureMs, processMs, 0)

		// Emit the output colors (first N values for the UI preview).
		if e.onColors != nil {
			flat := flattenColors(deviceColors, cfg.DeviceIDs)
			e.onColors(flat)
		}

		// Sleep for the remainder of the interval. If the frame already took
		// longer than the interval, start the next frame immediately.
		elapsed := time.Since(frameStart)
		if remaining := interval - elapsed; remaining > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(remaining):
			}
		} else {
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}
}

// sendDeviceColorsByBrand partitions toSend by brand and sends each brand in its own
// goroutine. Each brand has a permit slot; if a brand is still sending the previous
// frame, we skip that brand this frame (recordDrop) so Hue throttling doesn't block LIFX.
func (e *Engine) sendDeviceColorsByBrand(ctx context.Context, deviceColors map[string]lights.Color, doSend bool, latency time.Duration) {
	if len(deviceColors) == 0 || !doSend {
		return
	}

	e.lastSentMu.Lock()
	toSend := make(map[string]lights.Color, len(deviceColors))
	for id, c := range deviceColors {
		prev, seen := e.lastSent[id]
		if !seen || colorChangedEnoughToSend(prev, c) {
			toSend[id] = c
		}
	}
	e.lastSentMu.Unlock()

	if len(toSend) == 0 {
		return
	}

	byBrand := make(map[lights.Brand]map[string]lights.Color)
	for id, c := range toSend {
		brand := lights.BrandFromDeviceID(id)
		if brand == "" {
			continue
		}
		if byBrand[brand] == nil {
			byBrand[brand] = make(map[string]lights.Color)
		}
		byBrand[brand][id] = c
	}

	e.ensureBrandSlots()
	for brand, batch := range byBrand {
		if len(batch) == 0 {
			continue
		}
		slot, ok := e.brandSlots[brand]
		if !ok {
			continue
		}
		// Hue bridge: ~10 req/sec. Space batches so we don't hit 429.
		// Skip (don't record drop — this is intentional throttle, not blocking).
		if brand == lights.BrandHue {
			e.hueLastSendMu.Lock()
			minInterval := time.Duration(len(batch)*10) * time.Millisecond // 100ms per req
			if minInterval < 100*time.Millisecond {
				minInterval = 100 * time.Millisecond
			}
			elapsed := time.Since(e.hueLastSend)
			e.hueLastSendMu.Unlock()
			if elapsed < minInterval {
				continue
			}
		}
		select {
		case <-slot:
			// Acquired permit — update lastSent and send in background.
			e.lastSentMu.Lock()
			for id, c := range batch {
				e.lastSent[id] = c
			}
			e.lastSentMu.Unlock()
			brandCopy := brand
			go func(slotCh chan struct{}, b lights.Brand, colors map[string]lights.Color) {
				defer func() { slotCh <- struct{}{} }()
				sendStart := time.Now()
				count := e.sendBatch(ctx, colors)
				e.stats.recordSend(count)
				e.stats.recordSendDuration(time.Since(sendStart))
				if b == lights.BrandHue {
					e.hueLastSendMu.Lock()
					e.hueLastSend = time.Now()
					e.hueLastSendMu.Unlock()
				}
			}(slot, brandCopy, batch)
		default:
			// Brand still busy — skip. Don't record Hue drops (expected to be slow).
			if brand != lights.BrandHue {
				e.stats.recordDrop()
			}
		}
	}
}

// sendBatch sends a device→color map to the light manager. Devices are updated
// concurrently. Returns the number of SetState calls performed.
func (e *Engine) sendBatch(ctx context.Context, batch map[string]lights.Color) int {
	if len(batch) == 0 {
		return 0
	}
	sendCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for devID, c := range batch {
		wg.Add(1)
		go func(id string, col lights.Color) {
			defer wg.Done()
			state := lights.DeviceState{
				On:         true,
				Brightness: col.B,
				Color:      &lights.Color{H: col.H, S: col.S, B: 1.0},
			}
			_ = e.lightMgr.SetDeviceState(sendCtx, id, state)
		}(devID, c)
	}
	wg.Wait()
	return len(batch)
}

// colorChangedEnoughToSend returns true when the RGB difference between two
// HSB colours exceeds the threshold on any channel. Lower = smoother transitions
// (more commands); higher = fewer commands, can cause visible stepping.
func colorChangedEnoughToSend(prev, curr lights.Color) bool {
	const threshold = 1 // ~0.4% of 0-255; was 3 (~1.2%)
	pr, pg, pb := lights.HSBToRGB(prev.H, prev.S, prev.B)
	cr, cg, cb := lights.HSBToRGB(curr.H, curr.S, curr.B)
	diff := func(a, b uint8) int {
		d := int(a) - int(b)
		if d < 0 {
			return -d
		}
		return d
	}
	return diff(pr, cr) > threshold || diff(pg, cg) > threshold || diff(pb, cb) > threshold
}

// flattenColors returns an ordered slice of colors in deviceIDs order.
func flattenColors(deviceColors map[string]lights.Color, deviceIDs []string) []lights.Color {
	out := make([]lights.Color, 0, len(deviceIDs))
	for _, id := range deviceIDs {
		if c, ok := deviceColors[id]; ok {
			out = append(out, c)
		}
	}
	return out
}

// mapChangedMeaningfully returns true when any device color shifted by more
// than ~8% on any RGB channel relative to the previous frame.
func mapChangedMeaningfully(prev, curr map[string]lights.Color) bool {
	if len(prev) == 0 || len(curr) == 0 {
		return false
	}
	const threshold = 20 // ~8% of 0-255
	for id, c := range curr {
		p, ok := prev[id]
		if !ok {
			return true
		}
		pr, pg, pb := lights.HSBToRGB(p.H, p.S, p.B)
		cr, cg, cb := lights.HSBToRGB(c.H, c.S, c.B)
		diff := func(a, b uint8) float64 {
			d := float64(a) - float64(b)
			if d < 0 {
				return -d
			}
			return d
		}
		if diff(pr, cr) > threshold || diff(pg, cg) > threshold || diff(pb, cb) > threshold {
			return true
		}
	}
	return false
}

// assignmentRewired returns true when any device's color changed enough to
// indicate the assignment model re-routed it to a different extracted color.
func assignmentRewired(prev, curr map[string]lights.Color) bool {
	if len(prev) == 0 {
		return false
	}
	const threshold = 40.0 // RGB channel delta to count as a rewire
	for id, c := range curr {
		p, ok := prev[id]
		if !ok {
			return true
		}
		pr, pg, pb := lights.HSBToRGB(p.H, p.S, p.B)
		cr, cg, cb := lights.HSBToRGB(c.H, c.S, c.B)
		diff := func(a, b uint8) float64 {
			d := float64(a) - float64(b)
			if d < 0 {
				return -d
			}
			return d
		}
		if diff(pr, cr) > threshold || diff(pg, cg) > threshold || diff(pb, cb) > threshold {
			return true
		}
	}
	return false
}

// speedPresetInterval returns the target tick interval for the capture loop.
func speedPresetInterval(preset store.SpeedPreset) time.Duration {
	switch preset {
	case store.SpeedPresetVerySlow:
		return 200 * time.Millisecond
	case store.SpeedPresetSlow:
		return 100 * time.Millisecond
	case store.SpeedPresetFast:
		return 33 * time.Millisecond
	case store.SpeedPresetRealtime:
		return 17 * time.Millisecond
	default: // medium
		return 50 * time.Millisecond
	}
}
