import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";
import type { LightMode, Color } from "@/lib/types";
import { DEFAULT_KELVIN } from "@/lib/types";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

/** Determine which mode (color / kelvin) a device should display. */
export function resolveMode(
  deviceId: string,
  overrides: Record<string, LightMode>,
  color: Record<string, { h: number; s: number; b: number }>,
  kelvin: Record<string, number>
): LightMode {
  if (overrides[deviceId]) return overrides[deviceId];
  if (color[deviceId]) return "color";
  if (kelvin[deviceId]) return "kelvin";
  return "color";
}

function hsbToRGB(h: number, s: number, b: number): [number, number, number] {
  if (s === 0) {
    const v = Math.round(b * 255);
    return [v, v, v];
  }
  const hh = h / 60;
  const i = Math.floor(hh);
  const ff = hh - i;
  const p = b * (1 - s);
  const q = b * (1 - s * ff);
  const t = b * (1 - s * (1 - ff));
  let r: number, g: number, bl: number;
  switch (i) {
    case 0: r = b; g = t; bl = p; break;
    case 1: r = q; g = b; bl = p; break;
    case 2: r = p; g = b; bl = t; break;
    case 3: r = p; g = q; bl = b; break;
    case 4: r = t; g = p; bl = b; break;
    default: r = b; g = p; bl = q; break;
  }
  return [Math.round(r * 255), Math.round(g * 255), Math.round(bl * 255)];
}

export function hsbToCSS(h: number, s: number, b: number): string {
  const [r, g, bl] = hsbToRGB(h, s, b);
  return `rgb(${r}, ${g}, ${bl})`;
}

/**
 * Approximates the correlated color temperature (K) for a given hue angle.
 *
 * Method: HSV(hue, 1, 1) → linear sRGB → CIE XYZ → xy chromaticity →
 * McCamy (1992) CCT approximation.  McCamy's formula is accurate for colors
 * near the Planckian locus (reds, oranges, warm whites, cool blues).  For
 * off-locus hues (greens, magentas) it can produce extreme values; those are
 * replaced by a perceptual cosine fallback that maps the warm-cool axis of the
 * colour wheel (red ≈ 2000 K … cyan ≈ 9000 K) smoothly.
 */
export function hueToKelvin(hue: number): number {
  // --- HSV (s=1, v=1) → sRGB ---
  const h6 = hue / 60;
  const i = Math.floor(h6) % 6;
  const f = h6 - Math.floor(h6);
  // p=0, q=1-f, t=f  (since s=1, v=1)
  const vals: [number, number, number][] = [
    [1, f, 0],   // case 0
    [1 - f, 1, 0], // case 1
    [0, 1, f],   // case 2
    [0, 1 - f, 1], // case 3
    [f, 0, 1],   // case 4
    [1, 0, 1 - f], // case 5
  ];
  const [rs, gs, bs] = vals[i];

  // sRGB gamma → linear light
  const lin = (c: number) =>
    c <= 0.04045 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4);
  const rl = lin(rs), gl = lin(gs), bl = lin(bs);

  // linear sRGB → CIE XYZ (D65 reference white)
  const X = 0.4124564 * rl + 0.3575761 * gl + 0.1804375 * bl;
  const Y = 0.2126729 * rl + 0.7151522 * gl + 0.0721750 * bl;
  const Z = 0.0193339 * rl + 0.1191920 * gl + 0.9503041 * bl;

  const sum = X + Y + Z;
  if (sum < 1e-6) return DEFAULT_KELVIN;

  const xc = X / sum;
  const yc = Y / sum;

  // McCamy (1992) CCT from xy chromaticity
  const n = (xc - 0.332) / (0.1858 - yc);
  const cct = 449 * n * n * n + 3525 * n * n + 6823.3 * n + 5520.33;

  // McCamy is only reliable near the Planckian locus.
  // Fall back to a cosine mapping on the perceptual warm-cool axis when the
  // result lands outside the practical lighting range [1800 K, 12000 K].
  if (cct < 1800 || cct > 12000 || !isFinite(cct)) {
    // cos(0°)=1 at red (warmest), cos(180°)=-1 at cyan (coolest)
    const warmth = Math.cos((hue * Math.PI) / 180);
    return Math.round(2000 + ((1 - warmth) / 2) * 7000);
  }

  return Math.round(Math.max(2000, Math.min(9000, cct)));
}

/**
 * Converts Kelvin temperature to RGB via black-body radiation approximation.
 * Shared by kelvinToHSB and kelvinToCSS.
 */
function kelvinToRGB(k: number): [number, number, number] {
  const t = k / 100;
  let r: number, g: number, b: number;

  if (t <= 66) {
    r = 255;
    g = 99.4708025861 * Math.log(t) - 161.1195681661;
    b = t <= 19 ? 0 : 138.5177312231 * Math.log(t - 10) - 305.0447927307;
  } else {
    r = 329.698727446 * Math.pow(t - 60, -0.1332047592);
    g = 288.1221695283 * Math.pow(t - 60, -0.0755148492);
    b = 255;
  }

  const clamp = (v: number) => Math.max(0, Math.min(255, Math.round(v)));
  return [clamp(r), clamp(g), clamp(b)];
}

/**
 * Converts a Kelvin temperature to an approximate HSB colour (s=0 for whites,
 * slight saturation for warm/cool tints). Used to preview temperature on
 * RGB-only lights that don't support native kelvin mode.
 */
export function kelvinToHSB(k: number): { h: number; s: number; b: number } {
  const [r, g, b] = kelvinToRGB(k);
  const rn = r / 255;
  const gn = g / 255;
  const bn = b / 255;

  const max = Math.max(rn, gn, bn);
  const min = Math.min(rn, gn, bn);
  const delta = max - min;

  let h = 0;
  if (delta !== 0) {
    if (max === rn) h = ((gn - bn) / delta) % 6;
    else if (max === gn) h = (bn - rn) / delta + 2;
    else h = (rn - gn) / delta + 4;
    h = (h * 60 + 360) % 360;
  }

  return { h, s: max === 0 ? 0 : delta / max, b: max };
}

export function kelvinToCSS(k: number): string {
  const [r, g, b] = kelvinToRGB(k);
  return `rgb(${r}, ${g}, ${b})`;
}

/**
 * Builds a conic-gradient CSS string from an array of CSS color strings,
 * distributing them evenly around the circle and looping back to the first
 * color — the same style as the ColorPicker swatch wheel.
 * Returns null when the array is empty.
 */
export function swatchBackground(colors: string[]): string | null {
  if (colors.length === 0) return null;
  if (colors.length === 1) return `conic-gradient(${colors[0]}, ${colors[0]})`;
  const stops = [...colors, colors[0]]
    .map((c, i) => `${c} ${Math.round((i / colors.length) * 360)}deg`)
    .join(", ");
  return `conic-gradient(${stops})`;
}
