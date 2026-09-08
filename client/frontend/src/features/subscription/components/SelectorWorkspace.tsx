import {
  AimOutlined,
  ApartmentOutlined,
  AppstoreOutlined,
  ArrowRightOutlined,
  DownOutlined,
  GlobalOutlined,
  LoadingOutlined,
  TagsOutlined,
  ThunderboltOutlined,
} from "@ant-design/icons";
import { Alert, Empty, Segmented, Tabs, Tooltip } from "antd";
import { useEffect } from "react";
import { useTranslation } from "react-i18next";

import { FeatureHelp } from "../../../components/help/FeatureHelp";
import type { OutboundMode } from "../../../types/runtime";
import type {
  FallbackSelection,
  SubscriptionSummary,
  SelectorDensity,
  SelectorSortMode,
  SelectorViewMode,
  SubscriptionProjection,
  SubscriptionSelector,
  SubscriptionSelectorMember,
} from "../../../types/subscription";
import {
  setSelectorExpanded,
  useSubscriptionPageStateField,
} from "../SubscriptionPageStateContext";
import { selectorPresentation } from "../selectorPresentation";
import { FallbackControl } from "./FallbackControl";
import { NormalizationNotice } from "./NormalizationNotice";
import { sortSelectorMembers } from "../selectorSort";
import type { ProxyDelayResult } from "../useProxyDelayQueue";

interface SelectorWorkspaceProps {
  subscription?: SubscriptionSummary;
  fallbackBusy: boolean;
  onFallbackChange: (selection: FallbackSelection) => Promise<void>;
  activeSelections: Record<string, string>;
  busyDelayNodes: ReadonlySet<string>;
  busySelection: string;
  delayQueueActive: boolean;
  delayResults: Readonly<Record<string, ProxyDelayResult>>;
  density: SelectorDensity;
  displayPreferencesBusy: boolean;
  selectors: SubscriptionSelector[];
  outboundMode: OutboundMode;
  outboundModeBusy: boolean;
  onOutboundModeChange: (mode: OutboundMode) => Promise<void>;
  onDensityChange: (density: SelectorDensity) => Promise<void>;
  onSelect: (group: string, proxy: string) => Promise<void>;
  onSortModeChange: (mode: SelectorSortMode) => Promise<void>;
  onTestDelay: (proxy: string) => Promise<void>;
  onViewModeChange: (mode: SelectorViewMode) => Promise<void>;
  projection: SubscriptionProjection | null;
  query: string;
  runtimeReady: boolean;
  sortMode: SelectorSortMode;
  viewMode: SelectorViewMode;
}

