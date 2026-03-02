import { useEffect, useState, useCallback } from "react";
import { Search, X, RefreshCw, Monitor } from "lucide-react";
import { GetWindows, GetWindowThumbnail } from "../../../wailsjs/go/main/App";
import { Button } from "@/components/ui/Button";
import type { WindowInfo } from "@/lib/types";

interface WindowPickerProps {
  selectedHwnd?: number;
  selectedTitle?: string;
  onSelect: (hwnd: number, title: string) => void;
  onClose: () => void;
}

/**
 * Modal dialog for selecting an application window as the capture source.
 * Lists all visible windows with live thumbnail previews and a search filter.
 */
export function WindowPicker({ selectedHwnd, onSelect, onClose }: WindowPickerProps) {
  const [windows, setWindows] = useState<WindowInfo[]>([]);
  const [search, setSearch] = useState("");
  const [thumbnails, setThumbnails] = useState<Record<number, string>>({});
  const [loading, setLoading] = useState(true);

  const loadWindows = useCallback(async () => {
    setLoading(true);
    try {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const wins: any[] = (await GetWindows()) || [];
      setWindows(wins);
      // Load thumbnails lazily.
      for (const w of wins.slice(0, 20)) {
        GetWindowThumbnail(w.hwnd)
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          .then((b64: any) => {
            if (b64) {
              setThumbnails((prev) => ({ ...prev, [w.hwnd]: b64 }));
            }
          })
          .catch(() => {});
      }
    } catch {
      //
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadWindows();
  }, [loadWindows]);

  const filtered = windows.filter(
    (w) =>
      w.title.toLowerCase().includes(search.toLowerCase()) ||
      w.exeName.toLowerCase().includes(search.toLowerCase())
  );

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="bg-card border border-border rounded-xl w-[520px] max-h-[600px] flex flex-col shadow-2xl">
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b border-border">
          <h3 className="font-semibold">Select Window</h3>
          <div className="flex items-center gap-2">
            <Button variant="ghost" size="icon" onClick={loadWindows} title="Refresh">
              <RefreshCw className="h-4 w-4" />
            </Button>
            <Button variant="ghost" size="icon" onClick={onClose}>
              <X className="h-4 w-4" />
            </Button>
          </div>
        </div>

        {/* Search */}
        <div className="p-3 border-b border-border">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search windows…"
              className="w-full bg-background/50 border border-border rounded-lg pl-9 pr-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-primary"
              autoFocus
            />
          </div>
        </div>

        {/* Window list */}
        <div className="flex-1 overflow-y-auto p-2 space-y-1">
          {loading && (
            <div className="text-center text-sm text-muted-foreground py-8">
              Loading windows…
            </div>
          )}
          {!loading && filtered.length === 0 && (
            <div className="text-center text-sm text-muted-foreground py-8">
              No windows found
            </div>
          )}
          {filtered.map((w) => {
            const isSelected = w.hwnd === selectedHwnd;
            const thumb = thumbnails[w.hwnd];
            return (
              <button
                key={w.hwnd}
                type="button"
                onClick={() => { onSelect(w.hwnd, w.title); onClose(); }}
                className={`w-full flex items-center gap-3 p-2.5 rounded-lg transition-all text-left ${
                  isSelected
                    ? "bg-primary/15 ring-1 ring-primary"
                    : "hover:bg-white/5"
                }`}
              >
                {/* Thumbnail or placeholder */}
                <div className="h-12 w-20 rounded shrink-0 overflow-hidden bg-background/50 flex items-center justify-center">
                  {thumb ? (
                    <img
                      src={`data:image/png;base64,${thumb}`}
                      alt={w.title}
                      className="h-full w-full object-cover"
                    />
                  ) : (
                    <Monitor className="h-5 w-5 text-muted-foreground/40" />
                  )}
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium truncate">{w.title}</p>
                  <p className="text-xs text-muted-foreground truncate">{w.exeName}</p>
                </div>
              </button>
            );
          })}
        </div>
      </div>
    </div>
  );
}
