import {
  CheckCircleFilled,
  ExclamationCircleFilled,
  LoadingOutlined,
  PauseCircleFilled,
  PlayCircleFilled,
  ReloadOutlined,
  StopFilled,
} from "@ant-design/icons";
import { Segmented, Spin, Tooltip } from "antd";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import appIcon from "../../assets/appicon.png";
import { homeSettingsSectionPath } from "../../app/homeSettingsNavigation";
import { useRuntimeStatus } from "../../app/runtime/RuntimeStatusContext";
import { FeatureHelp } from "../../components/help/FeatureHelp";
import {
  runtimeActionDisplayState,
  runtimePrimaryAction,
} from "../../app/runtime/runtimeActionState";
import type {
  OutboundMode,
  ProxyMode,
  RuntimeStatus,
  TunStack,
} from "../../types/runtime";
import { HomeAppearanceQuickControls } from "./HomeAppearanceQuickControls";
import {
  formatMemoryBytes,
  platformFailureMessageKey,
  shouldShowMihomoUptime,
} from "./runtimePresentation";
import { RuntimeUptimeLabels } from "./RuntimeUptimeLabels";
import type { RuntimePreferencesEditorState } from "./useRuntimePreferencesDraft";

const transitionalStates = new Set([
  "preparing",
  "validating",
  "starting",
  "reloading",
  "stopping",
  "recovering",
]);

interface RuntimeStatusHeroProps {
  preferences: RuntimePreferencesEditorState;
}

