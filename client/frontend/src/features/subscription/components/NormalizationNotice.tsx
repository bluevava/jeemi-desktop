import { Alert } from "antd";
import { useTranslation } from "react-i18next";
import { FeatureHelp } from "../../../components/help/FeatureHelp";
import type { SubscriptionCompositionSummary } from "../../../types/subscription";
import { normalizationMessage } from "../normalization";

export function NormalizationNotice({ summary }: { summary: SubscriptionCompositionSummary }) {
  const { t } = useTranslation();
  const report = summary.normalization;
  if (summary.normalizationError) {
    return <Alert showIcon type="error" description={normalizationMessage(t, summary.normalizationError)} />;
  }
  if (!report || report.format === "mihomo") return null;
  const diagnostics = report.diagnostics.slice(0, 100);
  return (
    <div className="subscription-normalization">
      <div className="subscription-normalization-heading">
        <span>{t("subscription.normalization.summary", { format: report.format === "surge" ? "Surge" : "URI", count: report.proxyCount, skipped: report.skippedNodes })}</span>
        <FeatureHelp compact translationBase="subscription.normalization.help" />
      </div>
      {report.requiredCore ? <p>{t("subscription.normalization.requiredCore", { version: report.requiredCore })}</p> : null}
      {diagnostics.length > 0 ? (
        <details>
          <summary>{t("subscription.normalization.details")}</summary>
          <ul>{diagnostics.map((item, index) => <li key={index}>{normalizationMessage(t, item)}</li>)}</ul>
          {report.diagnostics.length > diagnostics.length ? <p>{t("subscription.normalization.more", { count: report.diagnostics.length - diagnostics.length })}</p> : null}
        </details>
      ) : null}
    </div>
  );
}
