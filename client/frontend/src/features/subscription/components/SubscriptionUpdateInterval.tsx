import { Select } from "antd";
import { useTranslation } from "react-i18next";
import { FeatureHelp } from "../../../components/help/FeatureHelp";

export const SUBSCRIPTION_UPDATE_INTERVALS = [10, 30, 60, 180, 720] as const;

export function SubscriptionUpdateInterval({
  value,
  onChange,
  disabled,
}: {
  value: number;
  onChange: (minutes: number) => void;
  disabled?: boolean;
}) {
  const { t } = useTranslation();
  return (
    <label>
      <span className="feature-title-with-help">
        {t("subscriptionAutoUpdate.label")}
        <FeatureHelp compact topic="subscriptionFetch" />
      </span>
      <Select
        aria-label={t("subscriptionAutoUpdate.label")}
        disabled={disabled}
        showSearch={false}
        value={value}
        onChange={onChange}
        options={SUBSCRIPTION_UPDATE_INTERVALS.map((minutes) => ({
          value: minutes,
          label: t(
            minutes < 60
              ? "subscriptionAutoUpdate.minutes"
              : "subscriptionAutoUpdate.hours",
            { count: minutes < 60 ? minutes : minutes / 60 },
          ),
        }))}
      />
    </label>
  );
}
