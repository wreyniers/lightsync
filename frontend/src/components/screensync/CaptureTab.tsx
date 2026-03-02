import { useState, useEffect } from "react";
import { Monitor, Crop, AppWindow, Focus, Crosshair } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { Toggle } from "@/components/ui/Toggle";
import { Tooltip } from "@/components/ui/Tooltip";
import { SettingsSection, SettingsLabel, SliderControl, OptionTile } from "./settings";
import { MonitorSelector } from "./MonitorSelector";
import { WindowPicker } from "./WindowPicker";
import { ColorPreview } from "./ColorPreview";
import { App } from "@bindings";
import { Events } from "@wailsio/runtime";
import type { ScreenSyncConfig, SpeedPreset } from "@/lib/types";

interface CaptureTabProps {
  config: ScreenSyncConfig;
  onChange: (patch: Partial<ScreenSyncConfig>) => void;
  isRunning: boolean;
}

const captureModes = [
  { value: "monitor" as const, icon: Monitor, label: "Monitor", desc: "Full display" },
  { value: "region" as const, icon: Crop, label: "Region", desc: "Custom area" },
  { value: "window" as const, icon: AppWindow, label: "Window", desc: "App window" },
  { value: "active_window" as const, icon: Focus, label: "Active", desc: "Focused app" },
];

const speedPresets: { value: SpeedPreset; label: string; fps: number; desc: string }[] = [
  { value: "very_slow", label: "Very Slow", fps: 5, desc: "5 FPS · gentle ambient" },
  { value: "slow", label: "Slow", fps: 10, desc: "10 FPS · relaxed mood" },
  { value: "medium", label: "Medium", fps: 20, desc: "20 FPS · balanced default" },
  { value: "fast", label: "Fast", fps: 30, desc: "30 FPS · gaming / action" },
  { value: "realtime", label: "Realtime", fps: 60, desc: "60 FPS · minimal latency" },
];

