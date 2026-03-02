import { Slider } from "@/components/ui/Slider";
import { Toggle } from "@/components/ui/Toggle";
import { Tooltip } from "@/components/ui/Tooltip";
import type { SpeedPreset, ScreenSyncConfig } from "@/lib/types";

interface TransitionsTabProps {
  config: ScreenSyncConfig;
  onChange: (patch: Partial<ScreenSyncConfig>) => void;
}

const speedPresets: { value: SpeedPreset; label: string; fps: number; desc: string }[] = [
  { value: "very_slow", label: "Very Slow", fps: 5,  desc: "5 FPS · gentle ambient" },
  { value: "slow",      label: "Slow",      fps: 10, desc: "10 FPS · relaxed mood" },
  { value: "medium",    label: "Medium",    fps: 20, desc: "20 FPS · balanced default" },
  { value: "fast",      label: "Fast",      fps: 30, desc: "30 FPS · gaming / action" },
  { value: "realtime",  label: "Realtime",  fps: 60, desc: "60 FPS · minimal latency" },
];

export function TransitionsTab({ config, onChange }: TransitionsTabProps) {
  return (
    <div className="space-y-6">
      {/* Speed preset */}
      <div>
        <p className="text-xs font-medium uppercase tracking-widest text-muted-foreground mb-2.5">
          Speed Preset
        </p>
        <div className="grid grid-cols-5 gap-1.5">
          {speedPresets.map(({ value, label, fps }) => (
            <Tooltip key={value} content={speedPresets.find(p => p.value === value)!.desc} side="bottom">
              <button
                type="button"
                onClick={() => onChange({ speedPreset: value })}
                className={`flex flex-col items-center gap-1 py-2.5 px-1 rounded-xl text-center transition-all w-full ${
                  config.speedPreset === value
                    ? "bg-primary/15 ring-1 ring-primary/40 text-primary"
                    : "bg-background/30 text-muted-foreground hover:bg-background/60 hover:text-foreground"
                }`}
              >
                <span className="text-xs font-semibold">{label}</span>
                <span className="text-[10px] opacity-70">{fps} FPS</span>
              </button>
            </Tooltip>
          ))}
        </div>
        <p className="text-xs text-muted-foreground mt-1.5">
          {speedPresets.find((p) => p.value === config.speedPreset)?.desc}
        </p>
      </div>

      {/* ── Smoothing ──────────────────────────────────────────────── */}
      <div className="space-y-4">
        <p className="text-xs font-medium uppercase tracking-widest text-muted-foreground">
          Smoothing
        </p>

        {/* Color smoothing */}
        <div>
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm">Color Smoothing</span>
            <span className="text-xs text-muted-foreground font-mono">
              {Math.round(config.colorSmoothing * 100)}%
            </span>
          </div>
          <Slider
            value={Math.round(config.colorSmoothing * 100)}
            min={0}
            max={200}
            step={1}
            onChange={(v) => onChange({ colorSmoothing: v / 100 })}
          />
          <div className="flex justify-between text-[10px] text-muted-foreground mt-1">
            <span>Off (raw)</span>
            <span>Max (ultra smooth)</span>
          </div>
        </div>

        {/* Brightness ramp speed */}
        <div>
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm">Assignment Handoff</span>
            <span className="text-xs text-muted-foreground font-mono">
              {config.assignmentHandoffMs}ms
            </span>
          </div>
          <Slider
            value={config.assignmentHandoffMs}
            min={0}
            max={1500}
            step={25}
            onChange={(v) => onChange({ assignmentHandoffMs: v })}
          />
          <div className="flex justify-between text-[10px] text-muted-foreground mt-1">
            <span>Off (instant swaps)</span>
            <span>Very gradual handoff</span>
          </div>
          <p className="text-xs text-muted-foreground mt-1.5">
            Adds a post-assignment crossfade per light so color-slot swaps transition smoothly instead of snapping.
          </p>
        </div>

        {/* Brightness ramp speed */}
        <div>
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm">Brightness Ramp Speed</span>
            <span className="text-xs text-muted-foreground font-mono">
              {config.brightnessSmoothing === 0
                ? "Instant"
                : `~${(10 * Math.pow(0.01, config.brightnessSmoothing)).toFixed(1)}/s`}
            </span>
          </div>
          <Slider
            value={Math.round(config.brightnessSmoothing * 100)}
            min={0}
            max={100}
            step={1}
            onChange={(v) => onChange({ brightnessSmoothing: v / 100 })}
          />
          <div className="flex justify-between text-[10px] text-muted-foreground mt-1">
            <span>Instant</span>
            <span>Very slow (~10s full range)</span>
          </div>
          <p className="text-xs text-muted-foreground mt-1.5">
            Limits how fast brightness can change per second. Incoming readings are averaged over a{" "}
            {config.brightnessSmoothing === 0
              ? "100ms"
              : `${((100 + config.brightnessSmoothing * 2900) / 1000).toFixed(1)}s`}{" "}
            window before ramping.
          </p>
        </div>

        {/* Brightness deviation band */}
        <div>
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm">Brightness Deviation Band</span>
            <span className="text-xs text-muted-foreground font-mono">
              ±{Math.round(config.brightnessMaxDeviation * 100)}%
            </span>
          </div>
          <Slider
            value={Math.round(config.brightnessMaxDeviation * 100)}
            min={1}
            max={100}
            step={1}
            onChange={(v) => onChange({ brightnessMaxDeviation: v / 100 })}
          />
          <div className="flex justify-between text-[10px] text-muted-foreground mt-1">
            <span>±1% (very tight)</span>
            <span>±100% (unrestricted)</span>
          </div>
          <p className="text-xs text-muted-foreground mt-1.5">
            How far each light can deviate from the smoothed average brightness. Tighter = more uniform across lights, wider = more spatial variation.
          </p>
        </div>
      </div>

      {/* ── Scene Cut Detection ──────────────────────────────────── */}
      <div>
        <div className="flex items-center justify-between mb-2.5">
          <p className="text-xs font-medium uppercase tracking-widest text-muted-foreground">
            Scene Cut Detection
          </p>
          <Toggle
            checked={config.sceneCutMode !== "off"}
            onChange={(checked) => onChange({ sceneCutMode: checked ? "on" : "off" })}
          />
        </div>
        <div className={config.sceneCutMode === "off" ? "opacity-50 pointer-events-none" : ""}>
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm">Sensitivity</span>
            <span className="text-xs text-muted-foreground font-mono">
              {Math.round(config.sceneCutSensitivity * 100)}%
            </span>
          </div>
          <Slider
            value={Math.round(config.sceneCutSensitivity * 100)}
            min={0}
            max={100}
            step={1}
            onChange={(v) => onChange({ sceneCutSensitivity: v / 100 })}
          />
          <div className="flex justify-between text-[10px] text-muted-foreground mt-1">
            <span>Very conservative</span>
            <span>Very sensitive</span>
          </div>
        </div>
        <p className="text-xs text-muted-foreground mt-1.5">
          {config.sceneCutMode === "off"
            ? "Disabled. Temporal smoothing will not reset on cuts."
            : "When a scene cut is detected, smoothing resets instantly so lights snap to the new content."}
        </p>
      </div>

      {/* ── Brightness Range ─────────────────────────────────────── */}
      <div className="space-y-4">
        <p className="text-xs font-medium uppercase tracking-widest text-muted-foreground">
          Brightness Range
        </p>
        <p className="text-xs text-muted-foreground -mt-2">
          Compress the output brightness into a narrower band. Dark scenes won't go fully dark; bright scenes won't blast full brightness.
        </p>

        {/* Floor */}
        <div>
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm">Floor</span>
            <span className="text-xs text-muted-foreground font-mono">
              {Math.round(config.brightnessFloor * 100)}%
            </span>
          </div>
          <Slider
            value={Math.round(config.brightnessFloor * 100)}
            min={0}
            max={95}
            step={1}
            onChange={(v) => {
              const floor = v / 100;
              const ceiling = config.brightnessCeiling;
              if (ceiling - floor < 0.05) return;
              onChange({ brightnessFloor: floor });
            }}
          />
          <div className="flex justify-between text-[10px] text-muted-foreground mt-1">
            <span>0% (allow dark)</span>
            <span>Higher = brighter minimum</span>
          </div>
        </div>

        {/* Ceiling */}
        <div>
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm">Ceiling</span>
            <span className="text-xs text-muted-foreground font-mono">
              {Math.round(config.brightnessCeiling * 100)}%
            </span>
          </div>
          <Slider
            value={Math.round(config.brightnessCeiling * 100)}
            min={5}
            max={100}
            step={1}
            onChange={(v) => {
              const ceiling = v / 100;
              const floor = config.brightnessFloor;
              if (ceiling - floor < 0.05) return;
              onChange({ brightnessCeiling: ceiling });
            }}
          />
          <div className="flex justify-between text-[10px] text-muted-foreground mt-1">
            <span>Lower = dimmer maximum</span>
            <span>100% (full bright)</span>
          </div>
        </div>
      </div>
    </div>
  );
}
