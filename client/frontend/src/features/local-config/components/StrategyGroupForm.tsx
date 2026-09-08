import {
  AutoComplete,
  Card,
  Input,
  InputNumber,
  Segmented,
  Select,
  Switch,
  Tag,
} from "antd";
import { useTranslation } from "react-i18next";
import type {
  LocalConfigResourceState,
  StrategyGroupResource,
} from "../../../types/localConfig";
import {
  DEFAULT_SELECTOR_TEST_URL,
  SELECTOR_TEST_URLS,
  STRATEGY_GROUP_EMOJIS,
} from "../strategyGroupEmoji";
import { isRemoteRuleSet } from "../resourceModel";
import {
  ResourceFieldLabel,
  StrategyRuleSetReferencesEditor,
} from "./StrategyRuleSetReferencesEditor";
import { NodeNameFilter } from "./NodeNameFilter";
const groupTypeOptions = ["select", "url-test", "fallback", "load-balance"];
const proxyTypeOptions = [
  "ss",
  "ssr",
  "vmess",
  "vless",
  "trojan",
  "hysteria",
  "hysteria2",
  "tuic",
  "wireguard",
  "anytls",
  "socks5",
  "http",
];

export function StrategyGroupForm({
  draft,
  setDraft,
  state,
}: {
  draft: StrategyGroupResource;
  setDraft: (value: StrategyGroupResource) => void;
  state: LocalConfigResourceState;
}) {
  const { t } = useTranslation();
  return (
    <div className="page-stack config-resource-form">
      <Card
        className="feature-card"
        variant="borderless"
        title={
          <span className="feature-title-with-help">
            {t("localConfig.redesign.groupInfo")}{" "}
            <Tag>{t(`localConfig.resources.groups.kinds.${draft.kind}`)}</Tag>
          </span>
        }
      >
        <div className="config-resource-form-row strategy-group-info-row">
          <label>
            <span>{t("localConfig.resources.groups.emoji")}</span>
            <Select
              allowClear
              aria-label={t("localConfig.resources.groups.emoji")}
              className="config-emoji-select"
              value={draft.emoji || undefined}
              onChange={(emoji) => setDraft({ ...draft, emoji: emoji ?? "" })}
              options={STRATEGY_GROUP_EMOJIS.map((emoji) => ({
                value: emoji,
                label: emoji,
              }))}
            />
          </label>
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
        </div>
      </Card>
      <Card
        className="feature-card"
        variant="borderless"
        title={t("localConfig.redesign.ruleSetConfig")}
      >
        <div className="config-resource-form">
          {" "}
          <div className="config-resource-form-row">
            <div className="config-resource-field">
              <ResourceFieldLabel
                helpBase="localConfig.resources.groups.help.inline"
                label={t("localConfig.resources.groups.inline")}
              />
              <div className="config-resource-switch-control">
                <Switch
                  aria-label={t("localConfig.resources.groups.inline")}
                  checked={draft.ruleOutput === "inline"}
                  disabled={draft.ruleSetReferences.some((reference) =>
                    isRemoteRuleSet(reference.ruleSetId, state),
                  )}
                  onChange={(checked) =>
                    setDraft({
                      ...draft,
                      ruleOutput: checked ? "inline" : "rule-set",
                    })
                  }
                />
                <span>
                  {draft.ruleOutput === "inline" ? "inline" : "RULE-SET"}
                </span>
              </div>
            </div>
            {draft.kind === "rule" ? (
              <div className="config-resource-field">
                <ResourceFieldLabel
                  helpBase="localConfig.resources.groups.help.policy"
                  label={t("localConfig.resources.groups.policy")}
                />
                <Segmented
                  block
                  aria-label={t("localConfig.resources.groups.policy")}
                  className="config-segmented-control"
                  onChange={(value) => {
                    const mode = value as "proxy" | "direct" | "reject";
                    setDraft({
                      ...draft,
                      policy: { mode },
                    });
                  }}
                  options={[
                    {
                      value: "proxy",
                      label: t("localConfig.resources.groups.policyProxy"),
                    },
                    {
                      value: "direct",
                      label: t("localConfig.resources.groups.policyDirect"),
                    },
                    {
                      value: "reject",
                      label: t("localConfig.resources.groups.policyReject"),
                    },
                  ]}
                  value={draft.policy.mode || "direct"}
                />
              </div>
            ) : (
              <div className="config-resource-field">
                <ResourceFieldLabel
                  helpBase="localConfig.resources.groups.help.policy"
                  label={t("localConfig.resources.groups.policy")}
                />
                <strong className="config-resource-readonly-control">
                  {t("localConfig.resources.groups.selectorPolicyFixed")}
                </strong>
              </div>
            )}
          </div>
          <StrategyRuleSetReferencesEditor
            draft={draft}
            onChange={setDraft}
            state={state}
          />
        </div>
      </Card>
      {draft.kind === "selector" ? (
        <Card
          className="feature-card"
          variant="borderless"
          title={t("localConfig.redesign.nodeFilter")}
        >
          <div className="config-resource-form">
            {" "}
            {draft.kind === "selector" ? (
              <div
                className={`config-resource-form-row strategy-selector-basics${
                  draft.type !== "select" ? " with-lazy" : ""
                }`}
              >
                <label>
                  <span>{t("localConfig.resources.groups.type")}</span>
                  <Select
                    onChange={(type) =>
                      setDraft({
                        ...draft,
                        type,
                        url:
                          type === "select"
                            ? draft.url
                            : draft.url || DEFAULT_SELECTOR_TEST_URL,
                      })
                    }
                    options={groupTypeOptions.map((value) => ({
                      value,
                      label: value,
                    }))}
                    value={draft.type || "select"}
                  />
                </label>
                <label>
                  <span>{t("localConfig.resources.groups.icon")}</span>
                  <Input
                    onChange={(event) =>
                      setDraft({ ...draft, icon: event.target.value })
                    }
                    placeholder="https://…"
                    value={draft.icon}
                  />
                </label>
                {draft.type !== "select" ? (
                  <div className="config-resource-field">
                    <ResourceFieldLabel
                      helpBase="localConfig.resources.groups.help.lazy"
                      label={t("localConfig.resources.groups.lazy")}
                    />
                    <div className="config-resource-switch-control">
                      <Switch
                        aria-label={t("localConfig.resources.groups.lazy")}
                        checked={draft.lazy}
                        onChange={(lazy) => setDraft({ ...draft, lazy })}
                      />
                    </div>
                  </div>
                ) : null}
              </div>
            ) : null}
            {draft.kind === "selector" && draft.type !== "select" ? (
              <div className="config-resource-form-row three-columns">
                <label>
                  <span>{t("localConfig.resources.groups.testUrl")}</span>
                  <AutoComplete
                    onChange={(url) => setDraft({ ...draft, url })}
                    options={SELECTOR_TEST_URLS.map((url) => ({
                      value: url,
                      label: url,
                    }))}
                    value={draft.url}
                  />
                </label>
                <label>
                  <span>{t("localConfig.resources.groups.interval")}</span>
                  <InputNumber
                    min={10}
                    onChange={(value) =>
                      setDraft({ ...draft, interval: value ?? 300 })
                    }
                    value={draft.interval}
                  />
                </label>
                {draft.type === "url-test" ? (
                  <label>
                    <span>{t("localConfig.resources.groups.tolerance")}</span>
                    <InputNumber
                      min={0}
                      onChange={(value) =>
                        setDraft({ ...draft, tolerance: value ?? 0 })
                      }
                      value={draft.tolerance}
                    />
                  </label>
                ) : draft.type === "load-balance" ? (
                  <label>
                    <span>{t("localConfig.resources.groups.strategy")}</span>
                    <Select
                      onChange={(strategy) => setDraft({ ...draft, strategy })}
                      options={[
                        "round-robin",
                        "consistent-hashing",
                        "sticky-sessions",
                      ].map((value) => ({ value, label: value }))}
                      value={draft.strategy || "consistent-hashing"}
                    />
                  </label>
                ) : (
                  <span aria-hidden="true" />
                )}
              </div>
            ) : null}
            <div className="config-resource-field">
              <ResourceFieldLabel
                label={t("localConfig.resources.groups.proxyTypes")}
              />
              <Select
                aria-label={t("localConfig.resources.groups.proxyTypes")}
                mode="tags"
                value={draft.filter.proxyTypes}
                options={proxyTypeOptions.map((value) => ({
                  value,
                  label: value,
                }))}
                onChange={(proxyTypes) =>
                  setDraft({
                    ...draft,
                    filter: { ...draft.filter, proxyTypes },
                  })
                }
                tokenSeparators={[",", " "]}
              />
            </div>
            <NodeNameFilter
              filter={draft.filter}
              onChange={(filter) => setDraft({ ...draft, filter })}
            />
          </div>
        </Card>
      ) : null}
    </div>
  );
}
