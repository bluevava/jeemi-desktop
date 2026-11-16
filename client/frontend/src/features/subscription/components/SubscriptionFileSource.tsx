import { FileOutlined } from "@ant-design/icons";
import { Button } from "antd";
import { useTranslation } from "react-i18next";

export function SubscriptionFileSource({ fileName, changed, busy, onChoose }: {
  fileName: string;
  changed: boolean;
  busy: boolean;
  onChoose: () => void;
}) {
  const { t } = useTranslation();
  return (
    <div className="subscription-readonly-source">
      <span>{t("subscription.form.file")}</span>
      <strong>{fileName}</strong>
      <Button disabled={busy} icon={<FileOutlined />} onClick={onChoose}>
        {t("subscription.form.chooseSourceFile")}
      </Button>
      <span>{t(changed ? "subscription.form.filePending" : "subscription.form.fileRefreshHint")}</span>
    </div>
  );
}
