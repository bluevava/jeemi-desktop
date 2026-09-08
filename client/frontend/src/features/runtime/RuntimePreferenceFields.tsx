import { Segmented } from "antd";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { FeatureHelp, type HelpTopic } from "../../components/help/FeatureHelp";
import type { RuntimeMergeMode } from "../../types/runtime";

export function RuntimePreferenceSection({
  children,
  helpTopic,
  title,
}: {
  children: ReactNode;
  helpTopic: HelpTopic;
  title: string;
}) {
  return (
    <section className="runtime-preferences-section">
      <div className="runtime-preferences-section-heading">
        <h3>{title}</h3>
        <FeatureHelp compact topic={helpTopic} />
      </div>
      {children}
    </section>
  );
}

export function RuntimePreferenceItem({
  children,
  helpTopic,
  title,
  wide = false,
}: {
  children: ReactNode;
  helpTopic?: HelpTopic;
  title: string;
  wide?: boolean;
}) {
  return (
    <section className={`runtime-preference-item${wide ? " wide" : ""}`}>
      <div className="runtime-preference-copy">
        <strong>{title}</strong>
        {helpTopic ? <FeatureHelp compact topic={helpTopic} /> : null}
      </div>
      <div className="runtime-preference-control">{children}</div>
    </section>
  );
}

export function RuntimeMergeControl({
  ariaLabel,
  disabled = false,
  onChange,
  value,
}: {
  ariaLabel: string;
  disabled?: boolean;
  onChange: (value: RuntimeMergeMode) => void;
  value: RuntimeMergeMode;
}) {
  const { t } = useTranslation();

  return (
    <Segmented
      aria-label={ariaLabel}
      disabled={disabled}
      onChange={(next) => onChange(next as RuntimeMergeMode)}
      options={(["append", "override"] as const).map((mode) => ({
        label: t(`home.runtimePreferences.mergeModes.${mode}`),
        value: mode,
      }))}
      size="small"
      value={value}
    />
  );
}
