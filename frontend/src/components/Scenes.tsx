import { useState, useEffect, useCallback, useMemo } from "react";
import {
  Plus,
  Camera,
  CameraOff,
  Film,
  Play,
  X,
  Check,
  Lightbulb,
  Palette,
} from "lucide-react";
import { Card } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Badge } from "@/components/ui/Badge";
import { LightCard } from "@/components/ui/LightCard";
import { ColorWheel, KelvinSlider } from "@/components/ui/ColorPanel";
import { SegmentedControl } from "@/components/ui/SegmentedControl";
import type { Device, DeviceState, Color } from "@/lib/types";
import { hueToKelvin, kelvinToHSB } from "@/lib/utils";
import { getBrandInfo, groupByBrand } from "@/lib/brands";
import { sceneSwatchBackground } from "@/lib/sceneColors";
import { SceneRow } from "@/components/SceneRow";
import { useLightStore, lightActions } from "@/hooks/useLightStore";
import { lights, main, store } from "../../wailsjs/go/models";
import {
  GetScenes,
  CreateScene,
  UpdateScene,
  DeleteScene,
  ActivateScene,
} from "../../wailsjs/go/main/App";

type Scene = store.Scene;
type LightMode = "color" | "kelvin";

const DEFAULT_COLOR: Color = { h: 30, s: 1, b: 1 };
type GlobalMode = "none" | "color" | "kelvin";

/** Build DeviceState from store values. Store uses brightness 0-100; DeviceState uses 0-1. */
function captureDeviceState(
  deviceOn: Record<string, boolean>,
  brightness: Record<string, number>,
  kelvin: Record<string, number>,
  color: Record<string, Color>,
  deviceId: string
): DeviceState {
  const on = deviceOn[deviceId] ?? false;
  const bright = brightness[deviceId] ?? 80;
  return {
    on,
    brightness: bright <= 1 ? bright : bright / 100,
    kelvin: kelvin[deviceId],
    color: color[deviceId],
  };
}

