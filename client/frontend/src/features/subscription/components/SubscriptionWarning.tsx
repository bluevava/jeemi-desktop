import { Alert, type AlertProps } from "antd";
import { useTranslation } from "react-i18next";

import { useSubscriptionPageStateField } from "../SubscriptionPageStateContext";

type SubscriptionWarningProps = Pick<AlertProps, "title" | "description" | "className"> & {
  noticeKey: string;
};

// Keys describe the warning and its relevant data, never its translated text.
export function SubscriptionWarning({ noticeKey, ...props }: SubscriptionWarningProps) {
  const { t } = useTranslation();
  const [dismissed, setDismissed] = useSubscriptionPageStateField("dismissedWarnings");
  if (dismissed[noticeKey]) return null;
  return (
    <Alert
      {...props}
      key={noticeKey}
      type="warning"
      showIcon
      closable={{
        "aria-label": t("subscription.dismissWarning"),
        onClose: () => setDismissed((current) => ({ ...current, [noticeKey]: true })),
      }}
    />
  );
}
