import { Switch } from "antd";
import { useTranslation } from "react-i18next";

export function ResourceActivationControl({
  name,
  disabled,
  busy,
  onChange,
}: {
  name: string;
  disabled?: boolean;
  busy: boolean;
  onChange: (enabled: boolean) => void;
}) {
  const { t } = useTranslation();
  return (
    <span className="config-resource-activation">
      <Switch
        size="small"
        aria-label={t("offlineConfig.toggleResource", { name })}
        checked={!disabled}
        disabled={busy}
        onChange={onChange}
      />
      <span>
        {t(disabled ? "offlineConfig.disabled" : "offlineConfig.enabled")}
      </span>
    </span>
  );
}
