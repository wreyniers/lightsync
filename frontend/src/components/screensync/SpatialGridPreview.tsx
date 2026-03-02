import { useEffect, useState } from "react";
import { Events } from "@wailsio/runtime";
import type { Color } from "@/lib/types";

interface SpatialGridPreviewProps {
  /** Number of devices (= number of zones). */
  deviceCount: number;
}

/** Mirror of the Go gridDims function in extract/spatial.go */
function gridDims(n: number): [cols: number, rows: number] {
  if (n <= 1) return [1, 1];
  if (n <= 2) return [2, 1];
  if (n <= 4) return [2, 2];
  if (n <= 6) return [3, 2];
  if (n <= 9) return [3, 3];
  return [4, Math.ceil(n / 4)];
}

/**
 * Shows a miniature representation of the capture area divided into the same
 * spatial grid used by the Go SpatialExtractor. Each zone is numbered to match
 * its device slot. When the engine is running, zones are live-tinted with the
 * extracted colors received from the screensync:colors event.
 */
export function SpatialGridPreview({ deviceCount }: SpatialGridPreviewProps) {
  const n = Math.max(1, deviceCount);
  const [cols, rows] = gridDims(n);
  const [colors, setColors] = useState<Color[]>([]);

  useEffect(() => {
    const off = Events.On("screensync:colors", (e) => {
      const incoming = e.data as Color[];
      if (Array.isArray(incoming)) setColors(incoming.slice(0, n));
    });
    return () => { off?.(); };
  }, [n]);

  // Build zone list in row-major order, capped at n.
  const zones: number[] = [];
  for (let r = 0; r < rows; r++) {
    for (let c = 0; c < cols; c++) {
      const idx = r * cols + c;
      if (idx < n) zones.push(idx);
    }
  }

  return (
    <div className="space-y-1.5">
      <p className="text-[10px] text-muted-foreground">
        Zone layout — {n} device{n !== 1 ? "s" : ""} ({cols}×{rows} grid)
      </p>

      {/* 16:9 monitor shell */}
      <div className="relative w-full rounded-lg overflow-hidden border border-white/10 bg-black/40"
        style={{ aspectRatio: "16/9" }}
      >
        <div
          className="absolute inset-0"
          style={{
            display: "grid",
            gridTemplateColumns: `repeat(${cols}, 1fr)`,
            gridTemplateRows: `repeat(${rows}, 1fr)`,
          }}
        >
          {zones.map((zoneIdx) => {
            const color = colors[zoneIdx];
            const bg = color
              ? `hsla(${color.h}, ${Math.round(color.s * 100)}%, ${Math.round(color.b * 50)}%, 0.65)`
              : "rgba(255,255,255,0.04)";
            const isLive = !!color;

            return (
              <div
                key={zoneIdx}
                className="relative flex items-center justify-center border border-white/10 transition-colors duration-300"
                style={{ background: bg }}
              >
                {/* Zone number badge */}
                <div className={`flex flex-col items-center gap-0.5 select-none ${isLive ? "opacity-90" : "opacity-40"}`}>
                  <span className="text-[10px] font-semibold text-white drop-shadow-sm leading-none">
                    {zoneIdx + 1}
                  </span>
                  {isLive && (
                    <span className="text-[8px] text-white/70 leading-none font-mono">
                      {Math.round(color.h)}°
                    </span>
                  )}
                </div>

                {/* Corner pulse when live */}
                {isLive && (
                  <span className="absolute top-1 right-1 h-1 w-1 rounded-full bg-white/50" />
                )}
              </div>
            );
          })}
        </div>

        {/* "Live" badge — only shown when colors are flowing */}
        {colors.length > 0 && (
          <div className="absolute bottom-1.5 right-2 flex items-center gap-1 text-[9px] text-white/50">
            <span className="h-1 w-1 rounded-full bg-success animate-pulse" />
            Live
          </div>
        )}
      </div>

      <p className="text-[10px] text-muted-foreground/60">
        Zone N always maps to the Nth device in your device list. No reassignment occurs.
      </p>
    </div>
  );
}
