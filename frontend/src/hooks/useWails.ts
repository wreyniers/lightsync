import { useEffect, useState, useCallback, useRef } from "react";
import { EventsOn } from "../../wailsjs/runtime/runtime";

function isRuntimeReady(): boolean {
  return typeof (window as unknown as Record<string, unknown>).runtime !== "undefined";
}

export function useWailsEvent<T>(eventName: string, initialValue: T): T {
  const [value, setValue] = useState<T>(initialValue);
  const cancelRef = useRef<(() => void) | null>(null);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    let mounted = true;

    const setup = () => {
      if (!mounted) return;
      if (!isRuntimeReady()) {
        timerRef.current = setTimeout(setup, 250);
        return;
      }
      try {
        // EventsOn returns a per-listener cancel function,
        // unlike EventsOff which removes ALL listeners for the event.
        const cancel = EventsOn(eventName, (data: T) => {
          if (mounted) setValue(data);
        });
        cancelRef.current = cancel;
      } catch {
        timerRef.current = setTimeout(setup, 250);
      }
    };
    setup();

    return () => {
      mounted = false;
      if (timerRef.current) clearTimeout(timerRef.current);
      cancelRef.current?.();
      cancelRef.current = null;
    };
  }, [eventName]);

  return value;
}

export function usePolling<T>(
  fetcher: () => Promise<T>,
  intervalMs: number,
  deps: unknown[] = []
): { data: T | null; loading: boolean; refresh: () => void } {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(() => {
    fetcher()
      .then(setData)
      .catch(console.error)
      .finally(() => setLoading(false));
  }, deps);

  useEffect(() => {
    refresh();
    const id = setInterval(refresh, intervalMs);
    return () => clearInterval(id);
  }, [refresh, intervalMs]);

  return { data, loading, refresh };
}
