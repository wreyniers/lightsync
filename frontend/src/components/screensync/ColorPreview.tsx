import { useEffect, useState } from "react";
import { Events } from "@wailsio/runtime";
import type { Color } from "@/lib/types";

interface ColorPreviewProps {
  /** Number of swatches to show; 1 for single-color, N for multi. */
  count?: number;
  className?: string;
}

// Same perceptual-alignment thresholds as ScenePalettePreview.
const ALIGN_HUE_DEG = 3;
const ALIGN_SAT = 0.03;
const ALIGN_BRIGHT = 0.03;

function isAligned(a: Color | undefined, b: Color | undefined): boolean {
  if (!a || !b) return false;
  let dh = Math.abs(a.h - b.h);
  if (dh > 180) dh = 360 - dh;
  return (
    dh <= ALIGN_HUE_DEG &&
    Math.abs(a.s - b.s) <= ALIGN_SAT &&
    Math.abs(a.b - b.b) <= ALIGN_BRIGHT
  );
}

/**
 * Circular swatches showing, per slot:
 *   left half  = active colour (what was just sent to the bulb)
 *   right half = target colour (what the handoff ramp is driving toward)
 *
 * A white outer ring appears when the two halves are perceptually aligned.
 */
export function ColorPreview({ count = 1, className }: ColorPreviewProps) {
  const [active, setActive] = useState<Color[]>([]);
  const [target, setTarget] = useState<Color[]>([]);

  useEffect(() => {
    const take = (setter: (c: Color[]) => void) => (e: { data: unknown }) => {
      const incoming = e.data as Color[];
      if (Array.isArray(incoming)) {
        setter(incoming.slice(0, Math.min(count, incoming.length)));
      }
    };
    const offA = Events.On("screensync:colors", take(setActive));
    const offT = Events.On("screensync:targetColors", take(setTarget));
    return () => {
      offA?.();
      offT?.();
    };
  }, [count]);

  const slots = Math.max(count, active.length, target.length);
  const toCss = (c: Color) =>
    `hsl(${c.h}, ${Math.round(c.s * 100)}%, ${Math.round(c.b * 50)}%)`;

  if (slots === 0 || (active.length === 0 && target.length === 0)) {
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
      {Array.from({ length: slots }).map((_, i) => {
        const a = active[i];
        const t = target[i];
        const aligned = isAligned(a, t);
        const pct = a ? Math.round(a.b * 100) : t ? Math.round(t.b * 100) : 0;

        return (
          <div
            key={i}
            className={`relative h-8 w-8 rounded-full overflow-hidden shadow transition-shadow ${
              aligned
                ? "ring-2 ring-white/80"
                : "ring-1 ring-white/20"
            }`}
            title={
              `Slot ${i + 1}\n` +
              `  active:  ${a ? `H:${Math.round(a.h)}° S:${Math.round(a.s * 100)}% B:${Math.round(a.b * 100)}%` : "—"}\n` +
              `  target:  ${t ? `H:${Math.round(t.h)}° S:${Math.round(t.s * 100)}% B:${Math.round(t.b * 100)}%` : "—"}` +
              (aligned ? "\n  ✓ aligned" : "")
            }
          >
            <div
              className="absolute inset-y-0 left-0 w-1/2 transition-colors duration-300"
              style={{ background: a ? toCss(a) : "rgba(255,255,255,0.06)" }}
            />
            <div
              className="absolute inset-y-0 right-0 w-1/2 transition-colors duration-300"
              style={{ background: t ? toCss(t) : "rgba(255,255,255,0.06)" }}
            />
            {!aligned && (a || t) && (
              <span className="absolute inset-y-0 left-1/2 -translate-x-1/2 w-px bg-white/20 pointer-events-none" />
            )}
            <span className="absolute inset-0 flex items-center justify-center text-[10px] font-medium text-white drop-shadow-sm pointer-events-none">
              {pct}%
            </span>
          </div>
        );
      })}
      <span className="text-xs text-muted-foreground ml-1">Live</span>
    </div>
  );
}
