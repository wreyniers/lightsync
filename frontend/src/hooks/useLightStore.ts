import { useSyncExternalStore } from "react";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import type { Device, DeviceState, Color } from "@/lib/types";
import {
  GetDevices,
  GetLightState,
  TurnOnLight,
  TurnOffLight,
  SetLightState,
  DiscoverLights,
} from "../../wailsjs/go/main/App";
import { lights } from "../../wailsjs/go/models";

interface LightStoreState {
  devices: Device[];
  deviceOn: Record<string, boolean>;
  brightness: Record<string, number>;
  kelvin: Record<string, number>;
  color: Record<string, Color>;
}

let state: LightStoreState = {
  devices: [],
  deviceOn: {},
  brightness: {},
  kelvin: {},
  color: {},
};

const listeners = new Set<() => void>();

// Debounce timers for API calls — one per device.
// The optimistic store update fires immediately; the actual HTTP call is held
// back and only sent after the user has stopped changing for SEND_DEBOUNCE_MS.
// This prevents flooding slow devices (Elgato HTTP) with one request per pixel.
const sendTimers: Record<string, ReturnType<typeof setTimeout>> = {};
const SEND_DEBOUNCE_MS = 80;

function scheduleSend(deviceId: string, fn: () => Promise<void>) {
  clearTimeout(sendTimers[deviceId]);
  sendTimers[deviceId] = setTimeout(() => { fn(); }, SEND_DEBOUNCE_MS);
}

// Tracks the last time a user action explicitly set a device's light state.
// fetchLightStates skips overwriting brightness/kelvin/color for devices that
// were updated recently — this prevents a slow device (e.g. Elgato over HTTP)
// from returning its pre-change state and clobbering the optimistic update.
const userSetAt: Record<string, number> = {};
const USER_SET_GRACE_MS = 2000;

function markUserSet(deviceId: string) {
  userSetAt[deviceId] = Date.now();
}

function recentlySetByUser(deviceId: string): boolean {
  return Date.now() - (userSetAt[deviceId] ?? 0) < USER_SET_GRACE_MS;
}

function emit() {
  state = { ...state };
  for (const l of listeners) l();
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => { listeners.delete(listener); };
}

function getSnapshot(): LightStoreState {
  return state;
}

// ── Actions ────────────────────────────────────────────────────────

async function fetchLightStates(deviceList: Device[]) {
  const results = await Promise.allSettled(
    deviceList.map((d) =>
      GetLightState(d.id).then((s) => ({ id: d.id, state: s }))
    )
  );
  const onOff = { ...state.deviceOn };
  const bright = { ...state.brightness };
  const temps = { ...state.kelvin };
  const colors = { ...state.color };
  for (const r of results) {
    if (r.status === "fulfilled") {
      const { id, state: s } = r.value;
      onOff[id] = s.on;
      // Skip brightness/kelvin/color while a user action is still in flight.
      // Slow devices (Elgato HTTP) can return stale state before the hardware
      // applies the change, which would clobber the optimistic store update.
      if (!recentlySetByUser(id)) {
        if (s.brightness != null) bright[id] = Math.round(s.brightness * 100);
        if (s.kelvin != null) temps[id] = s.kelvin;
        if (s.color != null) colors[id] = s.color;
      }
    }
  }
  state = { ...state, deviceOn: onOff, brightness: bright, kelvin: temps, color: colors };
  emit();
}

async function refreshDevices() {
  try {
    const d = await GetDevices();
    const devs = d || [];

    const results = await Promise.allSettled(
      devs.map((dev) =>
        GetLightState(dev.id).then((s) => ({ id: dev.id, state: s }))
      )
    );
    const onOff: Record<string, boolean> = {};
    const bright: Record<string, number> = {};
    const temps: Record<string, number> = {};
    const colors: Record<string, Color> = {};
    for (const r of results) {
      if (r.status === "fulfilled") {
        const { id, state: s } = r.value;
        onOff[id] = s.on;
        if (s.brightness != null) bright[id] = Math.round(s.brightness * 100);
        if (s.kelvin != null) temps[id] = s.kelvin;
        if (s.color != null) colors[id] = s.color;
      }
    }

    state = { ...state, devices: devs, deviceOn: onOff, brightness: bright, kelvin: temps, color: colors };
    emit();

    if (devs.length > 0 && Object.keys(onOff).length < devs.length) {
      setTimeout(() => fetchLightStates(state.devices), 2000);
    }
  } catch { /* swallow – caller can retry */ }
}

