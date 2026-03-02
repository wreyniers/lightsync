import { Slider } from "@/components/ui/Slider";
import type { BrightnessMode, ScreenSyncConfig } from "@/lib/types";

interface BrightnessTabProps {
  config: ScreenSyncConfig;
  onChange: (patch: Partial<ScreenSyncConfig>) => void;
}

const brightnessModes: { value: BrightnessMode; label: string; range: string; desc: string }[] = [
  { value: "fully_dynamic", label: "Fully Dynamic", range: "0–100%", desc: "Full range, tracks content exactly" },
  { value: "dark",          label: "Dark",          range: "0–10%",  desc: "Very dim ambient glow" },
  { value: "medium",        label: "Medium",        range: "45–75%", desc: "Comfortable background light" },
  { value: "bright",        label: "Bright",        range: "75–100%",desc: "High-energy, vivid" },
  { value: "full_bright",   label: "Full Bright",   range: "100%",   desc: "Always maximum brightness" },
];

export function BrightnessTab({ config, onChange }: BrightnessTabProps) {
  return (
    <div className="space-y-5">
      {/* Brightness mode */}
      <div>
        <p className="text-xs font-medium uppercase tracking-widest text-muted-foreground mb-2.5">
          Brightness Mode
        </p>
        <div className="space-y-1.5">
          {brightnessModes.map(({ value, label, range, desc }) => (
            <button
              key={value}
              type="button"
              onClick={() => onChange({ brightnessMode: value })}
              className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-left transition-all ${
                config.brightnessMode === value
                  ? "bg-primary/15 ring-1 ring-primary/40"
                  : "bg-background/30 hover:bg-background/60"
              }`}
            >
              <div className="flex-1">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium">{label}</span>
                  <span className={`text-xs px-1.5 py-0.5 rounded font-mono ${
                    config.brightnessMode === value
                      ? "bg-primary/20 text-primary"
                      : "bg-background/50 text-muted-foreground"
                  }`}>
                    {range}
                  </span>
                </div>
                <p className="text-xs text-muted-foreground mt-0.5">{desc}</p>
              </div>
            </button>
          ))}
        </div>
      </div>

      {/* Brightness multiplier */}
      <div>
        <div className="flex items-center justify-between mb-2">
          <p className="text-xs font-medium uppercase tracking-widest text-muted-foreground">
            Brightness Multiplier
          </p>
          <span className="text-xs text-muted-foreground font-mono">
            {config.brightnessMultiplier.toFixed(1)}×
          </span>
        </div>
        <Slider
          value={Math.round((config.brightnessMultiplier - 0.1) / 9.9 * 100)}
          min={0}
          max={100}
          step={1}
          onChange={(v) => onChange({ brightnessMultiplier: 0.1 + (v / 100) * 9.9 })}
        />
        <div className="flex justify-between text-[10px] text-muted-foreground mt-1">
          <span>0.1× (dim)</span>
          <span>1.0× (natural)</span>
          <span>10× (amplified)</span>
        </div>
      </div>
    </div>
  );
}
