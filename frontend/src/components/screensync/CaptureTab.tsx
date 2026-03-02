import { useState, useEffect } from "react";
import { Monitor, Crop, AppWindow, Focus, Crosshair } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { MonitorSelector } from "./MonitorSelector";
import { WindowPicker } from "./WindowPicker";
import { ColorPreview } from "./ColorPreview";
import { App } from "@bindings";
import { Events } from "@wailsio/runtime";
import type { ScreenSyncConfig } from "@/lib/types";

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

export function CaptureTab({ config, onChange, isRunning }: CaptureTabProps) {
  const [showWindowPicker, setShowWindowPicker] = useState(false);

  // Subscribe to region-selected events from the native overlay.
  useEffect(() => {
    const handler = (data: { cancelled: boolean; x: number; y: number; width: number; height: number }) => {
      if (!data.cancelled) {
        onChange({
          region: {
            x: data.x,
            y: data.y,
            width: data.width,
            height: data.height,
          },
        });
      }
    };
    const off = Events.On("screensync:region-selected", (e) => handler(e.data));
    return () => off();
  }, [onChange]);

  return (
    <div className="space-y-5">
      {/* Capture mode selector */}
      <div>
        <p className="text-xs font-medium uppercase tracking-widest text-muted-foreground mb-2.5">
          Capture Mode
        </p>
        <div className="grid grid-cols-4 gap-2">
          {captureModes.map(({ value, icon: Icon, label, desc }) => {
            const active = config.captureMode === value;
            return (
              <button
                key={value}
                type="button"
                onClick={() => onChange({ captureMode: value })}
                className={`flex flex-col items-center gap-1.5 rounded-xl py-3 px-2 transition-all ${
                  active
                    ? "bg-primary/15 text-primary ring-1 ring-primary/40"
                    : "bg-background/30 text-muted-foreground hover:bg-background/60 hover:text-foreground"
                }`}
              >
                <Icon className="h-5 w-5" />
                <span className="text-xs font-medium">{label}</span>
                <span className="text-[10px] opacity-60">{desc}</span>
              </button>
            );
          })}
        </div>
      </div>

      {/* Mode-specific configuration */}
      {config.captureMode === "monitor" && (
        <div>
          <p className="text-xs font-medium uppercase tracking-widest text-muted-foreground mb-2.5">
            Display
          </p>
          <MonitorSelector
            selected={config.monitorIndex}
            onChange={(i) => onChange({ monitorIndex: i })}
          />
        </div>
      )}

      {config.captureMode === "region" && (
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <p className="text-xs font-medium uppercase tracking-widest text-muted-foreground">
              Region
            </p>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => App.StartRegionSelect()}
            >
              <Crosshair className="h-3.5 w-3.5 mr-1" />
              Draw Region
            </Button>
          </div>
          {/* Manual coordinate inputs */}
          <div className="grid grid-cols-2 gap-2">
            {(["x", "y", "width", "height"] as const).map((field) => (
              <div key={field}>
                <label className="text-xs text-muted-foreground capitalize mb-1 block">
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
        <div className="space-y-2">
          <p className="text-xs font-medium uppercase tracking-widest text-muted-foreground mb-1">
            Window
          </p>
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

      {/* Live preview */}
      {isRunning && (
        <div>
          <p className="text-xs font-medium uppercase tracking-widest text-muted-foreground mb-2">
            Live Preview
          </p>
          <ColorPreview count={config.colorMode === "multi" ? config.deviceIds.length || 1 : 1} />
        </div>
      )}

      {/* Pixel sampling note */}
      <div className="text-xs text-muted-foreground bg-background/20 rounded-lg px-3 py-2">
        Frames are downsampled to 25% before extraction for performance.
      </div>
    </div>
  );
}
