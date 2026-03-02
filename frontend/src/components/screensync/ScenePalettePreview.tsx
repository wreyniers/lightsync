import { useEffect, useState } from "react";
import { Events } from "@wailsio/runtime";
import type { Color } from "@/lib/types";

interface ScenePalettePreviewProps {
  /** Number of devices (= number of palette slots). */
  deviceCount: number;
}

/**
 * Shows the top N palette colors extracted from the full frame when using
 * Scene Palette mode. Subscribes to screensync:colors for live updates.
 */
export function ScenePalettePreview({ deviceCount }: ScenePalettePreviewProps) {
  const n = Math.max(1, deviceCount);
  const [colors, setColors] = useState<Color[]>([]);

  useEffect(() => {
    const off = Events.On("screensync:colors", (e) => {
      const incoming = e.data as Color[];
      if (Array.isArray(incoming)) setColors(incoming.slice(0, n));
    });
    return () => { off?.(); };
  }, [n]);

  return (
    <div className="space-y-2">
      <p className="text-xs text-muted-foreground">
        Palette — {n} color{n !== 1 ? "s" : ""}
      </p>

      <div className="flex flex-wrap gap-2 items-center">
        {Array.from({ length: n }).map((_, i) => {
          const color = colors[i];
          const bg = color
            ? `hsla(${color.h}, ${Math.round(color.s * 100)}%, ${Math.round(color.b * 50)}%, 0.8)`
            : "rgba(255,255,255,0.06)";
          const isLive = !!color;

          return (
            <div
              key={i}
              className="h-7 w-7 rounded-lg border border-white/10 transition-colors duration-300 flex items-center justify-center shrink-0"
              style={{ background: bg }}
              title={
                color
                  ? `Slot ${i + 1}: H:${Math.round(color.h)}° S:${Math.round(color.s * 100)}% B:${Math.round(color.b * 100)}%`
                  : `Slot ${i + 1}`
              }
            >
              {isLive && (
                <span className="text-[8px] font-medium text-white/90 drop-shadow-sm">
                  {i + 1}
                </span>
              )}
            </div>
          );
        })}
        {colors.length > 0 && (
          <span className="text-xs text-muted-foreground ml-1 flex items-center gap-1">
            <span className="h-1 w-1 rounded-full bg-success animate-pulse" />
            Live
          </span>
        )}
      </div>

      <p className="text-xs text-muted-foreground/80">
        Top N dominant colors from the full frame, assigned to your lights.
      </p>
    </div>
  );
}
