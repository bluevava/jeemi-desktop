import { Button, Switch, Typography } from "antd";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { homeSettingsSectionPath } from "../../app/homeSettingsNavigation";
import { RuntimePreferenceItem } from "../runtime/RuntimePreferenceFields";
import { useZashboardState } from "./useZashboardState";
import { ZashboardProgress } from "./ZashboardProgress";
import "./external-ui.css";

export function ExternalUIPreference({ enabled, onChange }: { enabled: boolean; onChange: (value: boolean) => void }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { state } = useZashboardState(enabled);
  return <div className="external-ui-preference">
    <RuntimePreferenceItem title={t("externalUI.enable")} helpTopic="externalUI">
      <div className="zashboard-controls">
        <Button size="small" type="link" onClick={() => navigate(homeSettingsSectionPath("zashboard"))}>{t("externalUI.manage")}</Button>
        <Switch aria-label={t("externalUI.enable")} checked={enabled} onChange={onChange}
          checkedChildren={t("runtime.on")} unCheckedChildren={t("runtime.off")} />
      </div>
    </RuntimePreferenceItem>
    <ZashboardProgress state={state} />
    {enabled && state?.lastError && !state.phase && <Typography.Text type="danger">{t("externalUI.prepareFailed")}</Typography.Text>}
  </div>;
}
