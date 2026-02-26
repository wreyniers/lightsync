import { hsbToCSS, kelvinToCSS, swatchBackground } from "@/lib/utils";
import type { Color, Device, DeviceState, Scene } from "@/lib/types";

/** Extract unique CSS color strings from a scene (global or per-device). */
export function sceneColors(scene: Scene): string[] {
  if (scene.globalColor) return [hsbToCSS(scene.globalColor.h, scene.globalColor.s, 1)];
  if (scene.globalKelvin != null) return [kelvinToCSS(scene.globalKelvin)];

  const seen = new Set<string>();
  const colors: string[] = [];
  for (const ds of Object.values(scene.devices || {})) {
    const css = ds.color
      ? hsbToCSS(ds.color.h, ds.color.s, 1)
      : ds.kelvin
      ? kelvinToCSS(ds.kelvin)
      : null;
    if (css && !seen.has(css)) {
      seen.add(css);
      colors.push(css);
    }
  }
  return colors;
}

/** Build conic-gradient swatch background from scene colors. */
export function sceneSwatchBackground(scene: Scene): string | null {
  return swatchBackground(sceneColors(scene));
}

/** Extract unique CSS color strings from live device states (on devices only). */
export function liveDeviceColors(
  devices: Device[],
  deviceOn: Record<string, boolean>,
  color: Record<string, Color>,
  kelvin: Record<string, number>
): string[] {
  const seen = new Set<string>();
  const colors: string[] = [];
  for (const device of devices) {
    if (!deviceOn[device.id]) continue;
    const lc = color[device.id];
    const lk = kelvin[device.id];
    const css = lc ? hsbToCSS(lc.h, lc.s, 1) : lk ? kelvinToCSS(lk) : null;
    if (css && !seen.has(css)) {
      seen.add(css);
      colors.push(css);
    }
  }
  return colors;
}

/** Build conic-gradient swatch background from live device states. */
export function liveSwatchBackground(
  devices: Device[],
  deviceOn: Record<string, boolean>,
  color: Record<string, Color>,
  kelvin: Record<string, number>
): string | null {
  return swatchBackground(liveDeviceColors(devices, deviceOn, color, kelvin));
}
