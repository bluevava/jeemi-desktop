import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  CameraOutlined,
  CodeOutlined,
  DeleteOutlined,
  EditOutlined,
  FileAddOutlined,
  FileSearchOutlined,
  FileTextOutlined,
  LinkOutlined,
  QrcodeOutlined,
  SwapOutlined,
} from "@ant-design/icons";
import { Alert, App, Button, Input, Modal, Select, Spin, Tag } from "antd";
import { normalizationErrorKey } from "../normalization";
import type { MenuProps } from "antd";
import { createPortal } from "react-dom";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { FeatureHelp } from "../../../components/help/FeatureHelp";
import { errorText } from "../../../lib/errorText";
import { useProxyOverview } from "../../../app/runtime/ProxyOverviewContext";
import { useRuntimeStatus } from "../../../app/runtime/RuntimeStatusContext";
import { useWindowActivity } from "../../../app/runtime/WindowActivityContext";
import {
  closeConnections,
  getConnections,
  getRuleProviderRuntimeState,
} from "../../../lib/mihomo/client";
import type { MihomoRuleProviderRuntimeState } from "../../../lib/mihomo/client";
import {
  deleteSubscription,
  getLocalConfigState,
  getLocalScriptState,
  getProxyDelayCache,
  getSubscription,
  getSubscriptionState,
  getSubscriptionText,
  importSubscriptionFile,
  importSubscriptionQRCodeImage,
  importSubscriptionQRCodeScreen,
  importSubscriptionURL,
  checkSubscriptionRefresh,
  resolveSubscriptionRefresh,
  saveSubscriptionPreferences,
  selectSubscription,
  saveProxyDelayCache,
  saveSubscriptionSelectorDisplayPreferences,
  setSubscriptionLocalConfig,
  setSubscriptionLocalScript,
  setSubscriptionFallback,
  setSubscriptionRuleProviderEnabled,
  updateSubscription,
  updateRuntimeRuleProvider,
} from "../../../services/appBridge";
import type { LocalConfigState } from "../../../types/localConfig";
import type { LocalScriptState } from "../../../types/localScript";
import type {
  SubscriptionRefreshConflict,
  FallbackSelection,
  ProxyDelayCacheScope,
  ProxyDelayCacheUpdateEntry,
  SelectorDensity,
  SelectorSortMode,
  SelectorViewMode,
  SubscriptionDetail,
  SubscriptionIconKind,
  SubscriptionState,
  SubscriptionSelector,
  SubscriptionSelectorMember,
  SubscriptionSummary,
  SubscriptionTextView,
} from "../../../types/subscription";
import { useSubscriptionPageStateField } from "../SubscriptionPageStateContext";
import { RuleProviderDrawer } from "../components/RuleProviderDrawer";
import { SelectorWorkspace } from "../components/SelectorWorkspace";
import { SubscriptionShelf } from "../components/SubscriptionShelf";
import { SubscriptionIconEditor } from "../components/SubscriptionIconEditor";
import { RuntimeConfigurationViewer } from "../../runtime/RuntimeConfigurationViewer";
import { connectionIDsForNodeSwitch } from "../connectionReset";
import { useProxyDelayQueue } from "../useProxyDelayQueue";
import { screenImportErrorKey } from "../screenImport";
import { selectAndRememberProxy } from "../proxySelection";
import { delayTargetsForSearch, filterSelectorsByNodeName } from "../selectorSearch";
import { shouldDisplaySelector } from "../selectorPresentation";

const noLocalHandlerValue = "__none__";
const emptySelections: Record<string, string> = {};

interface SubscriptionDraft {
  name: string;
  description: string;
  sourceUrl: string;
  iconKind: SubscriptionIconKind;
  icon: string;
}

const emptyDraft: SubscriptionDraft = {
  name: "",
  description: "",
  sourceUrl: "",
  iconKind: "",
  icon: "",
};

interface SubscriptionListPageProps {
  overlayHost: HTMLDivElement | null;
}

