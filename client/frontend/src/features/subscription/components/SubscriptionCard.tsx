import {
  CopyOutlined,
  MoreOutlined,
  ReloadOutlined,
} from "@ant-design/icons";
import { Button, Card, Dropdown, Progress, Tag, Tooltip } from "antd";
import type { MenuProps } from "antd";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import type { SubscriptionSummary } from "../../../types/subscription";
import {
  formatSubscriptionBytes,
  formatSubscriptionExpiry,
  formatSubscriptionUpdateAge,
  subscriptionUsagePercent,
  subscriptionUsedBytes,
} from "../model";
import { SubscriptionIcon } from "./SubscriptionIcon";
import { SubscriptionAssociations, type SubscriptionAssociationKind } from "./SubscriptionAssociations";

interface SubscriptionCardProps {
  busy: boolean;
  copying: boolean;
  pendingAssociation: SubscriptionAssociationKind | null;
  chainGroupNames: Record<string, string>;
  disabled: boolean;
  locale: string;
  localConfigName?: string;
  localScriptName?: string;
  menu: MenuProps;
  onCopy: () => void;
  onBind: (kind: SubscriptionAssociationKind) => void;
  onUnbind: (kind: SubscriptionAssociationKind) => void;
  onRefresh: () => void;
  onSelect: () => void;
  selected: boolean;
  subscription: SubscriptionSummary;
  sortHandle?: ReactNode;
}

export function SubscriptionCard({
  busy,
  copying,
  pendingAssociation,
  chainGroupNames,
  disabled,
  locale,
  localConfigName,
  localScriptName,
  menu,
  onCopy,
  onBind,
  onUnbind,
  onRefresh,
  onSelect,
  selected,
  subscription,
  sortHandle,
}: SubscriptionCardProps) {
  const { t } = useTranslation();
  const used = subscriptionUsedBytes(subscription);
  const total = subscription.remoteProfile.totalBytes;
  const expiry = formatSubscriptionExpiry(
    subscription.remoteProfile.expiresAt,
    locale,
  );

  return (
    <Card
      aria-current={selected ? "true" : undefined}
      aria-busy={busy || copying || pendingAssociation !== null}
      aria-disabled={disabled}
      className={`subscription-card${selected ? " selected" : ""}`}
      onClick={disabled ? undefined : onSelect}
      role="button"
      tabIndex={disabled ? -1 : 0}
      variant="borderless"
      onKeyDown={(event) => {
        if (event.target !== event.currentTarget || disabled) return;
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          onSelect();
        }
      }}
    >
      <div className="subscription-card-heading">
        <span className="subscription-card-title" title={subscription.name}>
          <SubscriptionIcon
            icon={subscription.icon}
            iconKind={subscription.iconKind}
            sourceKind={subscription.sourceKind}
          />
          <strong>{subscription.name}</strong>
        </span>
        <span className="subscription-card-actions">
          <Tooltip title={t(subscription.sourceKind === "url" ? "subscription.card.copyURL" : "subscription.card.copyFilePath")}>
            <Button
              aria-label={t(subscription.sourceKind === "url" ? "subscription.card.copyURL" : "subscription.card.copyFilePath")}
              disabled={disabled}
              icon={<CopyOutlined />}
              loading={copying}
              onClick={(event) => {
                event.stopPropagation();
                onCopy();
              }}
              size="small"
              type="text"
            />
          </Tooltip>
          <Tooltip
            title={
              subscription.sourceKind === "url"
                ? t("subscription.menu.refresh")
                : t("subscription.shelf.refreshUnavailable")
            }
          >
            <span>
              <Button
                aria-label={t("subscription.menu.refresh")}
                disabled={disabled || subscription.sourceKind !== "url"}
                icon={<ReloadOutlined spin={busy} />}
                onClick={(event) => {
                  event.stopPropagation();
                  onRefresh();
                }}
                size="small"
                type="text"
              />
            </span>
          </Tooltip>
          <Dropdown disabled={disabled} menu={menu} trigger={["click"]}>
            <Button
              aria-label={t("subscription.menu.label", {
                name: subscription.name,
              })}
              disabled={disabled}
              icon={<MoreOutlined />}
              onClick={(event) => event.stopPropagation()}
              size="small"
              type="text"
            />
          </Dropdown>
        </span>
      </div>

      <Progress
        percent={subscriptionUsagePercent(subscription)}
        showInfo={false}
        size="small"
        status={total > 0 && used > total ? "exception" : "normal"}
      />

      <div className="subscription-card-metadata">
        {subscription.remoteProfile.hasTraffic ? (
          <strong title={`${formatSubscriptionBytes(used, locale)} / ${total > 0 ? formatSubscriptionBytes(total, locale) : "—"}`}>
            {formatSubscriptionBytes(used, locale)} / {total > 0 ? formatSubscriptionBytes(total, locale) : "—"}
          </strong>
        ) : (
          <span>{t("subscription.card.trafficUnavailable")}</span>
        )}
        <span title={expiry ?? t("subscription.card.expiryUnavailable")}>{expiry ?? "—"}</span>
        <time dateTime={subscription.lastFetchedAt} title={t("subscription.card.updated")}>
          {formatSubscriptionUpdateAge(
            subscription.lastFetchedAt,
            locale,
          )}
        </time>
      </div>

      <SubscriptionAssociations
        chainGroupNames={chainGroupNames}
        disabled={disabled}
        localConfigName={localConfigName}
        localScriptName={localScriptName}
        onBind={onBind}
        onUnbind={onUnbind}
        pending={pendingAssociation}
        subscription={subscription}
      />

      <div className="subscription-card-footer">
        {sortHandle}
        <Tag bordered={false}>
          {t("subscription.card.ruleProviders", {
            count: subscription.composition.ruleProviderCount,
          })}
        </Tag>
      </div>
    </Card>
  );
}