export function SelectorWorkspace({
  subscription, fallbackBusy, onFallbackChange,
  activeSelections,
  busyDelayNodes,
  busySelection,
  delayQueueActive,
  delayResults,
  density,
  displayPreferencesBusy,
  selectors,
  outboundMode,
  outboundModeBusy,
  onOutboundModeChange,
  onDensityChange,
  onSelect,
  onSortModeChange,
  onTestDelay,
  onViewModeChange,
  projection,
  query,
  runtimeReady,
  sortMode,
  viewMode,
}: SelectorWorkspaceProps) {
  const { t, i18n } = useTranslation();
  const [activeTabByWorkspace, setActiveTabByWorkspace] =
    useSubscriptionPageStateField("activeTabByWorkspace");
  const [expandedSelectorsByWorkspace, setExpandedSelectorsByWorkspace] =
    useSubscriptionPageStateField("expandedSelectorsByWorkspace");
  const workspaceKey = `${projection?.subscriptionId || "none"}:${outboundMode}`;
  const activeTab = activeTabByWorkspace[workspaceKey] ?? "";
  const expandedSelectors =
    expandedSelectorsByWorkspace[workspaceKey] ?? [];
  const normalisedQuery = query.trim().toLocaleLowerCase();

  const toolbar = (
    <div className="selector-toolbar">
      <div className="selector-heading">
        <strong>{t("subscription.selector.title")}</strong>
        <FeatureHelp compact topic="selectorPreview" />
        <Segmented
          aria-label={t("subscription.selector.outboundMode")}
          className="selector-outbound-mode"
          disabled={outboundModeBusy}
          onChange={(value) =>
            void onOutboundModeChange(value as OutboundMode)
          }
          options={([
            ["rule", <ApartmentOutlined />],
            ["global", <GlobalOutlined />],
            ["direct", <ArrowRightOutlined />],
          ] as const).map(([value, icon]) => ({
            label: (
              <Tooltip
                title={t(`subscription.selector.outboundModes.${value}`)}
              >
                <span
                  aria-label={t(
                    `subscription.selector.outboundModes.${value}`,
                  )}
                  className="selector-outbound-mode-icon"
                >
                  {icon}
                </span>
              </Tooltip>
            ),
            value,
          }))}
          size="small"
          value={outboundMode}
        />
        <FallbackControl subscription={subscription} projection={projection} busy={fallbackBusy} onChange={onFallbackChange} />
      </div>
      <div
        aria-label={t("subscription.selector.tools")}
        className="selector-toolbar-actions"
      >
        <div className="selector-toolbar-control selector-density-control">
          <Segmented
            aria-label={t("subscription.selector.density.label")}
            disabled={displayPreferencesBusy}
            onChange={(value) =>
              void onDensityChange(value as SelectorDensity)
            }
            options={(["large", "medium", "small"] as const).map((value) => ({
              label: t(`subscription.selector.density.${value}`),
              value,
            }))}
            size="small"
            value={density}
          />
        </div>
        <div className="selector-toolbar-control">
          <Segmented
            aria-label={t("subscription.selector.sort.label")}
            disabled={displayPreferencesBusy}
            onChange={(value) =>
              void onSortModeChange(value as SelectorSortMode)
            }
            options={[
              {
                label: t("subscription.selector.sort.default"),
                value: "default",
              },
              {
                label: t("subscription.selector.sort.delay"),
                value: "delay",
              },
              {
                label: t("subscription.selector.sort.name"),
                value: "name",
              },
            ]}
            size="small"
            value={sortMode}
          />
        </div>
        <div className="selector-toolbar-control">
          <Segmented
            aria-label={t("subscription.selector.view.label")}
            disabled={displayPreferencesBusy}
            onChange={(value) =>
              void onViewModeChange(value as SelectorViewMode)
            }
            options={[
              {
                icon: <AppstoreOutlined />,
                label: t("subscription.selector.view.panel"),
                value: "panel",
              },
              {
                icon: <TagsOutlined />,
                label: t("subscription.selector.view.tabs"),
                value: "tabs",
              },
            ]}
            size="small"
            value={viewMode}
          />
        </div>
      </div>
    </div>
  );

  useEffect(() => {
    const next = selectors.some((selector) => selector.name === activeTab)
      ? activeTab
      : (selectors[0]?.name ?? "");
    if (next === activeTab) return;
    setActiveTabByWorkspace((current) => ({
      ...current,
      [workspaceKey]: next,
    }));
  }, [activeTab, selectors, setActiveTabByWorkspace, workspaceKey]);

  if (!projection) {
    return (
      <section className="selector-workspace empty">
        {toolbar}
        <Empty
          description={t("subscription.selector.selectSubscription")}
          image={Empty.PRESENTED_IMAGE_SIMPLE}
        />
      </section>
    );
  }

  if (projection.status !== "ready") {
    return (
      <section className={`selector-workspace density-${density}`}>
        {toolbar}
        <NormalizationNotice summary={projection.summary} />
        <Alert
          description={t(`subscription.projection.status.${projection.status}`)}
          message={t("subscription.projection.failed")}
          showIcon
          type="warning"
        />
      </section>
    );
  }

  const nodeGrid = (selector: SubscriptionSelector) => {
    const groupName = outboundMode === "global" ? "GLOBAL" : selector.name;
    const currentSelection =
      activeSelections[groupName] || selector.defaultSelection;
    return (
      <SelectorNodeGrid
        busyDelayNodes={busyDelayNodes}
        busySelection={busySelection}
        currentSelection={currentSelection}
        delayQueueActive={delayQueueActive}
        delayResults={delayResults}
        groupName={groupName}
        members={sortSelectorMembers(
          selector.members,
          sortMode,
          delayResults,
          i18n.language,
        )}
        onSelect={onSelect}
        onTestDelay={onTestDelay}
        runtimeReady={runtimeReady}
      />
    );
  };

  const providerWarning = (selector: SubscriptionSelector) =>
    selector.unresolvedProviderNames.length > 0 ? (
      <Alert
        description={t("subscription.selector.providersPending", {
          names: selector.unresolvedProviderNames.join(", "),
        })}
        showIcon
        type="warning"
      />
    ) : null;

  return (
    <section className={`selector-workspace density-${density}`}>
      {toolbar}
      <NormalizationNotice summary={projection.summary} />
      {outboundMode === "rule" && projection.warnings.includes(
        "selector_dynamic_filter_not_evaluated",
      ) ? (
        <Alert
          description={t("subscription.selector.dynamicFiltersPending")}
          showIcon
          type="warning"
        />
      ) : null}

      {outboundMode === "direct" ? (
        <Empty
          description={t("subscription.selector.directMode")}
          image={Empty.PRESENTED_IMAGE_SIMPLE}
        />
      ) : selectors.length === 0 ? (
        <Empty
          description={t(
            normalisedQuery
              ? "subscription.selector.noSearchResults"
              : outboundMode === "global"
                ? "subscription.selector.globalEmpty"
                : projection.selectors.some((selector) => selector.hidden)
                  ? "subscription.selector.hiddenOnly"
                  : "subscription.selector.empty",
          )}
          image={Empty.PRESENTED_IMAGE_SIMPLE}
        />
      ) : viewMode === "tabs" ? (
        <Tabs
          activeKey={activeTab || selectors[0]?.name}
          className="selector-tabs"
          items={selectors.map((selector) => {
            const groupName =
              outboundMode === "global" ? "GLOBAL" : selector.name;
            const currentSelection =
              activeSelections[groupName] || selector.defaultSelection;
            return {
              key: selector.name,
              label: (
                <SelectorTabLabel
                  currentSelection={currentSelection}
                  selector={selector}
                />
              ),
              children: (
                <div className="selector-tab-panel">
                  {providerWarning(selector)}
                  {nodeGrid(selector)}
                </div>
              ),
            };
          })}
          onChange={(value) =>
            setActiveTabByWorkspace((current) =>
              current[workspaceKey] === value
                ? current
                : { ...current, [workspaceKey]: value },
            )
          }
        />
      ) : (
        <div className="selector-groups">
          {selectors.map((selector) => {
            const groupName =
              outboundMode === "global" ? "GLOBAL" : selector.name;
            const currentSelection =
              activeSelections[groupName] || selector.defaultSelection;
            const selectorStateKey =
              outboundMode === "global" ? "GLOBAL" : selector.name;
            const presentation = selectorPresentation(
              selector.name,
              selector.icon,
            );
            return (
              <details
                className="selector-group"
                key={selector.name}
                onToggle={(event) => {
                  const expanded = event.currentTarget.open;
                  setExpandedSelectorsByWorkspace((current) =>
                    setSelectorExpanded(
                      current,
                      workspaceKey,
                      selectorStateKey,
                      expanded,
                    ),
                  );
                }}
                open={expandedSelectors.includes(selectorStateKey)}
              >
                <summary>
                  <span className="selector-summary-main">
                    <SelectorIcon selector={selector} />
                    <span className="selector-summary-copy">
                      <strong title={selector.name}>
                        {presentation.displayName}
                      </strong>
                      <small>
                        {selector.type ||
                          t("subscription.selector.unknownType")}
                        {" · "}
                        {t("subscription.selector.nodeCount", {
                          count: selector.members.length,
                        })}
                      </small>
                    </span>
                  </span>
                  <span className="selector-summary-side">
                    <span className="selector-current">
                      {t(
                        runtimeReady
                          ? "subscription.selector.currentSelection"
                          : "subscription.selector.defaultSelection",
                      )}
                      <strong>{currentSelection || "—"}</strong>
                    </span>
                    <DownOutlined className="selector-toggle-icon" />
                  </span>
                </summary>
                {providerWarning(selector)}
                {nodeGrid(selector)}
              </details>
            );
          })}
        </div>
      )}
    </section>
  );
}

