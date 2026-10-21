import { useEffect, useState } from "react";
import { Events } from "@wailsio/runtime";
import type { Color } from "@/lib/types";

interface ScenePalettePreviewProps {
  /** Number of devices (= number of palette slots). */
  deviceCount: number;
}

// Alignment epsilons — below these the user can't meaningfully distinguish
// the two halves, so treat the pair as "settled".
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
 * Preview swatch for Scene Palette mode.
 *
 * Each slot renders as two halves:
 *   left  = active colour (post-handoff, i.e. what was just sent to the bulb)
 *   right = target colour (pre-handoff, i.e. what the handoff ramp is driving toward)
 *
 * When the two halves match within a small perceptual epsilon, a solid ring
 * is drawn around the swatch to indicate the output has settled on target.
 */
export function ScenePalettePreview({ deviceCount }: ScenePalettePreviewProps) {
  const n = Math.max(1, deviceCount);
  const [active, setActive] = useState<Color[]>([]);
  const [target, setTarget] = useState<Color[]>([]);

  useEffect(() => {
    const offA = Events.On("screensync:colors", (e) => {
      const incoming = e.data as Color[];
      if (Array.isArray(incoming)) setActive(incoming.slice(0, n));
    });
    const offT = Events.On("screensync:targetColors", (e) => {
      const incoming = e.data as Color[];
      if (Array.isArray(incoming)) setTarget(incoming.slice(0, n));
    });
    return () => {
      offA?.();
      offT?.();
    };
  }, [n]);

  const toCss = (c: Color) =>
    `hsla(${c.h}, ${Math.round(c.s * 100)}%, ${Math.round(c.b * 50)}%, 0.9)`;
  const placeholder = "rgba(255,255,255,0.06)";

  const live = active.length > 0 || target.length > 0;

  return (
    <div className="space-y-2">
      <p className="text-xs text-muted-foreground">
        Palette — {n} color{n !== 1 ? "s" : ""}
      </p>

      <div className="flex flex-wrap gap-2 items-center">
        {Array.from({ length: n }).map((_, i) => {
          const a = active[i];
          const t = target[i];
          const aBg = a ? toCss(a) : placeholder;
          const tBg = t ? toCss(t) : placeholder;
          const aligned = isAligned(a, t);
          const hasLive = !!a || !!t;

          return (
            <div
              key={i}
              className={`relative h-7 w-7 rounded-lg overflow-hidden shrink-0 ${
                aligned
                  ? "ring-2 ring-white/70 shadow-[0_0_0_1px_rgba(255,255,255,0.25)]"
                  : "ring-1 ring-white/10"
              }`}
              title={
                hasLive
                  ? `Slot ${i + 1}\n` +
                    `  active:  ${a ? `H:${Math.round(a.h)}° S:${Math.round(a.s * 100)}% B:${Math.round(a.b * 100)}%` : "—"}\n` +
                    `  target:  ${t ? `H:${Math.round(t.h)}° S:${Math.round(t.s * 100)}% B:${Math.round(t.b * 100)}%` : "—"}` +
                    (aligned ? "\n  ✓ aligned" : "")
                  : `Slot ${i + 1}`
              }
            >
              <div
                className="absolute inset-y-0 left-0 w-1/2 transition-colors duration-300"
                style={{ background: aBg }}
              />
              <div
                className="absolute inset-y-0 right-0 w-1/2 transition-colors duration-300"
                style={{ background: tBg }}
              />
              {/* Subtle centre divider when the two halves disagree. */}
              {!aligned && hasLive && (
                <span className="absolute inset-y-0 left-1/2 -translate-x-1/2 w-px bg-white/20 pointer-events-none" />
              )}
              {hasLive && (
                <span className="absolute inset-0 flex items-center justify-center text-[8px] font-medium text-white/90 drop-shadow-sm pointer-events-none">
                  {i + 1}
                </span>
              )}
            </div>
          );
        })}
        {live && (
          <span className="text-xs text-muted-foreground ml-1 flex items-center gap-1">
            <span className="h-1 w-1 rounded-full bg-success animate-pulse" />
            Live
          </span>
        )}
      </div>

      <p className="text-xs text-muted-foreground/80">
        Left half: colour just sent to the bulb. Right half: target the handoff
        ramp is driving toward. A white ring appears when they align.
      </p>
    </div>
  );
}
