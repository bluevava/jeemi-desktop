import { Button, Progress, Spin } from "antd";
import { useTranslation } from "react-i18next";
import type { ZashboardState } from "../../types/externalUI";
import { cancelZashboardDownload } from "../../services/externalUIBridge";
import { useWindowActivity } from "../../app/runtime/WindowActivityContext";

export function ZashboardProgress({ state }: { state: ZashboardState | null }) {
  const { t } = useTranslation();
  const { isWindowVisible } = useWindowActivity();
  if (!state?.phase || !isWindowVisible) return null;
  return <div className="zashboard-progress" role="status" aria-live="polite">
    <span><Spin size="small" /> {t(`externalUI.phase.${state.phase}`)} {state.downloadingVersion}</span>
    {state.totalBytes > 0 && <Progress size="small" percent={Math.min(100, Math.round(100 * state.receivedBytes / state.totalBytes))} />}
    <Button size="small" type="text" onClick={() => void cancelZashboardDownload().catch(() => undefined)}>{t("common.cancel")}</Button>
  </div>;
}
