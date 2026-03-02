import { useState, useEffect, useRef } from "react";
import { Lightbulb, Settings2 } from "lucide-react";
import { LightCard } from "@/components/ui/LightCard";
import { RoomGroupHeader, RoomPanel } from "@/components/ui/RoomPanel";
import { Badge } from "@/components/ui/Badge";
import { useLightStore, lightActions } from "@/hooks/useLightStore";
import { groupByRoom } from "@/lib/brands";
import { sortedRoomKeys } from "@/lib/rooms";
import type { LightMode } from "@/lib/types";
import { DEFAULT_KELVIN } from "@/lib/types";
import { resolveMode } from "@/lib/utils";
import { GetCapturePreview, GetScreenSyncState } from "../../wailsjs/go/main/App";

/**
 * Standalone lights panel rendered in a popup window.
 * Since the popup window is a separate WebView2 instance, Go-emitted events
 * don't reach it — we poll refreshDevices() every 3 s as a fallback.
 */
export function LightsPopupPage() {
  const { devices, deviceOn, brightness, kelvin, color } = useLightStore();
  const [modeOverrides, setModeOverrides] = useState<Record<string, LightMode>>({});
  const [openRoomDeviceId, setOpenRoomDeviceId] = useState<string | null>(null);
  const cogButtonRefs = useRef<Record<string, HTMLButtonElement | null>>({});
  const activeAnchorRef = useRef<HTMLButtonElement | null>(null);

  const [previewSrc, setPreviewSrc] = useState<string | null>(null);
  const [syncRunning, setSyncRunning] = useState(false);

  // Initial load + periodic poll (events won't fire in this popup window).
  useEffect(() => {
    lightActions.refreshDevices();
    const interval = setInterval(() => lightActions.refreshDevices(), 3000);
    return () => clearInterval(interval);
  }, []);

  // Poll sync state every 2 s and preview frame every 1 s.
  useEffect(() => {
    let running = false;

    const syncInterval = setInterval(async () => {
      try {
        const state = await GetScreenSyncState();
        running = state.running;
        setSyncRunning(state.running);
        if (!state.running) setPreviewSrc(null);
      } catch {
        // ignore
      }
    }, 2000);

    const previewInterval = setInterval(async () => {
      if (!running) return;
      try {
        const b64 = await GetCapturePreview();
        if (b64) setPreviewSrc(`data:image/jpeg;base64,${b64}`);
      } catch {
        // ignore
      }
    }, 1000);

    // Kick off immediately.
    GetScreenSyncState()
      .then((s) => { running = s.running; setSyncRunning(s.running); })
      .catch(() => {});

    return () => {
      clearInterval(syncInterval);
      clearInterval(previewInterval);
    };
  }, []);

  function switchMode(deviceId: string, newMode: LightMode) {
    setModeOverrides((prev) => ({ ...prev, [deviceId]: newMode }));
    if (newMode === "kelvin" && color[deviceId]) {
      lightActions.setTemperature(
        deviceId,
        kelvin[deviceId] ?? DEFAULT_KELVIN,
        (brightness[deviceId] ?? 80) / 100
      );
    }
  }

  function handleCardBrightness(deviceId: string, value: number) {
    if (kelvin[deviceId]) {
      lightActions.setTemperature(deviceId, kelvin[deviceId], value / 100);
    } else {
      lightActions.setBrightness(deviceId, value);
    }
  }

  function openRoomPanel(deviceId: string) {
    activeAnchorRef.current = cogButtonRefs.current[deviceId] ?? null;
    setOpenRoomDeviceId(deviceId);
  }

  const grouped = groupByRoom(devices);
  const roomKeys = sortedRoomKeys(grouped);
  const onCount = devices.filter((d) => deviceOn[d.id]).length;

  return (
    <div className="flex flex-col h-screen bg-background text-foreground overflow-hidden">
      {/* Header */}
      <div className="flex items-center gap-3 px-4 py-3 border-b border-border/50 shrink-0">
        <div className="h-7 w-7 rounded-lg bg-primary/20 flex items-center justify-center">
          <Lightbulb className="h-4 w-4 text-primary" />
        </div>
        <span className="text-sm font-semibold flex-1">Lights</span>
        <span className="text-xs text-muted-foreground tabular-nums">
          {onCount}/{devices.length} on
        </span>
      </div>

      {/* Capture preview — only shown when screen sync is active */}
      {syncRunning && (
        <div className="shrink-0 border-b border-border/50 bg-black">
          {previewSrc ? (
            <img
              src={previewSrc}
              alt="Capture preview"
              className="w-full block"
              style={{ maxHeight: 160, objectFit: "contain" }}
            />
          ) : (
            <div className="flex items-center justify-center h-16 text-xs text-muted-foreground/50">
              Waiting for first frame…
            </div>
          )}
        </div>
      )}

      {/* Device list */}
      <div className="flex-1 overflow-y-auto px-3 py-3 space-y-5">
        {devices.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-full text-center py-16">
            <Lightbulb className="h-10 w-10 text-muted-foreground/30 mb-3" />
            <p className="text-sm text-muted-foreground">No lights discovered yet.</p>
          </div>
        ) : (
          roomKeys.map((roomKey) => {
            const roomDevices = grouped[roomKey];
            return (
              <div key={roomKey}>
                <div className="flex items-center gap-2 mb-2">
                  <RoomGroupHeader room={roomKey} />
                  <Badge variant="secondary">{roomDevices.length}</Badge>
                </div>
                <div className="space-y-2">
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
                          onBrightness={(value) => handleCardBrightness(device.id, value)}
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
          })
        )}
      </div>

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
