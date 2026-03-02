import { useState } from "react";
import { Monitor, Palette, Sun, Zap, Lightbulb } from "lucide-react";
import { CaptureTab } from "./CaptureTab";
import { ColorsTab } from "./ColorsTab";
import { BrightnessTab } from "./BrightnessTab";
import { TransitionsTab } from "./TransitionsTab";
import { DevicesTab } from "./DevicesTab";
import type { Device, ScreenSyncConfig } from "@/lib/types";

type TabId = "capture" | "colors" | "brightness" | "speed" | "devices";

const tabs: { id: TabId; icon: typeof Monitor; label: string }[] = [
  { id: "capture",     icon: Monitor,    label: "Capture" },
  { id: "colors",      icon: Palette,    label: "Colors" },
  { id: "brightness",  icon: Sun,        label: "Brightness" },
  { id: "speed",       icon: Zap,        label: "Speed" },
  { id: "devices",     icon: Lightbulb,  label: "Devices" },
];

interface ScreenSyncEditorProps {
  config: ScreenSyncConfig;
  devices: Device[];
  isRunning: boolean;
  /** True when unsaved config changes are being live-applied to the running engine. */
  isLive?: boolean;
  onChange: (patch: Partial<ScreenSyncConfig>) => void;
}

/**
 * Tabbed configuration editor for a Screen Sync scene.
 * Rendered inside the scene editor when trigger === "screen_sync".
 */
export function ScreenSyncEditor({
  config,
  devices,
  isRunning,
  isLive,
  onChange,
}: ScreenSyncEditorProps) {
  const [activeTab, setActiveTab] = useState<TabId>("capture");

  return (
    <div className="space-y-0">
      {/* Live-applying indicator */}
      {isRunning && isLive && (
        <div className="flex items-center gap-2 text-xs text-success mb-2">
          <span className="h-1.5 w-1.5 rounded-full bg-success animate-pulse" />
          Changes applying live
        </div>
      )}
      {isRunning && !isLive && (
        <div className="flex items-center gap-2 text-xs text-muted-foreground mb-2">
          <span className="h-1.5 w-1.5 rounded-full bg-success" />
          Running
        </div>
      )}

      {/* Tab navigation */}
      <div className="flex gap-0.5 bg-background/30 rounded-xl p-1">
        {tabs.map(({ id, icon: Icon, label }) => (
          <button
            key={id}
            type="button"
            onClick={() => setActiveTab(id)}
            className={`flex-1 flex flex-col items-center gap-1 py-2 px-1 rounded-lg text-center transition-all ${
              activeTab === id
                ? "bg-primary/15 text-primary"
                : "text-muted-foreground hover:text-foreground hover:bg-white/5"
            }`}
          >
            <Icon className="h-4 w-4" />
            <span className="text-[11px] font-medium">{label}</span>
          </button>
        ))}
      </div>

      {/* Tab content */}
      <div className="pt-4">
        {activeTab === "capture" && (
          <CaptureTab config={config} onChange={onChange} isRunning={isRunning} />
        )}
        {activeTab === "colors" && (
          <ColorsTab config={config} onChange={onChange} />
        )}
        {activeTab === "brightness" && (
          <BrightnessTab config={config} onChange={onChange} />
        )}
        {activeTab === "speed" && (
          <TransitionsTab config={config} onChange={onChange} />
        )}
        {activeTab === "devices" && (
          <DevicesTab config={config} devices={devices} onChange={onChange} />
        )}
      </div>
    </div>
  );
}
