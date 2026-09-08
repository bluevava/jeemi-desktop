import {
  Alert,
  App,
  Button,
  ConfigProvider,
  Input,
  Modal,
  Select,
  Spin,
  Switch,
} from "antd";
import { useTranslation } from "react-i18next";
import { FeatureHelp } from "../../components/help/FeatureHelp";
import { RuleDocumentationButton } from "../../components/help/RuleDocumentationButton";
import { ruleMatchTypes } from "../../types/ruleSetEntry";
import { canAppendRule, type ConnectionRuleSeed } from "./ruleEntryModel";
import { useRuleSetEntryDraft } from "./useRuleSetEntryDraft";

export function ConnectionRuleSetDialog({
  seed,
  onClose,
}: {
  seed: ConnectionRuleSeed;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const draft = useRuleSetEntryDraft(seed, () => {
    void message.success(t("ruleSetEntry.saved"));
    onClose();
  });
  const selected = draft.resources?.ruleSets.find(
    (item) => item.id === draft.ruleSetId,
  );
  return (
    <Modal
      open
      width={660}
      className="connection-rule-set-dialog"
      title={
        <div className="rule-documentation-heading">
          <span>{t("ruleSetEntry.title")}</span>
          <RuleDocumentationButton />
        </div>
      }
      maskClosable={false}
      keyboard={!draft.busy}
      closable={!draft.busy}
      onCancel={() => {
        if (!draft.busy) onClose();
      }}
      onOk={() => void draft.save()}
      okText={t("common.save")}
      cancelText={t("common.cancel")}
      confirmLoading={draft.busy}
      okButtonProps={{ disabled: !draft.canSave }}
      cancelButtonProps={{ disabled: draft.busy }}
    >
      <div className="rule-entry-form">
        {draft.loading && <Spin />}
        {draft.loadError && (
          <Alert
            type="error"
            showIcon
            description={t("ruleSetEntry.loadError")}
            action={
              <Button size="small" onClick={draft.refresh}>
                {t("ruleSetEntry.refresh")}
              </Button>
            }
          />
        )}
        <ConfigProvider
          componentDisabled={draft.busy || draft.loading || draft.loadError}
        >
          <div className="rule-entry-field">
            <div className="rule-entry-label">
              <label htmlFor="rule-entry-destination">
                {t("ruleSetEntry.destination")}
              </label>
              <FeatureHelp
                compact
                translationBase="ruleSetEntry.help.destination"
              />
              <Button
                className="rule-entry-refresh"
                size="small"
                type="text"
                onClick={draft.refresh}
              >
                {t("ruleSetEntry.refresh")}
              </Button>
            </div>
            <Select
              id="rule-entry-destination"
              showSearch
              optionFilterProp="label"
              value={draft.ruleSetId}
              onChange={draft.setRuleSetId}
              options={[
                { value: "", label: t("ruleSetEntry.newRuleSet") },
                ...(draft.resources?.ruleSets ?? []).map((item) => ({
                  value: item.id,
                  label: `${item.name} (${item.behavior})${item.sourceType === "http" ? " · " + t("ruleSetEntry.remote") : ""}`,
                  disabled: item.sourceType !== "inline",
                })),
              ]}
            />
          </div>
          {!draft.ruleSetId && (
            <div className="rule-entry-field">
              <label htmlFor="rule-entry-name">
                {t("localConfig.resources.name")}
              </label>
              <Input
                id="rule-entry-name"
                maxLength={80}
                value={draft.name}
                onChange={(event) => draft.setName(event.target.value)}
              />
            </div>
          )}
          <div className="rule-entry-field">
            <label htmlFor="rule-entry-type">
              {t("ruleSetEntry.matchType")}
            </label>
            <Select
              id="rule-entry-type"
              value={draft.matchType}
              onChange={draft.setMatchType}
              options={ruleMatchTypes.map((value) => ({
                value,
                label: t(`ruleSetEntry.types.${value}`),
                disabled: Boolean(selected && !canAppendRule(selected, value)),
              }))}
            />
          </div>
          <div className="rule-entry-field">
            <div className="rule-entry-label">
              <label htmlFor="rule-entry-value">
                {t(`ruleSetEntry.values.${draft.matchType}`)}
              </label>
              <FeatureHelp
                compact
                translationBase={`ruleSetEntry.help.${draft.matchType}`}
              />
            </div>
            <Input.TextArea
              id="rule-entry-value"
              autoSize={{ minRows: 2, maxRows: 6 }}
              spellCheck={false}
              value={draft.value}
              status={draft.previewError ? "error" : undefined}
              onChange={(event) => draft.setValue(event.target.value)}
            />
          </div>
          {draft.matchType === "processPath" && (
            <div className="rule-entry-label">
              <Switch
                id="rule-entry-ignore-case"
                checked={draft.caseInsensitive}
                onChange={draft.setCaseInsensitive}
              />
              <label htmlFor="rule-entry-ignore-case">
                {t("ruleSetEntry.ignoreCase")}
              </label>
            </div>
          )}
        </ConfigProvider>
        <div className="rule-entry-field" aria-live="polite">
          <div className="rule-entry-label">
            <span>{t("ruleSetEntry.preview")}</span>
            {draft.checking && <Spin size="small" />}
          </div>
          {draft.preview?.valid && (
            <pre className="rule-entry-preview">{draft.preview.yamlLine}</pre>
          )}
          {draft.previewError && (
            <Alert type="error" showIcon description={draft.previewError} />
          )}
        </div>
        {draft.saveError && (
          <Alert
            type="error"
            showIcon
            title={t("ruleSetEntry.notSaved")}
            description={draft.saveError}
          />
        )}
      </div>
    </Modal>
  );
}
