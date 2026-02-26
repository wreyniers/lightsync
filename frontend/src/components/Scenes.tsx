import { useState, useEffect, useCallback } from "react";
import {
  Plus,
  Trash2,
  Camera,
  CameraOff,
  Film,
  Play,
  Pencil,
  X,
  Check,
  Thermometer,
  Lightbulb,
  Palette,
} from "lucide-react";
import { Card } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Badge } from "@/components/ui/Badge";
import { Toggle } from "@/components/ui/Toggle";
import { RockerSwitch } from "@/components/ui/RockerSwitch";
import { ColorPicker } from "@/components/ui/ColorPicker";
import { TemperaturePicker } from "@/components/ui/TemperaturePicker";
import type { Device, DeviceState, Color } from "@/lib/types";
import { kelvinToCSS, hsbToCSS } from "@/lib/utils";
import { useLightStore, lightActions } from "@/hooks/useLightStore";
import { useWailsEvent } from "@/hooks/useWails";
import { lights, main, store } from "../../wailsjs/go/models";
import {
  GetScenes,
  CreateScene,
  UpdateScene,
  DeleteScene,
  ActivateScene,
  GetActiveScene,
} from "../../wailsjs/go/main/App";

type Scene = store.Scene;
type LightMode = "color" | "kelvin";

const brandInfo: Record<string, { color: string; label: string }> = {
  lifx: { color: "text-green-400", label: "LIFX" },
  hue: { color: "text-blue-400", label: "Philips Hue" },
  elgato: { color: "text-yellow-400", label: "Elgato" },
  govee: { color: "text-purple-400", label: "Govee" },
};

function groupByBrand(devices: Device[]): Record<string, Device[]> {
  const grouped: Record<string, Device[]> = {};
  for (const d of devices) {
    if (!grouped[d.brand]) grouped[d.brand] = [];
    grouped[d.brand].push(d);
  }
  return grouped;
}

function ColorSwatch({ color }: { color: Color }) {
  return (
    <span
      className="inline-block h-4 w-4 rounded-full border border-white/20 shrink-0"
      style={{ backgroundColor: hsbToCSS(color.h, color.s, color.b) }}
    />
  );
}

function TempSwatch({ kelvin }: { kelvin: number }) {
  return (
    <span
      className="inline-block h-4 w-4 rounded-full border border-white/20 shrink-0"
      style={{ backgroundColor: kelvinToCSS(kelvin) }}
    />
  );
}

function deviceSceneColor(state: DeviceState | undefined): string | undefined {
  if (!state) return undefined;
  if (state.color) return hsbToCSS(state.color.h, state.color.s, state.color.b);
  if (state.kelvin) return kelvinToCSS(state.kelvin);
  return undefined;
}

