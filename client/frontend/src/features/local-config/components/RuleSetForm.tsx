import { Card, Input, InputNumber, Select, Switch } from "antd";
import { useTranslation } from "react-i18next";
import type { RuleSetResource } from "../../../types/localConfig";
import { ResourceFieldLabel } from "./StrategyRuleSetReferencesEditor";
import { RuleDocumentationButton } from "../../../components/help/RuleDocumentationButton";

export function RuleSetForm({
  draft,
  setDraft,
}: {
  draft: RuleSetResource;
  setDraft: (value: RuleSetResource) => void;
}) {
  const { t } = useTranslation();
  return (
    <div className="rule-set-page-grid config-resource-form">
      <Card className="feature-card" variant="borderless">
        <div className="config-resource-form">
          <label>
            <span>{t("localConfig.resources.name")}</span>
            <Input
              maxLength={80}
              value={draft.name}
              onChange={(event) =>
                setDraft({ ...draft, name: event.target.value })
              }
            />
          </label>
          <label>
            <span>{t("localConfig.resources.description")}</span>
            <Input
              maxLength={500}
              value={draft.description}
              onChange={(event) =>
                setDraft({ ...draft, description: event.target.value })
              }
            />
          </label>
          <label>
            <span>{t("localConfig.resources.ruleSets.sourceType")}</span>
            <Select
              value={draft.sourceType}
              onChange={(sourceType) => setDraft({ ...draft, sourceType })}
              options={["inline", "http"].map((value) => ({
                value,
                label: t(`localConfig.resources.ruleSets.sourceTypes.${value}`),
              }))}
            />
          </label>
          <label>
            <span>{t("localConfig.resources.ruleSets.behavior")}</span>
            <Select
              value={draft.behavior}
              onChange={(behavior) =>
                setDraft({
                  ...draft,
                  behavior,
                  noResolve: behavior === "ipcidr" ? draft.noResolve : false,
                  format:
                    behavior === "classical" && draft.format === "mrs"
                      ? "yaml"
                      : draft.format,
                })
              }
              options={["classical", "domain", "ipcidr"].map((value) => ({
                value,
                label: t(`localConfig.resources.ruleSets.behaviors.${value}`),
              }))}
            />
          </label>
          {draft.sourceType === "http" ? (
            <>
              <label>
                <span>{t("localConfig.resources.ruleSets.format")}</span>
                <Select
                  value={draft.format}
                  onChange={(format) => setDraft({ ...draft, format })}
                  options={(draft.behavior === "classical"
                    ? ["yaml", "text"]
                    : ["yaml", "text", "mrs"]
                  ).map((value) => ({ value, label: value }))}
                />
              </label>
              <label>
                <span>{t("localConfig.resources.ruleSets.interval")}</span>
                <InputNumber
                  min={60}
                  value={draft.interval}
                  onChange={(interval) =>
                    setDraft({ ...draft, interval: interval ?? 86400 })
                  }
                />
              </label>
            </>
          ) : null}
          {draft.behavior === "ipcidr" ? (
            <div className="config-resource-field">
              <ResourceFieldLabel
                helpBase="localConfig.resources.ruleSets.help.noResolve"
                label={t("localConfig.resources.ruleSets.noResolve")}
              />
              <Switch
                aria-label={t("localConfig.resources.ruleSets.noResolve")}
                checked={draft.noResolve}
                onChange={(noResolve) => setDraft({ ...draft, noResolve })}
              />
            </div>
          ) : null}
        </div>
      </Card>
      <Card className="feature-card rule-set-content-card" variant="borderless">
        <div className="config-resource-form">
          <div className="rule-documentation-heading">
            <ResourceFieldLabel
              helpBase="localConfig.resources.ruleSets.help.payload"
              label={t(
                draft.sourceType === "http"
                  ? "localConfig.resources.ruleSets.url"
                  : "localConfig.resources.ruleSets.payload",
              )}
            />
            <RuleDocumentationButton />
          </div>
          <Input.TextArea
            className="config-yaml-input"
            aria-label={t(
              draft.sourceType === "http"
                ? "localConfig.resources.ruleSets.url"
                : "localConfig.resources.ruleSets.payload",
            )}
            autoSize={{ minRows: 18, maxRows: 32 }}
            spellCheck={false}
            value={draft.sourceType === "http" ? draft.url : draft.payloadYaml}
            placeholder={
              draft.sourceType === "http"
                ? "https://…"
                : t(
                    `localConfig.resources.ruleSets.payloadPlaceholders.${draft.behavior}`,
                  )
            }
            onChange={(event) =>
              setDraft(
                draft.sourceType === "http"
                  ? { ...draft, url: event.target.value }
                  : { ...draft, payloadYaml: event.target.value },
              )
            }
          />
        </div>
      </Card>
    </div>
  );
}
