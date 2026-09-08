import { useEffect, useState } from "react";

import { useWindowActivity } from "../../app/runtime/WindowActivityContext";
import { getAuthorizationHelperStatus } from "../../services/appBridge";
import type { ProxyAuthorizationStatus } from "../../types/authorization";
import { helperHealth } from "../authorization/helperStatus";

export function useAuthorizationHelperStatus(actionBusy: boolean) {
  const { isWindowVisible } = useWindowActivity();
  const [status, setStatus] = useState<ProxyAuthorizationStatus | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    if (!isWindowVisible || actionBusy) return;
    let active = true;
    let reading = false;
    const refresh = async () => {
      if (reading) return;
      reading = true;
      try {
        const next = await getAuthorizationHelperStatus();
        if (active) {
          setStatus(next);
          setFailed(false);
        }
      } catch {
        if (active) setFailed(true);
      } finally {
        reading = false;
      }
    };
    void refresh();
    const timer = window.setInterval(() => void refresh(), 5000);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [actionBusy, isWindowVisible]);

  return {
    present: status?.present === true,
    health: failed ? "failed" : status ? helperHealth(status) : null,
  } as const;
}
