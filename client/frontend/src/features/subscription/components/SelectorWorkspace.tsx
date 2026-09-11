import {
  ApartmentOutlined,
  AppstoreOutlined,
  ArrowRightOutlined,
  DownOutlined,
  GlobalOutlined,
  SearchOutlined,
  TagsOutlined,
} from "@ant-design/icons";
import { Empty, Input, Segmented, Tabs, Tooltip } from "antd";
import { useEffect, useMemo } from "react";
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
} from "../../../types/subscription";
import {
  setSelectorExpanded,
  useSubscriptionPageStateField,
} from "../SubscriptionPageStateContext";
import { selectorPresentation } from "../selectorPresentation";
import { filterSelectorsByName, hasNodeNameSearch } from "../selectorSearch";
import { createSelectorEgressResolver } from "../selectorEgress";
import { FallbackControl } from "./FallbackControl";
import { NormalizationNotice } from "./NormalizationNotice";
import { SelectorGroupContent } from "./SelectorGroupContent";
import { SelectorIcon } from "./SelectorIcon";
import { SubscriptionWarning } from "./SubscriptionWarning";
import { selectorMemberCounts, selectorTypeKey } from "../selectorNavigation";
import type { ProxyDelayResult } from "../useProxyDelayQueue";

interface SelectorWorkspaceProps {
  subscription?: SubscriptionSummary;
  fallbackBusy: boolean;
  onFallbackChange: (selection: FallbackSelection) => Promise<boolean>;
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
  const { t } = useTranslation();
  const [activeTabByWorkspace, setActiveTabByWorkspace] =
    useSubscriptionPageStateField("activeTabByWorkspace");
  const [expandedSelectorsByWorkspace, setExpandedSelectorsByWorkspace] =
    useSubscriptionPageStateField("expandedSelectorsByWorkspace");
  const [selectorNameQuery, setSelectorNameQuery] =
    useSubscriptionPageStateField("selectorNameQuery");
  const visibleSelectors = useMemo(
    () => filterSelectorsByName(selectors, selectorNameQuery),
    [selectors, selectorNameQuery],
  );
  const workspaceKey = `${projection?.subscriptionId || "none"}:${outboundMode}`;
  const activeTab = activeTabByWorkspace[workspaceKey] ?? "";
  const effectiveActiveTab = visibleSelectors.some((selector) => selector.name === activeTab)
    ? activeTab
    : (visibleSelectors[0]?.name ?? "");
  const expandedSelectors = expandedSelectorsByWorkspace[workspaceKey];
  const expandedSelectorNames = useMemo(
    () => new Set(expandedSelectors),
    [expandedSelectors],
  );
  const searchingNodes = hasNodeNameSearch(query);
  const searchingSelectors = hasNodeNameSearch(selectorNameQuery);
  const selectorsByName = useMemo(
    () => new Map((projection?.selectors ?? []).map((selector) => [selector.name, selector])),
    [projection?.selectors],
  );
  const resolveEgress = useMemo(
    () => createSelectorEgressResolver(selectorsByName, activeSelections, runtimeReady),
    [selectorsByName, activeSelections, runtimeReady],
  );
  const describeEgress = (selector: SubscriptionSelector, name: string) => {
    const { names, end } = resolveEgress(selector, name);
    if (end === "node" || end === "builtin") return names.at(-1) ?? "—";
    return {
      balanced: t("subscription.selector.nested.balanced"),
      relay: t("subscription.selector.egress.relay"),
      unavailable: "—",
      cycle: t("subscription.selector.egress.cycle"),
    }[end];
  };
  const egressTitle = (path: string) => t(
    runtimeReady ? "subscription.selector.egress.live" : "subscription.selector.egress.offline",
    { path },
  );

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
      <div className="selector-name-search">
        <FeatureHelp compact topic="subscriptionSelectorSearch" />
        <Input
          allowClear
          aria-label={t("subscription.selector.nameSearch")}
          onChange={(event) => setSelectorNameQuery(event.target.value)}
          placeholder={t("subscription.selector.nameSearchPlaceholder")}
          prefix={<SearchOutlined />}
          value={selectorNameQuery}
        />
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
    // A temporary search must not overwrite the remembered tab.
    if (searchingNodes || searchingSelectors || effectiveActiveTab === activeTab) return;
    setActiveTabByWorkspace((current) => ({
      ...current,
      [workspaceKey]: effectiveActiveTab,
    }));
  }, [activeTab, effectiveActiveTab, searchingNodes, searchingSelectors, setActiveTabByWorkspace, workspaceKey]);

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
        <SubscriptionWarning
          noticeKey={`projection:${projection.status}`}
          description={t(`subscription.projection.status.${projection.status}`)}
          title={t("subscription.projection.failed")}
        />
      </section>
    );
  }

  const nodeGrid = (selector: SubscriptionSelector) => {
    const groupName = outboundMode === "global" ? "GLOBAL" : selector.name;
    return (
      <SelectorGroupContent
        root={selector}
        rootGroupName={groupName}
        workspaceKey={workspaceKey}
        activeSelections={activeSelections}
        resolveEgress={resolveEgress}
        selectorsByName={selectorsByName}
        busyDelayNodes={busyDelayNodes}
        busySelection={busySelection}
        delayQueueActive={delayQueueActive}
        delayResults={delayResults}
        onSelect={onSelect}
        onTestDelay={onTestDelay}
        runtimeReady={runtimeReady}
        sortMode={sortMode}
      />
    );
  };

  return (
    <section className={`selector-workspace density-${density}`}>
      {toolbar}
      <NormalizationNotice summary={projection.summary} />
      {outboundMode === "rule" && projection.warnings.includes(
        "selector_dynamic_filter_not_evaluated",
      ) ? (
        <SubscriptionWarning
          noticeKey="selector-dynamic-filters"
          description={t("subscription.selector.dynamicFiltersPending")}
        />
      ) : null}

      {outboundMode === "direct" ? (
        <Empty
          description={t("subscription.selector.directMode")}
          image={Empty.PRESENTED_IMAGE_SIMPLE}
        />
      ) : visibleSelectors.length === 0 ? (
        <Empty
          description={t(
            searchingSelectors
              ? "subscription.selector.noSelectorSearchResults"
              : searchingNodes
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
          activeKey={effectiveActiveTab}
          className="selector-tabs"
          destroyOnHidden
          items={visibleSelectors.map((selector) => {
            const groupName =
              outboundMode === "global" ? "GLOBAL" : selector.name;
            const egress = describeEgress(selector, groupName);
            return {
              key: selector.name,
              label: (
                <SelectorTabLabel
                  egress={egress}
                  egressTitle={egressTitle(egress)}
                  selector={selector}
                />
              ),
              children: selector.name === effectiveActiveTab ? (
                <div className="selector-tab-panel">
                  {nodeGrid(selector)}
                </div>
              ) : null,
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
          {visibleSelectors.map((selector) => {
            const groupName =
              outboundMode === "global" ? "GLOBAL" : selector.name;
            const egress = describeEgress(selector, groupName);
            const selectorStateKey =
              outboundMode === "global" ? "GLOBAL" : selector.name;
            const expanded = expandedSelectorNames.has(selectorStateKey);
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
                open={expanded}
              >
                <summary>
                  <span className="selector-summary-main">
                    <SelectorIcon selector={selector} />
                    <span className="selector-summary-copy">
                      <strong title={selector.name}>
                        {presentation.displayName}
                      </strong>
                      <small title={t("subscription.selector.memberBreakdown", selectorMemberCounts(selector))}>
                        {t(selectorTypeKey(selector.type))}
                        {" · "}
                        {t("subscription.selector.memberCount", {
                          count: selector.members.length,
                        })}
                      </small>
                    </span>
                  </span>
                  <span className="selector-summary-side">
                    <span className="selector-current" title={egressTitle(egress)}>
                      <strong>{egress}</strong>
                    </span>
                    <DownOutlined className="selector-toggle-icon" />
                  </span>
                </summary>
                {expanded ? nodeGrid(selector) : null}
              </details>
            );
          })}
        </div>
      )}
    </section>
  );
}

function SelectorTabLabel({
  egress,
  egressTitle,
  selector,
}: {
  egress: string;
  egressTitle: string;
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
          <small title={t("subscription.selector.memberBreakdown", selectorMemberCounts(selector))}>
            {t("subscription.selector.memberCount", {
              count: selector.members.length,
            })}
          </small>
        </span>
        <span className="selector-tab-current" title={egressTitle}>
          {egress}
        </span>
      </span>
    </span>
  );
}
