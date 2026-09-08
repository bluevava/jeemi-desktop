import { emptyStrategyGroup } from "../resourceModel";
import { LocalMatchControl } from "./LocalMatchControl";
import { useState } from "react";
import {
  DeleteOutlined,
  FileTextOutlined,
  HolderOutlined,
  LockOutlined,
  PlusOutlined,
  ShareAltOutlined,
} from "@ant-design/icons";
import { Alert, Button, Empty, Select, Switch, Tag, Tooltip } from "antd";
import { useTranslation } from "react-i18next";

import { FeatureHelp } from "../../../components/help/FeatureHelp";
import type {
  LocalConfigResourcePlan,
  LocalConfigResourceState,
  StrategyGroupResource,
} from "../../../types/localConfig";

interface LocalConfigResourcePlanEditorProps {
  resources: LocalConfigResourceState;
  value: LocalConfigResourcePlan;
  onChange: (value: LocalConfigResourcePlan) => void;
}

export function LocalConfigRulesEditor({
  resources,
  value,
  onChange,
}: LocalConfigResourcePlanEditorProps) {
  const { t } = useTranslation();
  const [draggedIndex, setDraggedIndex] = useState<number | null>(null);
  const rulesEnabled = !value.rulesDisabled;
  const disabledGroups = new Set(value.disabledStrategyGroupIds);
  const selectedGroups = value.strategyGroupIds.map(
    (id) =>
      resources.strategyGroups.find((group) => group.id === id) ?? {
        ...emptyStrategyGroup("rule"),
        id,
        name: t("localConfig.redesign.missingGroup", { id }),
      },
  );
  const selected = new Set(value.strategyGroupIds);
  const availableGroups = resources.strategyGroups.filter(
    (group) => !selected.has(group.id),
  );

  const moveGroup = (from: number, to: number) => {
    if (from === to || from < 0 || to < 0) {
      return;
    }
    const next = [...value.strategyGroupIds];
    const [moved] = next.splice(from, 1);
    next.splice(to, 0, moved);
    onChange({ ...value, strategyGroupIds: next });
  };

  return (
    <div className="config-field-editor local-config-rules-editor">
      <div className="local-config-rules-activation">
        <div className="config-resource-field-label">
          <span>{t("localConfig.editorResources.rulesEnabled")}</span>
          <FeatureHelp
            compact
            translationBase="localConfig.editorResources.help.activation"
          />
        </div>
        <Switch
          aria-label={t("localConfig.editorResources.rulesEnabled")}
          checked={rulesEnabled}
          onChange={(enabled) =>
            onChange({ ...value, rulesDisabled: !enabled })
          }
        />
      </div>
      <div className="local-config-rule-plan-modes">
        <label>
          <span>{t("localConfig.editorResources.ruleStrategy")}</span>
          <Select
            aria-label={t("localConfig.editorResources.ruleStrategy")}
            disabled={!rulesEnabled}
            onSelect={(ruleStrategy) => onChange({ ...value, ruleStrategy })}
            options={[
              {
                value: "prepend",
                label: t("localConfig.editorResources.rulesPrepend"),
              },
              {
                value: "append_before_terminal",
                label: t("localConfig.editorResources.rulesBeforeTerminal"),
              },
              {
                value: "replace",
                label: t("localConfig.editorResources.rulesRebuild"),
              },
            ]}
            value={value.ruleStrategy}
          />
        </label>
        <LocalMatchControl
          resources={resources}
          value={value}
          onChange={onChange}
        />
      </div>
      {rulesEnabled &&
        value.ruleStrategy === "replace" &&
        !selectedGroups.some(
          (group) => group.kind === "selector" && !disabledGroups.has(group.id),
        ) && (
          <Alert
            showIcon
            type="warning"
            title={t("localConfig.editorResources.rebuildSelectorRequired")}
          />
        )}
      <DefaultProxySelector
        resources={resources}
        value={value}
        onChange={onChange}
      />

      <section className="local-config-ordered-groups">
        <div className="local-config-ordered-groups-heading">
          <div>
            <strong>{t("localConfig.editorResources.groupsTitle")}</strong>
          </div>
          <Select<string>
            disabled={!rulesEnabled || availableGroups.length === 0}
            onSelect={(id) =>
              onChange({
                ...value,
                strategyGroupIds: [...value.strategyGroupIds, id],
              })
            }
            options={availableGroups.map((group) => ({
              value: group.id,
              label: `${strategyGroupDisplayName(group)} · ${t(
                `localConfig.resources.groups.kinds.${group.kind}`,
              )}`,
            }))}
            placeholder={
              resources.strategyGroups.length > 0
                ? t("localConfig.editorResources.addGroup")
                : t("localConfig.editorResources.groupsEmpty")
            }
            suffixIcon={<PlusOutlined />}
            value={undefined}
          />
        </div>
        {selectedGroups.length > 0 ? (
          <div className="local-config-ordered-group-list">
            {selectedGroups.map((group, index) => (
              <div
                className={`local-config-ordered-group-row${!rulesEnabled || disabledGroups.has(group.id) ? " is-disabled" : ""}`}
                draggable={rulesEnabled}
                key={group.id}
                onDragEnd={() => setDraggedIndex(null)}
                onDragOver={(event) => event.preventDefault()}
                onDragStart={() => setDraggedIndex(index)}
                onDrop={() => {
                  if (rulesEnabled && draggedIndex !== null) {
                    moveGroup(draggedIndex, index);
                  }
                  setDraggedIndex(null);
                }}
              >
                <HolderOutlined className="strategy-drag-handle" />
                <span className="local-config-ordered-group-icon">
                  {group.kind === "selector" ? (
                    <ShareAltOutlined />
                  ) : (
                    <FileTextOutlined />
                  )}
                </span>
                <div>
                  <strong>{strategyGroupDisplayName(group)}</strong>
                  <small>
                    {t(`localConfig.resources.groups.kinds.${group.kind}`)} ·{" "}
                    {group.ruleOutput === "inline" ? "inline" : "RULE-SET"} ·{" "}
                    {t("localConfig.resources.groups.ruleSetCount", {
                      count: group.ruleSetReferences.length,
                    })}
                  </small>
                </div>
                <Tag>{index + 1}</Tag>
                <Tooltip
                  title={t("localConfig.editorResources.groupEnabled", {
                    name: strategyGroupDisplayName(group),
                  })}
                >
                  <Switch
                    aria-label={t("localConfig.editorResources.groupEnabled", {
                      name: strategyGroupDisplayName(group),
                    })}
                    checked={!disabledGroups.has(group.id)}
                    disabled={!rulesEnabled}
                    size="small"
                    onChange={(enabled) =>
                      onChange({
                        ...value,
                        disabledStrategyGroupIds: enabled
                          ? value.disabledStrategyGroupIds.filter(
                              (id) => id !== group.id,
                            )
                          : [...value.disabledStrategyGroupIds, group.id],
                      })
                    }
                  />
                </Tooltip>
                <Button
                  aria-label={t("localConfig.editorResources.removeGroup")}
                  danger
                  disabled={!rulesEnabled}
                  icon={<DeleteOutlined />}
                  onClick={() =>
                    onChange({
                      ...value,
                      strategyGroupIds: value.strategyGroupIds.filter(
                        (id) => id !== group.id,
                      ),
                      disabledStrategyGroupIds:
                        value.disabledStrategyGroupIds.filter(
                          (id) => id !== group.id,
                        ),
                    })
                  }
                  type="text"
                />
              </div>
            ))}
          </div>
        ) : (
          <Empty
            description={t("localConfig.editorResources.noGroups")}
            image={Empty.PRESENTED_IMAGE_SIMPLE}
          />
        )}
      </section>
    </div>
  );
}

