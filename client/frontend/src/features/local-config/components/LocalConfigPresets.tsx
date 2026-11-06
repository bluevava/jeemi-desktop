import { Button, Card } from "antd";
import { useTranslation } from "react-i18next";
import { ResourceFieldLabel } from "./StrategyRuleSetReferencesEditor";

export function LocalConfigPresets({
  busy,
  onCreate,
}: {
  busy: string | null;
  onCreate: (kind: "general" | "process") => void;
}) {
  const { t } = useTranslation();
  return (
    <>
      {(["general", "process"] as const).map((kind) => (
        <Card
          key={kind}
          className="local-config-card local-config-preset-card"
          variant="borderless"
        >
          <ResourceFieldLabel
            helpBase="offlineConfig.help.presets"
            label={t(`offlineConfig.${kind}`)}
          />
          <p>{t(`offlineConfig.${kind}Description`)}</p>
          <Button
            size="small"
            disabled={busy !== null}
            loading={busy === `preset:${kind}`}
            onClick={() => onCreate(kind)}
          >
            {t("offlineConfig.create")}
          </Button>
        </Card>
      ))}
    </>
  );
}
