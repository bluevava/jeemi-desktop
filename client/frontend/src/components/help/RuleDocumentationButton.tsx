import { App, Button } from "antd";
import { useTranslation } from "react-i18next";
import { openRuleDocumentation } from "../../services/appBridge";

export function RuleDocumentationButton() {
  const { t } = useTranslation();
  const { message } = App.useApp();
  return (
    <Button
      type="link"
      size="small"
      onClick={() => {
        void openRuleDocumentation().catch(() => {
          void message.error(t("ruleSetEntry.documentationError"));
        });
      }}
    >
      {t("ruleSetEntry.documentation")}
    </Button>
  );
}
