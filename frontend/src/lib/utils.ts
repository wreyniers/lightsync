import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
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

export function hsbToHex(h: number, s: number, b: number): string {
  const [r, g, bl] = hsbToRGB(h, s, b);
  return `#${r.toString(16).padStart(2, "0")}${g.toString(16).padStart(2, "0")}${bl.toString(16).padStart(2, "0")}`;
}

export function hexToHSB(hex: string): { h: number; s: number; b: number } {
  const r = parseInt(hex.slice(1, 3), 16) / 255;
  const g = parseInt(hex.slice(3, 5), 16) / 255;
  const b = parseInt(hex.slice(5, 7), 16) / 255;
  const max = Math.max(r, g, b);
  const min = Math.min(r, g, b);
  const delta = max - min;
  let h = 0;
  if (delta !== 0) {
    if (max === r) h = ((g - b) / delta) % 6;
    else if (max === g) h = (b - r) / delta + 2;
    else h = (r - g) / delta + 4;
    h = (h * 60 + 360) % 360;
  }
  return { h, s: max === 0 ? 0 : delta / max, b: max };
}

export function kelvinToCSS(k: number): string {
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
  return `rgb(${clamp(r)}, ${clamp(g)}, ${clamp(b)})`;
}
