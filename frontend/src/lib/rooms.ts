import {
  BedDouble,
  Monitor,
  UtensilsCrossed,
  Flower2,
  Car,
  Sofa,
  Lightbulb,
  type LucideIcon,
} from "lucide-react";
import { UNASSIGNED_KEY } from "@/lib/brands";

export interface RoomPreset {
  name: string;
  icon: LucideIcon;
}

export const ROOM_PRESETS: RoomPreset[] = [
  { name: "Bedroom",     icon: BedDouble },
  { name: "Office",      icon: Monitor },
  { name: "Kitchen",     icon: UtensilsCrossed },
  { name: "Patio",       icon: Flower2 },
  { name: "Garage",      icon: Car },
  { name: "Living Room", icon: Sofa },
];

/** Returns the icon for a given room name, falling back to Lightbulb for custom/unassigned rooms. */
export function getRoomIcon(room: string | undefined): LucideIcon {
  if (!room) return Lightbulb;
  return ROOM_PRESETS.find((p) => p.name === room)?.icon ?? Lightbulb;
}

/**
 * Returns room keys sorted: preset rooms first (in preset order), then custom
 * rooms alphabetically, then the unassigned bucket last.
 */
export function sortedRoomKeys(grouped: Record<string, unknown[]>): string[] {
  const presetNames = ROOM_PRESETS.map((p) => p.name);
  return [
    ...presetNames.filter((name) => grouped[name]),
    ...Object.keys(grouped)
      .filter((k) => k !== UNASSIGNED_KEY && !presetNames.includes(k))
      .sort(),
    ...(grouped[UNASSIGNED_KEY] ? [UNASSIGNED_KEY] : []),
  ];
}
