import { Button } from "antd";
import { useTranslation } from "react-i18next";
import { useRuntimeStatus } from "../../app/runtime/RuntimeStatusContext";
import { useJeemiUpdate } from "./JeemiUpdateProvider";

export function JeemiUpdateButton({ className }: { className?: string }) {
  const { t } = useTranslation();
  const { actionBusy } = useRuntimeStatus();
  const { checking, busy, check } = useJeemiUpdate();
  return <Button type="text" size="small" className={className} loading={checking} disabled={busy || actionBusy} onClick={() => void check()}>{t("appUpdate.check")}</Button>;
}