export function CaptureTab({ config, onChange, isRunning }: CaptureTabProps) {
  const [showWindowPicker, setShowWindowPicker] = useState(false);

  useEffect(() => {
    const handler = (data: { cancelled: boolean; x: number; y: number; width: number; height: number }) => {
      if (!data.cancelled) {
        onChange({
          region: { x: data.x, y: data.y, width: data.width, height: data.height },
        });
      }
    };
    const off = Events.On("screensync:region-selected", (e) => handler(e.data));
    return () => off();
  }, [onChange]);

  return (
    <div className="space-y-5">
      <SettingsSection title="Capture Source">
        <div className="grid grid-cols-4 gap-2">
          {captureModes.map(({ value, icon: Icon, label, desc }) => (
            <OptionTile
              key={value}
              selected={config.captureMode === value}
              onClick={() => onChange({ captureMode: value })}
              variant="grid"
              className="items-center"
            >
              <Icon className="h-5 w-5 mb-0.5" />
              <span className="text-sm font-medium">{label}</span>
              <span className="text-xs text-muted-foreground leading-snug">{desc}</span>
            </OptionTile>
          ))}
        </div>

        {config.captureMode === "monitor" && (
          <div>
            <SettingsLabel>Display</SettingsLabel>
            <MonitorSelector
              selected={config.monitorIndex}
              onChange={(i) => onChange({ monitorIndex: i })}
            />
          </div>
        )}

        {config.captureMode === "region" && (
          <div className="space-y-3">
            <div className="flex items-center justify-between mb-2">
              <p className="text-xs text-muted-foreground">Region</p>
              <Button variant="secondary" size="sm" onClick={() => App.StartRegionSelect()}>
                <Crosshair className="h-3.5 w-3.5 mr-1" />
                Draw Region
              </Button>
            </div>
            <div className="grid grid-cols-2 gap-2">
              {(["x", "y", "width", "height"] as const).map((field) => (
                <div key={field}>
                  <label className="text-xs text-muted-foreground capitalize mb-2 block">
                    {field === "width" ? "W" : field === "height" ? "H" : field.toUpperCase()}
                  </label>
                  <input
                    type="number"
                    value={config.region[field]}
                    onChange={(e) =>
                      onChange({ region: { ...config.region, [field]: Number(e.target.value) } })
                    }
                    className="w-full bg-background/50 border border-border rounded-lg px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-primary"
                  />
                </div>
              ))}
            </div>
            <p className="text-xs text-muted-foreground">
              {config.region.width}×{config.region.height} at ({config.region.x}, {config.region.y})
            </p>
          </div>
        )}

        {config.captureMode === "window" && (
          <div>
            <SettingsLabel>Window</SettingsLabel>
            <button
              type="button"
              onClick={() => setShowWindowPicker(true)}
              className="w-full flex items-center gap-3 px-4 py-3 rounded-xl bg-background/40 border border-border hover:border-primary/50 transition-colors text-left"
            >
              <AppWindow className="h-4 w-4 text-muted-foreground shrink-0" />
              <div className="flex-1 min-w-0">
                {config.windowTitle ? (
                  <>
                    <p className="text-sm font-medium truncate">{config.windowTitle}</p>
                    <p className="text-xs text-muted-foreground">Click to change</p>
                  </>
                ) : (
                  <p className="text-sm text-muted-foreground">Click to select a window…</p>
                )}
              </div>
            </button>
            {showWindowPicker && (
              <WindowPicker
                selectedHwnd={config.windowHwnd}
                selectedTitle={config.windowTitle}
                onSelect={(hwnd, title) => onChange({ windowHwnd: hwnd, windowTitle: title })}
                onClose={() => setShowWindowPicker(false)}
              />
            )}
          </div>
        )}

        {config.captureMode === "active_window" && (
          <div className="rounded-xl bg-background/30 px-4 py-3 text-sm text-muted-foreground">
            <Focus className="h-4 w-4 inline mr-2 text-primary" />
            Captures the currently focused application window automatically.
            Focus changes are detected every 500 ms.
          </div>
        )}
      </SettingsSection>

      <SettingsSection title="Capture Speed">
        <div className="grid grid-cols-5 gap-1.5">
          {speedPresets.map(({ value, label, fps, desc }) => (
            <Tooltip key={value} content={desc} side="bottom">
              <OptionTile
                selected={config.speedPreset === value}
                onClick={() => onChange({ speedPreset: value })}
                variant="grid"
                className="items-center w-full"
              >
                <span className="text-sm font-medium">{label}</span>
                <span className="text-xs text-muted-foreground leading-snug">{fps} FPS</span>
              </OptionTile>
            </Tooltip>
          ))}
        </div>
        <p className="text-xs text-muted-foreground">
          {speedPresets.find((p) => p.value === config.speedPreset)?.desc}
        </p>
      </SettingsSection>

      <SettingsSection
        header={
          <div className="flex items-center justify-between">
            <p className="text-xs font-medium uppercase tracking-widest text-muted-foreground">
              Scene Cut Detection
            </p>
            <Toggle
              checked={config.sceneCutMode !== "off"}
              onChange={(checked) => onChange({ sceneCutMode: checked ? "on" : "off" })}
            />
          </div>
        }
      >
        <div className={config.sceneCutMode === "off" ? "opacity-50 pointer-events-none" : ""}>
          <SliderControl
            label="Sensitivity"
            value={`${Math.round(config.sceneCutSensitivity * 100)}%`}
            sliderValue={Math.round(config.sceneCutSensitivity * 100)}
            sliderMin={0}
            sliderMax={100}
            sliderStep={1}
            onSliderChange={(v) => onChange({ sceneCutSensitivity: v / 100 })}
            hint={{ left: "Very conservative", right: "Very sensitive" }}
          />
        </div>
        <p className="text-xs text-muted-foreground">
          {config.sceneCutMode === "off"
            ? "Disabled. Temporal smoothing will not reset on cuts."
            : "When a scene cut is detected, smoothing resets instantly so lights snap to the new content."}
        </p>
      </SettingsSection>

      {isRunning && (
        <SettingsSection title="Live Preview">
          <ColorPreview count={config.colorMode === "multi" ? config.deviceIds.length || 1 : 1} />
        </SettingsSection>
      )}

      <div className="text-xs text-muted-foreground bg-muted rounded-xl px-4 py-3">
        Frames are downsampled to 25% before extraction for performance.
      </div>
    </div>
  );
}
