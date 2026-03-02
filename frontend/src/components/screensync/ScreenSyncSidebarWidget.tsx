import { useEffect, useRef, useState } from "react";
import { MonitorPlay, Lightbulb } from "lucide-react";
import { EventsOn } from "../../../wailsjs/runtime/runtime";
import { GetScreenSyncState } from "../../../wailsjs/go/main/App";
import type { Color, ScreenSyncStats } from "@/lib/types";

function openLightsPopup() {
  const base = window.location.href.replace(/#.*$/, "");
  window.open(
    `${base}#lights-popup`,
    "lightsync-lights",
    "width=420,height=660,resizable=yes,menubar=no,toolbar=no,location=no,status=no"
  );
}

/**
 * Always-visible sidebar widget shown whenever the Screen Sync engine is running.
 * Subscribes to screensync:state, screensync:stats, and screensync:colors independently
 * so it works from any page without prop-drilling.
 */
export function ScreenSyncSidebarWidget() {
  const [running, setRunning] = useState(false);
  const [sceneName, setSceneName] = useState("");
  const [stats, setStats] = useState<ScreenSyncStats | null>(null);
  const [colors, setColors] = useState<Color[]>([]);

  const flashSceneRef = useRef(false);
  const flashColorRef = useRef(false);
  const [sceneFlash, setSceneFlash] = useState(false);
  const [colorFlash, setColorFlash] = useState(false);
  const [cutReasonB, setCutReasonB] = useState(false);
  const [cutReasonH, setCutReasonH] = useState(false);

  // Hydrate on mount.
  useEffect(() => {
    GetScreenSyncState()
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
        setColors([]);
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

    const colorsHandler = (incoming: Color[]) => {
      if (Array.isArray(incoming)) setColors(incoming.slice(0, 8));
    };

    const offState = EventsOn("screensync:state", stateHandler);
    const offStats = EventsOn("screensync:stats", statsHandler);
    const offColors = EventsOn("screensync:colors", colorsHandler);

    return () => {
      offState?.();
      offStats?.();
      offColors?.();
    };
  }, []);

  if (!running) return null;

  const fpsPercent = stats ? Math.min(100, (stats.fps / Math.max(1, stats.targetFps)) * 100) : 0;
  const fpsColor =
    fpsPercent >= 80 ? "bg-success" : fpsPercent >= 50 ? "bg-warning" : "bg-destructive";
  const fpsTextColor =
    fpsPercent >= 80 ? "text-success" : fpsPercent >= 50 ? "text-warning" : "text-destructive";

  return (
    <div className="rounded-xl bg-primary/10 overflow-hidden">
      {/* Title row — matches camera/scene card style */}
      <div className="flex items-center gap-3 px-4 pt-3 pb-2">
        <div className="h-8 w-8 shrink-0 rounded-lg bg-primary/20 flex items-center justify-center">
          <MonitorPlay className="h-4 w-4 text-primary" />
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-1.5">
            <p className="text-sm font-semibold text-primary leading-tight">Screen Sync</p>
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
          className="shrink-0 h-7 w-7 rounded-lg flex items-center justify-center text-muted-foreground hover:text-primary hover:bg-primary/20 transition-colors"
        >
          <Lightbulb className="h-3.5 w-3.5" />
        </button>
      </div>

      {/* Divider */}
      <div className="mx-4 h-px bg-primary/10" />

      <div className="px-4 pb-3 pt-2.5 space-y-2.5">
        {/* Live color circles */}
        {colors.length > 0 ? (
          <div className="flex gap-2 flex-wrap">
            {colors.map((c, i) => {
              const bg = `hsl(${c.h}, ${Math.round(c.s * 100)}%, ${Math.round(c.b * 50)}%)`;
              const pct = Math.round(c.b * 100);
              return (
                <div
                  key={i}
                  className="h-8 w-8 rounded-full ring-1 ring-white/20 shadow transition-colors duration-300 shrink-0 relative flex items-center justify-center"
                  style={{ background: bg }}
                  title={`H:${Math.round(c.h)}° S:${Math.round(c.s * 100)}% B:${pct}%`}
                >
                  <span className="text-[10px] font-medium text-white drop-shadow-sm">{pct}%</span>
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
