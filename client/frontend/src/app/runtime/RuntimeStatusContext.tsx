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
import { App } from "antd";
import { useTranslation } from "react-i18next";
import { useProxyAuthorization } from "../../features/authorization/ProxyAuthorizationDialog";
import { authorizationErrorKey } from "../../features/authorization/authorizationFlow";

import {
  getRuntimePreferences,
  getProxyAuthorization,
  removeAuthorizationHelper as removeHelper,
  getBootstrapState,
  refreshRuntimeStatus,
  restartProxy,
  saveRuntimePreferences as persistRuntimePreferences,
  startProxy,
  stopProxy,
} from "../../services/appBridge";
import type {
  BootstrapState,
  LogLevel,
  OutboundMode,
  RuntimePreferences,
  RuntimeStatus,
} from "../../types/runtime";
import {
  runtimeActionReachedTarget,
  type RuntimeActionKind,
} from "./runtimeActionState";
import { useWindowActivity } from "./WindowActivityContext";

interface RuntimeStatusValue {
  bootstrap: BootstrapState | null;
  runtime: RuntimeStatus | null;
  liveDataReady: boolean;
  loading: boolean;
  actionBusy: boolean;
  actionError: boolean;
  actionKind: RuntimeActionKind | null;
  removingAuthorization: boolean;
  runtimePreferences: RuntimePreferences | null;
  preferencesLoading: boolean;
  preferencesBusy: boolean;
  preferencesError: boolean;
  refresh: () => Promise<void>;
  start: () => Promise<void>;
  stop: () => Promise<void>;
  restart: () => Promise<void>;
  removeAuthorization: () => Promise<void>;
  savePreferences: (input: RuntimePreferences) => Promise<RuntimePreferences>;
  setLogLevel: (level: LogLevel) => Promise<RuntimePreferences>;
  setOutboundMode: (mode: OutboundMode) => Promise<RuntimePreferences>;
}

const RuntimeStatusContext = createContext<RuntimeStatusValue | null>(null);

