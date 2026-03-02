import { Lightbulb, Check } from "lucide-react";
import { groupByRoom, UNASSIGNED_KEY } from "@/lib/brands";
import { getRoomIcon, sortedRoomKeys } from "@/lib/rooms";
import { Slider } from "@/components/ui/Slider";
import { Tooltip } from "@/components/ui/Tooltip";
import type { AssignmentStrategy, Device, ScreenSyncConfig } from "@/lib/types";

interface DevicesTabProps {
  config: ScreenSyncConfig;
  devices: Device[];
  onChange: (patch: Partial<ScreenSyncConfig>) => void;
}

const assignmentStrategies: {
  value: AssignmentStrategy;
  label: string;
  desc: string;
}[] = [
  {
    value: "identity_lock",
    label: "Identity Lock",
    desc: "Anchors each bulb to a colour. Only re-anchors on large, sustained shifts.",
  },
  {
    value: "flow_track",
    label: "Flow Track",
    desc: "Tracks per-bulb colour trajectories with EMA smoothing. Adapts gracefully.",
  },
  {
    value: "scene_cut_remap",
    label: "Scene Cut Remap",
    desc: "Flow-tracks normally; performs a clean global remap on detected scene cuts.",
  },
  {
    value: "zone_dominant",
    label: "Zone Dominant",
    desc: "Permanently maps bulb N to screen zone N. Fully deterministic.",
  },
];

