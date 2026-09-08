import { CloudDownloadOutlined } from "@ant-design/icons";
import { Button, Drawer, Empty, Switch, Tooltip } from "antd";
import { useTranslation } from "react-i18next";

import { FeatureHelp } from "../../../components/help/FeatureHelp";
import type { MihomoRuleProviderRuntimeState } from "../../../lib/mihomo/client";
import type { SubscriptionRuleProvider } from "../../../types/subscription";
import { formatSubscriptionUpdateAge } from "../model";

interface RuleProviderDrawerProps {
  busyProvider: string;
  busyToggle: string;
  locale: string;
  onClose: () => void;
  onRefresh: (name: string) => Promise<void>;
  onToggle: (name: string, enabled: boolean) => Promise<void>;
  open: boolean;
  providers: SubscriptionRuleProvider[];
  runtimeLoading: boolean;
  runtimeProviders: Readonly<
    Record<string, MihomoRuleProviderRuntimeState>
  >;
  runtimeReady: boolean;
}

export function RuleProviderDrawer({
  busyProvider,
  busyToggle,
  locale,
  onClose,
  onRefresh,
  onToggle,
  open,
  providers,
  runtimeLoading,
  runtimeProviders,
  runtimeReady,
}: RuleProviderDrawerProps) {
  const { t } = useTranslation();
  const enabledCount = providers.filter((provider) => provider.enabled).length;

  return (
    <Drawer
      className="rule-provider-drawer"
      onClose={onClose}
      open={open}
      title={
        <span className="rule-provider-drawer-title">
          {t("subscription.ruleProvider.title")}
          <FeatureHelp compact topic="ruleProviderRefresh" />
        </span>
      }
      width="min(560px, calc(100vw - 24px))"
    >
      <p className="rule-provider-drawer-count">
        {t("subscription.ruleProvider.count", {
          enabled: enabledCount,
          total: providers.length,
        })}
      </p>
      {providers.length === 0 ? (
        <Empty
          description={t("subscription.ruleProvider.empty")}
          image={Empty.PRESENTED_IMAGE_SIMPLE}
        />
      ) : (
        <div className="rule-provider-grid">
          {providers.map((provider) => {
            const runtimeProvider = runtimeProviders[provider.name];
            const updatedLabel = !provider.enabled
              ? t("subscription.ruleProvider.disabledDescription")
              : runtimeProvider?.updatedAt
                ? t("subscription.ruleProvider.updated", {
                    time: formatSubscriptionUpdateAge(
                      runtimeProvider.updatedAt,
                      locale,
                    ),
                  })
                : runtimeReady && runtimeLoading
                  ? t("subscription.ruleProvider.loadingStatus")
                  : runtimeReady
                    ? t("subscription.ruleProvider.updateUnknown")
                    : t("subscription.ruleProvider.updateRequiresCore");
            return (
              <div
                className={`rule-provider-item${
                  provider.enabled ? "" : " disabled"
                }`}
                key={provider.name}
              >
                <span className="rule-provider-copy">
                  <strong>{provider.name}</strong>
                  <small>
                    {[provider.behavior, provider.type, provider.format]
                      .filter(Boolean)
                      .join(" · ") || "—"}
                    {runtimeProvider
                      ? ` · ${t("subscription.ruleProvider.ruleCount", {
                          count: runtimeProvider.ruleCount,
                        })}`
                      : ""}
                  </small>
                  <small className="rule-provider-updated">
                    {updatedLabel}
                  </small>
                </span>
                <span className="rule-provider-actions">
                  <Tooltip
                    title={t(
                      provider.enabled
                        ? "subscription.ruleProvider.disable"
                        : "subscription.ruleProvider.enable",
                      { name: provider.name },
                    )}
                  >
                    <Switch
                      aria-label={t(
                        provider.enabled
                          ? "subscription.ruleProvider.disable"
                          : "subscription.ruleProvider.enable",
                        { name: provider.name },
                      )}
                      checked={provider.enabled}
                      disabled={busyProvider !== "" || busyToggle !== ""}
                      loading={busyToggle === provider.name}
                      onChange={(checked) =>
                        void onToggle(provider.name, checked)
                      }
                      size="small"
                    />
                  </Tooltip>
                  <Tooltip
                    title={
                      !provider.enabled
                        ? t("subscription.ruleProvider.disabledRefresh")
                        : runtimeReady
                          ? t("subscription.ruleProvider.refresh", {
                              name: provider.name,
                            })
                          : t("subscription.ruleProvider.requiresCore")
                    }
                  >
                    <span>
                      <Button
                        aria-label={t("subscription.ruleProvider.refresh", {
                          name: provider.name,
                        })}
                        disabled={
                          !provider.enabled ||
                          !runtimeReady ||
                          busyToggle !== ""
                        }
                        icon={<CloudDownloadOutlined />}
                        loading={busyProvider === provider.name}
                        onClick={() => void onRefresh(provider.name)}
                        size="small"
                      />
                    </span>
                  </Tooltip>
                </span>
              </div>
            );
          })}
        </div>
      )}
    </Drawer>
  );
}
