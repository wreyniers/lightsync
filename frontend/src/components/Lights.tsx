import { useState, useEffect, useCallback, useRef, type ReactNode } from "react";
import {
  Search,
  Lightbulb,
  Loader2,
  Wifi,
  CheckCircle2,
  Info,
  X,
  Camera,
  CameraOff,
  Film,
} from "lucide-react";
import { Card } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Badge } from "@/components/ui/Badge";
import { RockerSwitch } from "@/components/ui/RockerSwitch";
import { ColorPicker } from "@/components/ui/ColorPicker";
import { TemperaturePicker } from "@/components/ui/TemperaturePicker";
import { useLightStore, lightActions } from "@/hooks/useLightStore";
import { useWailsEvent } from "@/hooks/useWails";
import type { Device, Scene } from "@/lib/types";
import { kelvinToCSS, hsbToCSS } from "@/lib/utils";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";
import { GetScenes, GetActiveScene, CheckCameraNow } from "../../wailsjs/go/main/App";

const brandInfo: Record<string, { color: string; label: string }> = {
  lifx: { color: "text-green-400", label: "LIFX" },
  hue: { color: "text-blue-400", label: "Philips Hue" },
  elgato: { color: "text-yellow-400", label: "Elgato" },
  govee: { color: "text-purple-400", label: "Govee" },
};

type LightMode = "color" | "kelvin";

interface ScanProgress {
  phase: string;
  message: string;
  devices?: Device[];
}

function deviceLightColor(
  deviceId: string,
  color: Record<string, { h: number; s: number; b: number }>,
  kelvin: Record<string, number>
): string | undefined {
  const c = color[deviceId];
  if (c) return hsbToCSS(c.h, c.s, c.b);
  const k = kelvin[deviceId];
  if (k) return kelvinToCSS(k);
  return undefined;
}

