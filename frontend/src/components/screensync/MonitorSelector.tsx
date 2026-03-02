import { useEffect, useState } from "react";
import { App } from "@bindings";
import type { MonitorInfo } from "@/lib/types";

interface MonitorSelectorProps {
  selected: number;
  onChange: (index: number) => void;
}

/**
 * Visual monitor layout selector showing the arrangement of all active displays.
 * The user clicks a monitor to select it as the capture source.
 */
export function MonitorSelector({ selected, onChange }: MonitorSelectorProps) {
  const [monitors, setMonitors] = useState<MonitorInfo[]>([]);

  useEffect(() => {
    App.GetMonitors()
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      .then((m: any) => setMonitors(m || []))
      .catch(() => {});
  }, []);

  if (monitors.length === 0) {
    return (
      <div className="h-16 bg-background/50 rounded-lg flex items-center justify-center text-xs text-muted-foreground">
        Detecting displays…
      </div>
    );
  }

  // Compute a bounding box of all monitors for scaling.
  const minX = Math.min(...monitors.map((m) => m.x));
  const minY = Math.min(...monitors.map((m) => m.y));
  const maxX = Math.max(...monitors.map((m) => m.x + m.width));
  const maxY = Math.max(...monitors.map((m) => m.y + m.height));
  const totalW = maxX - minX || 1;
  const totalH = maxY - minY || 1;
  const containerW = 280;
  const containerH = 90;
  const scale = Math.min(containerW / totalW, containerH / totalH) * 0.85;

  return (
    <div className="space-y-2">
      <div
        className="relative mx-auto"
        style={{ width: containerW, height: containerH }}
      >
        {monitors.map((m) => {
          const left = (m.x - minX) * scale + (containerW - totalW * scale) / 2;
          const top = (m.y - minY) * scale + (containerH - totalH * scale) / 2;
          const w = m.width * scale;
          const h = m.height * scale;
          const isSelected = m.index === selected;
          return (
            <button
              key={m.index}
              type="button"
              onClick={() => onChange(m.index)}
              className={`absolute rounded transition-all border text-xs font-semibold flex flex-col items-center justify-center gap-0.5 ${
                isSelected
                  ? "bg-primary/20 border-primary text-primary"
                  : "bg-background/40 border-border text-muted-foreground hover:bg-background/70 hover:border-primary/50"
              }`}
              style={{ left, top, width: w, height: h }}
            >
              <span>{m.index + 1}</span>
              {m.isPrimary && (
                <span className="text-[9px] opacity-70">Primary</span>
              )}
            </button>
          );
        })}
      </div>
      <p className="text-xs text-center text-muted-foreground">
        {monitors[selected]
          ? `${monitors[selected].width}×${monitors[selected].height} — Display ${selected + 1}`
          : "Select a display"}
      </p>
    </div>
  );
}
