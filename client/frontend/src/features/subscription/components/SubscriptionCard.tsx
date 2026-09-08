import {
  MoreOutlined,
  ReloadOutlined,
} from "@ant-design/icons";
import { Button, Card, Dropdown, Progress, Tag, Tooltip } from "antd";
import type { MenuProps } from "antd";
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

interface SubscriptionCardProps {
  busy: boolean;
  disabled: boolean;
  locale: string;
  localConfigName?: string;
  localScriptName?: string;
  menu: MenuProps;
  onRefresh: () => void;
  onSelect: () => void;
  selected: boolean;
  subscription: SubscriptionSummary;
}

export function SubscriptionCard({
  busy,
  disabled,
  locale,
  localConfigName,
  localScriptName,
  menu,
  onRefresh,
  onSelect,
  selected,
  subscription,
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
      className={`subscription-card${selected ? " selected" : ""}`}
      loading={busy}
      onClick={onSelect}
      role="button"
      tabIndex={0}
      variant="borderless"
      onKeyDown={(event) => {
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

      <div className="subscription-card-traffic">
        {subscription.remoteProfile.hasTraffic ? (
          <strong>
            {formatSubscriptionBytes(used, locale)} / {total > 0 ? formatSubscriptionBytes(total, locale) : "—"}
          </strong>
        ) : (
          <span>{t("subscription.card.trafficUnavailable")}</span>
        )}
        {expiry ? <span>· {expiry}</span> : null}
      </div>

      <div className="subscription-card-footer">
        <time dateTime={subscription.lastFetchedAt}>
          {formatSubscriptionUpdateAge(
            subscription.lastFetchedAt,
            locale,
          )}
        </time>
        <Tag bordered={false}>
          {t("subscription.card.ruleProviders", {
            count: subscription.composition.ruleProviderCount,
          })}
        </Tag>
      </div>
      {localConfigName || localScriptName ? (
        <div className="subscription-card-association">
          {t("subscription.card.associated", {
            name: localConfigName || localScriptName,
            type: t(
              localConfigName
                ? "subscription.card.handlerConfig"
                : "subscription.card.handlerScript",
            ),
          })}
        </div>
      ) : null}
    </Card>
  );
}
