import { useEffect, useRef, useState } from "react";
import { MonitorPlay, Lightbulb } from "lucide-react";
import { App } from "@bindings";
import { Events } from "@wailsio/runtime";
import type { Color, ScreenSyncStats } from "@/lib/types";

function openLightsPopup() {
  App.OpenLightsPopup();
}

/**
 * Always-visible sidebar widget shown whenever the Screen Sync engine is running.
 * Subscribes to screensync:state, screensync:stats, and screensync:colors independently
 * so it works from any page without prop-drilling.
 */
// Perceptual alignment thresholds shared with the other swatch previews.
const ALIGN_HUE_DEG = 3;
const ALIGN_SAT = 0.03;
const ALIGN_BRIGHT = 0.03;

function isAligned(a: Color | undefined, b: Color | undefined): boolean {
  if (!a || !b) return false;
  let dh = Math.abs(a.h - b.h);
  if (dh > 180) dh = 360 - dh;
  return (
    dh <= ALIGN_HUE_DEG &&
    Math.abs(a.s - b.s) <= ALIGN_SAT &&
    Math.abs(a.b - b.b) <= ALIGN_BRIGHT
  );
}

export function ScreenSyncSidebarWidget() {
  const [running, setRunning] = useState(false);
  const [sceneName, setSceneName] = useState("");
  const [stats, setStats] = useState<ScreenSyncStats | null>(null);
  const [active, setActive] = useState<Color[]>([]);
  const [target, setTarget] = useState<Color[]>([]);

  const flashSceneRef = useRef(false);
  const flashColorRef = useRef(false);
  const [sceneFlash, setSceneFlash] = useState(false);
  const [colorFlash, setColorFlash] = useState(false);
  const [cutReasonB, setCutReasonB] = useState(false);
  const [cutReasonH, setCutReasonH] = useState(false);

  // Hydrate on mount.
  useEffect(() => {
    App.GetScreenSyncState()
      .then((s) => setRunning(s.running))
      .catch(() => {});
  }, []);

  // Wails event subscriptions.
  useEffect(() => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const stateHandler = (data: any) => {
      setRunning(data?.running ?? false);
      if (!data?.running) {
        setStats(null);
        setActive([]);
        setTarget([]);
        setSceneName("");
      }
    };

    const statsHandler = (s: ScreenSyncStats) => {
      setStats(s);
      if (s.sceneChange && !flashSceneRef.current) {
        flashSceneRef.current = true;
        setSceneFlash(true);
        setCutReasonB(s.cutReasonBrightness ?? false);
        setCutReasonH(s.cutReasonHue ?? false);
        setTimeout(() => {
          flashSceneRef.current = false;
          setSceneFlash(false);
          setCutReasonB(false);
          setCutReasonH(false);
        }, 800);
      }
      if (s.colorChanged && !flashColorRef.current) {
        flashColorRef.current = true;
        setColorFlash(true);
        setTimeout(() => { flashColorRef.current = false; setColorFlash(false); }, 400);
      }
    };

    const take = (setter: (c: Color[]) => void) => (incoming: Color[]) => {
      if (Array.isArray(incoming)) setter(incoming.slice(0, 8));
    };

    const offState = Events.On("screensync:state", (e) => stateHandler(e.data));
    const offStats = Events.On("screensync:stats", (e) => statsHandler(e.data));
    const offColors = Events.On("screensync:colors", (e) => take(setActive)(e.data as Color[]));
    const offTarget = Events.On("screensync:targetColors", (e) => take(setTarget)(e.data as Color[]));

    return () => {
      offState?.();
      offStats?.();
      offColors?.();
      offTarget?.();
    };
  }, []);

  if (!running) return null;

  const fpsPercent = stats ? Math.min(100, (stats.fps / Math.max(1, stats.targetFps)) * 100) : 0;
  const fpsColor =
    fpsPercent >= 80 ? "bg-success" : fpsPercent >= 50 ? "bg-warning" : "bg-destructive";
  const fpsTextColor =
    fpsPercent >= 80 ? "text-success" : fpsPercent >= 50 ? "text-warning" : "text-destructive";

  return (
    <div className="rounded-xl bg-muted overflow-hidden">
      {/* Title row — matches camera/scene card style */}
      <div className="flex items-center gap-3 px-4 pt-3 pb-2">
        <div className="h-8 w-8 shrink-0 rounded-xl bg-background/40 flex items-center justify-center">
          <MonitorPlay className="h-4 w-4 text-muted-foreground" />
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-1.5">
            <p className="text-sm font-semibold text-foreground leading-tight">Screen Sync</p>
            <span className="h-1.5 w-1.5 rounded-full bg-success animate-pulse shrink-0" />
          </div>
          <p className="text-xs text-muted-foreground mt-0.5 truncate">
            {sceneName || "Running"}
          </p>
        </div>
        <button
          type="button"
          onClick={openLightsPopup}
          title="Open lights panel"
          className="shrink-0 h-7 w-7 rounded-xl flex items-center justify-center text-muted-foreground hover:text-foreground hover:bg-background/40 transition-colors"
        >
          <Lightbulb className="h-3.5 w-3.5" />
        </button>
      </div>

      <div className="px-4 pb-3 pt-2.5 space-y-2.5">
        {/* Live split swatches — left half = active colour sent to the bulb;
            right half = target the handoff ramp is driving toward; white
            outer ring appears once the two halves are perceptually aligned. */}
        {active.length > 0 || target.length > 0 ? (
          <div className="flex gap-2 flex-wrap">
            {Array.from({ length: Math.max(active.length, target.length) }).map((_, i) => {
              const a = active[i];
              const t = target[i];
              const aligned = isAligned(a, t);
              const aBg = a
                ? `hsl(${a.h}, ${Math.round(a.s * 100)}%, ${Math.round(a.b * 50)}%)`
                : "rgba(255,255,255,0.06)";
              const tBg = t
                ? `hsl(${t.h}, ${Math.round(t.s * 100)}%, ${Math.round(t.b * 50)}%)`
                : "rgba(255,255,255,0.06)";
              const pct = a ? Math.round(a.b * 100) : t ? Math.round(t.b * 100) : 0;

              return (
                <div
                  key={i}
                  className={`relative h-8 w-8 rounded-full overflow-hidden shadow shrink-0 transition-shadow ${
                    aligned ? "ring-2 ring-white/80" : "ring-1 ring-white/20"
                  }`}
                  title={
                    `Slot ${i + 1}\n` +
                    `  active:  ${a ? `H:${Math.round(a.h)}° S:${Math.round(a.s * 100)}% B:${Math.round(a.b * 100)}%` : "—"}\n` +
                    `  target:  ${t ? `H:${Math.round(t.h)}° S:${Math.round(t.s * 100)}% B:${Math.round(t.b * 100)}%` : "—"}` +
                    (aligned ? "\n  ✓ aligned" : "")
                  }
                >
                  <div
                    className="absolute inset-y-0 left-0 w-1/2 transition-colors duration-300"
                    style={{ background: aBg }}
                  />
                  <div
                    className="absolute inset-y-0 right-0 w-1/2 transition-colors duration-300"
                    style={{ background: tBg }}
                  />
                  {!aligned && (a || t) && (
                    <span className="absolute inset-y-0 left-1/2 -translate-x-1/2 w-px bg-white/20 pointer-events-none" />
                  )}
                  <span className="absolute inset-0 flex items-center justify-center text-[10px] font-medium text-white drop-shadow-sm pointer-events-none">
                    {pct}%
                  </span>
                </div>
              );
            })}
          </div>
        ) : (
          /* Pulse placeholders while waiting for first frame */
          <div className="flex gap-2">
            {[0, 1, 2].map((i) => (
              <div key={i} className="h-8 w-8 rounded-full bg-white/10 ring-1 ring-border animate-pulse shrink-0" />
            ))}
          </div>
        )}

        {/* FPS */}
        <div>
          <div className="flex items-center justify-between mb-1">
            <span className="text-xs text-muted-foreground">FPS</span>
            <span className={`text-xs font-mono font-semibold ${fpsTextColor}`}>
              {stats ? `${stats.fps.toFixed(1)} / ${stats.targetFps}` : "—"}
            </span>
          </div>
          <div className="h-1 bg-background/40 rounded-full overflow-hidden">
            <div
              className={`h-full rounded-full transition-all duration-500 ${fpsColor}`}
              style={{ width: `${fpsPercent}%` }}
            />
          </div>
        </div>

        {/* Update rate + dropped frames */}
        {stats && (
          <div className="space-y-1">
            <div className="flex items-center justify-between">
              <span className="text-xs text-muted-foreground">Update rate</span>
              <span className="text-xs font-mono">
                {stats.updateRate != null ? `${stats.updateRate.toFixed(1)} req/s` : "—"}
              </span>
            </div>
            {(stats.framesDropped != null && stats.framesDropped > 0) ? (
              <div className="flex items-center justify-between">
                <span className="text-[10px] text-muted-foreground/60">Frames dropped</span>
                <span className="text-[10px] font-mono text-warning">
                  {stats.framesDropped}
                  {stats.framesDroppedPct != null ? ` (${stats.framesDroppedPct.toFixed(0)}%)` : ""}
                </span>
              </div>
            ) : null}
          </div>
        )}

        {/* Latency + breakdown */}
        <div className="flex items-center justify-between">
          <span className="text-xs text-muted-foreground">Latency</span>
          <span className="text-xs font-mono">
            {stats ? `${stats.latencyMs.toFixed(1)} ms` : "—"}
          </span>
        </div>
        {stats && (stats.captureMs > 0 || stats.processMs > 0 || stats.sendMs > 0) && (
          <div className="flex items-center justify-between">
            <span className="text-[10px] text-muted-foreground/60">↳ cap / proc / send</span>
            <span className="text-[10px] font-mono text-muted-foreground/60">
              {stats.captureMs.toFixed(0)} / {stats.processMs.toFixed(0)} / {stats.sendMs.toFixed(0)} ms
            </span>
          </div>
        )}

        {/* Scene cut / color change / drastic-blocked indicators */}
        <div className="flex flex-wrap gap-2">
          <div
            className={`flex items-center gap-1 text-[10px] px-1.5 py-0.5 rounded transition-all ${
              sceneFlash ? "bg-warning/20 text-warning" : "text-muted-foreground"
            }`}
            title={
              sceneFlash
                ? "Scene cut detected"
                : "Flashes on a detected scene cut."
            }
          >
            <span className={`h-1.5 w-1.5 rounded-full shrink-0 transition-colors ${
              sceneFlash ? "bg-warning" : "bg-white/20"
            }`} />
            Scene cut
          </div>

          {/* Cut reason sub-indicators — always visible, light up when active */}
          <div
            className={`flex items-center gap-1 text-[10px] px-1.5 py-0.5 rounded transition-all ${
              cutReasonB && sceneFlash ? "bg-sky-500/20 text-sky-300" : "text-muted-foreground/30"
            }`}
            title="Brightness jump ≥ 40% — dark↔bright transition"
          >
            <span className={`h-1.5 w-1.5 rounded-full shrink-0 transition-colors ${cutReasonB && sceneFlash ? "bg-sky-400" : "bg-white/10"}`} />
            Bright
          </div>
          <div
            className={`flex items-center gap-1 text-[10px] px-1.5 py-0.5 rounded transition-all ${
              cutReasonH && sceneFlash ? "bg-rose-500/20 text-rose-300" : "text-muted-foreground/30"
            }`}
            title="Hue jump ≥ 80° — colour-family change"
          >
            <span className={`h-1.5 w-1.5 rounded-full shrink-0 transition-colors ${cutReasonH && sceneFlash ? "bg-rose-400" : "bg-white/10"}`} />
            Hue
          </div>
          <div
            className={`flex items-center gap-1 text-[10px] px-1.5 py-0.5 rounded transition-colors ${
              colorFlash ? "bg-primary/20 text-primary" : "text-muted-foreground"
            }`}
            title="Flashes when the output color shifts noticeably"
          >
            <span className={`h-1.5 w-1.5 rounded-full shrink-0 ${colorFlash ? "bg-primary" : "bg-white/20"}`} />
            Color
          </div>
        </div>
      </div>
    </div>
  );
}