function SelectorIcon({ selector }: { selector: SubscriptionSelector }) {
  const presentation = selectorPresentation(selector.name, selector.icon);
  return (
    <span aria-hidden="true" className="selector-group-icon">
      {presentation.emoji || <AimOutlined />}
      {presentation.iconUrl ? (
        <img
          alt=""
          loading="lazy"
          onError={(event) => {
            event.currentTarget.hidden = true;
          }}
          referrerPolicy="no-referrer"
          src={presentation.iconUrl}
        />
      ) : null}
    </span>
  );
}

function SelectorTabLabel({
  currentSelection,
  selector,
}: {
  currentSelection: string;
  selector: SubscriptionSelector;
}) {
  const { t } = useTranslation();
  const presentation = selectorPresentation(selector.name, selector.icon);
  return (
    <span className="selector-tab-label">
      <SelectorIcon selector={selector} />
      <span className="selector-tab-copy">
        <span className="selector-tab-title">
          <strong title={selector.name}>{presentation.displayName}</strong>
          <small>
            {t("subscription.selector.nodeCount", {
              count: selector.members.length,
            })}
          </small>
        </span>
        <span className="selector-tab-current" title={currentSelection}>
          {currentSelection || "—"}
        </span>
      </span>
    </span>
  );
}

interface SelectorNodeGridProps {
  busyDelayNodes: ReadonlySet<string>;
  busySelection: string;
  currentSelection: string;
  delayQueueActive: boolean;
  delayResults: Readonly<Record<string, ProxyDelayResult>>;
  groupName: string;
  members: SubscriptionSelectorMember[];
  onSelect: (group: string, proxy: string) => Promise<void>;
  onTestDelay: (proxy: string) => Promise<void>;
  runtimeReady: boolean;
}