function ModeToggle({
  mode,
  onSwitch,
}: {
  mode: LightMode;
  onSwitch: (m: LightMode) => void;
}) {
  return (
    <div className="flex rounded-md border border-border overflow-hidden text-xs">
      <button
        type="button"
        onClick={() => onSwitch("color")}
        className={`px-2.5 py-1 transition-colors ${
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
        className={`px-2.5 py-1 transition-colors ${
          mode === "kelvin"
            ? "bg-primary/20 text-primary font-medium"
            : "text-muted-foreground hover:bg-secondary"
        }`}
      >
        Temp
      </button>
    </div>
  );
}

const DEFAULT_COLOR: Color = { h: 30, s: 1, b: 1 };

function resolveDeviceMode(
  state: DeviceState | undefined,
  overrides: Record<string, LightMode>
): LightMode {
  if (overrides["_"]) return overrides["_"]; // should not happen but guard
  if (state?.color) return "color";
  if (state?.kelvin) return "kelvin";
  return "color";
}

export function Scenes() {
  const { devices } = useLightStore();
  const activeSceneEvent = useWailsEvent<string>("scene:active", "");
  const [scenes, setScenes] = useState<Scene[]>([]);
  const [activeScene, setActiveScene] = useState("");
  const [editing, setEditing] = useState<Scene | null>(null);
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState("");
  const [newTrigger, setNewTrigger] = useState("camera_on");
  const [newDevices, setNewDevices] = useState<Record<string, DeviceState>>({});

  // Per-device mode overrides (keyed by device id).
  const [deviceModes, setDeviceModes] = useState<Record<string, LightMode>>({});

  // Global color settings.
  const [globalColor, setGlobalColor] = useState(false);
  const [globalColorValue, setGlobalColorValue] = useState<Color>(DEFAULT_COLOR);

  // Global temperature settings.
  const [globalTemp, setGlobalTemp] = useState(false);
  const [globalKelvin, setGlobalKelvin] = useState(4000);
  const [globalBrightness, setGlobalBrightness] = useState(0.8);

  const refresh = useCallback(() => {
    GetScenes()
      .then((s) => setScenes(s || []))
      .catch(() => {});
    lightActions.refreshDevices();
    GetActiveScene().then(setActiveScene).catch(() => {});
  }, []);

  useEffect(() => { refresh(); }, [refresh]);

  useEffect(() => {
    if (activeSceneEvent) setActiveScene(activeSceneEvent);
  }, [activeSceneEvent]);

  const startCreate = () => {
    setCreating(true);
    setNewName("");
    setNewTrigger("camera_on");
    setNewDevices({});
    setDeviceModes({});
    setGlobalColor(false);
    setGlobalColorValue(DEFAULT_COLOR);
    setGlobalTemp(false);
    setGlobalKelvin(4000);
    setGlobalBrightness(0.8);
    setEditing(null);
  };

  const startEdit = (scene: Scene) => {
    setEditing(scene);
    setNewName(scene.name);
    setNewTrigger(scene.trigger);
    const devs = scene.devices || {};
    setNewDevices(devs);
    setDeviceModes({});
    setCreating(false);

    // Detect if all included on-devices share the same color.
    const colors = Object.values(devs).filter((s) => s.on && s.color).map((s) => s.color!);
    const allSameColor =
      colors.length > 0 &&
      colors.every(
        (c) =>
          Math.round(c.h) === Math.round(colors[0].h) &&
          Math.round(c.s * 100) === Math.round(colors[0].s * 100)
      );
    setGlobalColor(allSameColor);
    setGlobalColorValue(allSameColor ? colors[0] : DEFAULT_COLOR);

    // Detect if all included on-devices share the same kelvin.
    const kelvins = Object.values(devs).filter((s) => s.on && !s.color && s.kelvin).map((s) => s.kelvin!);
    const allSameTemp = kelvins.length > 0 && kelvins.every((k) => k === kelvins[0]);
    setGlobalTemp(allSameTemp && !allSameColor);
    setGlobalKelvin(allSameTemp ? kelvins[0] : 4000);
    setGlobalBrightness(
      allSameTemp
        ? (Object.values(devs).find((s) => s.on && !s.color)?.brightness ?? 0.8)
        : 0.8
    );
  };

  const cancelEdit = () => { setEditing(null); setCreating(false); };

  const toggleDeviceInScene = (deviceId: string) => {
    setNewDevices((prev) => {
      if (prev[deviceId]) {
        const next = { ...prev };
        delete next[deviceId];
        return next;
      }
      return { ...prev, [deviceId]: { on: true, brightness: 0.8 } };
    });
  };

  const updateDeviceState = (deviceId: string, updates: Partial<DeviceState>) => {
    setNewDevices((prev) => ({
      ...prev,
      [deviceId]: { ...prev[deviceId], ...updates },
    }));
  };

  function switchDeviceMode(deviceId: string, mode: LightMode) {
    setDeviceModes((prev) => ({ ...prev, [deviceId]: mode }));
    if (mode === "kelvin") {
      // Clear color when switching to temperature.
      updateDeviceState(deviceId, { color: undefined });
    }
  }

  function getDeviceMode(deviceId: string): LightMode {
    if (deviceModes[deviceId]) return deviceModes[deviceId];
    const s = newDevices[deviceId];
    if (s?.color) return "color";
    if (s?.kelvin) return "kelvin";
    return "color";
  }

  const toModelDevices = (
    devs: Record<string, DeviceState>
  ): Record<string, lights.DeviceState> => {
    const result: Record<string, lights.DeviceState> = {};
    for (const [k, v] of Object.entries(devs)) {
      result[k] = new lights.DeviceState({
        ...v,
        color: v.color ? new lights.Color(v.color) : undefined,
      });
    }
    return result;
  };

  const handleSave = async () => {
    if (!newName.trim()) return;
    const finalDevices = { ...newDevices };

    if (globalColor) {
      for (const id of Object.keys(finalDevices)) {
        if (finalDevices[id].on) {
          const dev = devices.find((d) => d.id === id);
          if (dev?.supportsColor) {
            finalDevices[id] = { ...finalDevices[id], color: globalColorValue, kelvin: undefined };
          }
        }
      }
    } else if (globalTemp) {
      for (const id of Object.keys(finalDevices)) {
        if (finalDevices[id].on) {
          finalDevices[id] = {
            ...finalDevices[id],
            kelvin: globalKelvin,
            brightness: globalBrightness,
            color: undefined,
          };
        }
      }
    }

    try {
      if (editing) {
        await UpdateScene(
          new store.Scene({
            ...editing,
            name: newName,
            trigger: newTrigger,
            devices: toModelDevices(finalDevices),
          })
        );
      } else {
        await CreateScene(
          new main.CreateSceneRequest({
            name: newName,
            trigger: newTrigger,
            devices: toModelDevices(finalDevices),
          })
        );
      }
      cancelEdit();
      refresh();
    } catch (e) {
      console.error("Failed to save scene:", e);
    }
  };

  const handleDelete = async (id: string) => {
    try { await DeleteScene(id); refresh(); } catch (e) { console.error(e); }
  };

  const handleActivate = async (id: string) => {
    const scene = scenes.find((s) => s.id === id);
    if (scene?.devices) lightActions.applySceneStates(scene.devices);
    setActiveScene(id);
    try {
      await ActivateScene(id);
    } catch (e) {
      console.error("Failed to activate scene:", e);
      lightActions.refreshLightStates();
    }
  };

  const handleSetGlobalColor = (val: boolean) => {
    setGlobalColor(val);
    if (val) setGlobalTemp(false);
  };

  const handleSetGlobalTemp = (val: boolean) => {
    setGlobalTemp(val);
    if (val) setGlobalColor(false);
  };

  const anyDeviceSupportsColor = devices.some((d) => d.supportsColor);
  const isEditing = creating || editing !== null;

  return (
    <div className="space-y-8">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">Scenes</h2>
          <p className="text-muted-foreground mt-1">
            Configure what happens when your camera turns on or off
          </p>
        </div>
        {!isEditing && (
          <Button onClick={startCreate}>
            <Plus className="h-4 w-4" />
            New Scene
          </Button>
        )}
      </div>

      {isEditing && (
        <Card className="space-y-6">
          <div className="flex items-center justify-between">
            <h3 className="text-lg font-semibold">
              {creating ? "Create Scene" : "Edit Scene"}
            </h3>
            <Button variant="ghost" size="icon" onClick={cancelEdit}>
              <X className="h-4 w-4" />
            </Button>
          </div>

          <div className="space-y-4">
            {/* Name */}
            <div>
              <label className="text-sm font-medium mb-1.5 block">Scene Name</label>
              <input
                type="text"
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                placeholder="e.g., On Air, Meeting, Off Duty"
                className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
              />
            </div>

            {/* Trigger */}
            <div>
              <label className="text-sm font-medium mb-1.5 block">Trigger</label>
              <div className="flex gap-3">
                <button
                  onClick={() => setNewTrigger("camera_on")}
                  className={`flex-1 flex items-center justify-center gap-2 rounded-lg border p-3 text-sm transition-colors ${
                    newTrigger === "camera_on"
                      ? "border-primary bg-primary/10 text-primary"
                      : "border-border hover:bg-secondary"
                  }`}
                >
                  <Camera className="h-4 w-4" />
                  Camera On
                </button>
                <button
                  onClick={() => setNewTrigger("camera_off")}
                  className={`flex-1 flex items-center justify-center gap-2 rounded-lg border p-3 text-sm transition-colors ${
                    newTrigger === "camera_off"
                      ? "border-primary bg-primary/10 text-primary"
                      : "border-border hover:bg-secondary"
                  }`}
                >
                  <CameraOff className="h-4 w-4" />
                  Camera Off
                </button>
              </div>
            </div>

            {/* Global Color */}
            {anyDeviceSupportsColor && (
              <div className="rounded-lg border border-border p-4 space-y-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <Palette className="h-4 w-4 text-muted-foreground" />
                    <div>
                      <p className="text-sm font-medium">Global Color</p>
                      <p className="text-xs text-muted-foreground">
                        {globalColor
                          ? "Same color for all color-capable lights"
                          : "Set color per light individually"}
                      </p>
                    </div>
                  </div>
                  <Toggle checked={globalColor} onChange={handleSetGlobalColor} />
                </div>
                {globalColor && (
                  <div className="pt-1 flex items-center gap-3">
                    <label className="text-xs text-muted-foreground flex items-center gap-2">
                      <ColorSwatch color={globalColorValue} />
                      Color
                    </label>
                    <ColorPicker value={globalColorValue} onChange={setGlobalColorValue} />
                    <span className="text-xs text-muted-foreground">
                      {Math.round(globalColorValue.b * 100)}% brightness
                    </span>
                  </div>
                )}
              </div>
            )}

            {/* Global Temperature */}
            <div className="rounded-lg border border-border p-4 space-y-3">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <Thermometer className="h-4 w-4 text-muted-foreground" />
                  <div>
                    <p className="text-sm font-medium">Global Temperature</p>
                    <p className="text-xs text-muted-foreground">
                      {globalTemp
                        ? "Same temperature for all lights"
                        : "Set temperature per light individually"}
                    </p>
                  </div>
                </div>
                <Toggle checked={globalTemp} onChange={handleSetGlobalTemp} />
              </div>
              {globalTemp && (
                <div className="pt-1 flex items-center gap-3">
                  <label className="text-xs text-muted-foreground flex items-center gap-2">
                    <TempSwatch kelvin={globalKelvin} />
                    Temp
                  </label>
                  <TemperaturePicker
                    kelvin={globalKelvin}
                    brightness={globalBrightness}
                    onChange={(k, b) => { setGlobalKelvin(k); setGlobalBrightness(b); }}
                  />
                  <span className="text-xs text-muted-foreground">
                    {globalKelvin}K · {Math.round(globalBrightness * 100)}%
                  </span>
                </div>
              )}
            </div>

            {/* Per-device list */}
            <div>
              <label className="text-sm font-medium mb-1.5 block">Lights in Scene</label>
              {devices.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                  No lights discovered. Go to the Lights page to scan your network first.
                </p>
              ) : (
                <div className="space-y-5">
                  {Object.entries(groupByBrand(devices)).map(([brand, brandDevices]) => {
                    const info = brandInfo[brand] || { color: "text-foreground", label: brand };
                    return (
                      <div key={brand}>
                        <div className="flex items-center gap-2 mb-2">
                          <Lightbulb className={`h-4 w-4 ${info.color}`} />
                          <span className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                            {info.label}
                          </span>
                        </div>
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                          {brandDevices.map((device) => {
                            const included = !!newDevices[device.id];
                            const devState = newDevices[device.id];
                            const hasBoth = device.supportsColor && device.supportsKelvin;
                            const devMode = getDeviceMode(device.id);
                            const globalOverride = globalColor || globalTemp;

                            return (
                              <div
                                key={device.id}
                                className={`rounded-lg border p-4 transition-colors ${
                                  included ? "border-primary/40 bg-primary/5" : "border-border"
                                }`}
                              >
                                <div className="flex items-center justify-between">
                                  <div className="flex items-center gap-3">
                                    <Toggle
                                      checked={included}
                                      onChange={() => toggleDeviceInScene(device.id)}
                                    />
                                    <p className="text-sm font-medium">{device.name}</p>
                                  </div>
                                  {included && (
                                    <RockerSwitch
                                      checked={devState?.on ?? true}
                                      onChange={(on) => updateDeviceState(device.id, { on })}
                                      color={deviceSceneColor(devState)}
                                    />
                                  )}
                                </div>

                                {included && devState?.on && (
                                  <div className="mt-3 pl-14 space-y-3">
                                    {/* Mode toggle — only for devices with both, when not in global override */}
                                    {hasBoth && !globalOverride && (
                                      <ModeToggle
                                        mode={devMode}
                                        onSwitch={(m) => switchDeviceMode(device.id, m)}
                                      />
                                    )}

                                    {/* Color picker */}
                                    {device.supportsColor && !globalColor && devMode === "color" && (
                                      <div className="flex items-center gap-3">
                                        <label className="text-xs text-muted-foreground w-10 shrink-0">
                                          Color
                                        </label>
                                        <ColorPicker
                                          value={devState?.color ?? null}
                                          onChange={(c) =>
                                            updateDeviceState(device.id, {
                                              color: c,
                                              brightness: c.b,
                                              kelvin: undefined,
                                            })
                                          }
                                        />
                                        {devState?.color && (
                                          <span className="text-xs text-muted-foreground">
                                            {Math.round(devState.color.b * 100)}%
                                          </span>
                                        )}
                                      </div>
                                    )}

                                    {/* Temperature picker */}
                                    {device.supportsKelvin && !globalTemp && !globalColor && devMode === "kelvin" && (
                                      <div className="flex items-center gap-3">
                                        <label className="text-xs text-muted-foreground w-10 shrink-0">
                                          Temp
                                        </label>
                                        <TemperaturePicker
                                          kelvin={devState?.kelvin ?? 4000}
                                          brightness={devState?.brightness ?? 0.8}
                                          minBrightness={device.brand === "elgato" ? 0.03 : 0}
                                          minKelvin={device.minKelvin ? device.minKelvin : undefined}
                                          maxKelvin={device.maxKelvin ? device.maxKelvin : undefined}
                                          kelvinStep={device.kelvinStep && device.kelvinStep > 1 ? device.kelvinStep : 1}
                                          onChange={(k, b) =>
                                            updateDeviceState(device.id, {
                                              kelvin: k,
                                              brightness: b,
                                              color: undefined,
                                            })
                                          }
                                        />
                                        <span className="text-xs text-muted-foreground">
                                          {devState?.kelvin ?? 4000}K
                                        </span>
                                      </div>
                                    )}

                                    {/* Brightness slider — only for color-only mode when not using iro */}
                                    {!devState?.color && devMode !== "kelvin" && device.supportsColor && !globalColor && (
                                      <div className="text-xs text-muted-foreground">
                                        Pick a color above to set brightness.
                                      </div>
                                    )}
                                  </div>
                                )}
                              </div>
                            );
                          })}
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          </div>

          <div className="flex justify-end gap-3 pt-2">
            <Button variant="outline" onClick={cancelEdit}>Cancel</Button>
            <Button onClick={handleSave} disabled={!newName.trim()}>
              <Check className="h-4 w-4" />
              {creating ? "Create Scene" : "Save Changes"}
            </Button>
          </div>
        </Card>
      )}

      {!isEditing && scenes.length === 0 && (
        <Card className="flex flex-col items-center justify-center py-16 text-center">
          <Film className="h-12 w-12 text-muted-foreground/40 mb-4" />
          <h3 className="text-lg font-semibold">No Scenes Yet</h3>
          <p className="text-sm text-muted-foreground mt-2 max-w-md">
            Create a scene to define what your lights should do when your camera turns on or off.
          </p>
          <Button onClick={startCreate} className="mt-6">
            <Plus className="h-4 w-4" />
            Create Your First Scene
          </Button>
        </Card>
      )}

      {!isEditing &&
        scenes.map((scene) => (
          <Card key={scene.id} className="p-4">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                <div
                  className={`h-10 w-10 rounded-lg flex items-center justify-center ${
                    scene.trigger === "camera_on" ? "bg-success/20" : "bg-muted"
                  }`}
                >
                  {scene.trigger === "camera_on" ? (
                    <Camera className="h-5 w-5 text-success" />
                  ) : (
                    <CameraOff className="h-5 w-5 text-muted-foreground" />
                  )}
                </div>
                <div>
                  <div className="flex items-center gap-2">
                    <p className="font-semibold">{scene.name}</p>
                    {activeScene === scene.id && (
                      <Badge variant="success">Active</Badge>
                    )}
                  </div>
                  <p className="text-xs text-muted-foreground">
                    Trigger: {scene.trigger === "camera_on" ? "Camera On" : "Camera Off"}{" "}
                    · {Object.keys(scene.devices || {}).length} light(s)
                  </p>
                </div>
              </div>
              <div className="flex gap-1">
                <Button variant="ghost" size="icon" onClick={() => handleActivate(scene.id)} title="Activate">
                  <Play className="h-4 w-4 text-primary" />
                </Button>
                <Button variant="ghost" size="icon" onClick={() => startEdit(scene)} title="Edit">
                  <Pencil className="h-4 w-4" />
                </Button>
                <Button variant="ghost" size="icon" onClick={() => handleDelete(scene.id)} title="Delete">
                  <Trash2 className="h-4 w-4 text-destructive" />
                </Button>
              </div>
            </div>
          </Card>
        ))}
    </div>
  );
}
