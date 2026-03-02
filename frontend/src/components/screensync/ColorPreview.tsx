import { useEffect, useState } from "react";
import { Events } from "@wailsio/runtime";
import type { Color } from "@/lib/types";

interface ColorPreviewProps {
  /** Number of swatches to show; 1 for single-color, N for multi. */
  count?: number;
  className?: string;
}

/**
 * Displays real-time color swatches extracted from the screen by the engine.
 * Subscribes to the "screensync:colors" event emitted by the Go backend.
 */
export function ColorPreview({ count = 1, className }: ColorPreviewProps) {
  const [colors, setColors] = useState<Color[]>([]);

  useEffect(() => {
    const handler = (incoming: Color[]) => {
      if (Array.isArray(incoming)) {
        setColors(incoming.slice(0, Math.min(count, incoming.length)));
      }
    };
    const off = Events.On("screensync:colors", (e) => handler(e.data));
    return () => { off?.(); };
  }, [count]);

  if (colors.length === 0) {
    return (
      <div className={`flex gap-2 ${className ?? ""}`}>
        {Array.from({ length: count }).map((_, i) => (
          <div
            key={i}
            className="h-8 w-8 rounded-full bg-white/10 ring-1 ring-border animate-pulse"
          />
        ))}
      </div>
    );
  }

  return (
    <div className={`flex gap-2 items-center ${className ?? ""}`}>
      {colors.map((c, i) => {
        const bg = `hsl(${c.h}, ${Math.round(c.s * 100)}%, ${Math.round(c.b * 50)}%)`;
        const pct = Math.round(c.b * 100);
        return (
          <div
            key={i}
            className="h-8 w-8 rounded-full ring-1 ring-white/20 shadow transition-colors duration-300 relative flex items-center justify-center"
            style={{ background: bg }}
            title={`H:${Math.round(c.h)}° S:${Math.round(c.s * 100)}% B:${pct}%`}
          >
            <span className="text-[10px] font-medium text-white drop-shadow-sm">{pct}%</span>
          </div>
        );
      })}
      <span className="text-xs text-muted-foreground ml-1">Live</span>
    </div>
  );
}
