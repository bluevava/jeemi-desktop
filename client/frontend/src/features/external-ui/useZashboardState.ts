import { useCallback, useEffect, useState } from "react";
import { useWindowActivity } from "../../app/runtime/WindowActivityContext";
import { getZashboardState } from "../../services/externalUIBridge";
import type { ZashboardState } from "../../types/externalUI";

// Poll only local operation state, only while this surface is visible. Reading
// this state never triggers a GitHub request or a core lifecycle operation.
export function useZashboardState(watch: boolean) {
  const { isWindowVisible } = useWindowActivity();
  const [state, setState] = useState<ZashboardState | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [loadError, setLoadError] = useState(false);
  const refresh = useCallback(async () => {
    const next = await getZashboardState();
    setState(next);
    setLoadError(false);
    return next;
  }, []);
  useEffect(() => {
    if (!isWindowVisible) return;
    let active = true;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const load = async () => {
      try {
        const next = await getZashboardState();
        if (active) { setState(next); setLoadError(false); }
      } catch { if (active) setLoadError(true); }
      finally {
        if (active) {
          setLoaded(true);
          if (watch) timer = setTimeout(() => void load(), 1000);
        }
      }
    };
    void load();
    return () => { active = false; clearTimeout(timer); };
  }, [watch, isWindowVisible]);
  return { state, setState, refresh, loaded, loadError };
}
