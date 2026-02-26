import { useRef, useEffect, useState, type RefObject } from "react";
import { createPortal } from "react-dom";
import { Trash2, X } from "lucide-react";
import { ROOM_PRESETS, getRoomIcon } from "@/lib/rooms";
import { UNASSIGNED_KEY } from "@/lib/brands";

interface RoomPanelProps {
  open: boolean;
  anchorRef: RefObject<HTMLButtonElement>;
  currentRoom: string | undefined;
  deviceName: string;
  onRoomChange: (room: string) => void;
  onRemove: () => void;
  onClose: () => void;
}

export function RoomPanel({
  open,
  anchorRef,
  currentRoom,
  deviceName,
  onRoomChange,
  onRemove,
  onClose,
}: RoomPanelProps) {
  const panelRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const [customValue, setCustomValue] = useState("");

  // Always pre-fill the input with the current room name when the panel opens,
  // whether it's a preset or a custom name — so it can always be renamed.
  useEffect(() => {
    if (!open) return;
    setCustomValue(currentRoom ?? "");
    setTimeout(() => inputRef.current?.select(), 0);
  }, [open, currentRoom]);

  // Position panel below (or above) the anchor button
  useEffect(() => {
    if (!open || !anchorRef.current || !panelRef.current) return;
    const anchor = anchorRef.current.getBoundingClientRect();
    const panel = panelRef.current;
    const panelWidth = panel.offsetWidth || 288;

    let top = anchor.bottom + 8;
    let left = anchor.left + anchor.width / 2 - panelWidth / 2;

    left = Math.max(8, Math.min(left, window.innerWidth - panelWidth - 8));

    if (top + 320 > window.innerHeight) {
      top = anchor.top - 8;
      panel.style.transform = "translateY(-100%)";
    } else {
      panel.style.transform = "";
    }

    panel.style.top = `${top}px`;
    panel.style.left = `${left}px`;
    // anchorRef is a stable RefObject whose identity never changes, so omitting
    // it from deps is intentional — we only need to reposition when open toggles.
  }, [open]);

  // Close on click outside
  useEffect(() => {
    if (!open) return;
    function onDown(e: MouseEvent) {
      if (
        panelRef.current &&
        !panelRef.current.contains(e.target as Node) &&
        anchorRef.current &&
        !anchorRef.current.contains(e.target as Node)
      ) {
        onClose();
      }
    }
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, [open, onClose, anchorRef]);

  if (!open) return null;

  function handlePreset(name: string) {
    // Selecting a preset fills the draft — does not apply until Save is clicked.
    setCustomValue(name === customValue ? "" : name);
  }

  function handleSave() {
    onRoomChange(customValue.trim());
    onClose();
  }

  return createPortal(
    <div
      ref={panelRef}
      className="fixed z-50 bg-muted rounded-xl shadow-2xl p-4 space-y-3"
      style={{ top: 0, left: 0, width: 288 }}
    >
      {/* Header */}
      <div className="flex items-center justify-between">
        <p className="text-sm font-medium truncate pr-2">{deviceName}</p>
        <button
          type="button"
          onClick={onClose}
          className="shrink-0 h-6 w-6 rounded-full flex items-center justify-center text-muted-foreground hover:text-foreground hover:bg-white/10 transition-colors"
        >
          <X className="h-3.5 w-3.5" />
        </button>
      </div>

      <p className="text-xs text-muted-foreground -mt-1">Assign a room</p>

      {/* Preset grid — fixed height so all buttons are identical in size */}
      <div className="grid grid-cols-3 gap-1.5">
        {ROOM_PRESETS.map(({ name, icon: Icon }) => {
          const active = name === customValue;
          return (
            <button
              key={name}
              type="button"
              onClick={() => handlePreset(name)}
              className={`h-[72px] flex flex-col items-center justify-center gap-1.5 rounded-lg px-1 text-xs transition-colors focus:outline-none focus:ring-2 focus:ring-ring ${
                active
                  ? "bg-primary text-primary-foreground"
                  : "bg-background/50 hover:bg-background text-muted-foreground hover:text-foreground"
              }`}
            >
              <Icon className="h-6 w-6 shrink-0" />
              <span className="leading-tight text-center">{name}</span>
            </button>
          );
        })}
      </div>

      {/* Room name input — always pre-filled so presets can be renamed */}
      <div className="flex gap-1.5">
        <input
          ref={inputRef}
          type="text"
          placeholder="Room name…"
          value={customValue}
          onChange={(e) => setCustomValue(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") handleSave();
          }}
          className="flex-1 h-8 rounded-md bg-background/50 border border-border px-2.5 text-xs placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring transition-colors text-foreground"
        />
      </div>

      {/* Divider + Remove / Save row */}
      <div className="border-t border-border pt-2 flex items-center gap-2">
        <button
          type="button"
          title="Remove light from configuration"
          onClick={() => { onClose(); onRemove(); }}
          className="h-8 w-8 shrink-0 flex items-center justify-center rounded-lg text-destructive hover:bg-destructive/10 transition-colors focus:outline-none focus:ring-2 focus:ring-destructive/50"
        >
          <Trash2 className="h-3.5 w-3.5" />
        </button>
        <button
          type="button"
          onClick={handleSave}
          className="flex-1 h-8 rounded-lg bg-primary text-primary-foreground text-xs font-medium hover:opacity-90 transition-opacity focus:outline-none focus:ring-2 focus:ring-ring"
        >
          Save
        </button>
      </div>
    </div>,
    document.body
  );
}

/** Icon + label fragment for a room group header. Renders inline so the parent
 *  can place a Badge or other elements alongside it in a flex row. */
export function RoomGroupHeader({ room }: { room: string }) {
  const Icon = getRoomIcon(room === UNASSIGNED_KEY ? undefined : room);
  const label = room === UNASSIGNED_KEY ? "Unassigned" : room;
  return (
    <>
      <Icon className="h-5 w-5 text-muted-foreground" />
      <h3 className="text-lg font-semibold">{label}</h3>
    </>
  );
}