export function Scenes() {
  const { devices, deviceOn, brightness, kelvin, color, activeScene: storeActiveScene } = useLightStore();
  const [scenes, setScenes] = useState<Scene[]>([]);
  const [editing, setEditing] = useState<Scene | null>(null);
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState("");
  const [newTrigger, setNewTrigger] = useState("");
  const [newDevices, setNewDevices] = useState<Record<string, DeviceState>>({});

  // Snapshot of light states before edit mode; restored when exiting.
  const [preEditLightStates, setPreEditLightStates] = useState<Record<string, DeviceState>>({});

  // Per-device mode overrides (keyed by device id).
  const [deviceModes, setDeviceModes] = useState<Record<string, LightMode>>({});

  // Global override — one mode at a time; brightness stays per-device.
  const [globalMode, setGlobalMode] = useState<GlobalMode>("none");
  const [globalColorValue, setGlobalColorValue] = useState<Color>(DEFAULT_COLOR);
  const [globalKelvin, setGlobalKelvin] = useState(4000);

  // True when the editor has unsaved changes relative to the saved scene.
  // Always true when creating (nothing to compare against).
  const isDirty = useMemo(() => {
    if (creating) return true;
    if (!editing) return false;

    if (newName.trim() !== editing.name) return true;
    if (newTrigger !== editing.trigger) return true;

    // Deep-compare device maps via JSON — both are plain serialisable objects.
    if (JSON.stringify(newDevices) !== JSON.stringify(editing.devices ?? {})) return true;

    // Reconstruct the saved global mode from the persisted fields.
    const savedGlobalMode: GlobalMode = editing.globalColor
      ? "color"
      : editing.globalKelvin != null
      ? "kelvin"
      : "none";
    if (globalMode !== savedGlobalMode) return true;
    if (globalMode === "color" && JSON.stringify(globalColorValue) !== JSON.stringify(editing.globalColor)) return true;
    if (globalMode === "kelvin" && globalKelvin !== editing.globalKelvin) return true;

    return false;
  }, [creating, editing, newName, newTrigger, newDevices, globalMode, globalColorValue, globalKelvin]);

  const refresh = useCallback(() => {
    GetScenes()
      .then((s) => setScenes(s || []))
      .catch(() => {});
    lightActions.refreshDevices();
    lightActions.hydrateActiveScene();
  }, []);

  useEffect(() => { refresh(); }, [refresh]);

  const activeSceneId = storeActiveScene?.id ?? "";

  const startCreate = () => {
    setCreating(true);
    setNewName("");
    setNewTrigger("");
    setNewDevices({});
    setDeviceModes({});
    setGlobalMode("none");
    setGlobalColorValue(DEFAULT_COLOR);
    setGlobalKelvin(4000);
    setEditing(null);
    setPreEditLightStates({});
  };

  const startEdit = (scene: Scene) => {
    setEditing(scene);
    setNewName(scene.name);
    setNewTrigger(scene.trigger);
    const devs = scene.devices || {};
    setNewDevices(devs);
    setDeviceModes({});
    setCreating(false);

    // Restore global override that was saved with the scene; fall back to none.
    if (scene.globalColor) {
      setGlobalMode("color");
      setGlobalColorValue(scene.globalColor);
      setGlobalKelvin(4000);
    } else if (scene.globalKelvin != null) {
      setGlobalMode("kelvin");
      setGlobalKelvin(scene.globalKelvin);
      setGlobalColorValue(DEFAULT_COLOR);
    } else {
      setGlobalMode("none");
      setGlobalColorValue(DEFAULT_COLOR);
      setGlobalKelvin(4000);
    }

    // Capture current light states to restore on exit. Editing never activates the scene.
    const captured: Record<string, DeviceState> = {};
    for (const id of Object.keys(devs)) {
      captured[id] = captureDeviceState(deviceOn, brightness, kelvin, color, id);
    }
    setPreEditLightStates(captured);

    // Apply scene to lights immediately for live preview (no ActivateScene).
    lightActions.previewSceneStates(devs);
  };

  const exitEdit = useCallback(async (restoreLights: boolean) => {
    if (restoreLights && Object.keys(preEditLightStates).length > 0) {
      await lightActions.restoreLightStates(preEditLightStates, devices);
    }
    setEditing(null);
    setCreating(false);
    setPreEditLightStates({});
  }, [preEditLightStates, devices]);

  const cancelEdit = async () => {
    try {
      await exitEdit(true);
    } catch (e) {
      console.error("Failed to restore light states on cancel:", e);
    }
  };

  const toggleDeviceInScene = (deviceId: string) => {
    setNewDevices((prev) => {
      if (prev[deviceId]) {
        const next = { ...prev };
        delete next[deviceId];
        return next;
      }
      return { ...prev, [deviceId]: { on: true, brightness: 0.8 } };
    });
    // When adding a device in create mode, capture its current state for restore on exit.
    if (creating && !newDevices[deviceId] && !preEditLightStates[deviceId]) {
      setPreEditLightStates((p) => ({
        ...p,
        [deviceId]: captureDeviceState(deviceOn, brightness, kelvin, color, deviceId),
      }));
    }
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

    if (globalMode === "color") {
      for (const id of Object.keys(finalDevices)) {
        if (finalDevices[id].on) {
          const dev = devices.find((d) => d.id === id);
          if (dev?.supportsColor) {
            // RGB device: preserve per-device brightness, override hue + saturation.
            finalDevices[id] = {
              ...finalDevices[id],
              color: { ...globalColorValue, b: finalDevices[id].brightness },
              kelvin: undefined,
            };
          } else if (dev?.supportsKelvin) {
            // Kelvin-only device: extrapolate CCT from the selected hue.
            const k = Math.max(
              dev.minKelvin ?? 2000,
              Math.min(dev.maxKelvin ?? 9000, hueToKelvin(globalColorValue.h))
            );
            finalDevices[id] = { ...finalDevices[id], kelvin: k, color: undefined };
          }
        }
      }
    } else if (globalMode === "kelvin") {
      for (const id of Object.keys(finalDevices)) {
        if (finalDevices[id].on) {
          // Preserve individual brightness; only override kelvin.
          finalDevices[id] = {
            ...finalDevices[id],
            kelvin: globalKelvin,
            color: undefined,
          };
        }
      }
    }

    const globalOverride = globalMode === "color"
      ? { globalColor: globalColorValue, globalKelvin: undefined }
      : globalMode === "kelvin"
      ? { globalColor: undefined, globalKelvin: globalKelvin }
      : { globalColor: undefined, globalKelvin: undefined };

    try {
      if (editing) {
        await UpdateScene(
          new store.Scene({
            ...editing,
            name: newName,
            trigger: newTrigger,
            devices: toModelDevices(finalDevices),
            ...globalOverride,
          })
        );
      } else {
        await CreateScene(
          new main.CreateSceneRequest({
            name: newName,
            trigger: newTrigger,
            devices: toModelDevices(finalDevices),
            ...globalOverride,
          })
        );
      }
      await exitEdit(false);
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
    if (scene) lightActions.setActiveSceneOptimistic(scene);
    try {
      await ActivateScene(id);
    } catch (e) {
      console.error("Failed to activate scene:", e);
      lightActions.refreshLightStates();
    }
  };

  const anyDeviceSupportsColor = devices.some((d) => d.supportsColor);
  const anyDeviceSupportsKelvin = devices.some((d) => d.supportsKelvin);

  // Triggers already claimed by another scene (excluding the one currently being edited).
  const takenTriggers = useMemo(() => {
    const editingId = editing?.id;
    return new Set(
      scenes
        .filter((s) => s.trigger && s.id !== editingId)
        .map((s) => s.trigger)
    );
  }, [scenes, editing]);

  /** Push a global color value to all currently included+on devices for live preview.
   *  RGB-capable devices receive the full colour; kelvin-only devices get an
   *  approximated correlated colour temperature derived from the hue. */
  function previewGlobalColor(c: Color) {
    Object.entries(newDevices).forEach(([id, state]) => {
      if (!state.on) return;
      const dev = devices.find((d) => d.id === id);
      if (!dev) return;
      if (dev.supportsColor) {
        lightActions.setColor(id, { ...c, b: state.brightness });
      } else if (dev.supportsKelvin) {
        const k = Math.max(
          dev.minKelvin ?? 2000,
          Math.min(dev.maxKelvin ?? 9000, hueToKelvin(c.h))
        );
        lightActions.setTemperature(id, k, state.brightness);
      }
    });
  }
  function previewGlobalKelvin(k: number) {
    Object.entries(newDevices).forEach(([id, state]) => {
      if (!state.on) return;
      const dev = devices.find((d) => d.id === id);
      if (!dev) return;
      if (dev.supportsKelvin) {
        const clamped = Math.max(
          dev.minKelvin ?? 2000,
          Math.min(dev.maxKelvin ?? 9000, k)
        );
        lightActions.setTemperature(id, clamped, state.brightness);
      } else if (dev.supportsColor) {
        const hsb = kelvinToHSB(k);
        lightActions.setColor(id, { ...hsb, b: state.brightness });
      }
    });
  }
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
        <Card className="space-y-6 bg-secondary">
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
                className="w-full rounded-lg bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
              />
            </div>

            {/* Trigger */}
            <div>
              <label className="text-sm font-medium mb-1.5 block">Trigger</label>
              <div className="flex gap-3">
                <button
                  onClick={() => setNewTrigger("")}
                  className={`flex-1 flex items-center justify-center gap-2 rounded-lg p-3 text-sm transition-colors ${
                    newTrigger === ""
                      ? "bg-primary/10 text-primary"
                      : "hover:bg-secondary text-muted-foreground"
                  }`}
                >
                  <Play className="h-4 w-4" />
                  Manual Only
                </button>
                {(["camera_on", "camera_off"] as const).map((t) => {
                  const taken = takenTriggers.has(t);
                  const Icon = t === "camera_on" ? Camera : CameraOff;
                  const label = t === "camera_on" ? "Camera On" : "Camera Off";
                  return (
                    <button
                      key={t}
                      onClick={() => !taken && setNewTrigger(t)}
                      disabled={taken}
                      title={taken ? "Already used by another scene" : undefined}
                      className={`flex-1 flex items-center justify-center gap-2 rounded-lg p-3 text-sm transition-colors ${
                        newTrigger === t
                          ? "bg-primary/10 text-primary"
                          : taken
                          ? "opacity-40 cursor-not-allowed text-muted-foreground"
                          : "hover:bg-secondary text-muted-foreground"
                      }`}
                    >
                      <Icon className="h-4 w-4" />
                      {label}
                    </button>
                  );
                })}
              </div>
            </div>

            {/* Global Override */}
            {(anyDeviceSupportsColor || anyDeviceSupportsKelvin) && (
              <div className="bg-card rounded-xl p-4 space-y-4">
                <div className="flex items-center gap-2">
                  <Palette className="h-4 w-4 text-muted-foreground" />
                  <p className="text-sm font-medium">Global Colors</p>
                  <p className="text-xs text-muted-foreground ml-1">— applies the same color or temperature to all lights</p>
                </div>

                {/* Mode tabs */}
                <SegmentedControl<GlobalMode>
                  options={[
                    { value: "none", label: "Off" },
                    ...(anyDeviceSupportsColor
                      ? [{ value: "color" as const, label: "Color" }]
                      : []),
                    ...(anyDeviceSupportsKelvin
                      ? [{ value: "kelvin" as const, label: "Temperature" }]
                      : []),
                  ]}
                  value={globalMode}
                  onChange={(mode) => {
                    setGlobalMode(mode);
                    if (mode === "color") previewGlobalColor(globalColorValue);
                    if (mode === "kelvin") previewGlobalKelvin(globalKelvin);
                  }}
                />

                {globalMode === "color" && (
                  <div className="flex flex-col items-center gap-2">
                    <ColorWheel
                      color={globalColorValue}
                      brightness={100}
                      onChange={(c) => {
                        setGlobalColorValue(c);
                        previewGlobalColor(c);
                      }}
                    />
                    <p className="text-xs text-muted-foreground">
                      {Math.round(globalColorValue.h)}°&thinsp;hue · {Math.round(globalColorValue.s * 100)}%&thinsp;sat
                    </p>
                  </div>
                )}

                {globalMode === "kelvin" && (
                  <div className="flex flex-col items-center gap-1">
                    <KelvinSlider
                      kelvin={globalKelvin}
                      onChange={(k) => {
                        setGlobalKelvin(k);
                        previewGlobalKelvin(k);
                      }}
                    />
                    <p className="text-center text-xs text-muted-foreground">{globalKelvin}K</p>
                  </div>
                )}
              </div>
            )}

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
                    const info = getBrandInfo(brand);
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
                            const devMode = getDeviceMode(device.id);

                            // When a global override is active, pass the global values
                            // to the card so it reflects the live preview color.
                            // For kelvin-only devices under a global color override, derive
                            // the equivalent Kelvin temperature from the hue instead.
                            const isKelvinOnly = device.supportsKelvin && !device.supportsColor;
                            const derivedKelvin = isKelvinOnly && globalMode === "color"
                              ? Math.min(
                                  device.maxKelvin ?? 9000,
                                  Math.max(device.minKelvin ?? 2000, hueToKelvin(globalColorValue.h))
                                )
                              : undefined;

                            const displayColor =
                              globalMode === "color" && !isKelvinOnly ? globalColorValue
                              : globalMode === "kelvin" ? undefined
                              : devState?.color;
                            const displayKelvin =
                              derivedKelvin ??
                              (globalMode === "kelvin" ? globalKelvin : (devState?.kelvin ?? 4000));
                            const displayMode: LightMode =
                              globalMode === "color" && isKelvinOnly ? "kelvin"
                              : globalMode === "color" ? "color"
                              : globalMode === "kelvin" ? "kelvin"
                              : devMode;

                            if (!included) {
                              return (
                                <button
                                  key={device.id}
                                  type="button"
                                  onClick={() => toggleDeviceInScene(device.id)}
                                  className="flex items-center gap-3 rounded-xl bg-card/60 px-4 py-3 text-sm text-muted-foreground hover:text-foreground hover:bg-card transition-all w-full opacity-50 hover:opacity-100"
                                >
                                  <div className="h-10 w-10 rounded-full bg-white/5 flex items-center justify-center shrink-0">
                                    <Lightbulb className="h-5 w-5" />
                                  </div>
                                  <span className="flex-1 text-left font-medium">{device.name}</span>
                                  <Plus className="h-4 w-4 shrink-0" />
                                </button>
                              );
                            }

                            return (
                              <div key={device.id} className="relative group/scenecard">
                                <LightCard
                                  device={device}
                                  on={devState?.on ?? true}
                                  brightness={Math.round((devState?.brightness ?? 0.8) * 100)}
                                  kelvin={displayKelvin}
                                  color={displayColor}
                                  mode={displayMode}
                                  colorLocked={globalMode !== "none"}
                                  onToggle={(on) => {
                                    updateDeviceState(device.id, { on });
                                    lightActions.toggleLight(device.id, on);
                                  }}
                                  onBrightness={(value) => {
                                    const k = newDevices[device.id]?.kelvin;
                                    updateDeviceState(device.id, { brightness: value / 100 });
                                    if (k) {
                                      lightActions.setTemperature(device.id, k, value / 100);
                                    } else {
                                      lightActions.setBrightness(device.id, value);
                                    }
                                  }}
                                  onModeSwitch={(m) => switchDeviceMode(device.id, m)}
                                  onKelvin={(k) => {
                                    updateDeviceState(device.id, { kelvin: k, color: undefined });
                                    lightActions.setTemperature(
                                      device.id,
                                      k,
                                      newDevices[device.id]?.brightness ?? 0.8
                                    );
                                  }}
                                  onColor={(c) => {
                                    updateDeviceState(device.id, {
                                      color: c,
                                      brightness: c.b,
                                      kelvin: undefined,
                                    });
                                    lightActions.setColor(device.id, c);
                                  }}
                                />
                                {/* Remove from scene — appears on hover */}
                                <button
                                  type="button"
                                  title="Remove from scene"
                                  onClick={() => toggleDeviceInScene(device.id)}
                                  className="absolute -top-1.5 -right-1.5 z-30 h-5 w-5 rounded-full bg-card border border-border shadow-sm flex items-center justify-center opacity-0 group-hover/scenecard:opacity-100 transition-opacity"
                                >
                                  <X className="h-3 w-3 text-muted-foreground" />
                                </button>
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
            <Button onClick={handleSave} disabled={!newName.trim() || !isDirty}>
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
          <SceneRow
            key={scene.id}
            scene={scene}
            isActive={activeSceneId === scene.id}
            onActivate={() => handleActivate(scene.id)}
            onEdit={() => startEdit(scene)}
            onDelete={() => handleDelete(scene.id)}
          />
        ))}
    </div>
  );
}