export function SubscriptionListPage({
  overlayHost,
}: SubscriptionListPageProps) {
  const { t, i18n } = useTranslation();
  const { modal } = App.useApp();
  const navigate = useNavigate();
  const [refreshConflict, setRefreshConflict] =
    useState<SubscriptionRefreshConflict | null>(null);
  const [refreshError, setRefreshError] = useState("");
  const {
    runtime,
    liveDataReady,
    runtimePreferences,
    preferencesBusy,
    setOutboundMode,
  } = useRuntimeStatus();
  const { isWindowVisible } = useWindowActivity();
  const {
    proxyRuntime,
    syncSubscriptionState,
    commitProxySelection,
    refreshProxyRuntime,
  } = useProxyOverview();
  const [state, setState] = useState<SubscriptionState | null>(null);
  const [localConfigs, setLocalConfigs] = useState<LocalConfigState | null>(
    null,
  );
  const [localScripts, setLocalScripts] = useState<LocalScriptState | null>(
    null,
  );
  const [loading, setLoading] = useState(true);
  const [busyAction, setBusyAction] = useState<string | null>(null);
  const [errorKey, setErrorKey] = useState<string | null>(null);
  const [shelfExpanded, setShelfExpanded] =
    useSubscriptionPageStateField("shelfExpanded");
  const [urlModalOpen, setURLModalOpen] = useState(false);
  const [urlDraft, setURLDraft] = useState<SubscriptionDraft>(emptyDraft);
  const [editing, setEditing] = useState<SubscriptionDetail | null>(null);
  const [editDraft, setEditDraft] = useState<SubscriptionDraft>(emptyDraft);
  const [iconFeedback, setIconFeedback] = useState<
    "" | "found" | "not_found" | "error"
  >("");
  const [rawText, setRawText] = useState<SubscriptionTextView | null>(null);
  const [associationTarget, setAssociationTarget] =
    useState<SubscriptionSummary | null>(null);
  const [associationDraft, setAssociationDraft] = useState(noLocalHandlerValue);
  const [scriptAssociationTarget, setScriptAssociationTarget] =
    useState<SubscriptionSummary | null>(null);
  const [scriptAssociationDraft, setScriptAssociationDraft] =
    useState(noLocalHandlerValue);
  const [associationError, setAssociationError] = useState("");
  const [runtimeViewerSignal, setRuntimeViewerSignal] = useState(0);
  const [ruleProvidersOpen, setRuleProvidersOpen] = useState(false);
  const [fallbackBusy, setFallbackBusy] = useState(false);
  const [busyRuleProvider, setBusyRuleProvider] = useState("");
  const [busyRuleProviderToggle, setBusyRuleProviderToggle] = useState("");
  const [ruleProviderRuntimeLoading, setRuleProviderRuntimeLoading] =
    useState(false);
  const [ruleProviderRuntime, setRuleProviderRuntime] = useState<
    Record<string, MihomoRuleProviderRuntimeState>
  >({});
  const [selectorQuery, setSelectorQuery] =
    useSubscriptionPageStateField("selectorQuery");
  const [showHiddenSelectors, setShowHiddenSelectors] =
    useSubscriptionPageStateField("showHiddenSelectors");
  const [selectorDisplayPreferencesBusy, setSelectorDisplayPreferencesBusy] =
    useState(false);
  const [busySelection, setBusySelection] = useState("");
  const automaticDelaySignature = useRef("");
  const ruleProviderRuntimeRequest = useRef(0);

  const activeSelections = proxyRuntime?.selections ?? emptySelections;

  useEffect(() => {
    let active = true;
    Promise.allSettled([
      getSubscriptionState(),
      getLocalConfigState(),
      getLocalScriptState(),
    ])
      .then(([subscriptionState, localConfigState, localScriptState]) => {
        if (active) {
          if (subscriptionState.status === "fulfilled")
            setState(subscriptionState.value);
          if (localConfigState.status === "fulfilled")
            setLocalConfigs(localConfigState.value);
          else setLocalConfigs({ directory: "", configs: [] });
          if (localScriptState.status === "fulfilled")
            setLocalScripts(localScriptState.value);
          else setLocalScripts({ directory: "", scripts: [] });
          if (
            [subscriptionState, localConfigState, localScriptState].some(
              (item) => item.status === "rejected",
            )
          )
            setErrorKey("subscription.errors.load");
        }
      })
      .catch(() => {
        if (active) setErrorKey("subscription.errors.load");
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    if (state) syncSubscriptionState(state);
  }, [state, syncSubscriptionState]);

  useEffect(() => {
    if (state?.subscriptions.length === 0) setShelfExpanded(true);
  }, [state?.subscriptions.length]);

  const controllerSession = runtime?.mihomo.controllerSession ?? null;
  const runtimeReady =
    runtime?.mihomo.state === "running" &&
    runtime?.mihomo.controllerReady === true &&
    controllerSession !== null &&
    runtime.mihomo.subscriptionId === state?.selectedSubscriptionId &&
    runtime.mihomo.source.normalizationFingerprint === state?.projection?.normalizationFingerprint &&
    runtime.mihomo.source.subscriptionRevision ===
      state?.projection?.revisionId &&
    runtime.mihomo.source.localConfigId === state?.projection?.localConfigId &&
    runtime.mihomo.source.localConfigRevision ===
      state?.projection?.localConfigRevision &&
    runtime.mihomo.source.localScriptId === state?.projection?.localScriptId &&
    runtime.mihomo.source.localScriptRevision ===
      state?.projection?.localScriptRevision &&
    runtime.mihomo.source.ruleProviderOverrideRevision ===
      state?.projection?.ruleProviderOverrideRevision &&
    runtime.mihomo.source.fallbackOverrideRevision ===
      state?.projection?.fallbackOverrideRevision &&
    runtime.mihomo.source.geoDataFingerprint ===
      state?.projection?.geoDataFingerprint;
  const runtimeInteractive = runtimeReady && isWindowVisible && liveDataReady;
  const runtimeInteractiveRef = useRef(runtimeInteractive);
  runtimeInteractiveRef.current = runtimeInteractive;
  const delayTargets = useMemo(
    () =>
      Array.from(
        new Set([
          ...(state?.projection?.selectors ?? []).flatMap((selector) =>
            selector.members
              .filter(
                (member) =>
                  member.source === "proxy" || member.source === "provider",
              )
              .map((member) => member.name),
          ),
          ...(state?.projection?.proxies ?? []).map((proxy) => proxy.name),
          ...(proxyRuntime?.allProxies ?? []).map((proxy) => proxy.name),
        ]),
      ),
    [proxyRuntime?.allProxies, state?.projection],
  );
  const globalMembers = useMemo<SubscriptionSelectorMember[]>(
    () =>
      runtimeInteractive && proxyRuntime?.allProxies.length
        ? proxyRuntime.allProxies.map((proxy) => ({
            ...proxy,
            source: "proxy" as const,
            providerName: "",
          }))
        : (state?.projection?.proxies ?? []),
    [proxyRuntime?.allProxies, runtimeInteractive, state?.projection?.proxies],
  );
  const outboundMode =
    runtimePreferences?.outboundMode || runtime?.mihomo.outboundMode || "rule";
  const visibleSelectors = useMemo<SubscriptionSelector[]>(() => {
    if (outboundMode === "direct") return [];
    const modeSelectors: SubscriptionSelector[] =
      outboundMode === "global"
        ? [
            {
              name: `🌐 ${t("subscription.selector.globalProxy")}`,
              icon: "",
              hidden: false,
              type: "select",
              defaultSelection:
                (runtimeInteractive && activeSelections.GLOBAL) ||
                globalMembers[0]?.name ||
                "",
              members: globalMembers,
              providerNames: [],
              unresolvedProviderNames: [],
              referencedByRules: true,
            },
          ]
        : state?.projection?.selectors ?? [];
    return filterSelectorsByNodeName(
      modeSelectors.filter((selector) =>
        shouldDisplaySelector(selector.hidden, showHiddenSelectors),
      ),
      selectorQuery,
    );
  }, [
    activeSelections.GLOBAL,
    globalMembers,
    outboundMode,
    runtimeInteractive,
    selectorQuery,
    showHiddenSelectors,
    state?.projection?.selectors,
    t,
  ]);
  const batchDelayTargets = useMemo(
    () => delayTargetsForSearch(delayTargets, visibleSelectors, selectorQuery),
    [delayTargets, visibleSelectors, selectorQuery],
  );
  const delayCacheScope = useMemo<ProxyDelayCacheScope | null>(() => {
    const projection = state?.projection;
    if (!projection?.subscriptionId || !projection.revisionId) return null;
    return {
      normalizationFingerprint: projection.normalizationFingerprint,
      subscriptionId: projection.subscriptionId,
      subscriptionRevision: projection.revisionId,
      localConfigId: projection.localConfigId,
      localConfigRevision: projection.localConfigRevision,
      localScriptId: projection.localScriptId,
      localScriptRevision: projection.localScriptRevision,
    };
  }, [state?.projection]);
  const delayCacheScopeKey = delayCacheScope
    ? [
        delayCacheScope.normalizationFingerprint,
        delayCacheScope.subscriptionId,
        delayCacheScope.subscriptionRevision,
        delayCacheScope.localConfigId,
        delayCacheScope.localConfigRevision,
        delayCacheScope.localScriptId,
        delayCacheScope.localScriptRevision,
      ].join(":")
    : "";
  const persistDelayResults = useCallback(
    async (results: ProxyDelayCacheUpdateEntry[]) => {
      if (!delayCacheScope || results.length === 0) return;
      try {
        await saveProxyDelayCache({ scope: delayCacheScope, results });
      } catch (error) {
        setErrorKey("subscription.errors.delayCacheSave");
        throw error;
      }
    },
    [delayCacheScope],
  );
  const delayQueue = useProxyDelayQueue(
    runtimeInteractive ? controllerSession : null,
    state?.preferences.delayTestConcurrency || 8,
    delayCacheScopeKey,
    persistDelayResults,
  );

  useEffect(() => {
    let active = true;
    if (!delayCacheScope) return () => undefined;
    void getProxyDelayCache(delayCacheScope)
      .then((snapshot) => {
        if (active) delayQueue.hydrate(snapshot.results);
      })
      .catch(() => {
        if (active) setErrorKey("subscription.errors.delayCacheLoad");
      });
    return () => {
      active = false;
    };
  }, [delayCacheScopeKey, delayQueue.hydrate]);

  useEffect(() => {
    automaticDelaySignature.current = "";
  }, [delayCacheScopeKey]);

  useEffect(() => {
    if (!runtimeInteractive || !proxyRuntime) return;
    const targetNames = new Set(delayTargets);
    const automaticResults: ProxyDelayCacheUpdateEntry[] = proxyRuntime.delays
      .filter((result) => targetNames.has(result.name))
      .map((result) => ({ ...result, source: "mihomo" as const }));
    const signature = JSON.stringify(automaticResults);
    if (
      automaticResults.length === 0 ||
      signature === automaticDelaySignature.current
    ) {
      return;
    }
    automaticDelaySignature.current = signature;
    delayQueue.observe(automaticResults);
    void persistDelayResults(automaticResults).catch(() => undefined);
  }, [
    delayQueue.observe,
    delayTargets,
    persistDelayResults,
    proxyRuntime,
    runtimeInteractive,
  ]);

  const refreshRuleProviderRuntime = useCallback(async () => {
    const request = ruleProviderRuntimeRequest.current + 1;
    ruleProviderRuntimeRequest.current = request;
    if (!runtimeInteractive || !controllerSession) {
      setRuleProviderRuntime({});
      setRuleProviderRuntimeLoading(false);
      return;
    }
    setRuleProviderRuntimeLoading(true);
    try {
      const next = await getRuleProviderRuntimeState(controllerSession);
      if (
        request === ruleProviderRuntimeRequest.current &&
        runtimeInteractiveRef.current
      ) {
        setRuleProviderRuntime(next);
      }
    } catch {
      if (
        request === ruleProviderRuntimeRequest.current &&
        runtimeInteractiveRef.current
      ) {
        setRuleProviderRuntime({});
      }
    } finally {
      if (
        request === ruleProviderRuntimeRequest.current &&
        runtimeInteractiveRef.current
      ) {
        setRuleProviderRuntimeLoading(false);
      }
    }
  }, [controllerSession?.id, runtimeInteractive]);

  useEffect(() => {
    if (!ruleProvidersOpen || !runtimeInteractive || !controllerSession) {
      ruleProviderRuntimeRequest.current += 1;
      setRuleProviderRuntime({});
      setRuleProviderRuntimeLoading(false);
      return () => undefined;
    }
    void refreshRuleProviderRuntime();
    const timer = globalThis.setInterval(refreshRuleProviderRuntime, 10000);
    return () => {
      ruleProviderRuntimeRequest.current += 1;
      globalThis.clearInterval(timer);
    };
  }, [
    controllerSession?.id,
    refreshRuleProviderRuntime,
    ruleProvidersOpen,
    runtimeInteractive,
  ]);

  const localConfigOptions = useMemo(
    () => [
      { label: t("subscription.association.none"), value: noLocalHandlerValue },
      ...(localConfigs?.configs ?? []).map((config) => ({
        label: config.name,
        value: config.id,
      })),
    ],
    [localConfigs, t],
  );
  const localScriptOptions = useMemo(
    () => [
      {
        label: t("subscription.scriptAssociation.none"),
        value: noLocalHandlerValue,
      },
      ...(localScripts?.scripts ?? []).map((script) => ({
        label: script.name,
        value: script.id,
      })),
    ],
    [localScripts, t],
  );
  const localConfigNames = useMemo(
    () =>
      Object.fromEntries(
        (localConfigs?.configs ?? []).map((config) => [config.id, config.name]),
      ),
    [localConfigs],
  );
  const localScriptNames = useMemo(
    () =>
      Object.fromEntries(
        (localScripts?.scripts ?? []).map((script) => [script.id, script.name]),
      ),
    [localScripts],
  );

  const setOperationError = (key = "subscription.errors.operation") => {
    setErrorKey(key);
  };

  const importFromFile = async () => {
    setBusyAction("file");
    setErrorKey(null);
    try {
      const result = await importSubscriptionFile({
        title: t("subscription.import.fileDialogTitle"),
        filterName: t("subscription.import.fileFilter"),
      });
      if (!result.cancelled) setState(result.state);
    } catch (cause) {
      setOperationError(normalizationErrorKey(cause, "subscription.errors.fileImport"));
    } finally {
      setBusyAction(null);
    }
  };

  const importFromQRImage = async () => {
    setBusyAction("qr-image");
    setErrorKey(null);
    try {
      const result = await importSubscriptionQRCodeImage({
        title: t("subscription.import.qrImageDialogTitle"),
        filterName: t("subscription.import.qrImageFilter"),
      });
      if (!result.cancelled) setState(result.state);
    } catch (cause) {
      setOperationError(normalizationErrorKey(cause, "subscription.errors.qrImport"));
    } finally {
      setBusyAction(null);
    }
  };

  const importFromScreen = async () => {
    setBusyAction("qr-screen");
    setErrorKey(null);
    try {
      const result = await importSubscriptionQRCodeScreen();
      const errorKey = screenImportErrorKey(result);
      if (errorKey) setOperationError(errorKey);
      else if (!result.cancelled && result.state) setState(result.state);
    } catch {
      setOperationError("subscription.errors.screenImport");
    } finally {
      setBusyAction(null);
    }
  };

  const submitURLImport = async () => {
    if (!urlDraft.sourceUrl.trim()) {
      setOperationError("subscription.errors.urlRequired");
      return;
    }
    setBusyAction("url");
    setErrorKey(null);
    try {
      setState(
        await importSubscriptionURL({
          name: urlDraft.name,
          description: urlDraft.description,
          sourceUrl: urlDraft.sourceUrl,
          importMethod: "url",
          iconKind: urlDraft.iconKind,
          icon: urlDraft.icon,
        }),
      );
      setURLModalOpen(false);
      setURLDraft(emptyDraft);
      setIconFeedback("");
    } catch (cause) {
      setOperationError(normalizationErrorKey(cause, "subscription.errors.urlImport"));
    } finally {
      setBusyAction(null);
    }
  };

  const refresh = async (subscription: SubscriptionSummary) => {
    setBusyAction(subscription.id);
    setErrorKey(null);
    try {
      const result = await checkSubscriptionRefresh(subscription.id);
      if (result.state) setState(result.state);
      setRefreshError("");
      setRefreshConflict(result.conflict);
    } catch (cause) {
      setOperationError(normalizationErrorKey(cause, "subscription.errors.refresh"));
      setRefreshError(errorText(cause, t("subscription.errors.refresh")));
    } finally {
      setBusyAction(null);
    }
  };

  const resolveRefresh = async (detach: boolean, edit = false) => {
    if (!refreshConflict || busyAction) return;
    const conflict = refreshConflict;
    setBusyAction(conflict.subscriptionId);
    try {
      setState(await resolveSubscriptionRefresh(conflict.token, detach));
      setRefreshConflict(null);
      setRefreshError("");
      if (edit)
        navigate(
          conflict.localScriptId
            ? `/config/scripts/${conflict.localScriptId}/edit`
            : `/config/${conflict.localConfigId}/edit`,
        );
    } catch (cause) {
      setRefreshConflict(null);
      setRefreshError(errorText(cause, t("subscription.errors.refresh")));
    } finally {
      setBusyAction(null);
    }
  };

  const select = async (subscription: SubscriptionSummary) => {
    if (subscription.id === state?.selectedSubscriptionId || busyAction) return;
    setBusyAction(subscription.id);
    setErrorKey(null);
    try {
      setState(await selectSubscription(subscription.id));
    } catch {
      setOperationError("subscription.errors.select");
    } finally {
      setBusyAction(null);
    }
  };

  const openEditor = async (subscription: SubscriptionSummary) => {
    setBusyAction(subscription.id);
    setErrorKey(null);
    try {
      const detail = await getSubscription(subscription.id);
      setEditing(detail);
      setEditDraft({
        name: detail.name,
        description: detail.description,
        sourceUrl: detail.sourceUrl,
        iconKind: detail.iconKind,
        icon: detail.icon,
      });
      setIconFeedback("");
    } catch {
      setOperationError("subscription.errors.detail");
    } finally {
      setBusyAction(null);
    }
  };

  const submitEdit = async () => {
    if (!editing || !editDraft.name.trim()) {
      setOperationError("subscription.errors.required");
      return;
    }
    setBusyAction("edit");
    setErrorKey(null);
    try {
      setState(
        await updateSubscription({
          id: editing.id,
          name: editDraft.name,
          description: editDraft.description,
          sourceUrl: editing.sourceKind === "url" ? editDraft.sourceUrl : "",
          iconKind: editDraft.iconKind,
          icon: editDraft.icon,
        }),
      );
      setEditing(null);
    } catch (cause) {
      setOperationError(normalizationErrorKey(cause, "subscription.errors.edit"));
    } finally {
      setBusyAction(null);
    }
  };

  const openRawText = async (subscription: SubscriptionSummary) => {
    setBusyAction(subscription.id);
    setErrorKey(null);
    try {
      setRawText(await getSubscriptionText(subscription.id));
    } catch {
      setOperationError("subscription.errors.rawText");
    } finally {
      setBusyAction(null);
    }
  };

  const openAssociation = (subscription: SubscriptionSummary) => {
    if (subscription.localScriptId) {
      modal.warning({
        title: t("subscription.association.conflictTitle"),
        content: t("subscription.association.unlinkScriptFirst"),
      });
      return;
    }
    setAssociationError("");
    setAssociationTarget(subscription);
    setAssociationDraft(subscription.localConfigId || noLocalHandlerValue);
  };

  const saveAssociation = async () => {
    if (!associationTarget) return;
    setBusyAction("association");
    setErrorKey(null);
    try {
      setState(
        await setSubscriptionLocalConfig(
          associationTarget.id,
          associationDraft === noLocalHandlerValue ? "" : associationDraft,
        ),
      );
      setAssociationTarget(null);
    } catch (cause) {
      setAssociationError(errorText(cause, t("common.unknownError")));
    } finally {
      setBusyAction(null);
    }
  };

  const openScriptAssociation = (subscription: SubscriptionSummary) => {
    if (subscription.localConfigId) {
      modal.warning({
        title: t("subscription.scriptAssociation.conflictTitle"),
        content: t("subscription.scriptAssociation.unlinkConfigFirst"),
      });
      return;
    }
    setAssociationError("");
    setScriptAssociationTarget(subscription);
    setScriptAssociationDraft(
      subscription.localScriptId || noLocalHandlerValue,
    );
  };

  const saveScriptAssociation = async () => {
    if (!scriptAssociationTarget) return;
    setBusyAction("script-association");
    setAssociationError("");
    try {
      setState(
        await setSubscriptionLocalScript(
          scriptAssociationTarget.id,
          scriptAssociationDraft === noLocalHandlerValue
            ? ""
            : scriptAssociationDraft,
        ),
      );
      setScriptAssociationTarget(null);
    } catch (cause) {
      setAssociationError(
        t("subscription.scriptAssociation.validationFailed", {
          error: errorText(cause, t("common.unknownError")),
        }),
      );
    } finally {
      setBusyAction(null);
    }
  };

  const refreshRuleProvider = async (name: string) => {
    if (!runtimeInteractive) return;
    setBusyRuleProvider(name);
    setErrorKey(null);
    try {
      await updateRuntimeRuleProvider(name);
      await refreshRuleProviderRuntime();
    } catch {
      setOperationError("subscription.errors.ruleProviderRefresh");
    } finally {
      setBusyRuleProvider("");
    }
  };

  const changeFallback = async (input: FallbackSelection) => {
    const subscription = state?.subscriptions.find(
      (item) => item.id === state.selectedSubscriptionId,
    );
    if (!subscription || fallbackBusy || busyAction) return;
    setFallbackBusy(true);
    setErrorKey(null);
    try {
      const result = await setSubscriptionFallback(
        subscription.id,
        input,
        subscription.fallbackOverrideRevision,
      );
      setState(result.state);
      if (result.connectionResetFailed) {
        setOperationError("subscription.fallback.connectionResetFailed");
      }
    } catch {
      setOperationError("subscription.fallback.saveFailed");
      // A concurrent reset may have changed the independent fallback revision.
      try {
        setState(await getSubscriptionState());
      } catch {
        /* Keep the saved UI snapshot. */
      }
    } finally {
      setFallbackBusy(false);
    }
  };

  const toggleRuleProvider = async (name: string, enabled: boolean) => {
    if (!state?.selectedSubscriptionId) return;
    setBusyRuleProviderToggle(name);
    setErrorKey(null);
    try {
      setState(
        await setSubscriptionRuleProviderEnabled(
          state.selectedSubscriptionId,
          name,
          enabled,
        ),
      );
      if (!enabled) {
        setRuleProviderRuntime((current) => {
          const next = { ...current };
          delete next[name];
          return next;
        });
      }
    } catch {
      setOperationError("subscription.errors.ruleProviderToggle");
    } finally {
      setBusyRuleProviderToggle("");
    }
  };

  const switchProxy = async (group: string, proxy: string) => {
    const subscriptionId = runtime?.mihomo.subscriptionId;
    if (!runtimeInteractive || !controllerSession || !subscriptionId) return;
    setBusySelection(group);
    setErrorKey(null);
    try {
      const remembered = await selectAndRememberProxy(
        controllerSession,
        subscriptionId,
        group,
        proxy,
        () => {
          commitProxySelection(group, proxy, controllerSession.id, subscriptionId);
          void refreshProxyRuntime().catch(() => undefined);
        },
      );
      if (!remembered) setOperationError("subscription.errors.proxySelectionSave");
      const resetMode = state?.preferences.connectionResetMode ?? "selector";
      if (resetMode !== "off") {
        try {
          const snapshot = await getConnections(controllerSession);
          const targetIDs = connectionIDsForNodeSwitch(
            snapshot.connections,
            resetMode,
            group,
          );
          await closeConnections(controllerSession, targetIDs);
        } catch {
          setOperationError("subscription.errors.connectionReset");
        }
      }
    } catch {
      setOperationError("subscription.errors.proxySelection");
    } finally {
      setBusySelection("");
    }
  };

  const switchOutboundMode = async (mode: "rule" | "global" | "direct") => {
    if (mode === runtimePreferences?.outboundMode || preferencesBusy) return;
    setErrorKey(null);
    try {
      await setOutboundMode(mode);
    } catch {
      setOperationError("subscription.errors.outboundMode");
    }
  };

  const saveSelectorDisplayPreferences = async (
    selectorSortMode: SelectorSortMode,
    selectorViewMode: SelectorViewMode,
  ) => {
    if (!state || selectorDisplayPreferencesBusy) return;
    const previous = state.preferences;
    if (
      previous.selectorSortMode === selectorSortMode &&
      previous.selectorViewMode === selectorViewMode
    ) {
      return;
    }
    const optimistic = {
      ...previous,
      selectorSortMode,
      selectorViewMode,
    };
    setSelectorDisplayPreferencesBusy(true);
    setErrorKey(null);
    setState((current) =>
      current ? { ...current, preferences: optimistic } : current,
    );
    try {
      const saved = await saveSubscriptionSelectorDisplayPreferences({
        selectorSortMode,
        selectorViewMode,
      });
      setState((current) =>
        current ? { ...current, preferences: saved } : current,
      );
    } catch {
      setState((current) =>
        current ? { ...current, preferences: previous } : current,
      );
      setOperationError("subscription.errors.selectorDisplayPreferences");
    } finally {
      setSelectorDisplayPreferencesBusy(false);
    }
  };

  const saveSelectorDensity = async (selectorDensity: SelectorDensity) => {
    if (!state || selectorDisplayPreferencesBusy) return;
    const previous = state.preferences;
    if (previous.selectorDensity === selectorDensity) return;
    const optimistic = { ...previous, selectorDensity };
    setSelectorDisplayPreferencesBusy(true);
    setErrorKey(null);
    setState((current) =>
      current ? { ...current, preferences: optimistic } : current,
    );
    try {
      const saved = await saveSubscriptionPreferences({
        selectorDensity,
        delayTestConcurrency: previous.delayTestConcurrency,
        connectionResetMode: previous.connectionResetMode,
      });
      setState((current) =>
        current ? { ...current, preferences: saved } : current,
      );
    } catch {
      setState((current) =>
        current ? { ...current, preferences: previous } : current,
      );
      setOperationError("subscription.errors.selectorDisplayPreferences");
    } finally {
      setSelectorDisplayPreferencesBusy(false);
    }
  };

  const confirmDelete = (subscription: SubscriptionSummary) => {
    modal.confirm({
      title: t("subscription.delete.title"),
      content: t("subscription.delete.description", {
        name: subscription.name,
      }),
      okText: t("subscription.menu.delete"),
      cancelText: t("common.cancel"),
      okButtonProps: { danger: true },
      onOk: async () => {
        setBusyAction(subscription.id);
        setErrorKey(null);
        try {
          setState(await deleteSubscription(subscription.id));
        } catch {
          setOperationError("subscription.errors.delete");
          throw new Error("delete subscription failed");
        } finally {
          setBusyAction(null);
        }
      },
    });
  };

  const cardMenu = (subscription: SubscriptionSummary): MenuProps => ({
    items: [
      {
        key: "edit",
        icon: <EditOutlined />,
        label: t("subscription.menu.edit"),
      },
      {
        key: "text",
        icon: <FileTextOutlined />,
        label: t("subscription.menu.rawText"),
      },
      {
        key: "runtime",
        disabled: subscription.id !== state?.selectedSubscriptionId,
        icon: <FileSearchOutlined />,
        label: t("subscription.menu.runtimeConfiguration"),
      },
      {
        key: "associate",
        icon: <SwapOutlined />,
        label: t("subscription.menu.associateConfig"),
      },
      {
        key: "associate-script",
        icon: <CodeOutlined />,
        label: t("subscription.menu.associateScript"),
      },
      { type: "divider" },
      {
        key: "delete",
        danger: true,
        icon: <DeleteOutlined />,
        label: t("subscription.menu.delete"),
      },
    ],
    onClick: ({ key, domEvent }) => {
      domEvent.stopPropagation();
      if (key === "edit") void openEditor(subscription);
      else if (key === "text") void openRawText(subscription);
      else if (key === "runtime") setRuntimeViewerSignal((value) => value + 1);
      else if (key === "associate") openAssociation(subscription);
      else if (key === "associate-script") openScriptAssociation(subscription);
      else if (key === "delete") confirmDelete(subscription);
    },
  });

  const addMenu: MenuProps = {
    items: [
      {
        key: "url",
        icon: <LinkOutlined />,
        label: t("subscription.import.url"),
      },
      {
        key: "file",
        icon: <FileAddOutlined />,
        label: t("subscription.import.file"),
      },
      {
        key: "screen",
        icon: <CameraOutlined />,
        label: t("subscription.import.qrScreen"),
      },
      {
        key: "image",
        icon: <QrcodeOutlined />,
        label: t("subscription.import.qrImage"),
      },
    ],
    onClick: ({ key }) => {
      if (key === "url") setURLModalOpen(true);
      else if (key === "file") void importFromFile();
      else if (key === "screen") void importFromScreen();
      else void importFromQRImage();
    },
  };

  if (loading) {
    return (
      <div className="subscription-loading">
        <Spin />
        <span>{t("subscription.loading")}</span>
      </div>
    );
  }
  if (!state || !localConfigs || !localScripts) {
    return (
      <Alert
        description={t(errorKey ?? "subscription.errors.backendUnavailable")}
        showIcon
        type="error"
      />
    );
  }

  const subscriptionShelf = (
    <SubscriptionShelf
      addMenu={addMenu}
      busyAction={busyAction}
      expanded={shelfExpanded || state.subscriptions.length === 0}
      locale={i18n.language}
      localConfigNames={localConfigNames}
      localScriptNames={localScriptNames}
      menuFor={cardMenu}
      onExpandedChange={(expanded) =>
        setShelfExpanded(state.subscriptions.length === 0 ? true : expanded)
      }
      onOpenRuleProviders={() => setRuleProvidersOpen(true)}
      onRefreshSelected={(subscription) => void refresh(subscription)}
      onSelect={(subscription) => void select(subscription)}
      onSelectorQueryChange={setSelectorQuery}
      onShowHiddenSelectorsChange={setShowHiddenSelectors}
      onSpeedTest={() => void delayQueue.testAll(batchDelayTargets)}
      runtimeReady={runtimeInteractive}
      selectorQuery={selectorQuery}
      showHiddenSelectors={showHiddenSelectors}
      speedTestAvailable={batchDelayTargets.length > 0}
      speedTestBusy={delayQueue.queueActive}
      state={state}
    />
  );

  return (
    <>
      {overlayHost ? createPortal(subscriptionShelf, overlayHost) : null}

      <div className="page-stack subscription-page">
        <RuntimeConfigurationViewer
          hideTrigger
          openSignal={runtimeViewerSignal}
        />

        {errorKey ? (
          <Alert
            closable
            description={t(errorKey)}
            onClose={() => setErrorKey(null)}
            showIcon
            type="error"
          />
        ) : null}

        <SelectorWorkspace
          subscription={state.subscriptions.find(
            (item) => item.id === state.selectedSubscriptionId,
          )}
          fallbackBusy={
            fallbackBusy || busyAction !== null || busyRuleProviderToggle !== ""
          }
          onFallbackChange={changeFallback}
          activeSelections={
            runtimeInteractive ? activeSelections : emptySelections
          }
          busyDelayNodes={delayQueue.busyNodes}
          busySelection={busySelection}
          density={state.preferences.selectorDensity}
          delayQueueActive={delayQueue.queueActive}
          delayResults={delayQueue.results}
          displayPreferencesBusy={selectorDisplayPreferencesBusy}
          selectors={visibleSelectors}
          outboundMode={outboundMode}
          outboundModeBusy={preferencesBusy || runtimePreferences === null}
          onOutboundModeChange={switchOutboundMode}
          onDensityChange={saveSelectorDensity}
          onSelect={switchProxy}
          onSortModeChange={(mode) =>
            saveSelectorDisplayPreferences(
              mode,
              state.preferences.selectorViewMode,
            )
          }
          onTestDelay={delayQueue.testNode}
          onViewModeChange={(mode) =>
            saveSelectorDisplayPreferences(
              state.preferences.selectorSortMode,
              mode,
            )
          }
          projection={state.projection}
          query={selectorQuery}
          runtimeReady={runtimeInteractive}
          sortMode={state.preferences.selectorSortMode}
          viewMode={state.preferences.selectorViewMode}
        />

        <RuleProviderDrawer
          busyProvider={busyRuleProvider}
          busyToggle={busyRuleProviderToggle}
          locale={i18n.language}
          onClose={() => setRuleProvidersOpen(false)}
          onRefresh={refreshRuleProvider}
          onToggle={toggleRuleProvider}
          open={ruleProvidersOpen}
          providers={state.projection?.ruleProviders ?? []}
          runtimeLoading={ruleProviderRuntimeLoading}
          runtimeProviders={ruleProviderRuntime}
          runtimeReady={runtimeInteractive}
        />

        {refreshError ? (
          <Alert
            showIcon
            type="error"
            closable
            description={refreshError}
            onClose={() => setRefreshError("")}
          />
        ) : null}
        <Modal
          open={refreshConflict !== null}
          width={680}
          title={t("localConfig.redesign.refreshConflict")}
          closable={!busyAction}
          onCancel={() => void resolveRefresh(false)}
          footer={[
            <Button
              key="reject"
              disabled={Boolean(busyAction)}
              onClick={() => void resolveRefresh(false)}
            >
              {t("localConfig.redesign.rejectRefresh")}
            </Button>,
            <Button
              key="edit"
              disabled={Boolean(busyAction)}
              onClick={() => void resolveRefresh(false, true)}
            >
              {t("localConfig.redesign.editHandler")}
            </Button>,
            <Button
              key="detach"
              type="primary"
              loading={Boolean(busyAction)}
              onClick={() => void resolveRefresh(true)}
            >
              {t("localConfig.redesign.detachRefresh")}
            </Button>,
          ]}
        >
          <p>{t("localConfig.redesign.refreshConflictDescription")}</p>
          <Alert showIcon type="error" description={refreshConflict?.message} />
        </Modal>
        <Modal
          cancelText={t("common.cancel")}
          confirmLoading={busyAction === "url"}
          okText={t("subscription.import.confirmURL")}
          onCancel={() => {
            setURLModalOpen(false);
            setIconFeedback("");
          }}
          onOk={() => void submitURLImport()}
          open={urlModalOpen}
          title={
            <span className="feature-title-with-help">
              {t("subscription.import.urlTitle")}
              <FeatureHelp compact topic="subscriptionSource" />
            </span>
          }
        >
          <div className="subscription-form">
            {errorKey && urlModalOpen ? (
              <Alert description={t(errorKey)} showIcon type="error" />
            ) : null}
            {iconFeedback ? (
              <Alert
                closable
                description={t(
                  `subscription.form.iconDetection.${iconFeedback}`,
                )}
                onClose={() => setIconFeedback("")}
                showIcon
                type={
                  iconFeedback === "found"
                    ? "success"
                    : iconFeedback === "not_found"
                      ? "info"
                      : "error"
                }
              />
            ) : null}
            <div className="subscription-icon-name-row">
              <SubscriptionIconEditor
                icon={urlDraft.icon}
                iconKind={urlDraft.iconKind}
                onChange={(iconKind, icon) =>
                  setURLDraft((draft) => ({ ...draft, iconKind, icon }))
                }
                onDetectionResult={setIconFeedback}
                sourceUrl={urlDraft.sourceUrl}
              />
              <label>
                <span>{t("subscription.form.nameOptional")}</span>
                <Input
                  maxLength={80}
                  onChange={(event) =>
                    setURLDraft((draft) => ({
                      ...draft,
                      name: event.target.value,
                    }))
                  }
                  placeholder={t("subscription.form.nameAutoPlaceholder")}
                  value={urlDraft.name}
                />
              </label>
            </div>
            <label>
              <span>{t("subscription.form.url")}</span>
              <Input.Password
                autoComplete="off"
                onChange={(event) =>
                  setURLDraft((draft) => ({
                    ...draft,
                    sourceUrl: event.target.value,
                  }))
                }
                placeholder="https://example.com/subscription"
                value={urlDraft.sourceUrl}
              />
            </label>
            <label>
              <span>{t("subscription.form.description")}</span>
              <Input.TextArea
                maxLength={500}
                onChange={(event) =>
                  setURLDraft((draft) => ({
                    ...draft,
                    description: event.target.value,
                  }))
                }
                placeholder={t("subscription.form.descriptionPlaceholder")}
                rows={3}
                value={urlDraft.description}
              />
            </label>
          </div>
        </Modal>

        <Modal
          cancelText={t("common.cancel")}
          confirmLoading={busyAction === "edit"}
          okText={t("subscription.edit.save")}
          onCancel={() => {
            setEditing(null);
            setIconFeedback("");
          }}
          onOk={() => void submitEdit()}
          open={editing !== null}
          title={
            <span className="feature-title-with-help">
              {t("subscription.edit.title")}
              <FeatureHelp
                compact
                topic={
                  editing?.sourceKind === "url"
                    ? "subscriptionSource"
                    : "subscriptionImport"
                }
              />
            </span>
          }
        >
          <div className="subscription-form">
            {errorKey && editing ? (
              <Alert description={t(errorKey)} showIcon type="error" />
            ) : null}
            {iconFeedback ? (
              <Alert
                closable
                description={t(
                  `subscription.form.iconDetection.${iconFeedback}`,
                )}
                onClose={() => setIconFeedback("")}
                showIcon
                type={
                  iconFeedback === "found"
                    ? "success"
                    : iconFeedback === "not_found"
                      ? "info"
                      : "error"
                }
              />
            ) : null}
            <div className="subscription-icon-name-row">
              <SubscriptionIconEditor
                icon={editDraft.icon}
                iconKind={editDraft.iconKind}
                onChange={(iconKind, icon) =>
                  setEditDraft((draft) => ({ ...draft, iconKind, icon }))
                }
                onDetectionResult={setIconFeedback}
                sourceKind={editing?.sourceKind}
                sourceUrl={
                  editing?.sourceKind === "url" ? editDraft.sourceUrl : ""
                }
              />
              <label>
                <span>{t("subscription.form.name")}</span>
                <Input
                  maxLength={80}
                  onChange={(event) =>
                    setEditDraft((draft) => ({
                      ...draft,
                      name: event.target.value,
                    }))
                  }
                  value={editDraft.name}
                />
              </label>
            </div>
            {editing?.sourceKind === "url" ? (
              <label>
                <span>{t("subscription.form.url")}</span>
                <Input.Password
                  autoComplete="off"
                  onChange={(event) =>
                    setEditDraft((draft) => ({
                      ...draft,
                      sourceUrl: event.target.value,
                    }))
                  }
                  value={editDraft.sourceUrl}
                />
              </label>
            ) : (
              <div className="subscription-readonly-source">
                <span>{t("subscription.form.file")}</span>
                <strong>{editing?.sourceLabel}</strong>
              </div>
            )}
            <label>
              <span>{t("subscription.form.description")}</span>
              <Input.TextArea
                maxLength={500}
                onChange={(event) =>
                  setEditDraft((draft) => ({
                    ...draft,
                    description: event.target.value,
                  }))
                }
                rows={3}
                value={editDraft.description}
              />
            </label>
          </div>
        </Modal>

        <Modal
          footer={null}
          onCancel={() => setRawText(null)}
          open={rawText !== null}
          title={
            <span className="subscription-modal-title">
              {t("subscription.raw.title", { name: rawText?.name ?? "" })}
              <FeatureHelp compact topic="subscriptionRawText" />
            </span>
          }
          width={900}
        >
          <div className="subscription-raw-view">
            <div className="subscription-raw-meta">
              <Tag>{rawText?.format.toUpperCase()}</Tag>
              <code>{rawText?.revisionId}</code>
            </div>
            <pre>{rawText?.contents}</pre>
          </div>
        </Modal>

        <Modal
          cancelText={t("common.cancel")}
          confirmLoading={busyAction === "association"}
          okText={t("subscription.association.save")}
          onCancel={() => {
            setAssociationTarget(null);
            setAssociationError("");
          }}
          onOk={() => void saveAssociation()}
          open={associationTarget !== null}
          title={
            <span className="subscription-modal-title">
              {t("subscription.association.title")}
              <FeatureHelp compact topic="subscriptionAssociation" />
            </span>
          }
        >
          <div className="subscription-association-form">
            {associationError && associationTarget ? (
              <Alert description={associationError} showIcon type="error" />
            ) : null}
            <strong className="subscription-association-target">
              {associationTarget?.name}
            </strong>
            <Select
              aria-label={t("subscription.association.title")}
              onChange={setAssociationDraft}
              options={localConfigOptions}
              value={associationDraft}
            />
            {localConfigs.configs.length === 0 ? (
              <Alert
                description={t("subscription.association.empty")}
                showIcon
                type="info"
              />
            ) : null}
          </div>
        </Modal>

        <Modal
          cancelText={t("common.cancel")}
          confirmLoading={busyAction === "script-association"}
          okText={t("subscription.scriptAssociation.save")}
          onCancel={() => {
            setScriptAssociationTarget(null);
            setAssociationError("");
          }}
          onOk={() => void saveScriptAssociation()}
          open={scriptAssociationTarget !== null}
          title={
            <span className="subscription-modal-title">
              {t("subscription.scriptAssociation.title")}
              <FeatureHelp compact topic="subscriptionAssociation" />
            </span>
          }
        >
          <div className="subscription-association-form">
            {associationError && scriptAssociationTarget ? (
              <Alert description={associationError} showIcon type="error" />
            ) : null}
            <strong className="subscription-association-target">
              {scriptAssociationTarget?.name}
            </strong>
            <Select
              aria-label={t("subscription.scriptAssociation.title")}
              onChange={setScriptAssociationDraft}
              options={localScriptOptions}
              value={scriptAssociationDraft}
            />
            {localScripts.scripts.length === 0 ? (
              <Alert
                description={t("subscription.scriptAssociation.empty")}
                showIcon
                type="info"
              />
            ) : null}
          </div>
        </Modal>
      </div>
    </>
  );
}
