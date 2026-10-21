import {
  SettingsSection,
  SettingsLabel,
  SliderControl,
  OptionTile,
  NestedCard,
} from "./settings";
import { SpatialGridPreview } from "./SpatialGridPreview";
import { ScenePalettePreview } from "./ScenePalettePreview";
import type { ScreenSyncConfig, ExtractionMethod, MultiColorApproach, AssignmentStrategy, Device } from "@/lib/types";

interface ColorsTabProps {
  config: ScreenSyncConfig;
  devices: Device[];
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

const assignmentStrategies: {
  value: AssignmentStrategy;
  label: string;
  desc: string;
}[] = [
  {
    value: "identity_lock",
    label: "Identity Lock",
    desc: "Anchors each bulb to a colour. Only re-anchors on large, sustained shifts.",
  },
  {
    value: "flow_track",
    label: "Flow Track",
    desc: "Tracks per-bulb colour trajectories with EMA smoothing. Adapts gracefully.",
  },
  {
    value: "scene_cut_remap",
    label: "Scene Cut Remap",
    desc: "Flow-tracks normally; performs a clean global remap on detected scene cuts.",
  },
  {
    value: "zone_dominant",
    label: "Zone Dominant",
    desc: "Permanently maps bulb N to screen zone N. Fully deterministic.",
  },
];

function paletteStabilityLabel(val: number) {
  if (val >= 1.6) return "Ultra stable";
  if (val >= 1.2) return "Very stable";
  if (val >= 0.90) return "Stable+";
  if (val >= 0.70) return "Stable";
  if (val >= 0.45) return "Balanced";
  if (val >= 0.20) return "Reactive";
  return "Live";
}

export function ColorsTab({ config, devices, onChange }: ColorsTabProps) {
  const multiColor = config.colorMode === "multi";

  return (
    <div className="space-y-5">
      <SettingsSection title="Color Extraction">
        {/* Color Mode */}
        <div>
          <div className="grid grid-cols-2 gap-2">
            {[
              { value: "single" as const, label: "Single Color" },
              { value: "multi" as const, label: "Multi Color" },
            ].map(({ value, label }) => (
              <OptionTile
                key={value}
                selected={config.colorMode === value}
                onClick={() => onChange({ colorMode: value })}
                variant="grid"
              >
                <span className="text-sm font-medium">{label}</span>
              </OptionTile>
            ))}
          </div>
          <p className="text-xs text-muted-foreground mt-2">
            {config.colorMode === "single"
              ? "One color sent to all assigned lights."
              : "One color per assigned light, based on screen position or palette."}
          </p>
        </div>

        {multiColor && (
          <div>
            <SettingsLabel>Multi-Color Approach</SettingsLabel>
            <div className="grid grid-cols-2 gap-2">
              {multiApproaches.map(({ value, label, desc }) => (
                <OptionTile
                  key={value}
                  selected={config.multiColorApproach === value}
                  onClick={() => onChange({ multiColorApproach: value })}
                  variant="grid"
                >
                  <span className="text-sm font-medium">{label}</span>
                  <span className="text-xs text-muted-foreground leading-snug">{desc}</span>
                </OptionTile>
              ))}
            </div>

            {config.multiColorApproach === "spatial_grid" && config.deviceIds.length > 0 && (
              <SpatialGridPreview deviceCount={config.deviceIds.length} />
            )}

            {config.multiColorApproach === "scene_palette" && (
              <div className="mt-3 pt-3 space-y-4">
                {config.deviceIds.length > 0 && (
                  <ScenePalettePreview deviceCount={config.deviceIds.length} />
                )}
                <SliderControl
                  label="Palette Stability"
                  tooltip="Accumulates pixel data across frames before choosing colours. Higher values require a colour to consistently dominate before it enters the palette."
                  value={paletteStabilityLabel(config.paletteStability)}
                  sliderValue={Math.round(config.paletteStability * 100)}
                  sliderMin={0}
                  sliderMax={200}
                  sliderStep={5}
                  onSliderChange={(v) => onChange({ paletteStability: v / 100 })}
                  hint={{ left: "← Reactive", right: "Stable → Ultra stable" }}
                />
              </div>
            )}
          </div>
        )}

        <div>
          <SettingsLabel>Color Pick Strategy</SettingsLabel>
          <div className="space-y-2">
            {extractionMethods.map(({ value, label, desc }) => (
              <OptionTile
                key={value}
                selected={(multiColor ? config.subMethod : config.extractionMethod) === value}
                onClick={() =>
                  multiColor
                    ? onChange({ subMethod: value })
                    : onChange({ extractionMethod: value })
                }
                variant="list"
              >
                <span className="text-sm font-medium shrink-0">{label}</span>
                <span className="text-xs text-muted-foreground ml-3 text-right leading-tight">
                  {desc}
                </span>
              </OptionTile>
            ))}
          </div>
        </div>
      </SettingsSection>

      <SettingsSection title="Color Tuning">
        <SliderControl
          label="Saturation Boost"
          tooltip="Amplifies or mutes the saturation of extracted colors before sending to your lights."
          value={`${config.saturationBoost.toFixed(1)}×`}
          sliderValue={config.saturationBoost * 50}
          sliderMin={0}
          sliderMax={100}
          sliderStep={5}
          onSliderChange={(v) => onChange({ saturationBoost: v / 50 })}
          hint={{ left: "0× (muted)", right: "2× (vivid)" }}
        />

        <SliderControl
          label="White Bias"
          tooltip="Shifts color selection toward or away from whites and grays. Pull left to favor vivid colors; push right to keep whites."
          value={
            config.whiteBias === 0
              ? "Neutral"
              : config.whiteBias < 0
              ? `Colorful ${(Math.abs(config.whiteBias) * 100).toFixed(0)}%`
              : `White ${(config.whiteBias * 100).toFixed(0)}%`
          }
          sliderValue={(config.whiteBias + 1) * 50}
          sliderMin={0}
          sliderMax={100}
          sliderStep={5}
          onSliderChange={(v) => onChange({ whiteBias: v / 50 - 1 })}
          hint={{ left: "← Filter grays", right: "Prefer white →" }}
        />
      </SettingsSection>

      <SettingsSection title="Color Transitions">
        <SliderControl
          label="Color Blending"
          tooltip="How much colors blend between frames. Higher values produce smoother, slower transitions."
          value={`${Math.round(config.colorSmoothing * 100)}%`}
          sliderValue={Math.round(config.colorSmoothing * 100)}
          sliderMin={0}
          sliderMax={200}
          sliderStep={1}
          onSliderChange={(v) => onChange({ colorSmoothing: v / 100 })}
          hint={{ left: "Off (raw)", right: "Max (ultra smooth)" }}
        />

        <SliderControl
          label="Handoff Fade"
          tooltip="When a light switches to a new color slot, this adds a crossfade so the swap looks smooth instead of snapping."
          value={`${config.assignmentHandoffMs}ms`}
          sliderValue={config.assignmentHandoffMs}
          sliderMin={0}
          sliderMax={1500}
          sliderStep={25}
          onSliderChange={(v) => onChange({ assignmentHandoffMs: v })}
          hint={{ left: "Off (instant swaps)", right: "Very gradual handoff" }}
        />
      </SettingsSection>

      {multiColor && (
        <SettingsSection
          title="Bulb Color Matching"
          titleTooltip="Method for reducing color swapping between light bulbs."
        >
          <div className="grid grid-cols-2 gap-2">
            {assignmentStrategies.map(({ value, label, desc }) => (
              <OptionTile
                key={value}
                selected={config.assignmentStrategy === value}
                onClick={() => onChange({ assignmentStrategy: value })}
                variant="grid"
              >
                <span className="text-sm font-medium">{label}</span>
                <span className="text-xs text-muted-foreground leading-snug">{desc}</span>
              </OptionTile>
            ))}
          </div>

          {config.assignmentStrategy === "identity_lock" && (
            <NestedCard title="Identity Lock">
              <SliderControl
                label="Anchor sensitivity"
                tooltip="How far a colour must shift before the bulb's anchor is updated. Lower = more stable, slower to adapt. Higher = adapts faster but may swap more readily."
                value={config.identityLockBreachThreshold.toFixed(2)}
                sliderValue={config.identityLockBreachThreshold}
                sliderMin={0.1}
                sliderMax={0.8}
                sliderStep={0.05}
                onSliderChange={(v) => onChange({ identityLockBreachThreshold: v })}
                description="Lower values keep anchors stable longer. 0.30 is a good default."
              />
            </NestedCard>
          )}

          {config.assignmentStrategy === "flow_track" && (
            <NestedCard title="Flow Track">
              <SliderControl
                label="Tracking speed"
                tooltip="How quickly each bulb's colour trajectory follows the screen. Lower = smoother, slower to respond to sudden changes. Higher = reacts faster but may be less stable."
                value={config.flowTrackEmaAlpha.toFixed(2)}
                sliderValue={config.flowTrackEmaAlpha}
                sliderMin={0.05}
                sliderMax={1}
                sliderStep={0.05}
                onSliderChange={(v) => onChange({ flowTrackEmaAlpha: v })}
              />
              <SliderControl
                label="Solve interval"
                tooltip="Minimum time between full assignment solves. Lower = re-evaluates more often (reactive). Higher = less CPU, more stable between solves."
                value={`${config.flowTrackSolveIntervalMs} ms`}
                sliderValue={config.flowTrackSolveIntervalMs}
                sliderMin={16}
                sliderMax={500}
                sliderStep={16}
                onSliderChange={(v) => onChange({ flowTrackSolveIntervalMs: v })}
              />
            </NestedCard>
          )}

          {config.assignmentStrategy === "scene_cut_remap" && (
            <NestedCard title="Scene Cut Remap">
              <SliderControl
                label="Post-cut hold"
                tooltip="After a scene cut triggers a global remap, the assignment is held for this duration before resuming adaptive tracking. Prevents the tracker from immediately re-shuffling the fresh assignment."
                value={`${config.sceneCutRemapHoldMs} ms`}
                sliderValue={config.sceneCutRemapHoldMs}
                sliderMin={0}
                sliderMax={2000}
                sliderStep={100}
                onSliderChange={(v) => onChange({ sceneCutRemapHoldMs: v })}
                description="FlowTrack tracking speed also applies between cuts."
              />
              <SliderControl
                label="Tracking speed"
                tooltip="How quickly each bulb's colour trajectory follows the screen during normal (non-cut) frames."
                value={config.flowTrackEmaAlpha.toFixed(2)}
                sliderValue={config.flowTrackEmaAlpha}
                sliderMin={0.05}
                sliderMax={1}
                sliderStep={0.05}
                onSliderChange={(v) => onChange({ flowTrackEmaAlpha: v })}
              />
            </NestedCard>
          )}

          {config.assignmentStrategy === "zone_dominant" && (
            <NestedCard>
              <p className="text-xs text-muted-foreground leading-relaxed">
                Bulb N always shows the colour extracted from screen zone N. Works
                best with <strong>Spatial Grid</strong> multi-colour extraction.
                No configuration needed — the mapping is fixed for the session.
              </p>
            </NestedCard>
          )}
        </SettingsSection>
      )}
    </div>
  );
}
