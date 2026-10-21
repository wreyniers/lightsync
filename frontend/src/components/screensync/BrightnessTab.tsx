import { SettingsSection, SliderControl, OptionTile } from "./settings";
import type { BrightnessMode, ScreenSyncConfig } from "@/lib/types";

interface BrightnessTabProps {
  config: ScreenSyncConfig;
  onChange: (patch: Partial<ScreenSyncConfig>) => void;
}

const brightnessModes: { value: BrightnessMode; label: string; range: string; desc: string }[] = [
  { value: "fully_dynamic", label: "Fully Dynamic", range: "0–100%", desc: "Full range, tracks content exactly" },
  { value: "dark", label: "Dark", range: "0–10%", desc: "Very dim ambient glow" },
  { value: "medium", label: "Medium", range: "45–75%", desc: "Comfortable background light" },
  { value: "bright", label: "Bright", range: "75–100%", desc: "High-energy, vivid" },
  { value: "full_bright", label: "Full Bright", range: "100%", desc: "Always maximum brightness" },
];

export function BrightnessTab({ config, onChange }: BrightnessTabProps) {
  return (
    <div className="space-y-5">
      <SettingsSection title="Brightness Level">
        <div className="space-y-2">
          {brightnessModes.map(({ value, label, range, desc }) => (
            <OptionTile
              key={value}
              selected={config.brightnessMode === value}
              onClick={() => onChange({ brightnessMode: value })}
              variant="list"
            >
              <div className="flex-1">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium">{label}</span>
                  <span
                    className={`text-xs px-1.5 py-0.5 rounded font-mono ${
                      config.brightnessMode === value
                        ? "bg-primary/20 text-primary"
                        : "bg-background/50 text-muted-foreground"
                    }`}
                  >
                    {range}
                  </span>
                </div>
                <p className="text-xs text-muted-foreground mt-0.5">{desc}</p>
              </div>
            </OptionTile>
          ))}
        </div>

        <SliderControl
          label="Multiplier"
          tooltip="Scales the calculated brightness up or down. Use this to globally boost or dim the output."
          value={`${config.brightnessMultiplier.toFixed(1)}×`}
          sliderValue={Math.round((config.brightnessMultiplier - 0.1) / 9.9 * 100)}
          sliderMin={0}
          sliderMax={100}
          sliderStep={1}
          onSliderChange={(v) => onChange({ brightnessMultiplier: 0.1 + (v / 100) * 9.9 })}
          hint={{ left: "0.1× (dim)", right: "10× (amplified)" }}
        />
      </SettingsSection>

      <SettingsSection title="Output Range">
        <p className="text-xs text-muted-foreground mt-1">
          Compress the output brightness into a narrower band. Dark scenes won't go fully dark; bright scenes won't blast full brightness.
        </p>

        <SliderControl
          label="Minimum Brightness"
          tooltip="The lowest brightness your lights will reach. Raise this to prevent lights from going fully dark during dim scenes."
          value={`${Math.round(config.brightnessFloor * 100)}%`}
          sliderValue={Math.round(config.brightnessFloor * 100)}
          sliderMin={0}
          sliderMax={95}
          sliderStep={1}
          onSliderChange={(v) => {
            const floor = v / 100;
            const ceiling = config.brightnessCeiling;
            if (ceiling - floor < 0.05) return;
            onChange({ brightnessFloor: floor });
          }}
          hint={{ left: "0% (allow dark)", right: "Higher = brighter minimum" }}
        />

        <SliderControl
          label="Maximum Brightness"
          tooltip="The highest brightness your lights will reach. Lower this to prevent lights from blasting at full power during bright scenes."
          value={`${Math.round(config.brightnessCeiling * 100)}%`}
          sliderValue={Math.round(config.brightnessCeiling * 100)}
          sliderMin={5}
          sliderMax={100}
          sliderStep={1}
          onSliderChange={(v) => {
            const ceiling = v / 100;
            const floor = config.brightnessFloor;
            if (ceiling - floor < 0.05) return;
            onChange({ brightnessCeiling: ceiling });
          }}
          hint={{ left: "Lower = dimmer maximum", right: "100% (full bright)" }}
        />

        <SliderControl
          label="Uniformity"
          tooltip="How similar brightness should be across all lights. Tighter means more uniform; wider allows each light to vary independently."
          value={`±${Math.round(config.brightnessMaxDeviation * 100)}%`}
          sliderValue={Math.round(config.brightnessMaxDeviation * 100)}
          sliderMin={1}
          sliderMax={100}
          sliderStep={1}
          onSliderChange={(v) => onChange({ brightnessMaxDeviation: v / 100 })}
          hint={{ left: "±1% (very tight)", right: "±100% (unrestricted)" }}
        />

        <SliderControl
          label="Fade Speed"
          tooltip="How quickly brightness changes. Lower values give instant response; higher values produce slow, gradual fades."
          value={
            config.brightnessSmoothing === 0
              ? "Instant"
              : `~${(10 * Math.pow(0.01, config.brightnessSmoothing)).toFixed(1)}/s`
          }
          sliderValue={Math.round(config.brightnessSmoothing * 100)}
          sliderMin={0}
          sliderMax={100}
          sliderStep={1}
          onSliderChange={(v) => onChange({ brightnessSmoothing: v / 100 })}
          hint={{ left: "Instant", right: "Very slow (~10s full range)" }}
        />
      </SettingsSection>
    </div>
  );
}