export function RuntimeStatusHero({ preferences }: RuntimeStatusHeroProps) {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const { runtime, actionError, actionKind, restart, start, stop } =
    useRuntimeStatus();
  const mihomo = runtime?.mihomo;
  const configuration = runtime?.configuration;
  const coreState = runtime?.core.state ?? "not_installed";
  const displayState = runtimeActionDisplayState(coreState, actionKind);
  const running = mihomo?.state === "running";
  const transitioning =
    actionKind !== null || Boolean(runtime?.coreAuthorizationPending) || transitionalStates.has(mihomo?.state ?? "");
  const stopping = actionKind === "stop" || mihomo?.state === "stopping";
  const coreUnavailable =
    coreState === "not_installed" || coreState === "downloading";
  const showSplitControls =
    runtimePrimaryAction(
      mihomo?.state ?? "stopped",
      Boolean(mihomo?.desiredRunning),
    ) === "restart";
  const failure = runtimeFailure(runtime);
  const platformFailureKey = failure ? platformFailureMessageKey(failure.code) : null;
  const geoStatus = runtime?.geoData.status ?? "missing";
  const geoUnavailable = geoStatus === "missing" || geoStatus === "invalid";
  const coreNeedsInstall = coreState === "not_installed";
  const coreUsable =
    coreState !== "not_installed" &&
    coreState !== "downloading" &&
    coreState !== "failed";
  const aligned = Boolean(
    running &&
      configuration?.desiredFingerprint &&
      configuration.desiredFingerprint === configuration.appliedFingerprint,
  );
  const health = runtime?.coreAuthorizationPending
    ? t(runtime?.platform.os === "darwin" ? "macNetwork.preparing" : "runtime.authorizationPending")
    : actionKind
    ? t(`runtime.status.${displayState}`)
    : failure
      ? t("home.statusControl.failure", {
          code: failure.code,
          message: platformFailureKey ? t(platformFailureKey) : failure.message,
          phase: failure.phase,
        })
      : actionError
        ? t("home.runtimeActionError")
      : transitioning
          ? t(`runtime.status.${displayState}`)
          : coreUsable && geoUnavailable
            ? t(
                geoStatus === "missing" && !running
                  ? "home.statusControl.geoMissing"
                  : "home.statusControl.geoInvalid",
              )
          : running
            ? t(
                aligned
                  ? "home.statusControl.healthy"
                  : "home.statusControl.pending",
              )
            : coreState === "not_installed" ||
                coreState === "downloading" ||
                coreState === "failed"
              ? t(runtime?.statusMessageKey ?? "runtime.status.frameworkReady")
              : !configuration?.subscriptionId
                ? t("home.statusControl.noSubscription")
                : t(
                    runtime?.statusMessageKey ?? "runtime.status.frameworkReady",
                  );
  const healthIsGeoAction = Boolean(
    !actionKind &&
      !failure &&
      !actionError &&
      !transitioning &&
      coreUsable &&
      geoUnavailable,
  );
  const healthError =
    !actionKind && Boolean(failure || actionError || healthIsGeoAction);
  const showMihomoUptime = shouldShowMihomoUptime({
    state: mihomo?.state,
    controllerReady: Boolean(mihomo?.controllerReady),
    transitioning,
    hasError: Boolean(failure || actionError),
  });
  const healthClass = healthError
    ? " error"
    : !transitioning && running
      ? " healthy"
      : "";
  const orbClass = transitioning
    ? stopping
      ? " transitioning stopping"
      : " transitioning starting"
    : running
      ? " running"
      : " stopped";

  return (
    <section className="overview-hero">
      <div className={`connection-orb${orbClass}`}>
        <img
          alt=""
          aria-hidden="true"
          className="connection-orb-app-icon"
          draggable={false}
          src={appIcon}
        />
        {transitioning ? (
          <div
            aria-label={t(`runtime.status.${displayState}`)}
            className="connection-orb-busy"
          >
            <LoadingOutlined spin />
          </div>
        ) : showSplitControls ? (
          <div className="connection-orb-split">
            <Tooltip title={t("action.restart")}>
              <button
                aria-label={t("action.restart")}
                className="connection-orb-action restart"
                disabled={coreUnavailable}
                onClick={() => void restart()}
                type="button"
              >
                <ReloadOutlined />
              </button>
            </Tooltip>
            <Tooltip placement="bottom" title={t("action.stop")}>
              <button
                aria-label={t("action.stop")}
                className="connection-orb-action stop"
                onClick={() => void stop()}
                type="button"
              >
                <StopFilled />
              </button>
            </Tooltip>
          </div>
        ) : (
          <Tooltip title={t("action.start")}>
            <button
              aria-label={t("action.start")}
              className="connection-orb-action start"
              disabled={coreUnavailable}
              onClick={() => void start()}
              type="button"
            >
              <PlayCircleFilled />
            </button>
          </Tooltip>
        )}
      </div>

      <div className="overview-hero-copy">
        <span className="state-label">{t("home.stateTitle")}</span>
        {coreNeedsInstall && !actionKind ? (
          <button
            className="hero-runtime-state error status-navigation-action"
            onClick={() =>
              navigate(homeSettingsSectionPath("mihomo", { check: true }))
            }
            type="button"
          >
            <ExclamationCircleFilled />
            <strong>{t("home.statusControl.mihomoMissing")}</strong>
          </button>
        ) : (
          <div className={`hero-runtime-state ${statusTone(displayState)}`}>
            {statusIcon(displayState, transitioning)}
            <strong>{t(`runtime.status.${displayState}`)}</strong>
          </div>
        )}
        <div className="runtime-memory-grid">
          <MemoryValue
            label={t("home.statusControl.memory.client")}
            value={formatMemoryBytes(
              runtime?.memory.clientBytes ?? null,
              i18n.language,
            )}
          />
          <MemoryValue
            label={t(runtime?.platform.os === "linux"
              ? "home.statusControl.memory.webKitGTK"
              : runtime?.platform.os === "darwin"
                ? "home.statusControl.memory.webKit"
                : "home.statusControl.memory.webView")}
            value={formatMemoryBytes(
              runtime?.memory.webViewBytes ?? null,
              i18n.language,
            )}
          />
          <MemoryValue
            label={t("home.statusControl.memory.mihomo")}
            value={formatMemoryBytes(
              runtime?.memory.mihomoBytes ?? null,
              i18n.language,
            )}
          />
          <MemoryValue
            label={t("home.statusControl.memory.total")}
            help
            value={formatMemoryBytes(
              runtime?.memory.totalBytes ?? null,
              i18n.language,
            )}
          />
        </div>
        {healthIsGeoAction ? (
          <button
            className={`runtime-health-summary${healthClass} status-navigation-action`}
            onClick={() =>
              navigate(homeSettingsSectionPath("geodata", { check: true }))
            }
            type="button"
          >
            <ExclamationCircleFilled />
            <span>{health}</span>
          </button>
        ) : (
          <div className={`runtime-health-summary${healthClass}`}>
            {healthError ? (
              <ExclamationCircleFilled />
            ) : transitioning ? (
              <LoadingOutlined spin />
            ) : running ? (
              <CheckCircleFilled />
            ) : (
              <PauseCircleFilled />
            )}
            <span>{health}</span>
          </div>
        )}
        <RuntimeUptimeLabels
          clientStartedAt={runtime?.clientStartedAt ?? ""}
          mihomoStartedAt={
            showMihomoUptime ? (mihomo?.startedAt ?? null) : null
          }
        />
      </div>

      <div className="overview-mode-controls">
        {preferences.loading || !preferences.draft ? (
          <div className="overview-mode-loading">
            <Spin size="small" />
            <span>{t("home.runtimePreferences.loading")}</span>
          </div>
        ) : (
          <>
            <Segmented
              aria-label={t("home.runtimePreferences.proxyMode")}
              block
              onChange={(value) =>
                preferences.updateDraft({ proxyMode: value as ProxyMode })
              }
              options={(["system_proxy", "tun"] as const).map((value) => ({
                label: t(`home.runtimePreferences.proxyModes.${value}`),
                value,
              }))}
              value={preferences.draft.proxyMode}
            />
            <Segmented
              aria-label={t("home.runtimePreferences.outboundMode")}
              block
              onChange={(value) =>
                preferences.updateDraft({ outboundMode: value as OutboundMode })
              }
              options={(["rule", "global", "direct"] as const).map((value) => ({
                label: t(`home.runtimePreferences.outboundModes.${value}`),
                value,
              }))}
              value={preferences.draft.outboundMode}
            />
            <Segmented
              aria-label={t("home.runtimePreferences.tunStack")}
              block
              onChange={(value) =>
                preferences.updateDraft({ tunStack: value as TunStack })
              }
              options={(["system", "gvisor", "mixed"] as const).map((value) => ({
                label: t(`home.runtimePreferences.tunStacks.${value}`),
                value,
              }))}
              value={preferences.draft.tunStack}
            />
            <HomeAppearanceQuickControls />
          </>
        )}
      </div>
    </section>
  );
}

function MemoryValue({ label, value, help }: { label: string; value: string; help?: boolean }) {
  return (
    <div>
      <span className="runtime-memory-label">
        <span>{label}</span>
        {help && <FeatureHelp topic="processMemory" compact />}
      </span>
      <strong>{value}</strong>
    </div>
  );
}

function runtimeFailure(runtime: RuntimeStatus | null) {
  if (!runtime) return null;
  if (runtime.mihomo.state === "failed" && runtime.mihomo.lastError) {
    return runtime.mihomo.lastError;
  }
  return runtime.configuration.lastError ?? runtime.mihomo.lastError;
}

function statusTone(state: RuntimeStatus["core"]["state"]): string {
  if (state === "running") return "healthy";
  if (state === "failed" || state === "not_installed") return "error";
  if (state === "starting" || state === "downloading") return "busy";
  return "idle";
}

function statusIcon(
  state: RuntimeStatus["core"]["state"],
  transitioning: boolean,
) {
  if (transitioning || state === "starting" || state === "downloading") {
    return <LoadingOutlined spin />;
  }
  if (state === "running") return <CheckCircleFilled />;
  if (state === "failed" || state === "not_installed") {
    return <ExclamationCircleFilled />;
  }
  return <PauseCircleFilled />;
}
