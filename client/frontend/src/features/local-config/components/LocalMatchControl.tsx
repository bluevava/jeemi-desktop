import { Alert, Select } from "antd";
import { useTranslation } from "react-i18next";

import { FeatureHelp } from "../../../components/help/FeatureHelp";
import type {
  LocalConfigResourcePlan,
  LocalConfigResourceState,
} from "../../../types/localConfig";

export function LocalMatchControl({
  resources,
  value,
  onChange,
}: {
  resources: LocalConfigResourceState;
  value: LocalConfigResourcePlan;
  onChange: (value: LocalConfigResourcePlan) => void;
}) {
  const { t } = useTranslation();
  const selectors = resources.strategyGroups.filter(
    (group) =>
      group.kind === "selector" && value.strategyGroupIds.includes(group.id),
  );
  const selected =
    value.match.mode === "selector"
      ? selectors.find((group) => group.id === value.match.selectorId)
      : undefined;
  const missingSelector =
    value.match.mode === "selector" &&
    (!selected ||
      value.disabledStrategyGroupIds.includes(value.match.selectorId));
  const needsMatch =
    value.ruleStrategy === "replace" && value.match.mode === "none";
  const invalid =
    !value.rulesDisabled &&
    value.version >= 2 &&
    (missingSelector || needsMatch);
  return (
    <div className="config-resource-field">
      <div className="config-resource-field-label">
        <span>{t("localConfig.editorResources.localMatch")}</span>
        <FeatureHelp
          compact
          translationBase="localConfig.editorResources.help.localMatch"
        />
      </div>
      <Select
        aria-label={t("localConfig.editorResources.localMatch")}
        disabled={value.rulesDisabled}
        status={invalid ? "error" : undefined}
        placeholder={t("localConfig.editorResources.matchPlaceholder")}
        value={
          needsMatch
            ? undefined
            : value.match.mode === "selector"
              ? `selector:${value.match.selectorId}`
              : value.match.mode
        }
        options={[
          {
            value: "none",
            label: t("localConfig.editorResources.matchPreserve"),
            disabled: value.ruleStrategy === "replace",
          },
          { value: "direct", label: t("subscription.fallback.direct") },
          ...selectors.map((group) => ({
            value: `selector:${group.id}`,
            label: group.emoji ? `${group.emoji} ${group.name}` : group.name,
            disabled: value.disabledStrategyGroupIds.includes(group.id),
          })),
          ...(value.match.mode === "selector" && !selected
            ? [
                {
                  value: `selector:${value.match.selectorId}`,
                  label: t("localConfig.editorResources.matchUnavailable"),
                  disabled: true,
                },
              ]
            : []),
        ]}
        onChange={(choice) =>
          onChange({
            ...value,
            match: choice.startsWith("selector:")
              ? {
                  mode: "selector",
                  selectorId: choice.slice("selector:".length),
                }
              : { mode: choice as "none" | "direct", selectorId: "" },
          })
        }
      />
      {invalid && (
        <Alert
          showIcon
          type="warning"
          title={t(
            missingSelector
              ? "localConfig.editorResources.matchUnavailable"
              : "localConfig.editorResources.matchRequired",
          )}
        />
      )}
    </div>
  );
}
