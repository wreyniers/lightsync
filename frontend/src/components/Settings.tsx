import { useState, useEffect, useRef, useCallback } from "react";
import { Info, Plus, Search, Loader2, Wifi } from "lucide-react";
import { Card } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Toggle } from "@/components/ui/Toggle";
import { Slider } from "@/components/ui/Slider";
import type { Settings as SettingsType } from "@/lib/types";
import { APP_VERSION } from "@/lib/types";
import { store } from "../../wailsjs/go/models";
import {
  GetSettings,
  UpdateSettings,
  GetHueBridges,
  DiscoverHueBridges,
  PairHueBridge,
} from "../../wailsjs/go/main/App";

interface HueBridgeInfo {
  id: string;
  ip: string;
  username: string;
}

interface DiscoveredBridge {
  ip: string;
  name: string;
}

type AddBridgeStep = "idle" | "scanning" | "results" | "pairing" | "paired";

export function Settings() {
  const [settings, setSettings] = useState<SettingsType>({
    pollIntervalMs: 1000,
    startMinimized: false,
    launchAtLogin: false,
  });
  const [bridges, setBridges] = useState<HueBridgeInfo[]>([]);
  const isInitialLoad = useRef(true);
  const saveTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const [step, setStep] = useState<AddBridgeStep>("idle");
  const [discovered, setDiscovered] = useState<DiscoveredBridge[]>([]);
  const [pairingIp, setPairingIp] = useState("");
  const [pairError, setPairError] = useState("");
  const pairIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    GetSettings().then(setSettings).catch(() => {});
    GetHueBridges()
      .then((b) => setBridges(b || []))
      .catch(() => {});
  }, []);

  useEffect(() => {
    return () => {
      if (pairIntervalRef.current) clearInterval(pairIntervalRef.current);
    };
  }, []);

  useEffect(() => {
    if (isInitialLoad.current) {
      isInitialLoad.current = false;
      return;
    }
    if (saveTimeoutRef.current) clearTimeout(saveTimeoutRef.current);
    saveTimeoutRef.current = setTimeout(() => {
      UpdateSettings(new store.Settings(settings)).catch((e) =>
        console.error("Failed to save settings:", e),
      );
    }, 300);
    return () => {
      if (saveTimeoutRef.current) clearTimeout(saveTimeoutRef.current);
    };
  }, [settings]);

  const handleScan = async () => {
    setStep("scanning");
    setDiscovered([]);
    setPairError("");
    try {
      const found = await DiscoverHueBridges();
      setDiscovered(found || []);
      setStep("results");
    } catch (e) {
      console.error("Failed to scan:", e);
      setDiscovered([]);
      setStep("results");
    }
  };

  const stopPairing = useCallback(() => {
    if (pairIntervalRef.current) {
      clearInterval(pairIntervalRef.current);
      pairIntervalRef.current = null;
    }
  }, []);

  const handlePair = useCallback(
    (ip: string) => {
      stopPairing();
      setPairingIp(ip);
      setPairError("");
      setStep("pairing");

      pairIntervalRef.current = setInterval(async () => {
        try {
          const result = await PairHueBridge(ip);
          if (result.success) {
            stopPairing();
            setStep("paired");
            const updated = await GetHueBridges();
            setBridges(updated || []);
            setTimeout(() => setStep("idle"), 2000);
          } else if (
            result.error &&
            !result.error.includes("link button not pressed")
          ) {
            stopPairing();
            setPairError(result.error);
          }
        } catch (e) {
          stopPairing();
          setPairError("Failed to communicate with bridge");
        }
      }, 2000);
    },
    [stopPairing],
  );

  const handleCancel = useCallback(() => {
    stopPairing();
    setStep("idle");
    setDiscovered([]);
    setPairingIp("");
    setPairError("");
  }, [stopPairing]);

  const alreadyConfigured = useCallback(
    (ip: string) => bridges.some((b) => b.ip === ip),
    [bridges],
  );

  return (
    <div className="space-y-8">
      <div>
        <h2 className="text-2xl font-bold tracking-tight">Settings</h2>
        <p className="text-muted-foreground mt-1">
          Configure LightSync preferences
        </p>
      </div>

      <Card className="space-y-6">
        <h3 className="text-lg font-semibold">Monitoring</h3>

        <div>
          <label className="text-sm font-medium mb-1 block">
            Webcam Poll Interval: {settings.pollIntervalMs}ms
          </label>
          <p className="text-xs text-muted-foreground mb-3">
            How often LightSync checks if your camera is active. Lower values
            are more responsive but use slightly more CPU.
          </p>
          <Slider
            value={settings.pollIntervalMs}
            min={250}
            max={5000}
            step={250}
            onChange={(v) =>
              setSettings((s) => ({ ...s, pollIntervalMs: v }))
            }
          />
          <div className="flex justify-between text-xs text-muted-foreground mt-1">
            <span>250ms (fast)</span>
            <span>5000ms (slow)</span>
          </div>
        </div>

        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm font-medium">Start Minimized</p>
            <p className="text-xs text-muted-foreground">
              Start LightSync minimized to system tray
            </p>
          </div>
          <Toggle
            checked={settings.startMinimized}
            onChange={(v) =>
              setSettings((s) => ({ ...s, startMinimized: v }))
            }
          />
        </div>

        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm font-medium">Launch at Login</p>
            <p className="text-xs text-muted-foreground">
              Automatically start LightSync when you log in
            </p>
          </div>
          <Toggle
            checked={settings.launchAtLogin}
            onChange={(v) =>
              setSettings((s) => ({ ...s, launchAtLogin: v }))
            }
          />
        </div>

      </Card>

      <Card className="space-y-6">
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-semibold">Hue Bridges</h3>
          {step === "idle" && (
            <Button variant="outline" size="sm" onClick={handleScan}>
              <Plus className="h-4 w-4" />
              Add Bridge
            </Button>
          )}
        </div>

        {step === "scanning" && (
          <div className="rounded-lg p-6 flex flex-col items-center gap-3">
            <Loader2 className="h-6 w-6 animate-spin text-primary" />
            <p className="text-sm font-medium">Scanning network for Hue bridges...</p>
            <p className="text-xs text-muted-foreground">This may take a few seconds</p>
          </div>
        )}

        {step === "results" && (
          <div className="rounded-lg p-4 space-y-3">
            {discovered.length === 0 ? (
              <div className="text-center py-4 space-y-2">
                <Search className="h-6 w-6 mx-auto text-muted-foreground" />
                <p className="text-sm font-medium">No bridges found</p>
                <p className="text-xs text-muted-foreground">
                  Make sure your Hue bridge is powered on and connected to the same network.
                </p>
              </div>
            ) : (
              <>
                <p className="text-sm font-medium">
                  Found {discovered.length} bridge{discovered.length !== 1 ? "s" : ""} on your network
                </p>
                <div className="space-y-2">
                  {discovered.map((b) => (
                    <div
                      key={b.ip}
                      className="flex items-center justify-between rounded-lg p-3"
                    >
                      <div className="flex items-center gap-3">
                        <Wifi className="h-4 w-4 text-primary" />
                        <div>
                          <p className="text-sm font-medium">{b.name || "Hue Bridge"}</p>
                          <p className="text-xs text-muted-foreground">{b.ip}</p>
                        </div>
                      </div>
                      {alreadyConfigured(b.ip) ? (
                        <span className="text-xs text-muted-foreground">Already added</span>
                      ) : (
                        <Button size="sm" onClick={() => handlePair(b.ip)}>
                          Pair
                        </Button>
                      )}
                    </div>
                  ))}
                </div>
              </>
            )}
            <div className="flex gap-2">
              <Button size="sm" variant="outline" onClick={handleScan}>
                <Search className="h-3 w-3" />
                Scan Again
              </Button>
              <Button size="sm" variant="outline" onClick={handleCancel}>
                Cancel
              </Button>
            </div>
          </div>
        )}

        {step === "pairing" && (
          <div className="rounded-lg p-6 flex flex-col items-center gap-3">
            <Loader2 className="h-6 w-6 animate-spin text-primary" />
            <p className="text-sm font-medium">Press the button on your Hue bridge</p>
            <p className="text-xs text-muted-foreground">
              Waiting for bridge at {pairingIp} to authorize...
            </p>
            {pairError && (
              <p className="text-xs text-destructive">{pairError}</p>
            )}
            <Button size="sm" variant="outline" onClick={handleCancel}>
              Cancel
            </Button>
          </div>
        )}

        {step === "paired" && (
          <div className="rounded-lg bg-green-500/5 p-6 flex flex-col items-center gap-2">
            <p className="text-sm font-medium text-green-500">Bridge paired successfully!</p>
          </div>
        )}

        {bridges.length === 0 && step === "idle" && (
          <p className="text-sm text-muted-foreground">
            No Hue bridges configured. Add a bridge to control Philips Hue lights.
          </p>
        )}

        {bridges.map((bridge) => (
          <div
            key={bridge.id || bridge.ip}
            className="flex items-center justify-between rounded-lg p-3"
          >
            <div className="flex items-center gap-3">
              <Wifi className="h-4 w-4 text-muted-foreground" />
              <div>
                <p className="text-sm font-medium">{bridge.ip}</p>
                <p className="text-xs text-muted-foreground">
                  Key: {bridge.username.slice(0, 8)}...
                </p>
              </div>
            </div>
          </div>
        ))}
      </Card>

      <Card>
        <div className="flex items-center gap-3 mb-4">
          <Info className="h-5 w-5 text-primary" />
          <h3 className="text-lg font-semibold">About</h3>
        </div>
        <div className="space-y-2 text-sm text-muted-foreground">
          <p>
            <span className="text-foreground font-medium">LightSync</span>{" "}
            v{APP_VERSION}
          </p>
          <p>
            Monitors your webcam and automatically controls your smart lights.
          </p>
          <p>Supports LIFX, Philips Hue, Elgato Key Light, and Govee.</p>
          <p className="pt-2">
            Built with Wails, Go, React, and TypeScript.
          </p>
        </div>
      </Card>
    </div>
  );
}
