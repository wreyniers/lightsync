import { useState } from "react";
import { Monitor, Palette, Sun, Lightbulb, Play, Square } from "lucide-react";
import { CaptureTab } from "./CaptureTab";
import { ColorsTab } from "./ColorsTab";
import { BrightnessTab } from "./BrightnessTab";
import { DevicesTab } from "./DevicesTab";
import type { Device, ScreenSyncConfig } from "@/lib/types";

type TabId = "capture" | "colors" | "brightness" | "devices";

const tabs: { id: TabId; icon: typeof Monitor; label: string }[] = [
  { id: "capture",     icon: Monitor,    label: "Capture" },
  { id: "colors",      icon: Palette,    label: "Colors" },
  { id: "brightness",  icon: Sun,        label: "Brightness" },
  { id: "devices",     icon: Lightbulb,  label: "Lights" },
];

interface ScreenSyncEditorProps {
  config: ScreenSyncConfig;
  devices: Device[];
  isRunning: boolean;
  onChange: (patch: Partial<ScreenSyncConfig>) => void;
  onPlay?: () => void;
  onStop?: () => void;
  canPlay?: boolean;
}

/**
 * Tabbed configuration editor for a Screen Sync scene.
 * Rendered inside the scene editor when trigger === "screen_sync".
 */
export function ScreenSyncEditor({
  config,
  devices,
  isRunning,
  onChange,
  onPlay,
  onStop,
  canPlay = true,
}: ScreenSyncEditorProps) {
  const [activeTab, setActiveTab] = useState<TabId>("capture");

  return (
    <div className="space-y-4">
      {/* Header — title, running indicator, play/stop */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <p className="text-xs font-medium uppercase tracking-widest text-muted-foreground">
            Screen Sync
          </p>
          {isRunning && (
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <span className="h-1.5 w-1.5 rounded-full bg-success animate-pulse" />
              Running
            </div>
          )}
        </div>
        <div className="flex items-center gap-1">
          {isRunning && onStop ? (
            <button
              type="button"
              onClick={onStop}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium text-destructive hover:bg-destructive/10 transition-colors"
            >
              <Square className="h-3.5 w-3.5" />
              Stop
            </button>
          ) : canPlay && onPlay ? (
            <button
              type="button"
              onClick={onPlay}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium text-foreground hover:bg-muted transition-colors"
            >
              <Play className="h-3.5 w-3.5" />
              Play
            </button>
          ) : null}
        </div>
      </div>

      <div className="flex gap-4">
        {/* Sidebar nav — vertical list */}
        <nav className="shrink-0 w-32 space-y-0.5">
        {tabs.map(({ id, icon: Icon, label }) => (
          <button
            key={id}
            type="button"
            onClick={() => setActiveTab(id)}
            className={`w-full flex items-center gap-2.5 px-3 py-2.5 rounded-xl text-left text-sm transition-colors ${
              activeTab === id
                ? "bg-muted text-foreground font-medium"
                : "text-foreground/45 hover:text-foreground/70 hover:bg-muted/30"
            }`}
          >
            <Icon className="h-4 w-4 shrink-0" />
            <span>{label}</span>
          </button>
        ))}
        </nav>

        {/* Content area */}
        <div className="min-w-0 flex-1">
        {activeTab === "capture" && (
          <CaptureTab config={config} onChange={onChange} isRunning={isRunning} />
        )}
        {activeTab === "colors" && (
          <ColorsTab config={config} devices={devices} onChange={onChange} />
        )}
        {activeTab === "brightness" && (
          <BrightnessTab config={config} onChange={onChange} />
        )}
        {activeTab === "devices" && (
          <DevicesTab config={config} devices={devices} onChange={onChange} />
        )}
        </div>
      </div>
    </div>
  );
}
