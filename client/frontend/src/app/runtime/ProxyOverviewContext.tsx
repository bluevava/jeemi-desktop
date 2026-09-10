import {
  createContext,
  type PropsWithChildren,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import {
  getProxyRuntimeState,
  type MihomoProxyRuntimeState,
} from "../../lib/mihomo/client";
import { getSubscriptionState } from "../../services/appBridge";
import type { SubscriptionState } from "../../types/subscription";
import { useRuntimeStatus } from "./RuntimeStatusContext";
import { useWindowActivity } from "./WindowActivityContext";

interface ProxyOverviewValue {
  subscriptionState: SubscriptionState | null;
  proxyRuntime: MihomoProxyRuntimeState | null;
  syncSubscriptionState: (state: SubscriptionState) => void;
  commitProxySelection: (
    group: string,
    proxy: string,
    sessionId: string,
    subscriptionId: string,
  ) => void;
  refreshProxyRuntime: () => Promise<void>;
}

const ProxyOverviewContext = createContext<ProxyOverviewValue | null>(null);

export function ProxyOverviewProvider({ children }: PropsWithChildren) {
  const { liveDataReady, runtime } = useRuntimeStatus();
  const { isWindowVisible } = useWindowActivity();
  const [subscriptionState, setSubscriptionState] =
    useState<SubscriptionState | null>(null);
  const [proxyRuntime, setProxyRuntime] =
    useState<MihomoProxyRuntimeState | null>(null);
  const session = runtime?.mihomo.controllerSession ?? null;
  const runtimeReady = Boolean(
    runtime?.mihomo.state === "running" &&
      runtime.mihomo.controllerReady &&
      session,
  );
  const liveDataAllowed = Boolean(
    isWindowVisible && liveDataReady && runtimeReady && session,
  );
  const liveDataAllowedRef = useRef(liveDataAllowed);
  const sessionIDRef = useRef(session?.id ?? "");
  const subscriptionIDRef = useRef(runtime?.mihomo.subscriptionId ?? "");
  liveDataAllowedRef.current = liveDataAllowed;
  sessionIDRef.current = session?.id ?? "";
  subscriptionIDRef.current = runtime?.mihomo.subscriptionId ?? "";
  const subscriptionSourceKey = [
    runtime?.configuration.chainProxyFingerprint ?? "",
    runtime?.configuration.subscriptionId ?? "",
    runtime?.configuration.subscriptionRevision ?? "",
    runtime?.configuration.localConfigId ?? "",
    runtime?.configuration.localConfigRevision ?? 0,
    runtime?.configuration.localScriptId ?? "",
    runtime?.configuration.localScriptRevision ?? 0,
    runtime?.configuration.ruleProviderOverrideRevision ?? 0,
    runtime?.configuration.fallbackOverrideRevision ?? 0,
  ].join(":");

  useEffect(() => {
    let active = true;
    void getSubscriptionState()
      .then((state) => {
        if (active) setSubscriptionState(state);
      })
      .catch(() => undefined);
    return () => {
      active = false;
    };
  }, [subscriptionSourceKey]);

  const refreshProxyRuntime = useCallback(async () => {
    if (!liveDataAllowed || !session) {
      setProxyRuntime(null);
      return;
    }
    const sessionID = session.id;
    const next = await getProxyRuntimeState(session);
    if (
      liveDataAllowedRef.current &&
      sessionIDRef.current === sessionID
    ) {
      setProxyRuntime(next);
    }
  }, [liveDataAllowed, session?.id]);

  useEffect(() => {
    if (!liveDataAllowed || !session) {
      setProxyRuntime(null);
      return () => undefined;
    }
    let active = true;
    let refreshing = false;
    const refresh = async () => {
      if (refreshing) return;
      refreshing = true;
      try {
        const next = await getProxyRuntimeState(session);
        if (active) setProxyRuntime(next);
      } catch {
        if (active) setProxyRuntime(null);
      } finally {
        refreshing = false;
      }
    };
    void refresh();
    const timer = globalThis.setInterval(refresh, 10000);
    return () => {
      active = false;
      globalThis.clearInterval(timer);
    };
  }, [liveDataAllowed, session?.id]);

  const syncSubscriptionState = useCallback((state: SubscriptionState) => {
    setSubscriptionState(state);
  }, []);

  const commitProxySelection = useCallback(
    (group: string, proxy: string, sessionId: string, subscriptionId: string) => {
      if (
        !liveDataAllowedRef.current ||
        sessionIDRef.current !== sessionId ||
        subscriptionIDRef.current !== subscriptionId
      ) {
        return;
      }
      setProxyRuntime((current) =>
        current
          ? {
              ...current,
              selections: { ...current.selections, [group]: proxy },
            }
          : current,
      );
    },
    [],
  );

  const value = useMemo<ProxyOverviewValue>(
    () => ({
      subscriptionState,
      proxyRuntime,
      syncSubscriptionState,
      commitProxySelection,
      refreshProxyRuntime,
    }),
    [
      commitProxySelection,
      proxyRuntime,
      refreshProxyRuntime,
      subscriptionState,
      syncSubscriptionState,
    ],
  );

  return (
    <ProxyOverviewContext.Provider value={value}>
      {children}
    </ProxyOverviewContext.Provider>
  );
}

export function useProxyOverview(): ProxyOverviewValue {
  const value = useContext(ProxyOverviewContext);
  if (!value) {
    throw new Error("useProxyOverview must be used within ProxyOverviewProvider");
  }
  return value;
}
