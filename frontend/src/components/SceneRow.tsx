import { useState } from "react";
import {
  Play, Square, Pencil, Trash2, MonitorPlay, Copy, ChevronDown, Lightbulb,
  Monitor, Crop, AppWindow, LayoutGrid, Palette, SlidersHorizontal, Scissors,
} from "lucide-react";
import { Card } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Badge } from "@/components/ui/Badge";
import { sceneSwatchBackground } from "@/lib/sceneColors";
import type { Device, Scene, DeviceState, ScreenSyncConfig } from "@/lib/types";
import { SCREEN_SYNC_TRIGGER } from "@/lib/types";
import { cn } from "@/lib/utils";

interface SceneRowProps {
  scene: Scene;
  isActive: boolean;
  devices: Device[];
  onActivate: () => void;
  onStop: () => void;
  onEdit: () => void;
  onDelete: () => void;
  onClone: () => void;
}

// ─── Shared primitives ────────────────────────────────────────────────────────

function ColorDot({ color, kelvin }: { color?: { h: number; s: number; b: number }; kelvin?: number }) {
  if (color) {
    const bg = `hsl(${color.h}, ${Math.round(color.s * 100)}%, ${Math.round(color.b * 50)}%)`;
    return (
      <span
        className="inline-block h-3 w-3 rounded-full ring-1 ring-white/20 shrink-0"
        style={{ background: bg }}
        title={`H:${Math.round(color.h)}° S:${Math.round(color.s * 100)}%`}
      />
    );
  }
  if (kelvin) {
    const warmth = Math.max(0, Math.min(1, (kelvin - 2700) / (6500 - 2700)));
    const r = Math.round(255 - warmth * 80);
    const g = Math.round(200 + warmth * 55);
    const b = Math.round(100 + warmth * 155);
    return (
      <span
        className="inline-block h-3 w-3 rounded-full ring-1 ring-white/20 shrink-0"
        style={{ background: `rgb(${r},${g},${b})` }}
        title={`${kelvin}K`}
      />
    );
  }
  return <span className="inline-block h-3 w-3 rounded-full bg-white/10 ring-1 ring-white/20 shrink-0" />;
}

/** A labelled key/value row used inside config sections. */
function Row({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="flex items-baseline justify-between gap-3 text-xs py-0.5">
      <span className="text-muted-foreground shrink-0">{label}</span>
      <span className="text-foreground/80 text-right">{value}</span>
    </div>
  );
}

/** Section header with an icon. */
function SectionHeader({ icon: Icon, label }: { icon: React.ComponentType<{ className?: string }>; label: string }) {
  return (
    <div className="flex items-center gap-2 pt-1">
      <Icon className="h-3.5 w-3.5 text-muted-foreground/60 shrink-0" />
      <span className="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/60">{label}</span>
    </div>
  );
}

function fmt(raw: string | undefined) {
  return (raw ?? "").replace(/_/g, " ");
}

function pct(v: number | undefined, decimals = 0) {
  if (v == null) return "—";
  return `${(v * 100).toFixed(decimals)}%`;
}

// ─── Regular scene accordion ──────────────────────────────────────────────────

