import {
  DatabaseOutlined,
  DownOutlined,
  EyeInvisibleOutlined,
  EyeOutlined,
  LoadingOutlined,
  PlusOutlined,
  ReloadOutlined,
  SearchOutlined,
  ThunderboltOutlined,
  UpOutlined,
} from "@ant-design/icons";
import { Button, Dropdown, Empty, Input, Tooltip } from "antd";
import type { MenuProps } from "antd";
import type { CSSProperties, KeyboardEvent } from "react";
import { useTranslation } from "react-i18next";

import { FeatureHelp } from "../../../components/help/FeatureHelp";
import { RuntimeConfigurationViewer } from "../../runtime/RuntimeConfigurationViewer";
import type {
  SubscriptionState,
  SubscriptionSummary,
} from "../../../types/subscription";
import {
  formatSubscriptionBytes,
  formatSubscriptionExpiry,
  formatSubscriptionUpdateAge,
  subscriptionUsagePercent,
  subscriptionUsedBytes,
} from "../model";
import { SubscriptionCard } from "./SubscriptionCard";
import { SubscriptionIcon } from "./SubscriptionIcon";

interface SubscriptionShelfProps {
  addMenu: MenuProps;
  busyAction: string | null;
  expanded: boolean;
  locale: string;
  localConfigNames: Record<string, string>;
  localScriptNames: Record<string, string>;
  menuFor: (subscription: SubscriptionSummary) => MenuProps;
  onExpandedChange: (expanded: boolean) => void;
  onOpenRuleProviders: () => void;
  onRefreshSelected: (subscription: SubscriptionSummary) => void;
  onSelect: (subscription: SubscriptionSummary) => void;
  onSelectorQueryChange: (query: string) => void;
  onShowHiddenSelectorsChange: (show: boolean) => void;
  onSpeedTest: () => void;
  runtimeReady: boolean;
  selectorQuery: string;
  showHiddenSelectors: boolean;
  speedTestBusy: boolean;
  speedTestAvailable: boolean;
  state: SubscriptionState;
}

