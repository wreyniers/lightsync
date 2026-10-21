import { Lightbulb, Check } from "lucide-react";
import { SettingsSection } from "./settings";
import { groupByRoom, UNASSIGNED_KEY } from "@/lib/brands";
import { getRoomIcon, sortedRoomKeys } from "@/lib/rooms";
import type { Device, ScreenSyncConfig } from "@/lib/types";

interface DevicesTabProps {
  config: ScreenSyncConfig;
  devices: Device[];
  onChange: (patch: Partial<ScreenSyncConfig>) => void;
}

export function DevicesTab({ config, devices, onChange }: DevicesTabProps) {
  const selectedIds = new Set(config.deviceIds);

  const toggleDevice = (id: string) => {
    const next = new Set(selectedIds);
    if (next.has(id)) {
      next.delete(id);
    } else {
      next.add(id);
    }
    onChange({ deviceIds: Array.from(next) });
  };

  const toggleAll = () => {
    if (selectedIds.size === devices.length) {
      onChange({ deviceIds: [] });
    } else {
      onChange({ deviceIds: devices.map((d) => d.id) });
    }
  };

  if (devices.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">
        No lights discovered. Go to the Lights page to scan your network first.
      </p>
    );
  }

  const grouped = groupByRoom(devices);
  const roomKeys = sortedRoomKeys(grouped);
  const allSelected = selectedIds.size === devices.length;

  return (
    <div className="space-y-5">
      <SettingsSection
        header={
          <div className="flex items-center justify-between">
            <p className="text-xs font-medium uppercase tracking-widest text-muted-foreground">
              Light Selection
            </p>
            <button
              type="button"
              onClick={toggleAll}
              className="text-xs text-primary hover:underline"
            >
              {allSelected ? "Deselect all" : "Select all"}
            </button>
          </div>
        }
      >
        <p className="text-xs text-muted-foreground mt-1">
          {selectedIds.size} of {devices.length} light{devices.length !== 1 ? "s" : ""} selected
        </p>

        {roomKeys.map((roomKey) => {
          const roomDevices = grouped[roomKey];
          const RoomIcon = getRoomIcon(roomKey === UNASSIGNED_KEY ? undefined : roomKey);
          const roomLabel = roomKey === UNASSIGNED_KEY ? "Unassigned" : roomKey;
          return (
            <div key={roomKey} className="space-y-2">
              <div className="flex items-center gap-2">
                <RoomIcon className="h-4 w-4 text-muted-foreground" />
                <span className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                  {roomLabel}
                </span>
              </div>
              <div className="space-y-2">
                {roomDevices.map((device) => {
                  const included = selectedIds.has(device.id);
                  return (
                    <button
                      key={device.id}
                      type="button"
                      onClick={() => toggleDevice(device.id)}
                      className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-xl text-left transition-all ${
                        included
                          ? "bg-card/80 ring-1 ring-primary/30"
                          : "bg-card/40 hover:bg-card/60 opacity-60 hover:opacity-100"
                      }`}
                    >
                      <div
                        className={`h-8 w-8 rounded-full flex items-center justify-center shrink-0 ${
                          included ? "bg-primary/20" : "bg-white/5"
                        }`}
                      >
                        {included ? (
                          <Check className="h-4 w-4 text-primary" />
                        ) : (
                          <Lightbulb className="h-4 w-4 text-muted-foreground" />
                        )}
                      </div>
                      <div className="flex-1 min-w-0">
                        <p className="text-sm font-medium truncate">{device.name}</p>
                        <p className="text-xs text-muted-foreground">
                          {device.brand}
                          {device.supportsColor ? " · Color" : ""}
                          {device.supportsKelvin && !device.supportsColor ? " · Temperature" : ""}
                        </p>
                      </div>
                    </button>
                  );
                })}
              </div>
            </div>
          );
        })}
      </SettingsSection>
    </div>
  );
}
