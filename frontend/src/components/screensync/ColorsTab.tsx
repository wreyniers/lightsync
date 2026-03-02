import { SegmentedControl } from "@/components/ui/SegmentedControl";
import { Slider } from "@/components/ui/Slider";
import { SpatialGridPreview } from "./SpatialGridPreview";
import type { ScreenSyncConfig, ExtractionMethod, MultiColorApproach } from "@/lib/types";

interface ColorsTabProps {
  config: ScreenSyncConfig;
  onChange: (patch: Partial<ScreenSyncConfig>) => void;
}

const extractionMethods: { value: ExtractionMethod; label: string; desc: string }[] = [
  { value: "vivid", label: "Vivid", desc: "Colorful areas win over grey backgrounds — best default" },
  { value: "dominant", label: "Dominant", desc: "Most pixels by area — picks backgrounds too" },
  { value: "brightest", label: "Brightest", desc: "Highest luminance pixel" },
  { value: "saturated", label: "Saturated", desc: "Single most vivid pixel — sensitive to tiny details" },
  { value: "diverse", label: "Diverse", desc: "Maximise color distance" },
];

const multiApproaches: { value: MultiColorApproach; label: string; desc: string }[] = [
  { value: "spatial_grid", label: "Spatial Grid", desc: "Ambilight-style: one color per screen zone" },
  { value: "scene_palette", label: "Scene Palette", desc: "Top N dominant colors from full frame" },
];

export function ColorsTab({ config, onChange }: ColorsTabProps) {
  return (
    <div className="space-y-5">
      {/* Single vs Multi */}
      <div>
        <p className="text-xs font-medium uppercase tracking-widest text-muted-foreground mb-2.5">
          Color Mode
        </p>
        <SegmentedControl
          options={[
            { value: "single", label: "Single Color" },
            { value: "multi", label: "Multi Color" },
          ]}
          value={config.colorMode}
          onChange={(v) => onChange({ colorMode: v })}
        />
        <p className="text-xs text-muted-foreground mt-1.5">
          {config.colorMode === "single"
            ? "One color sent to all assigned lights."
            : "One color per assigned light, based on screen position or palette."}
        </p>
      </div>

      {/* Multi-color approach */}
      {config.colorMode === "multi" && (
        <div>
          <p className="text-xs font-medium uppercase tracking-widest text-muted-foreground mb-2.5">
            Multi-Color Approach
          </p>
          <div className="grid grid-cols-2 gap-2">
            {multiApproaches.map(({ value, label, desc }) => (
              <button
                key={value}
                type="button"
                onClick={() => onChange({ multiColorApproach: value })}
                className={`flex flex-col gap-1 p-3 rounded-xl text-left transition-all ${
                  config.multiColorApproach === value
                    ? "bg-primary/15 ring-1 ring-primary/40 text-primary"
                    : "bg-background/30 text-muted-foreground hover:bg-background/60 hover:text-foreground"
                }`}
              >
                <span className="text-xs font-semibold">{label}</span>
                <span className="text-[11px] opacity-70 leading-snug">{desc}</span>
              </button>
            ))}
          </div>

          {/* Spatial grid zone diagram */}
          {config.multiColorApproach === "spatial_grid" && config.deviceIds.length > 0 && (
            <SpatialGridPreview deviceCount={config.deviceIds.length} />
          )}

          {/* Palette stability — only visible for scene_palette */}
          {config.multiColorApproach === "scene_palette" && (
            <div className="mt-3 pt-3 border-t border-border/40">
              <div className="flex items-center justify-between mb-2">
                <p className="text-xs font-medium uppercase tracking-widest text-muted-foreground">
                  Palette Stability
                </p>
                <span className="text-xs text-muted-foreground">
                  {config.paletteStability >= 1.6
                    ? "Ultra stable"
                    : config.paletteStability >= 1.2
                    ? "Very stable"
                    : config.paletteStability >= 0.90
                    ? "Stable+"
                    : config.paletteStability >= 0.70
                    ? "Stable"
                    : config.paletteStability >= 0.45
                    ? "Balanced"
                    : config.paletteStability >= 0.20
                    ? "Reactive"
                    : "Live"}
                </span>
              </div>
              <Slider
                value={Math.round(config.paletteStability * 100)}
                min={0}
                max={200}
                step={5}
                onChange={(v) => onChange({ paletteStability: v / 100 })}
              />
              <div className="flex justify-between text-[10px] text-muted-foreground mt-1">
                <span>← Reactive</span>
                <span>Stable → Ultra stable</span>
              </div>
              <p className="text-xs text-muted-foreground mt-1.5">
                Accumulates pixel data across frames before choosing colours — higher values require a colour to consistently dominate before it enters the palette.
              </p>
            </div>
          )}
        </div>
      )}

      {/* Extraction method */}
      <div>
        <p className="text-xs font-medium uppercase tracking-widest text-muted-foreground mb-2">
          {config.colorMode === "multi" ? "Sub-Method (per zone)" : "Extraction Method"}
        </p>
        <div className="space-y-1.5">
          {extractionMethods.map(({ value, label, desc }) => (
            <button
              key={value}
              type="button"
              onClick={() =>
                config.colorMode === "multi"
                  ? onChange({ subMethod: value })
                  : onChange({ extractionMethod: value })
              }
              className={`w-full flex items-center justify-between px-3 py-2.5 rounded-lg text-left transition-all ${
                (config.colorMode === "multi" ? config.subMethod : config.extractionMethod) === value
                  ? "bg-primary/15 ring-1 ring-primary/40"
                  : "bg-background/30 hover:bg-background/60"
              }`}
            >
              <span className="text-sm font-medium">{label}</span>
              <span className="text-xs text-muted-foreground ml-2 text-right max-w-[160px] leading-tight">
                {desc}
              </span>
            </button>
          ))}
        </div>
      </div>

      {/* Saturation Boost */}
      <div>
        <div className="flex items-center justify-between mb-2">
          <p className="text-xs font-medium uppercase tracking-widest text-muted-foreground">
            Saturation Boost
          </p>
          <span className="text-xs text-muted-foreground">{config.saturationBoost.toFixed(1)}×</span>
        </div>
        <Slider
          value={config.saturationBoost * 50}
          min={0}
          max={100}
          step={5}
          onChange={(v) => onChange({ saturationBoost: v / 50 })}
        />
        <div className="flex justify-between text-[10px] text-muted-foreground mt-1">
          <span>0× (muted)</span>
          <span>1× (natural)</span>
          <span>2× (vivid)</span>
        </div>
      </div>

      {/* White Bias */}
      <div>
        <div className="flex items-center justify-between mb-2">
          <p className="text-xs font-medium uppercase tracking-widest text-muted-foreground">
            White Bias
          </p>
          <span className="text-xs text-muted-foreground">
            {config.whiteBias === 0
              ? "Neutral"
              : config.whiteBias < 0
              ? `Colorful ${(Math.abs(config.whiteBias) * 100).toFixed(0)}%`
              : `White ${(config.whiteBias * 100).toFixed(0)}%`}
          </span>
        </div>
        <Slider
          value={(config.whiteBias + 1) * 50}
          min={0}
          max={100}
          step={5}
          onChange={(v) => onChange({ whiteBias: (v / 50) - 1 })}
        />
        <div className="flex justify-between text-[10px] text-muted-foreground mt-1">
          <span>← Filter grays</span>
          <span>Neutral</span>
          <span>Prefer white →</span>
        </div>
      </div>
    </div>
  );
}