export function SubscriptionShelf({
  addMenu,
  busyAction,
  expanded,
  locale,
  localConfigNames,
  localScriptNames,
  menuFor,
  onExpandedChange,
  onOpenRuleProviders,
  onRefreshSelected,
  onSelect,
  onSelectorQueryChange,
  onShowHiddenSelectorsChange,
  onSpeedTest,
  runtimeReady,
  selectorQuery,
  showHiddenSelectors,
  speedTestBusy,
  speedTestAvailable,
  state,
}: SubscriptionShelfProps) {
  const { t } = useTranslation();
  const speedTestLabel = t(
    selectorQuery.trim()
      ? "subscription.shelf.speedTestMatched"
      : "subscription.shelf.speedTest",
  );
  const selected = state.subscriptions.find(
    (item) => item.id === state.selectedSubscriptionId,
  );

  if (!expanded && selected) {
    const used = subscriptionUsedBytes(selected);
    const total = selected.remoteProfile.totalBytes;
    const expiry = formatSubscriptionExpiry(
      selected.remoteProfile.expiresAt,
      locale,
    );
    const usagePercent = subscriptionUsagePercent(selected);
    const trafficLabel = selected.remoteProfile.hasTraffic
      ? `${formatSubscriptionBytes(used, locale)} / ${
          total > 0 ? formatSubscriptionBytes(total, locale) : "—"
        }`
      : t("subscription.card.trafficUnavailable");
    const trafficStyle = {
      "--subscription-usage-percent": `${usagePercent}%`,
    } as CSSProperties;

    return (
      <section className="subscription-shelf collapsed">
        <div className="subscription-shelf-summary">
          <div className="subscription-shelf-summary-main">
            <div
              aria-label={t("subscription.shelf.subscriptionTools")}
              className="subscription-shelf-actions subscription-shelf-actions-left"
            >
              <Tooltip
                title={
                  selected.sourceKind === "url"
                    ? t("subscription.shelf.refreshCurrent")
                    : t("subscription.shelf.refreshUnavailable")
                }
              >
                <span>
                  <Button
                    aria-label={t("subscription.shelf.refreshCurrent")}
                    disabled={busyAction !== null || selected.sourceKind !== "url"}
                    icon={<ReloadOutlined spin={busyAction === selected.id} />}
                    onClick={() => onRefreshSelected(selected)}
                    type="text"
                  />
                </span>
              </Tooltip>
              <Tooltip title={t("subscription.shelf.openRuleProviders")}>
                <span>
                  <Button
                    aria-label={t("subscription.shelf.openRuleProviders")}
                    disabled={!state.projection}
                    icon={<DatabaseOutlined />}
                    onClick={onOpenRuleProviders}
                    type="text"
                  />
                </span>
              </Tooltip>
              <RuntimeConfigurationViewer />
            </div>
            <button
              aria-label={t("subscription.shelf.expand")}
              className="subscription-summary-details"
              onClick={() => onExpandedChange(true)}
              type="button"
            >
              <span className="subscription-summary-name">
                <SubscriptionIcon
                  icon={selected.icon}
                  iconKind={selected.iconKind}
                  sourceKind={selected.sourceKind}
                />
                <strong>{selected.name}</strong>
              </span>
              <span
                aria-label={trafficLabel}
                aria-valuemax={selected.remoteProfile.hasTraffic ? 100 : undefined}
                aria-valuemin={selected.remoteProfile.hasTraffic ? 0 : undefined}
                aria-valuenow={
                  selected.remoteProfile.hasTraffic ? usagePercent : undefined
                }
                aria-valuetext={
                  selected.remoteProfile.hasTraffic ? trafficLabel : undefined
                }
                className="subscription-summary-traffic"
                role={
                  selected.remoteProfile.hasTraffic ? "progressbar" : undefined
                }
                style={trafficStyle}
              >
                {trafficLabel}
              </span>
              <span className="subscription-summary-expiry">
                {expiry ?? t("subscription.card.expiryUnavailable")}
              </span>
              <span className="subscription-summary-updated">
                {formatSubscriptionUpdateAge(selected.lastFetchedAt, locale)}
              </span>
            </button>
            <Input
              allowClear
              aria-label={t("subscription.selector.search")}
              className="subscription-summary-search"
              onChange={(event) => onSelectorQueryChange(event.target.value)}
              placeholder={t("subscription.selector.search")}
              prefix={<SearchOutlined />}
              size="small"
              value={selectorQuery}
            />
          </div>
          <div
            aria-label={t("subscription.shelf.selectorTools")}
            className="subscription-shelf-actions subscription-shelf-actions-right"
          >
            <Tooltip
              title={
                speedTestBusy
                  ? t("subscription.shelf.speedTestBusy")
                  : !runtimeReady
                    ? t("subscription.shelf.speedTestRequiresCore")
                    : !speedTestAvailable
                      ? t("subscription.shelf.speedTestEmpty")
                      : speedTestLabel
              }
            >
              <span>
                <Button
                  aria-label={speedTestLabel}
                  disabled={!runtimeReady || speedTestBusy || !speedTestAvailable}
                  icon={
                    speedTestBusy ? (
                      <LoadingOutlined spin />
                    ) : (
                      <ThunderboltOutlined />
                    )
                  }
                  onClick={onSpeedTest}
                  type="text"
                />
              </span>
            </Tooltip>
            <Tooltip
              title={t(
                showHiddenSelectors
                  ? "subscription.shelf.hideHiddenSelectors"
                  : "subscription.shelf.showHiddenSelectors",
              )}
            >
              <Button
                aria-label={t(
                  showHiddenSelectors
                    ? "subscription.shelf.hideHiddenSelectors"
                    : "subscription.shelf.showHiddenSelectors",
                )}
                className={showHiddenSelectors ? "active" : ""}
                icon={
                  showHiddenSelectors ? <EyeOutlined /> : <EyeInvisibleOutlined />
                }
                onClick={() =>
                  onShowHiddenSelectorsChange(!showHiddenSelectors)
                }
                type="text"
              />
            </Tooltip>
            <Tooltip title={t("subscription.shelf.expand")}>
              <Button
                aria-label={t("subscription.shelf.expand")}
                icon={<DownOutlined />}
                onClick={() => onExpandedChange(true)}
                type="text"
              />
            </Tooltip>
          </div>
        </div>
      </section>
    );
  }

  const collapseOnKeyboard = (event: KeyboardEvent<HTMLDivElement>) => {
    if (!selected || (event.key !== "Enter" && event.key !== " ")) {
      return;
    }
    event.preventDefault();
    onExpandedChange(false);
  };

  return (
    <section className="subscription-shelf expanded">
      <div
        aria-label={selected ? t("subscription.shelf.collapse") : undefined}
        className={`subscription-shelf-heading${selected ? " collapsible" : ""}`}
        onClick={selected ? () => onExpandedChange(false) : undefined}
        onKeyDown={collapseOnKeyboard}
        role={selected ? "button" : undefined}
        tabIndex={selected ? 0 : undefined}
      >
        <div>
          <strong>{t("subscription.shelf.title")}</strong>
          <span
            className="subscription-shelf-help"
            onClick={(event) => event.stopPropagation()}
            onKeyDown={(event) => event.stopPropagation()}
          >
            <FeatureHelp compact topic="subscriptionShelf" />
          </span>
        </div>
        {selected ? (
          <span className="subscription-shelf-collapse-hint">
            {t("subscription.shelf.collapseHint")}
          </span>
        ) : null}
        {selected ? <UpOutlined /> : null}
      </div>
      {state.subscriptions.length === 0 ? (
        <Empty
          description={t("subscription.list.emptyDescription")}
          image={Empty.PRESENTED_IMAGE_SIMPLE}
        />
      ) : null}
      <div className="subscription-card-grid">
        {state.subscriptions.map((subscription) => (
          <SubscriptionCard
            busy={busyAction === subscription.id}
            disabled={busyAction !== null}
            key={subscription.id}
            locale={locale}
            localConfigName={localConfigNames[subscription.localConfigId]}
            localScriptName={localScriptNames[subscription.localScriptId]}
            menu={menuFor(subscription)}
            onRefresh={() => onRefreshSelected(subscription)}
            onSelect={() => onSelect(subscription)}
            selected={subscription.id === state.selectedSubscriptionId}
            subscription={subscription}
          />
        ))}
        <Dropdown disabled={busyAction !== null} menu={addMenu} trigger={["click"]}>
          <button
            aria-label={t("subscription.import.add")}
            className="subscription-add-card"
            type="button"
          >
            <PlusOutlined />
            <strong>{t("subscription.import.add")}</strong>
          </button>
        </Dropdown>
      </div>
    </section>
  );
}
