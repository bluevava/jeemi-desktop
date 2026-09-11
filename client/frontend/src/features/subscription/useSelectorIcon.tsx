import { createContext, useContext, useEffect, useMemo, useState, type PropsWithChildren } from "react";

import { cancelSelectorIcon, getSelectorIcon } from "../../services/selectorIconBridge";
import { SelectorIconCache, waitForIconRetry } from "./selectorIconCache";

const icons = new SelectorIconCache(getSelectorIcon, cancelSelectorIcon);
const IconEnvironment = createContext({ active: true, retryKey: "" });

export function SelectorIconProvider({ active, retryKey, children }: PropsWithChildren<{ active: boolean; retryKey: string }>) {
  const [onlineRevision, setOnlineRevision] = useState(0);
  useEffect(() => {
    const online = () => setOnlineRevision((value) => value + 1);
    window.addEventListener("online", online);
    return () => window.removeEventListener("online", online);
  }, []);
  const value = useMemo(() => ({ active, retryKey: `${retryKey}:${onlineRevision}` }), [active, retryKey, onlineRevision]);
  return <IconEnvironment.Provider value={value}>{children}</IconEnvironment.Provider>;
}

export function useSelectorIcon(address: string | null): string {
  const { active, retryKey } = useContext(IconEnvironment);
  const [image, setImage] = useState({ address: "", source: "" });
  useEffect(() => {
    if (!address || !active) return;
    const controller = new AbortController();
    const load = async () => {
      for (let attempt = 0; attempt < 3; attempt++) {
        try {
          const source = await icons.load(address, controller.signal, retryKey);
          if (!controller.signal.aborted) setImage({ address, source });
          return;
        } catch {
          if (controller.signal.aborted || attempt === 2) return;
          try { await waitForIconRetry(attempt === 0 ? 1500 : 5000, controller.signal); }
          catch { return; }
        }
      }
    };
    void load();
    return () => controller.abort();
  }, [address, active, retryKey]);
  return image.address === address ? image.source : "";
}
