import { useEffect, useState } from "react";
import { MinusCircle, LogOut, X } from "lucide-react";
import { Events, Window } from "@wailsio/runtime";
import { App } from "@bindings";

/**
 * Listens for the `window:close-requested` event emitted by the Go backend
 * when the user clicks the window's X button. Shows a modal asking whether
 * to minimize to tray or quit entirely.
 */
export function CloseConfirmDialog() {
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    const off = Events.On("window:close-requested", () => setVisible(true));
    return () => off?.();
  }, []);

  if (!visible) return null;

  const handleMinimize = () => {
    setVisible(false);
    Window.Hide();
  };

  const handleQuit = () => {
    setVisible(false);
    App.QuitApp();
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center"
      style={{ background: "rgba(0,0,0,0.6)", backdropFilter: "blur(4px)" }}
    >
      <div
        className="relative w-80 rounded-2xl border border-border bg-card shadow-2xl p-6"
        style={{ boxShadow: "0 25px 60px rgba(0,0,0,0.5)" }}
      >
        {/* Dismiss without action — only hides the dialog, does not close the window */}
        <button
          type="button"
          onClick={() => setVisible(false)}
          className="absolute top-3 right-3 h-7 w-7 flex items-center justify-center rounded-lg text-muted-foreground hover:text-foreground hover:bg-white/10 transition-colors"
          title="Cancel"
        >
          <X className="h-4 w-4" />
        </button>

        <div className="mb-5">
          <h2 className="text-base font-semibold">Close LightSync?</h2>
          <p className="text-sm text-muted-foreground mt-1">
            Choose what happens when you close the window.
          </p>
        </div>

        <div className="space-y-2">
          <button
            type="button"
            onClick={handleMinimize}
            className="w-full flex items-center gap-3 rounded-xl px-4 py-3 text-left bg-secondary hover:bg-primary/10 hover:text-primary transition-colors group"
          >
            <div className="h-9 w-9 shrink-0 rounded-lg bg-primary/10 group-hover:bg-primary/20 flex items-center justify-center transition-colors">
              <MinusCircle className="h-4.5 w-4.5 text-primary" />
            </div>
            <div>
              <p className="text-sm font-medium">Minimize to Tray</p>
              <p className="text-xs text-muted-foreground">Keep running in the background</p>
            </div>
          </button>

          <button
            type="button"
            onClick={handleQuit}
            className="w-full flex items-center gap-3 rounded-xl px-4 py-3 text-left bg-secondary hover:bg-destructive/10 hover:text-destructive transition-colors group"
          >
            <div className="h-9 w-9 shrink-0 rounded-lg bg-destructive/10 group-hover:bg-destructive/20 flex items-center justify-center transition-colors">
              <LogOut className="h-4.5 w-4.5 text-destructive" />
            </div>
            <div>
              <p className="text-sm font-medium">Exit LightSync</p>
              <p className="text-xs text-muted-foreground">Quit the application entirely</p>
            </div>
          </button>
        </div>
      </div>
    </div>
  );
}