async function discoverLights(): Promise<Device[]> {
  const result = await DiscoverLights();
  const devs = result.devices || [];
  state = { ...state, devices: devs };
  emit();
  await fetchLightStates(devs);
  return devs;
}

async function toggleLight(deviceId: string, on: boolean) {
  state = { ...state, deviceOn: { ...state.deviceOn, [deviceId]: on } };
  emit();
  try {
    if (on) await TurnOnLight(deviceId);
    else await TurnOffLight(deviceId);
  } catch {
    state = { ...state, deviceOn: { ...state.deviceOn, [deviceId]: !on } };
    emit();
  }
}

async function setBrightness(deviceId: string, value: number) {
  markUserSet(deviceId);
  state = { ...state, brightness: { ...state.brightness, [deviceId]: value } };
  emit();
  scheduleSend(deviceId, async () => {
    const s = new lights.DeviceState({
      on: true,
      brightness: value / 100,
      kelvin: state.kelvin[deviceId] || 4000,
    });
    try {
      await SetLightState(deviceId, s);
    } catch (e) {
      console.error("Failed to set brightness:", e);
    }
  });
}

async function setKelvin(deviceId: string, value: number) {
  markUserSet(deviceId);
  const colors = { ...state.color };
  delete colors[deviceId];
  state = { ...state, kelvin: { ...state.kelvin, [deviceId]: value }, color: colors };
  emit();
  scheduleSend(deviceId, async () => {
    const s = new lights.DeviceState({
      on: true,
      brightness: (state.brightness[deviceId] || 80) / 100,
      kelvin: value,
    });
    try {
      await SetLightState(deviceId, s);
    } catch (e) {
      console.error("Failed to set temperature:", e);
    }
  });
}

// Combined kelvin + brightness update used by TemperaturePicker.
async function setTemperature(deviceId: string, kelvin: number, brightness: number) {
  markUserSet(deviceId);
  const colors = { ...state.color };
  delete colors[deviceId];
  state = {
    ...state,
    kelvin: { ...state.kelvin, [deviceId]: kelvin },
    brightness: { ...state.brightness, [deviceId]: Math.round(brightness * 100) },
    color: colors,
  };
  emit();
  scheduleSend(deviceId, async () => {
    const s = new lights.DeviceState({ on: true, brightness, kelvin });
    try {
      await SetLightState(deviceId, s);
    } catch (e) {
      console.error("Failed to set temperature:", e);
    }
  });
}

async function setColor(deviceId: string, color: Color) {
  markUserSet(deviceId);
  // b from the iro value slider IS the brightness; keep both in sync.
  state = {
    ...state,
    color: { ...state.color, [deviceId]: color },
    brightness: { ...state.brightness, [deviceId]: Math.round(color.b * 100) },
  };
  emit();
  scheduleSend(deviceId, async () => {
    const s = new lights.DeviceState({
      on: true,
      brightness: color.b,
      color: new lights.Color(color),
    });
    try {
      await SetLightState(deviceId, s);
    } catch (e) {
      console.error("Failed to set color:", e);
    }
  });
}

function applySceneStates(sceneDevices: Record<string, DeviceState>) {
  const onOff = { ...state.deviceOn };
  const bright = { ...state.brightness };
  const temps = { ...state.kelvin };
  const colors = { ...state.color };
  for (const [id, ds] of Object.entries(sceneDevices)) {
    onOff[id] = ds.on;
    if (ds.brightness != null) bright[id] = Math.round(ds.brightness * 100);
    if (ds.kelvin != null) temps[id] = ds.kelvin;
    if (ds.color != null) colors[id] = ds.color;
  }
  state = { ...state, deviceOn: onOff, brightness: bright, kelvin: temps, color: colors };
  emit();
}

export const lightActions = {
  refreshDevices,
  discoverLights,
  toggleLight,
  setBrightness,
  setKelvin,
  setTemperature,
  setColor,
  applySceneStates,
  refreshLightStates: () => fetchLightStates(state.devices),
};

export function useLightStore(): LightStoreState {
  return useSyncExternalStore(subscribe, getSnapshot);
}

// Auto-refresh when the backend triggers a scene via camera change.
// Staggered: quick check at 500ms, follow-up at 2s to catch stragglers.
function setupCameraListener() {
  try {
    EventsOn("camera:state", () => {
      setTimeout(() => fetchLightStates(state.devices), 500);
      setTimeout(() => fetchLightStates(state.devices), 2000);
    });
  } catch {
    setTimeout(setupCameraListener, 500);
  }
}
setupCameraListener();