/** Determine the active display mode for a device. */
function resolveMode(
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

function ModeToggle({
  mode,
  onSwitch,
}: {
  mode: LightMode;
  onSwitch: (m: LightMode) => void;
}) {
  return (
    <div className="flex rounded-lg border border-border overflow-hidden text-xs">
      <button
        type="button"
        onClick={() => onSwitch("color")}
        className={`px-3 py-1.5 transition-colors ${
          mode === "color"
            ? "bg-primary/20 text-primary font-medium"
            : "text-muted-foreground hover:bg-secondary"
        }`}
      >
        Color
      </button>
      <div className="w-px bg-border" />
      <button
        type="button"
        onClick={() => onSwitch("kelvin")}
        className={`px-3 py-1.5 transition-colors ${
          mode === "kelvin"
            ? "bg-primary/20 text-primary font-medium"
            : "text-muted-foreground hover:bg-secondary"
        }`}
      >
        Temperature
      </button>
    </div>
  );
}

function DeviceInfoRow({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="flex justify-between gap-2">
      <span className="text-muted-foreground shrink-0">{label}</span>
      <span className="text-right break-all">{value}</span>
    </div>
  );
}

function DeviceInfoTooltip({ device }: { device: Device }) {
  const minBright = device.brand === "elgato" ? 3 : 0;
  const tempRange =
    device.supportsKelvin && device.minKelvin && device.maxKelvin
      ? `${device.minKelvin.toLocaleString()}K – ${device.maxKelvin.toLocaleString()}K${device.kelvinStep && device.kelvinStep > 1 ? ` (step: ${device.kelvinStep}K)` : ""}`
      : device.supportsKelvin
      ? "Supported"
      : "Not supported";

  return (
    <div
      className="relative group/info"
      onClick={(e) => e.stopPropagation()}
    >
      <Info className="h-3.5 w-3.5 text-muted-foreground hover:text-foreground cursor-help transition-colors" />
      <div className="pointer-events-none invisible group-hover/info:visible opacity-0 group-hover/info:opacity-100 transition-opacity duration-150 absolute bottom-full left-1/2 -translate-x-1/2 mb-2 z-50 w-64 rounded-lg border border-border bg-popover text-popover-foreground shadow-lg p-3 space-y-1.5 text-xs">
        {/* Arrow */}
        <div className="absolute top-full left-1/2 -translate-x-1/2 border-4 border-transparent border-t-border" />
        <div className="font-medium text-foreground mb-2">Device info</div>
        <DeviceInfoRow label="IP" value={<span className="font-mono">{device.lastIp}</span>} />
        {device.model && <DeviceInfoRow label="Model" value={device.model} />}
        {device.firmwareVersion && <DeviceInfoRow label="Firmware" value={device.firmwareVersion} />}
        <DeviceInfoRow label="Brightness" value={`${minBright}% – 100%`} />
        <DeviceInfoRow label="Color" value={device.supportsColor ? "Full RGB" : "Not supported"} />
        <DeviceInfoRow label="Temperature" value={tempRange} />
      </div>
    </div>
  );
}

export function Lights() {
  const { devices, deviceOn, brightness, kelvin, color } = useLightStore();

  // Status state
  const cameraOn = useWailsEvent<boolean>("camera:state", false);
  const activeSceneEvent = useWailsEvent<string>("scene:active", "");
  const [scenes, setScenes] = useState<Scene[]>([]);
  const [activeSceneId, setActiveSceneId] = useState("");

  // Scan state
  const [scanning, setScanning] = useState(false);
  const [showScanCard, setShowScanCard] = useState(false);
  const [expanded, setExpanded] = useState<string | null>(null);
  const [scanMessage, setScanMessage] = useState("");
  const [scanPhase, setScanPhase] = useState("");
  const [foundDevices, setFoundDevices] = useState<Device[]>([]);
  const [modeOverrides, setModeOverrides] = useState<Record<string, LightMode>>({});
  const cleanupRef = useRef<(() => void) | null>(null);

  useEffect(() => {
    lightActions.refreshDevices();
    GetScenes().then((s) => setScenes(s || [])).catch(() => {});
    GetActiveScene().then(setActiveSceneId).catch(() => {});
    CheckCameraNow().catch(() => {});
  }, []);

  useEffect(() => {
    if (activeSceneEvent) setActiveSceneId(activeSceneEvent);
  }, [activeSceneEvent]);

  const activeSceneName = scenes.find((s) => s.id === activeSceneId)?.name;

  const dismissScan = useCallback(() => {
    setShowScanCard(false);
    setScanMessage("");
    setScanPhase("");
    setFoundDevices([]);
  }, []);

  const handleScan = useCallback(async () => {
    setScanning(true);
    setShowScanCard(true);
    setScanMessage("Starting network scan...");
    setScanPhase("");
    setFoundDevices([]);

    cleanupRef.current = EventsOn("scan:progress", (progress: ScanProgress) => {
      setScanMessage(progress.message);
      setScanPhase(progress.phase);
      if (progress.devices && progress.devices.length > 0) {
        setFoundDevices((prev) => {
          const existingIds = new Set(prev.map((d) => d.id));
          const incoming = progress.devices!.filter((d) => !existingIds.has(d.id));
          return incoming.length > 0 ? [...prev, ...incoming] : prev;
        });
      }
    });

    try {
      await lightActions.discoverLights();
    } catch (e) {
      console.error("Discovery failed:", e);
    }

    if (cleanupRef.current) {
      cleanupRef.current();
      cleanupRef.current = null;
    }
    EventsOff("scan:progress");
    setScanning(false);
  }, []);

  useEffect(() => {
    return () => {
      if (cleanupRef.current) cleanupRef.current();
    };
  }, []);

  function switchMode(deviceId: string, newMode: LightMode) {
    setModeOverrides((prev) => ({ ...prev, [deviceId]: newMode }));
    // Switching to temperature clears any active color immediately.
    if (newMode === "kelvin" && color[deviceId]) {
      lightActions.setTemperature(
        deviceId,
        kelvin[deviceId] ?? 4000,
        (brightness[deviceId] ?? 80) / 100
      );
    }
  }

  const grouped = devices.reduce(
    (acc, d) => {
      if (!acc[d.brand]) acc[d.brand] = [];
      acc[d.brand].push(d);
      return acc;
    },
    {} as Record<string, Device[]>
  );

  return (
    <div className="space-y-8">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">Lights</h2>
          <p className="text-muted-foreground mt-1">
            Discover and control your smart lights
          </p>
        </div>
        <Button onClick={handleScan} disabled={scanning}>
          {scanning ? (
            <Loader2 className="h-4 w-4 animate-spin" />
          ) : (
            <Search className="h-4 w-4" />
          )}
          {scanning ? "Scanning..." : "Scan Network"}
        </Button>
      </div>

      <div className="grid grid-cols-2 gap-4">
        <Card className="flex items-center gap-3 p-4">
          <div
            className={`h-9 w-9 shrink-0 rounded-full flex items-center justify-center ${
              cameraOn ? "bg-success/20" : "bg-muted"
            }`}
          >
            {cameraOn ? (
              <Camera className="h-4 w-4 text-success" />
            ) : (
              <CameraOff className="h-4 w-4 text-muted-foreground" />
            )}
          </div>
          <div>
            <p className="text-sm font-semibold leading-none">
              {cameraOn ? "Camera Active" : "Camera Off"}
            </p>
            <p className="text-xs text-muted-foreground mt-1">
              {cameraOn ? "Webcam in use" : "No camera detected"}
            </p>
          </div>
        </Card>

        <Card className="flex items-center gap-3 p-4">
          <div
            className={`h-9 w-9 shrink-0 rounded-full flex items-center justify-center ${
              activeSceneName ? "bg-primary/20" : "bg-muted"
            }`}
          >
            <Film className={`h-4 w-4 ${activeSceneName ? "text-primary" : "text-muted-foreground"}`} />
          </div>
          <div>
            <p className="text-sm font-semibold leading-none">
              {activeSceneName ?? "No Active Scene"}
            </p>
            <p className="text-xs text-muted-foreground mt-1">
              {scenes.length} scene{scenes.length !== 1 ? "s" : ""} configured
            </p>
          </div>
        </Card>
      </div>

      {devices.length === 0 && !scanning && !showScanCard && (
        <Card className="flex flex-col items-center justify-center py-16 text-center">
          <Wifi className="h-12 w-12 text-muted-foreground/40 mb-4" />
          <h3 className="text-lg font-semibold">No Lights Found</h3>
          <p className="text-sm text-muted-foreground mt-2 max-w-md">
            Click "Scan Network" to discover LIFX, Hue, Elgato, and Govee
            lights on your local network.
          </p>
          <Button onClick={handleScan} className="mt-6">
            <Search className="h-4 w-4" />
            Scan Network
          </Button>
        </Card>
      )}

      {showScanCard && (
        <Card className="py-6 px-6 space-y-4">
          <div className="flex items-center gap-3">
            {scanPhase === "done" ? (
              <CheckCircle2 className="h-5 w-5 text-green-500" />
            ) : (
              <Loader2 className="h-5 w-5 animate-spin text-primary" />
            )}
            <p className="text-sm font-medium flex-1">{scanMessage}</p>
            {!scanning && (
              <button
                type="button"
                onClick={dismissScan}
                className="text-muted-foreground hover:text-foreground transition-colors"
                aria-label="Dismiss"
              >
                <X className="h-4 w-4" />
              </button>
            )}
          </div>
          {foundDevices.length > 0 && (
            <div className="space-y-1.5 pt-2 border-t border-border">
              <p className="text-xs text-muted-foreground font-medium mb-2">
                Discovered lights
              </p>
              {foundDevices.map((d) => {
                const info = brandInfo[d.brand] || {
                  color: "text-foreground",
                  label: d.brand,
                };
                return (
                  <div
                    key={d.id}
                    className="flex items-center gap-2 text-sm animate-in fade-in slide-in-from-left-2 duration-300"
                  >
                    <Lightbulb className={`h-3.5 w-3.5 ${info.color}`} />
                    <span>{d.name}</span>
                    <span className="text-xs text-muted-foreground">
                      {info.label}
                    </span>
                  </div>
                );
              })}
            </div>
          )}
        </Card>
      )}

      {Object.entries(grouped).map(([brand, brandDevices]) => {
        const info = brandInfo[brand] || { color: "text-foreground", label: brand };
        return (
          <div key={brand}>
            <div className="flex items-center gap-2 mb-3">
              <Lightbulb className={`h-5 w-5 ${info.color}`} />
              <h3 className="text-lg font-semibold">{info.label}</h3>
              <Badge variant="secondary">{brandDevices.length}</Badge>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {brandDevices.map((device) => {
                const mode = resolveMode(device.id, modeOverrides, color, kelvin);
                const hasBoth = device.supportsColor && device.supportsKelvin;

                return (
                  <Card
                    key={device.id}
                    className="p-4 cursor-pointer hover:border-primary/40 transition-colors"
                    onClick={() =>
                      setExpanded(expanded === device.id ? null : device.id)
                    }
                  >
                    <div className="flex items-center justify-between">
                      <div>
                        <div className="flex items-center gap-1.5">
                          <p className="font-medium">{device.name}</p>
                          <DeviceInfoTooltip device={device} />
                        </div>
                        {device.model && (
                          <p className="text-xs text-muted-foreground mt-0.5">
                            {device.model}
                          </p>
                        )}
                      </div>
                      <div onClick={(e) => e.stopPropagation()}>
                        <RockerSwitch
                          checked={deviceOn[device.id] ?? false}
                          onChange={(on) => lightActions.toggleLight(device.id, on)}
                          color={deviceLightColor(device.id, color, kelvin)}
                        />
                      </div>
                    </div>

                    {expanded === device.id && (
                      <div
                        className="mt-4 pt-4 border-t border-border space-y-4"
                        onClick={(e) => e.stopPropagation()}
                      >
                        {/* Mode toggle — only for devices supporting both */}
                        {hasBoth && (
                          <ModeToggle
                            mode={mode}
                            onSwitch={(m) => switchMode(device.id, m)}
                          />
                        )}

                        {/* Color mode */}
                        {(device.supportsColor && mode === "color") && (
                          <div className="flex items-center gap-3">
                            <label className="text-xs text-muted-foreground w-12 shrink-0">
                              Color
                            </label>
                            <ColorPicker
                              value={color[device.id] ?? null}
                              onChange={(c) => lightActions.setColor(device.id, c)}
                            />
                            {color[device.id] && (
                              <span className="text-xs text-muted-foreground">
                                {Math.round((color[device.id].b) * 100)}% brightness
                              </span>
                            )}
                          </div>
                        )}

                        {/* Temperature mode */}
                        {(device.supportsKelvin && mode === "kelvin") && (
                          <div className="flex items-center gap-3">
                            <label className="text-xs text-muted-foreground w-12 shrink-0">
                              Temp
                            </label>
                            <TemperaturePicker
                              kelvin={kelvin[device.id] ?? 4000}
                              brightness={(brightness[device.id] ?? 80) / 100}
                              minBrightness={device.brand === "elgato" ? 0.03 : 0}
                              minKelvin={device.minKelvin ? device.minKelvin : undefined}
                              maxKelvin={device.maxKelvin ? device.maxKelvin : undefined}
                              kelvinStep={device.kelvinStep && device.kelvinStep > 1 ? device.kelvinStep : 1}
                              onChange={(k, b) =>
                                lightActions.setTemperature(device.id, k, b)
                              }
                            />
                            <span className="text-xs text-muted-foreground">
                              {kelvin[device.id] ?? 4000}K · {brightness[device.id] ?? 80}%
                            </span>
                          </div>
                        )}
                      </div>
                    )}
                  </Card>
                );
              })}
            </div>
          </div>
        );
      })}
    </div>
  );
}
