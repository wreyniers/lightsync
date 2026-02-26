import type { Device } from "@/lib/types";

export const brandInfo: Record<string, { color: string; label: string }> = {
  lifx: { color: "text-green-400", label: "LIFX" },
  hue: { color: "text-blue-400", label: "Philips Hue" },
  elgato: { color: "text-yellow-400", label: "Elgato" },
  govee: { color: "text-purple-400", label: "Govee" },
};

export function getBrandInfo(brand: string): { color: string; label: string } {
  return brandInfo[brand] ?? { color: "text-foreground", label: brand };
}

export function groupByBrand(devices: Device[]): Record<string, Device[]> {
  const grouped: Record<string, Device[]> = {};
  for (const d of devices) {
    if (!grouped[d.brand]) grouped[d.brand] = [];
    grouped[d.brand].push(d);
  }
  return grouped;
}

export const UNASSIGNED_KEY = "__unassigned__";

/**
 * Groups devices by their assigned room. Devices without a room fall into
 * UNASSIGNED_KEY. Groups are ordered: preset rooms first (in preset order),
 * then custom rooms alphabetically, then unassigned last.
 */
export function groupByRoom(devices: Device[]): Record<string, Device[]> {
  const grouped: Record<string, Device[]> = {};
  for (const d of devices) {
    const key = d.room?.trim() || UNASSIGNED_KEY;
    if (!grouped[key]) grouped[key] = [];
    grouped[key].push(d);
  }
  return grouped;
}