function SelectorNodeGrid({
  busyDelayNodes,
  busySelection,
  currentSelection,
  delayQueueActive,
  delayResults,
  groupName,
  members,
  onSelect,
  onTestDelay,
  runtimeReady,
}: SelectorNodeGridProps) {
  const { t } = useTranslation();
  return (
    <div className="selector-node-grid">
      {members.map((member) => {
        const delaySupported =
          member.source === "proxy" || member.source === "provider";
        const result = delayResults[member.name];
        const delayBusy = busyDelayNodes.has(member.name);
        const delayClass = result
          ? result.status === "error"
            ? " error"
            : result.delay <= 200
              ? " fast"
              : result.delay <= 500
                ? " medium"
                : " slow"
          : "";
        return (
          <div
            className={`selector-node${
              currentSelection === member.name ? " default" : ""
            }${delaySupported ? "" : " no-delay"}`}
            key={`${member.source}-${member.providerName}-${member.name}`}
          >
            <button
              aria-pressed={currentSelection === member.name}
              className="selector-node-select"
              disabled={!runtimeReady || busySelection === groupName}
              onClick={() => void onSelect(groupName, member.name)}
              type="button"
            >
              <strong title={member.name}>{member.name}</strong>
              <span>
                {member.type || t("subscription.selector.unknownType")}
                {member.providerName ? ` · ${member.providerName}` : ""}
              </span>
            </button>
            {delaySupported ? (
              <Tooltip
                title={t(
                  !runtimeReady
                    ? "subscription.delay.requiresCore"
                    : delayQueueActive
                      ? "subscription.delay.queueBusy"
                      : result?.source === "mihomo"
                        ? "subscription.delay.retestAutomatic"
                        : result
                          ? "subscription.delay.retestCached"
                          : "subscription.delay.test",
                )}
              >
                <span className="selector-node-delay-wrap">
                  <button
                    aria-label={t(
                      result
                        ? "subscription.delay.retestNode"
                        : "subscription.delay.testNode",
                      { name: member.name },
                    )}
                    className={`selector-node-delay${delayClass}`}
                    disabled={!runtimeReady || delayQueueActive}
                    onClick={() => void onTestDelay(member.name)}
                    type="button"
                  >
                    {delayBusy ? (
                      <LoadingOutlined spin />
                    ) : result?.status === "success" ? (
                      `${result.delay} ms`
                    ) : result?.status === "error" ? (
                      t("subscription.delay.failed")
                    ) : (
                      <ThunderboltOutlined />
                    )}
                  </button>
                </span>
              </Tooltip>
            ) : null}
          </div>
        );
      })}
    </div>
  );
}
