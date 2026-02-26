import React, { useState, useEffect, useRef } from "react";
import {
  Wifi,
  Settings2,
} from "lucide-react";
import { Card } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";
import { LightCard } from "@/components/ui/LightCard";
import { RoomPanel, RoomGroupHeader } from "@/components/ui/RoomPanel";
import { useLightStore, lightActions } from "@/hooks/useLightStore";
import { groupByRoom } from "@/lib/brands";
import { sortedRoomKeys } from "@/lib/rooms";
import type { Device, LightMode } from "@/lib/types";
import { DEFAULT_KELVIN } from "@/lib/types";
import { resolveMode } from "@/lib/utils";

export function Lights() {
  const { devices, deviceOn, brightness, kelvin, color } = useLightStore();

  const [modeOverrides, setModeOverrides] = useState<Record<string, LightMode>>({});
  const [openRoomDeviceId, setOpenRoomDeviceId] = useState<string | null>(null);
  const cogButtonRefs = useRef<Record<string, HTMLButtonElement | null>>({});
  const activeAnchorRef = useRef<HTMLButtonElement | null>(null);

  useEffect(() => {
    lightActions.refreshDevices();
  }, []);

  function switchMode(deviceId: string, newMode: LightMode) {
    setModeOverrides((prev) => ({ ...prev, [deviceId]: newMode }));
    // Switching to temperature mode while a color is active clears it immediately.
    if (newMode === "kelvin" && color[deviceId]) {
      lightActions.setTemperature(
        deviceId,
        kelvin[deviceId] ?? DEFAULT_KELVIN,
        (brightness[deviceId] ?? 80) / 100
      );
    }
  }

  /** Brightness change from the card while NOT in colour mode. */
  function handleCardBrightness(device: Device, value: number) {
    if (kelvin[device.id]) {
      lightActions.setTemperature(device.id, kelvin[device.id], value / 100);
    } else {
      lightActions.setBrightness(device.id, value);
    }
  }

  const grouped = groupByRoom(devices);
  const roomKeys = sortedRoomKeys(grouped);

  function openRoomPanel(deviceId: string) {
    activeAnchorRef.current = cogButtonRefs.current[deviceId] ?? null;
    setOpenRoomDeviceId(deviceId);
  }

  return (
    <div className="space-y-8">
      {devices.length === 0 && (
        <Card className="flex flex-col items-center justify-center py-16 text-center">
          <Wifi className="h-12 w-12 text-muted-foreground/40 mb-4" />
          <h3 className="text-lg font-semibold">No Lights Found</h3>
          <p className="text-sm text-muted-foreground mt-2 max-w-md">
            Go to Settings and scan your network to discover LIFX, Hue, Elgato,
            and Govee lights.
          </p>
        </Card>
      )}

      {roomKeys.map((roomKey) => {
        const roomDevices = grouped[roomKey];
        return (
          <div key={roomKey}>
            <div className="flex items-center gap-2 mb-3">
              <RoomGroupHeader room={roomKey} />
              <Badge variant="secondary">{roomDevices.length}</Badge>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
              {roomDevices.map((device) => {
                const mode = resolveMode(device.id, modeOverrides, color, kelvin);
                return (
                  <div key={device.id} className="relative group/lightcard">
                    <LightCard
                      device={device}
                      on={deviceOn[device.id] ?? false}
                      brightness={brightness[device.id] ?? 0}
                      kelvin={kelvin[device.id] ?? DEFAULT_KELVIN}
                      color={color[device.id]}
                      mode={mode}
                      onToggle={(on) => lightActions.toggleLight(device.id, on)}
                      onBrightness={(value) => handleCardBrightness(device, value)}
                      onModeSwitch={(m) => switchMode(device.id, m)}
                      onKelvin={(k) => lightActions.setKelvin(device.id, k)}
                      onColor={(c) => lightActions.setColor(device.id, c)}
                    />
                    <button
                      ref={(el) => { cogButtonRefs.current[device.id] = el; }}
                      type="button"
                      title="Light settings"
                      onClick={() => openRoomPanel(device.id)}
                      className="absolute -top-1.5 -right-1.5 z-30 h-5 w-5 rounded-full bg-card border border-border shadow-sm flex items-center justify-center opacity-0 group-hover/lightcard:opacity-100 transition-opacity hover:border-primary/50 hover:text-primary"
                    >
                      <Settings2 className="h-3 w-3 text-muted-foreground" />
                    </button>
                  </div>
                );
              })}
            </div>
          </div>
        );
      })}

      {/* Single RoomPanel rendered outside the groups loop so it never
          unmounts/remounts when a device moves between room groups. */}
      {(() => {
        const openDevice = openRoomDeviceId
          ? devices.find((d) => d.id === openRoomDeviceId)
          : undefined;
        return (
          <RoomPanel
            open={!!openDevice}
            anchorRef={activeAnchorRef as React.RefObject<HTMLButtonElement>}
            currentRoom={openDevice?.room}
            deviceName={openDevice?.name ?? ""}
            onRoomChange={(room) =>
              openRoomDeviceId && lightActions.setDeviceRoom(openRoomDeviceId, room)
            }
            onRemove={() =>
              openRoomDeviceId && lightActions.removeDevice(openRoomDeviceId)
            }
            onClose={() => setOpenRoomDeviceId(null)}
          />
        );
      })()}
    </div>
  );
}