export function LocalConfigMatchEditor() {
  const { t } = useTranslation();
  return (
    <div className="config-field-editor">
      <div className="config-field-title-line">
        <code>rules.MATCH</code>
        <Tag icon={<LockOutlined />}>{t("localConfig.editor.locked")}</Tag>
        <FeatureHelp
          compact
          translationBase="localConfig.editorResources.help.localMatch"
        />
      </div>
      <Alert
        showIcon
        type="info"
        title={t("localConfig.editorResources.matchLocked")}
      />
    </div>
  );
}

function DefaultProxySelector({
  resources,
  value,
  onChange,
}: LocalConfigResourcePlanEditorProps) {
  const { t } = useTranslation();
  const selectors = resources.strategyGroups.filter(
    (group) => group.kind === "selector",
  );
  const required = value.strategyGroupIds.some((id) => {
    const group = resources.strategyGroups.find((item) => item.id === id);
    return (
      !value.disabledStrategyGroupIds.includes(id) &&
      group?.kind === "rule" &&
      group.policy.mode === "proxy"
    );
  });
  const unavailable =
    !value.rulesDisabled &&
    required &&
    (!selectors.some((group) => group.id === value.defaultProxySelectorId) ||
      value.disabledStrategyGroupIds.includes(value.defaultProxySelectorId));
  if (!required) return null;
  return (
    <div className="config-resource-field">
      <div className="config-resource-field-label">
        <span>{t("localConfig.editorResources.proxySelector")}</span>
        <FeatureHelp
          compact
          translationBase="localConfig.editorResources.help.proxySelector"
        />
      </div>
      <Select
        allowClear
        aria-label={t("localConfig.editorResources.proxySelector")}
        disabled={value.rulesDisabled}
        onChange={(id) =>
          onChange({ ...value, defaultProxySelectorId: id ?? "" })
        }
        options={selectors.map((selector) => ({
          value: selector.id,
          label: strategyGroupDisplayName(selector),
          disabled: value.disabledStrategyGroupIds.includes(selector.id),
        }))}
        placeholder={t("localConfig.editorResources.proxySelectorPlaceholder")}
        status={unavailable ? "error" : undefined}
        value={value.defaultProxySelectorId || undefined}
      />
      {unavailable && (
        <Alert
          showIcon
          type="warning"
          title={t("localConfig.editorResources.proxySelectorRequired")}
        />
      )}
    </div>
  );
}

function strategyGroupDisplayName(group: StrategyGroupResource): string {
  return group.emoji
    ? `${group.emoji} ${group.name.trim()}`
    : group.name.trim();
}