export function RuntimeStatusProvider({ children }: PropsWithChildren) {
  const { t } = useTranslation();
  const { modal, message } = App.useApp();
  const authorization = useProxyAuthorization();
  const { isWindowVisible } = useWindowActivity();
  const [bootstrap, setBootstrap] = useState<BootstrapState | null>(null);
  const [loading, setLoading] = useState(true);
  const [actionKind, setActionKind] = useState<RuntimeActionKind | null>(null);
  const actionInFlight = useRef(false);
  const [failedActionKind, setFailedActionKind] =
    useState<RuntimeActionKind | null>(null);
  const [runtimePreferences, setRuntimePreferences] =
    useState<RuntimePreferences | null>(null);
  const [preferencesLoading, setPreferencesLoading] = useState(true);
  const [preferencesBusy, setPreferencesBusy] = useState(false);
  const [preferencesError, setPreferencesError] = useState(false);
  const [liveDataReady, setLiveDataReady] = useState(false);
  const [removingAuthorization, setRemovingAuthorization] = useState(false);
  const actionBusy = actionKind !== null || removingAuthorization;
  const actionError = failedActionKind !== null;

  const loadBootstrap = useCallback(async () => {
    setLoading(true);
    try {
      setBootstrap(await getBootstrapState());
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadBootstrap();
  }, [loadBootstrap]);

  useEffect(() => {
    let active = true;
    setPreferencesLoading(true);
    void getRuntimePreferences()
      .then((preferences) => {
        if (active) {
          setRuntimePreferences(preferences);
          setPreferencesError(false);
        }
      })
      .catch(() => {
        if (active) setPreferencesError(true);
      })
      .finally(() => {
        if (active) setPreferencesLoading(false);
      });
    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    setLiveDataReady(false);
    if (!isWindowVisible) return () => undefined;
    let active = true;
    let refreshing = false;
    const updateRuntime = async () => {
      if (refreshing) return;
      refreshing = true;
      try {
        const runtime = await refreshRuntimeStatus();
        if (!active) return;
        setBootstrap((current) =>
          current
            ? { ...current, runtime }
            : { app: { name: "Jeemi", version: "0.1.0-dev" }, runtime },
        );
        setLiveDataReady(true);
      } catch {
        // Keep the last trustworthy snapshot during a transient native error.
      } finally {
        refreshing = false;
      }
    };
    void updateRuntime();
    const timer = window.setInterval(() => {
      void updateRuntime();
    }, 2000);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [isWindowVisible]);

  useEffect(() => {
    const mihomoState = bootstrap?.runtime.mihomo.state;
    if (
      failedActionKind &&
      mihomoState &&
      runtimeActionReachedTarget(failedActionKind, mihomoState)
    ) {
      setFailedActionKind(null);
    }
  }, [bootstrap?.runtime.mihomo.state, failedActionKind]);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const runtime = await refreshRuntimeStatus();
      setBootstrap((current) =>
        current
          ? { ...current, runtime }
          : { app: { name: "Jeemi", version: "0.1.0-dev" }, runtime },
      );
    } finally {
      setLoading(false);
    }
  }, []);

  const runAction = useCallback(
    async (
      kind: RuntimeActionKind,
      operation: () => Promise<RuntimeStatus>,
    ) => {
      if (actionInFlight.current) return;
      actionInFlight.current = true;
      setActionKind(kind);
      setFailedActionKind(null);
      try {
        if (kind !== "stop" && !(await authorization.flow.request())) return;
        const runtime = await operation();
        setBootstrap((current) =>
          current
            ? { ...current, runtime }
            : { app: { name: "Jeemi", version: "0.1.0-dev" }, runtime },
        );
      } catch {
        setFailedActionKind(kind);
        try {
          const runtime = await refreshRuntimeStatus();
          setBootstrap((current) =>
            current
              ? { ...current, runtime }
              : { app: { name: "Jeemi", version: "0.1.0-dev" }, runtime },
          );
        } catch {
          // Preserve the last trustworthy runtime snapshot.
        }
      } finally {
        actionInFlight.current = false;
        setActionKind(null);
      }
    },
    [authorization.flow],
  );

  const start = useCallback(() => runAction("start", startProxy), [runAction]);
  const stop = useCallback(() => runAction("stop", stopProxy), [runAction]);
  const restart = useCallback(
    () => runAction("restart", restartProxy),
    [runAction],
  );
  const removeAuthorization = useCallback(async () => {
    if (actionInFlight.current) return;
    actionInFlight.current = true;
    setRemovingAuthorization(true);
    try {
      const state = await getProxyAuthorization();
      if (state.present) {
        const confirmed = await new Promise<boolean>(resolve => {
          modal.confirm({ title: t("authorization.removeTitle"), content: t("authorization.removeDescription"), okText: t("authorization.remove"), cancelText: t("common.cancel"), onOk: () => { resolve(true); }, onCancel: () => { resolve(false); } });
        });
        if (!confirmed) return;
      }
      const result = await removeHelper();
      if (result.present) throw new Error("authorization_cleanup_incomplete");
      void message.success(t("authorization.cleaned"));
      try { await refresh(); } catch { /* Cleanup already succeeded; polling will refresh runtime. */ }
    } catch (error) { void message.error(t(authorizationErrorKey(error))); }
    finally { actionInFlight.current = false; setRemovingAuthorization(false); }
  }, [message, modal, refresh, t]);
  const savePreferences = useCallback(
    async (input: RuntimePreferences) => {
      setPreferencesBusy(true);
      setPreferencesError(false);
      try {
        const saved = await persistRuntimePreferences(input);
        setRuntimePreferences(saved);
        try {
          const runtime = await refreshRuntimeStatus();
          setBootstrap((current) =>
            current
              ? { ...current, runtime }
              : { app: { name: "Jeemi", version: "0.1.0-dev" }, runtime },
          );
        } catch {
          // The preference is persisted even if this best-effort status refresh fails.
        }
        return saved;
      } catch (error) {
        setPreferencesError(true);
        throw error;
      } finally {
        setPreferencesBusy(false);
      }
    },
    [],
  );
  const setOutboundMode = useCallback(
    async (mode: OutboundMode) => {
      const current = runtimePreferences ?? (await getRuntimePreferences());
      return savePreferences({ ...current, outboundMode: mode });
    },
    [runtimePreferences, savePreferences],
  );
  const setLogLevel = useCallback(
    async (level: LogLevel) => {
      const current = runtimePreferences ?? (await getRuntimePreferences());
      return savePreferences({ ...current, logLevel: level });
    },
    [runtimePreferences, savePreferences],
  );
  const value = useMemo<RuntimeStatusValue>(
    () => ({
      bootstrap,
      runtime: bootstrap?.runtime ?? null,
      liveDataReady,
      loading,
      actionBusy,
      actionError,
      actionKind,
      removingAuthorization,
      runtimePreferences,
      preferencesLoading,
      preferencesBusy,
      preferencesError,
      refresh,
      start,
      stop,
      restart,
      removeAuthorization,
      savePreferences,
      setLogLevel,
      setOutboundMode,
    }),
    [
      actionBusy,
      actionError,
      actionKind,
      removingAuthorization,
      bootstrap,
      liveDataReady,
      loading,
      preferencesBusy,
      preferencesError,
      preferencesLoading,
      refresh,
      runtimePreferences,
      savePreferences,
      setLogLevel,
      setOutboundMode,
      start,
      stop,
      restart,
      removeAuthorization,
    ],
  );

  return (
    <RuntimeStatusContext.Provider value={value}>
      {children}
      {authorization.dialog}
    </RuntimeStatusContext.Provider>
  );
}

export function useRuntimeStatus(): RuntimeStatusValue {
  const value = useContext(RuntimeStatusContext);
  if (!value) {
    throw new Error("useRuntimeStatus must be used within RuntimeStatusProvider");
  }
  return value;
}
