import { useState, useEffect, useCallback, useRef } from "react";
import {
  Search,
  Lightbulb,
  Loader2,
  Wifi,
  CheckCircle2,
  X,
} from "lucide-react";
import { Card } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Badge } from "@/components/ui/Badge";
import { LightCard } from "@/components/ui/LightCard";
import { useLightStore, lightActions } from "@/hooks/useLightStore";
import { getBrandInfo, groupByBrand } from "@/lib/brands";
import type { Device, LightMode } from "@/lib/types";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";

interface ScanProgress {
  phase: string;
  message: string;
  devices?: Device[];
}

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

export function Lights() {
  const { devices, deviceOn, brightness, kelvin, color } = useLightStore();


  const [scanning, setScanning] = useState(false);
  const [showScanCard, setShowScanCard] = useState(false);
  const [scanMessage, setScanMessage] = useState("");
  const [scanPhase, setScanPhase] = useState("");
  const [foundDevices, setFoundDevices] = useState<Device[]>([]);
  const [modeOverrides, setModeOverrides] = useState<Record<string, LightMode>>({});
  const cleanupRef = useRef<(() => void) | null>(null);

  useEffect(() => {
    lightActions.refreshDevices();
  }, []);

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
    // Switching to temperature mode while a color is active clears it immediately.
    if (newMode === "kelvin" && color[deviceId]) {
      lightActions.setTemperature(
        deviceId,
        kelvin[deviceId] ?? 4000,
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

  const grouped = groupByBrand(devices);

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
            <div className="space-y-1.5 pt-2">
              <p className="text-xs text-muted-foreground font-medium mb-2">
                Discovered lights
              </p>
              {foundDevices.map((d) => {
                const info = getBrandInfo(d.brand);
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
        const info = getBrandInfo(brand);
        return (
          <div key={brand}>
            <div className="flex items-center gap-2 mb-3">
              <Lightbulb className={`h-5 w-5 ${info.color}`} />
              <h3 className="text-lg font-semibold">{info.label}</h3>
              <Badge variant="secondary">{brandDevices.length}</Badge>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
              {brandDevices.map((device) => {
                const mode = resolveMode(device.id, modeOverrides, color, kelvin);
                return (
                  <LightCard
                    key={device.id}
                    device={device}
                    on={deviceOn[device.id] ?? false}
                    brightness={brightness[device.id] ?? 0}
                    kelvin={kelvin[device.id] ?? 4000}
                    color={color[device.id]}
                    mode={mode}
                    onToggle={(on) => lightActions.toggleLight(device.id, on)}
                    onBrightness={(value) => handleCardBrightness(device, value)}
                    onModeSwitch={(m) => switchMode(device.id, m)}
                    onKelvin={(k) => lightActions.setKelvin(device.id, k)}
                    onColor={(c) => lightActions.setColor(device.id, c)}
                  />
                );
              })}
            </div>
          </div>
        );
      })}
    </div>
  );
}