/**
 * Lets the user select which discovered lights participate in the Screen Sync scene.
 * Also exposes the color assignment strategy and its per-strategy settings.
 */
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
  const multiColor = config.colorMode === "multi";

  return (
    <div className="space-y-5">
      <div className="flex items-center justify-between">
        <p className="text-xs text-muted-foreground">
          {selectedIds.size} of {devices.length} light{devices.length !== 1 ? "s" : ""} selected
        </p>
        <button
          type="button"
          onClick={toggleAll}
          className="text-xs text-primary hover:underline"
        >
          {allSelected ? "Deselect all" : "Select all"}
        </button>
      </div>

      {/* ── Color Assignment Strategy ─────────────────────────────────────── */}
      {multiColor && (
        <div className="space-y-3 pt-1">
          <p className="text-xs font-medium uppercase tracking-widest text-muted-foreground">
            Color Assignment
          </p>

          <div className="grid grid-cols-2 gap-1.5">
            {assignmentStrategies.map(({ value, label, desc }) => (
              <button
                key={value}
                type="button"
                onClick={() => onChange({ assignmentStrategy: value })}
                className={`flex flex-col items-start gap-1 p-3 rounded-xl text-left transition-all ${
                  config.assignmentStrategy === value
                    ? "bg-primary/15 ring-1 ring-primary/40 text-primary"
                    : "bg-background/30 text-muted-foreground hover:bg-background/60 hover:text-foreground"
                }`}
              >
                <span className="text-xs font-semibold">{label}</span>
                <span className="text-[10px] opacity-70 leading-tight">{desc}</span>
              </button>
            ))}
          </div>

          {/* ── IdentityLock settings ────────────────────────────────────── */}
          {config.assignmentStrategy === "identity_lock" && (
            <div className="rounded-xl bg-background/20 p-3 space-y-3">
              <p className="text-xs font-medium text-muted-foreground">Identity Lock</p>
              <div>
                <div className="flex items-center justify-between mb-1.5">
                  <Tooltip
                    content="How far a colour must shift (as a fraction of the full distance range) before the bulb's anchor is updated. Lower = more stable, slower to adapt. Higher = adapts faster but may swap more readily."
                    side="right"
                  >
                    <p className="text-xs text-muted-foreground cursor-help">Anchor sensitivity</p>
                  </Tooltip>
                  <span className="text-xs text-muted-foreground">
                    {config.identityLockBreachThreshold.toFixed(2)}
                  </span>
                </div>
                <Slider
                  value={config.identityLockBreachThreshold}
                  min={0.10}
                  max={0.80}
                  step={0.05}
                  onChange={(v) => onChange({ identityLockBreachThreshold: v })}
                />
                <p className="text-[10px] text-muted-foreground mt-1">
                  Lower values keep anchors stable longer. 0.30 is a good default.
                </p>
              </div>
            </div>
          )}

          {/* ── FlowTrack settings ───────────────────────────────────────── */}
          {config.assignmentStrategy === "flow_track" && (
            <div className="rounded-xl bg-background/20 p-3 space-y-3">
              <p className="text-xs font-medium text-muted-foreground">Flow Track</p>
              <div>
                <div className="flex items-center justify-between mb-1.5">
                  <Tooltip
                    content="How quickly each bulb's colour trajectory follows the screen. Lower = smoother, slower to respond to sudden changes. Higher = reacts faster but may be less stable."
                    side="right"
                  >
                    <p className="text-xs text-muted-foreground cursor-help">Tracking speed</p>
                  </Tooltip>
                  <span className="text-xs text-muted-foreground">
                    {config.flowTrackEmaAlpha.toFixed(2)}
                  </span>
                </div>
                <Slider
                  value={config.flowTrackEmaAlpha}
                  min={0.05}
                  max={1.0}
                  step={0.05}
                  onChange={(v) => onChange({ flowTrackEmaAlpha: v })}
                />
              </div>
              <div>
                <div className="flex items-center justify-between mb-1.5">
                  <Tooltip
                    content="Minimum time between full assignment solves in milliseconds. Lower = re-evaluates more often (reactive). Higher = less CPU, more stable between solves."
                    side="right"
                  >
                    <p className="text-xs text-muted-foreground cursor-help">Solve interval</p>
                  </Tooltip>
                  <span className="text-xs text-muted-foreground">
                    {config.flowTrackSolveIntervalMs} ms
                  </span>
                </div>
                <Slider
                  value={config.flowTrackSolveIntervalMs}
                  min={16}
                  max={500}
                  step={16}
                  onChange={(v) => onChange({ flowTrackSolveIntervalMs: v })}
                />
              </div>
            </div>
          )}

          {/* ── SceneCutRemap settings ───────────────────────────────────── */}
          {config.assignmentStrategy === "scene_cut_remap" && (
            <div className="rounded-xl bg-background/20 p-3 space-y-3">
              <p className="text-xs font-medium text-muted-foreground">Scene Cut Remap</p>
              <div>
                <div className="flex items-center justify-between mb-1.5">
                  <Tooltip
                    content="After a scene cut triggers a global remap, the assignment is held for this duration before resuming adaptive tracking. Prevents the tracker from immediately re-shuffling the fresh assignment."
                    side="right"
                  >
                    <p className="text-xs text-muted-foreground cursor-help">Post-cut hold</p>
                  </Tooltip>
                  <span className="text-xs text-muted-foreground">
                    {config.sceneCutRemapHoldMs} ms
                  </span>
                </div>
                <Slider
                  value={config.sceneCutRemapHoldMs}
                  min={0}
                  max={2000}
                  step={100}
                  onChange={(v) => onChange({ sceneCutRemapHoldMs: v })}
                />
                <p className="text-[10px] text-muted-foreground mt-1">
                  FlowTrack tracking speed also applies between cuts.
                </p>
              </div>
              <div>
                <div className="flex items-center justify-between mb-1.5">
                  <Tooltip
                    content="How quickly each bulb's colour trajectory follows the screen during normal (non-cut) frames."
                    side="right"
                  >
                    <p className="text-xs text-muted-foreground cursor-help">Tracking speed</p>
                  </Tooltip>
                  <span className="text-xs text-muted-foreground">
                    {config.flowTrackEmaAlpha.toFixed(2)}
                  </span>
                </div>
                <Slider
                  value={config.flowTrackEmaAlpha}
                  min={0.05}
                  max={1.0}
                  step={0.05}
                  onChange={(v) => onChange({ flowTrackEmaAlpha: v })}
                />
              </div>
            </div>
          )}

          {/* ── ZoneDominant info ────────────────────────────────────────── */}
          {config.assignmentStrategy === "zone_dominant" && (
            <div className="rounded-xl bg-background/20 p-3">
              <p className="text-[10px] text-muted-foreground leading-relaxed">
                Bulb N always shows the colour extracted from screen zone N. Works
                best with <strong>Spatial Grid</strong> multi-colour extraction.
                No configuration needed — the mapping is fixed for the session.
              </p>
            </div>
          )}
        </div>
      )}

      {roomKeys.map((roomKey) => {
        const roomDevices = grouped[roomKey];
        const RoomIcon = getRoomIcon(roomKey === UNASSIGNED_KEY ? undefined : roomKey);
        const roomLabel = roomKey === UNASSIGNED_KEY ? "Unassigned" : roomKey;
        return (
          <div key={roomKey}>
            <div className="flex items-center gap-2 mb-2">
              <RoomIcon className="h-4 w-4 text-muted-foreground" />
              <span className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                {roomLabel}
              </span>
            </div>
            <div className="space-y-1.5">
              {roomDevices.map((device) => {
                const included = selectedIds.has(device.id);
                return (
                  <button
                    key={device.id}
                    type="button"
                    onClick={() => toggleDevice(device.id)}
                    className={`w-full flex items-center gap-3 px-4 py-3 rounded-xl text-left transition-all ${
                      included
                        ? "bg-primary/10 ring-1 ring-primary/30"
                        : "bg-background/30 hover:bg-background/60 opacity-60 hover:opacity-100"
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
    </div>
  );
}
