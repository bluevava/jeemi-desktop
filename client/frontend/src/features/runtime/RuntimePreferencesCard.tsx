import { Alert, Spin } from "antd";
import { useTranslation } from "react-i18next";

import { FeatureCard } from "../../components/layout/FeatureCard";
import { RuntimeBasicPreferences } from "./RuntimeBasicPreferences";
import { RuntimeDnsPreferences } from "./RuntimeDnsPreferences";
import type { RuntimePreferencesEditorState } from "./useRuntimePreferencesDraft";

export function RuntimePreferencesCard({
  preferences,
}: {
  preferences: RuntimePreferencesEditorState;
}) {
  const { t } = useTranslation();
  const { clearError, draft, errorKey, loading, updateDraft } = preferences;

  return (
    <FeatureCard
      className="runtime-preferences-card"
      helpTopic="runtimePreferences"
      title={t("home.runtimePreferences.title")}
    >
      {errorKey ? (
        <Alert
          closable
          description={t(errorKey)}
          onClose={clearError}
          showIcon
          type="error"
        />
      ) : null}

      {loading ? (
        <div className="runtime-preferences-loading">
          <Spin size="small" />
          <span>{t("home.runtimePreferences.loading")}</span>
        </div>
      ) : draft ? (
        <div className="runtime-preferences-sections">
          <RuntimeBasicPreferences draft={draft} updateDraft={updateDraft} />
          <RuntimeDnsPreferences draft={draft} updateDraft={updateDraft} />
        </div>
      ) : null}
    </FeatureCard>
  );
}
