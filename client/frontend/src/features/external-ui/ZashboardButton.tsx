import { useState } from "react";
import { App, Button, Tooltip } from "antd";
import { useTranslation } from "react-i18next";
import { useRuntimeStatus } from "../../app/runtime/RuntimeStatusContext";
import { openZashboard } from "../../services/externalUIBridge";
import icon from "../../assets/zashboard.ico";

export function ZashboardButton() {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const { runtimePreferences, runtime } = useRuntimeStatus();
  const [busy, setBusy] = useState(false);
  if (!runtimePreferences?.externalUIEnabled) return null;
  const ready = runtime?.mihomo.state === "running" && runtime.mihomo.controllerReady;
  const open = async () => {
    setBusy(true);
    try { await openZashboard(); }
    catch { void message.error(t("externalUI.openFailed")); }
    finally { setBusy(false); }
  };
  return <Tooltip title={t(ready ? "externalUI.open" : "externalUI.startRequired")}>
    <span className="runtime-configuration-trigger-wrap">
      <Button aria-label={t("externalUI.open")} disabled={!ready} loading={busy}
        icon={<img src={icon} width={18} height={18} alt="" />} onClick={() => void open()} size="small" type="text" />
    </span>
  </Tooltip>;
}
