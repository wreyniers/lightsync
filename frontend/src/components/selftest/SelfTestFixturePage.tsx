import { useEffect, useState } from "react";
import { Events } from "@wailsio/runtime";

/**
 * Fullscreen fixture display rendered in its own borderless window. It
 * receives the current fixture's image URL via the `selftest:fixture-url`
 * Wails event and paints only that image — nothing else. The backend captures
 * this window's screen real estate and feeds the frame through the extraction
 * pipeline.
 *
 * Handshake protocol (prevents the cold-boot race where the backend emits
 * `selftest:fixture-url` before React has mounted and subscribed, which
 * caused the very first fixture of a suite run to capture only the empty
 * placeholder):
 *
 *   1. On mount, once we've called `Events.On("selftest:fixture-url", …)`,
 *      we emit `selftest:page-ready`. The backend waits for this before
 *      sending the first URL.
 *   2. On every `<img>` `onLoad`, we emit `selftest:image-loaded` with the
 *      fixture id. The backend waits for this (per fixture) before the
 *      first capture so it never samples a half-painted window.
 */
export function SelfTestFixturePage() {
  const [imgUrl, setImgUrl] = useState<string | null>(null);
  const [fixtureId, setFixtureId] = useState<string | null>(null);

  useEffect(() => {
    const off = Events.On("selftest:fixture-url", (e) => {
      const data = e.data as { id?: string; url?: string };
      if (data?.url) setImgUrl(data.url);
      if (data?.id) setFixtureId(data.id);
    });
    // IMPORTANT: emit ready AFTER the subscription above is installed, so
    // the backend's first fixture-url emission is guaranteed to land on a
    // live listener. Without this, the initial suite run loses its first
    // URL event to a mount race and the red fixture never paints.
    Events.Emit("selftest:page-ready", null).catch(() => {});
    return () => { off?.(); };
  }, []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        window.close();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const handleImgLoad = () => {
    if (!fixtureId) return;
    Events.Emit("selftest:image-loaded", { id: fixtureId }).catch(() => {});
  };

  // NOTE: No on-screen chrome, no hint, no text. Every non-fixture pixel is
  // a potential false positive for the single-pixel extraction methods
  // (saturated / brightest) and for palette diversity scoring. The keyboard
  // handler above still listens for ESC, so the window is still dismissable
  // — it just isn't labelled. The fixture fills the viewport 1:1 so the
  // captured frame is bit-identical to the file the probe measured against.
  return (
    <div className="fixed inset-0 bg-black">
      {imgUrl ? (
        <img
          src={imgUrl}
          alt={fixtureId ?? "self-test fixture"}
          className="w-full h-full object-fill select-none pointer-events-none"
          draggable={false}
          onLoad={handleImgLoad}
        />
      ) : (
        <div className="flex h-full w-full items-center justify-center text-white/40 text-sm">
          Waiting for fixture…
        </div>
      )}
    </div>
  );
}
