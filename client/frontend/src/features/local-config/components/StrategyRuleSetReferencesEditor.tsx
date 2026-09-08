import { useState } from "react";
import { Button, Select } from "antd";
import {
  DeleteOutlined,
  HolderOutlined,
  PlusOutlined,
} from "@ant-design/icons";
import { useTranslation } from "react-i18next";
import { FeatureHelp } from "../../../components/help/FeatureHelp";
import type {
  LocalConfigResourceState,
  RuleSetResource,
  RuleSetReference,
  StrategyGroupResource,
} from "../../../types/localConfig";
import { strategyGroupDisplayName } from "../resourceModel";

export function StrategyRuleSetReferencesEditor({
  draft,
  onChange,
  state,
}: {
  draft: StrategyGroupResource;
  onChange: (value: StrategyGroupResource) => void;
  state: LocalConfigResourceState;
}) {
  const { t } = useTranslation();
  const [draggedIndex, setDraggedIndex] = useState<number | null>(null);
  const used = new Set(draft.ruleSetReferences.map((item) => item.ruleSetId));
  const ownerByRuleSet = new Map<string, StrategyGroupResource>();
  for (const group of state.strategyGroups) {
    if (group.id === draft.id) {
      continue;
    }
    for (const reference of group.ruleSetReferences) {
      ownerByRuleSet.set(reference.ruleSetId, group);
    }
  }
  const canReference = (item: RuleSetResource) =>
    !ownerByRuleSet.has(item.id) &&
    (draft.ruleOutput !== "inline" || item.sourceType === "inline");
  const addReference = () => {
    const available = state.ruleSets.find(
      (item) => !used.has(item.id) && canReference(item),
    );
    if (!available) {
      return;
    }
    onChange({
      ...draft,
      ruleSetReferences: [
        ...draft.ruleSetReferences,
        { ruleSetId: available.id },
      ],
    });
  };
  const updateReference = (
    index: number,
    update: Partial<RuleSetReference>,
  ) => {
    onChange({
      ...draft,
      ruleSetReferences: draft.ruleSetReferences.map((item, candidate) =>
        candidate === index ? { ...item, ...update } : item,
      ),
    });
  };
  const moveReference = (from: number, to: number) => {
    if (from === to || from < 0 || to < 0) {
      return;
    }
    const next = [...draft.ruleSetReferences];
    const [moved] = next.splice(from, 1);
    next.splice(to, 0, moved);
    onChange({ ...draft, ruleSetReferences: next });
  };

  return (
    <section className="strategy-rule-set-editor">
      <div className="strategy-rule-set-heading">
        <div>
          <ResourceFieldLabel
            helpBase="localConfig.resources.groups.help.ruleSets"
            label={t("localConfig.resources.groups.ruleSets")}
            strong
          />
        </div>
        <Button
          disabled={
            !state.ruleSets.some(
              (item) => !used.has(item.id) && canReference(item),
            )
          }
          icon={<PlusOutlined />}
          onClick={addReference}
          size="small"
        >
          {t("localConfig.resources.groups.addRuleSet")}
        </Button>
      </div>
      {draft.ruleSetReferences.length > 0 ? (
        <div className="strategy-rule-set-list">
          {draft.ruleSetReferences.map((reference, index) => (
            <div
              className="strategy-rule-set-row"
              draggable
              key={`${reference.ruleSetId}-${index}`}
              onDragEnd={() => setDraggedIndex(null)}
              onDragOver={(event) => event.preventDefault()}
              onDragStart={() => setDraggedIndex(index)}
              onDrop={() => {
                if (draggedIndex !== null) {
                  moveReference(draggedIndex, index);
                }
                setDraggedIndex(null);
              }}
            >
              <HolderOutlined className="strategy-drag-handle" />
              <Select
                onChange={(ruleSetId) => updateReference(index, { ruleSetId })}
                options={state.ruleSets.map((item) => ({
                  value: item.id,
                  label: ownerByRuleSet.has(item.id)
                    ? t("localConfig.resources.groups.ruleSetOwned", {
                        name: item.name,
                        owner: strategyGroupDisplayName(
                          ownerByRuleSet.get(item.id)!,
                        ),
                      })
                    : `${item.name} · ${item.behavior}`,
                  disabled:
                    (item.id !== reference.ruleSetId && used.has(item.id)) ||
                    !canReference(item),
                }))}
                value={reference.ruleSetId}
              />
              <Button
                aria-label={t("localConfig.resources.groups.removeRuleSet")}
                danger
                icon={<DeleteOutlined />}
                onClick={() =>
                  onChange({
                    ...draft,
                    ruleSetReferences: draft.ruleSetReferences.filter(
                      (_, candidate) => candidate !== index,
                    ),
                  })
                }
                type="text"
              />
            </div>
          ))}
        </div>
      ) : (
        <small className="strategy-rule-set-empty">
          {t("localConfig.resources.groups.noRuleSets")}
        </small>
      )}
    </section>
  );
}

export function ResourceFieldLabel({
  helpBase,
  label,
  strong = false,
}: {
  helpBase?: string;
  label: string;
  strong?: boolean;
}) {
  return (
    <div className="config-resource-field-label">
      {strong ? <strong>{label}</strong> : <span>{label}</span>}
      {helpBase ? <FeatureHelp compact translationBase={helpBase} /> : null}
    </div>
  );
}