function RegularSceneDetails({ scene, devices }: { scene: Scene; devices: Device[] }) {
  const entries = Object.entries(scene.devices ?? {});
  const hasGlobal = scene.globalColor != null || scene.globalKelvin != null;

  return (
    <div className="grid grid-cols-2 gap-x-8 gap-y-4">
      {/* Lights */}
      <div className={cn(!hasGlobal && "col-span-2")}>
        <SectionHeader icon={Lightbulb} label={`Lights (${entries.length})`} />
        <div className="mt-1.5 space-y-1.5">
          {entries.length === 0 && (
            <p className="text-xs text-muted-foreground italic">No lights configured.</p>
          )}
          {entries.map(([id, ds]: [string, DeviceState]) => {
            const device = devices.find((d) => d.id === id);
            return (
              <div key={id} className="flex items-center gap-2 text-xs min-w-0">
                <span className={cn("h-1.5 w-1.5 rounded-full shrink-0", ds.on ? "bg-success" : "bg-muted-foreground/30")} />
                <ColorDot color={ds.color} kelvin={ds.kelvin} />
                <span className="text-foreground/80 truncate flex-1">{device?.name ?? id}</span>
                <span className="text-muted-foreground shrink-0">
                  {ds.on
                    ? ds.color
                      ? `H${Math.round(ds.color.h)}° ${Math.round((ds.brightness ?? 0.8) * 100)}%`
                      : ds.kelvin
                      ? `${ds.kelvin}K ${Math.round((ds.brightness ?? 0.8) * 100)}%`
                      : `${Math.round((ds.brightness ?? 0.8) * 100)}%`
                    : "Off"}
                </span>
              </div>
            );
          })}
        </div>
      </div>

      {/* Global color override */}
      {hasGlobal && (
        <div>
          <SectionHeader icon={Palette} label="Global Override" />
          <div className="mt-1.5 space-y-1">
            {scene.globalColor && (
              <div className="flex items-center justify-between gap-2 text-xs py-0.5">
                <span className="text-muted-foreground">Color</span>
                <span className="flex items-center gap-1.5">
                  <ColorDot color={scene.globalColor} />
                  <span className="text-foreground/80">
                    H{Math.round(scene.globalColor.h)}° S{Math.round(scene.globalColor.s * 100)}%
                  </span>
                </span>
              </div>
            )}
            {scene.globalKelvin != null && (
              <div className="flex items-center justify-between gap-2 text-xs py-0.5">
                <span className="text-muted-foreground">Temperature</span>
                <span className="flex items-center gap-1.5">
                  <ColorDot kelvin={scene.globalKelvin} />
                  <span className="text-foreground/80">{scene.globalKelvin} K</span>
                </span>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

// ─── Screen Sync accordion ────────────────────────────────────────────────────

function ScreenSyncDetails({ cfg, devices }: { cfg: ScreenSyncConfig; devices: Device[] }) {
  const isRegion = cfg.captureMode === "region";
  const isWindow = cfg.captureMode === "window";
  const isMonitor = cfg.captureMode === "monitor";
  const isActiveWindow = cfg.captureMode === "active_window";

  const isMulti = cfg.colorMode === "multi";
  const isPalette = cfg.multiColorApproach === "scene_palette";

  const isIdentityLock = cfg.assignmentStrategy === "identity_lock";
  const isFlowTrack = cfg.assignmentStrategy === "flow_track";
  const isSceneCutRemap = cfg.assignmentStrategy === "scene_cut_remap";

  const CaptureIcon = isMonitor ? Monitor : isRegion ? Crop : isWindow || isActiveWindow ? AppWindow : Monitor;
  const syncDeviceEntries = (cfg.deviceIds ?? []).map((id) => devices.find((d) => d.id === id));

  return (
    <div className="grid grid-cols-2 gap-x-8 gap-y-4">

      {/* ── Capture ── */}
      <div>
        <SectionHeader icon={CaptureIcon} label="Capture" />
        <div className="mt-1.5 space-y-1">
          <Row label="Mode" value={fmt(cfg.captureMode)} />
          {isMonitor && <Row label="Monitor" value={`#${cfg.monitorIndex}`} />}
          {isRegion && (
            <>
              <Row label="Origin" value={`${cfg.region.x}, ${cfg.region.y}`} />
              <Row label="Size" value={`${cfg.region.width} × ${cfg.region.height} px`} />
            </>
          )}
          {isWindow && <Row label="Window" value={cfg.windowTitle || "—"} />}
        </div>
      </div>

      {/* ── Lights ── */}
      <div>
        <SectionHeader icon={Lightbulb} label={`Lights (${cfg.deviceIds?.length ?? 0})`} />
        <div className="mt-1.5 space-y-1">
          {syncDeviceEntries.length === 0 && (
            <p className="text-xs text-muted-foreground italic">No lights assigned.</p>
          )}
          {syncDeviceEntries.map((dev, i) => (
            <div key={cfg.deviceIds[i]} className="flex items-center gap-2 text-xs">
              <span className="h-1.5 w-1.5 rounded-full bg-muted-foreground/40 shrink-0" />
              <span className="text-foreground/80 truncate">{dev?.name ?? cfg.deviceIds[i]}</span>
            </div>
          ))}
        </div>
      </div>

      {/* ── Color Extraction ── */}
      <div>
        <SectionHeader icon={Palette} label="Color Extraction" />
        <div className="mt-1.5 space-y-1">
          <Row label="Mode" value={isMulti ? "Multi-color" : "Single color"} />
          <Row label="Method" value={fmt(cfg.extractionMethod)} />
          {isMulti && <Row label="Approach" value={fmt(cfg.multiColorApproach)} />}
          {isMulti && isPalette && <Row label="Sub-method" value={fmt(cfg.subMethod)} />}
          {isMulti && isPalette && <Row label="Palette stability" value={cfg.paletteStability.toFixed(2)} />}
          <Row label="Saturation boost" value={`×${cfg.saturationBoost.toFixed(2)}`} />
          <Row label="White bias" value={cfg.whiteBias.toFixed(2)} />
        </div>
      </div>

      {/* ── Brightness ── */}
      <div>
        <SectionHeader icon={SlidersHorizontal} label="Brightness" />
        <div className="mt-1.5 space-y-1">
          <Row label="Mode" value={fmt(cfg.brightnessMode)} />
          <Row label="Multiplier" value={`×${cfg.brightnessMultiplier.toFixed(2)}`} />
          <Row label="Floor" value={pct(cfg.brightnessFloor)} />
          <Row label="Ceiling" value={pct(cfg.brightnessCeiling)} />
          <Row label="Smoothing" value={cfg.brightnessSmoothing.toFixed(2)} />
          <Row label="Max deviation" value={pct(cfg.brightnessMaxDeviation)} />
        </div>
      </div>

      {/* ── Color Assignment ── */}
      <div>
        <SectionHeader icon={LayoutGrid} label="Color Assignment" />
        <div className="mt-1.5 space-y-1">
          <Row label="Strategy" value={fmt(cfg.assignmentStrategy)} />
          {isIdentityLock && <Row label="Breach threshold" value={pct(cfg.identityLockBreachThreshold)} />}
          {isFlowTrack && <Row label="EMA alpha" value={cfg.flowTrackEmaAlpha.toFixed(2)} />}
          {isFlowTrack && <Row label="Solve interval" value={`${cfg.flowTrackSolveIntervalMs} ms`} />}
          {isSceneCutRemap && <Row label="Hold after cut" value={`${cfg.sceneCutRemapHoldMs} ms`} />}
        </div>
      </div>

      {/* ── Smoothing & Scene Cut ── */}
      <div>
        <SectionHeader icon={Scissors} label="Smoothing & Scene Cut" />
        <div className="mt-1.5 space-y-1">
          <Row label="Color smoothing" value={cfg.colorSmoothing.toFixed(2)} />
          <Row label="Assignment handoff" value={`${cfg.assignmentHandoffMs} ms`} />
          <Row label="Scene cut" value={fmt(cfg.sceneCutMode)} />
          <Row label="Cut sensitivity" value={cfg.sceneCutSensitivity.toFixed(2)} />
          <Row label="Speed preset" value={fmt(cfg.speedPreset)} />
        </div>
      </div>

    </div>
  );
}

// ─── Main component ───────────────────────────────────────────────────────────

export function SceneRow({
  scene,
  isActive,
  devices,
  onActivate,
  onStop,
  onEdit,
  onDelete,
  onClone,
}: SceneRowProps) {
  const [expanded, setExpanded] = useState(false);
  const isScreenSync = scene.trigger === SCREEN_SYNC_TRIGGER;
  const swatchBg = isScreenSync ? undefined : sceneSwatchBackground(scene);

  const triggerLabel =
    scene.trigger === "camera_on" ? "Camera On"
    : scene.trigger === "camera_off" ? "Camera Off"
    : scene.trigger === SCREEN_SYNC_TRIGGER ? "Screen Sync"
    : "Manual";

  const deviceCount = isScreenSync
    ? scene.screenSync?.deviceIds?.length ?? 0
    : Object.keys(scene.devices || {}).length;

  const cfg = scene.screenSync as ScreenSyncConfig | undefined;

  return (
    <Card className="p-0 overflow-hidden">
      {/* ── Main row ── */}
      <div className="flex items-center justify-between px-4 py-3">
        <div className="flex items-center gap-3 min-w-0">
          <div
            className={cn(
              "h-10 w-10 shrink-0 rounded-full flex items-center justify-center",
              !swatchBg && "bg-muted"
            )}
            style={swatchBg ? { background: swatchBg } : undefined}
          >
            {isScreenSync && <MonitorPlay className="h-4 w-4 text-primary" />}
            {!isScreenSync && !swatchBg && <Lightbulb className="h-4 w-4 text-muted-foreground" />}
          </div>
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <p className="font-semibold truncate">{scene.name}</p>
              {isActive && <Badge variant="success">Active</Badge>}
            </div>
            <p className="text-xs text-muted-foreground">
              {triggerLabel} · {deviceCount} light{deviceCount !== 1 ? "s" : ""}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-1 shrink-0">
          {/* Play / Stop toggle */}
          {isActive ? (
            <Button
              variant="ghost"
              size="icon"
              onClick={onStop}
              title="Stop scene"
              className="text-destructive hover:text-destructive hover:bg-destructive/10"
            >
              <Square className="h-4 w-4 fill-current" />
            </Button>
          ) : (
            <Button variant="ghost" size="icon" onClick={onActivate} title="Activate scene">
              <Play className="h-4 w-4 text-primary" />
            </Button>
          )}

          <Button variant="ghost" size="icon" onClick={onEdit} title="Edit">
            <Pencil className="h-4 w-4" />
          </Button>
          <Button variant="ghost" size="icon" onClick={onClone} title="Clone scene">
            <Copy className="h-4 w-4" />
          </Button>
          <Button variant="ghost" size="icon" onClick={onDelete} title="Delete">
            <Trash2 className="h-4 w-4 text-destructive" />
          </Button>
          <button
            type="button"
            onClick={() => setExpanded((v) => !v)}
            title={expanded ? "Collapse" : "Expand settings"}
            className="h-8 w-8 flex items-center justify-center rounded-md text-muted-foreground hover:text-foreground hover:bg-white/10 transition-colors"
          >
            <ChevronDown className={cn("h-4 w-4 transition-transform duration-200", expanded && "rotate-180")} />
          </button>
        </div>
      </div>

      {/* ── Accordion ── */}
      {expanded && (
        <div className="px-4 py-4 bg-card">
          {/* Trigger badge */}
          <div className="flex items-center gap-2 mb-3">
            <span className="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/60">
              Trigger
            </span>
            <span className="text-xs text-foreground/70">{triggerLabel}</span>
          </div>

          {isScreenSync && cfg ? (
            <ScreenSyncDetails cfg={cfg} devices={devices} />
          ) : (
            <RegularSceneDetails scene={scene} devices={devices} />
          )}
        </div>
      )}
    </Card>
  );
}
