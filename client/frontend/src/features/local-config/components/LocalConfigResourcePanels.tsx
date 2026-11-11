import { useEffect, useMemo, useState } from "react";
import {
  DeleteOutlined,
  EditOutlined,
  FileTextOutlined,
  GlobalOutlined,
  MoreOutlined,
  PlusOutlined,
  ShareAltOutlined,
} from "@ant-design/icons";
import {
  App,
  Button,
  Card,
  Checkbox,
  Dropdown,
  Empty,
  Tag,
  Tooltip,
} from "antd";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { FlagText } from "../../../components/common/FlagText";
import { FeatureHelp } from "../../../components/help/FeatureHelp";
import {
  deleteLocalConfigResources,
  getLocalConfigResources,
  saveRuleSet,
  saveStrategyGroup,
} from "../../../services/appBridge";
import type {
  LocalConfigResourceState,
  RuleSetResource,
  StrategyGroupResource,
} from "../../../types/localConfig";
import {
  formatDate,
  policyLabel,
  strategyGroupDisplayName,
} from "../resourceModel";
import {
  defaultResourceFilters,
  filterResources,
  resourceDeletionPreview,
  resourceOwners,
  type ResourceKind,
  type ResourceFilters,
} from "../resourceLibrary";
import {
  resourceDisplayName,
  resourcePresentation,
} from "../resourcePresentation";
import { ResourceActivationControl } from "./ResourceActivationControl";
import { ResourceLibraryToolbar } from "./ResourceLibraryToolbar";

interface ResourcePanelProps {
  state: LocalConfigResourceState;
  onChange: (state: LocalConfigResourceState) => void;
  onError: () => void;
}

export function StrategyGroupLibraryPanel(props: ResourcePanelProps) {
  return <ResourceLibraryPanel {...props} kind="groups" />;
}
export function RuleSetLibraryPanel(props: ResourcePanelProps) {
  return <ResourceLibraryPanel {...props} kind="rules" />;
}

function ResourceLibraryPanel({
  state,
  onChange,
  onError,
  kind,
}: ResourcePanelProps & { kind: ResourceKind }) {
  const { t, i18n } = useTranslation();
  const { modal, message } = App.useApp();
  const navigate = useNavigate();
  const [busy, setBusy] = useState(false);
  const [filters, setFilters] = useState(defaultResourceFilters);
  const [selecting, setSelecting] = useState(false);
  const [selected, setSelected] = useState<string[]>([]);
  const items = kind === "groups" ? state.strategyGroups : state.ruleSets;
  const visible = useMemo(
    () =>
      filterResources<StrategyGroupResource | RuleSetResource>(
        items,
        state,
        kind,
        filters,
      ),
    [items, state, kind, filters],
  );
  useEffect(() => setSelected([]), [state.revision, state.configReferences]);
  const changeFilter = (next: ResourceFilters) => {
    setFilters(next);
    setSelected([]);
  };
  const changeState = (next: LocalConfigResourceState) =>
    onChange({
      ...next,
      platform: state.platform,
      configReferences: next.configReferences ?? state.configReferences,
    });
  const reload = async () => {
    setBusy(true);
    try {
      onChange(await getLocalConfigResources());
      setSelected([]);
    } catch {
      onError();
    } finally {
      setBusy(false);
    }
  };
  const toggle = async (
    item: StrategyGroupResource | RuleSetResource,
    enabled: boolean,
  ) => {
    setBusy(true);
    try {
      changeState(
        kind === "groups"
          ? await saveStrategyGroup({
              ...(item as StrategyGroupResource),
              disabled: !enabled,
            })
          : await saveRuleSet({
              ...(item as RuleSetResource),
              disabled: !enabled,
            }),
      );
    } catch {
      onError();
    } finally {
      setBusy(false);
    }
  };
  const confirmDelete = (ids: string[]) => {
    const preview = resourceDeletionPreview(state, kind, ids);
    const eligibleIds = preview.deletable.map((item) => item.id);
    modal.confirm({
      title: t("resourceLibrary.deleteTitle"),
      content: (
        <div className="resource-delete-preview">
          {preview.deletable.length ? (
            <>
              <p>
                {t("resourceLibrary.deletable", {
                  count: preview.deletable.length,
                })}
              </p>
              <ul>
                {preview.deletable.map((item) => (
                  <li key={item.id}>
                    <FlagText>{item.name}</FlagText>
                  </li>
                ))}
              </ul>
            </>
          ) : null}
          {preview.blocked.length ? (
            <>
              <p>
                {t("resourceLibrary.blocked", {
                  count: preview.blocked.length,
                })}
              </p>
              <ul>
                {preview.blocked.map(({ item, parents }) => (
                  <li key={item.id}>
                    <FlagText>{item.name}</FlagText>
                    <span className="resource-delete-parent">
                      {" "}
                      →{" "}
                      <FlagText>
                        {parents.map((parent) => parent.name).join("、")}
                      </FlagText>
                    </span>
                  </li>
                ))}
              </ul>
            </>
          ) : null}
          <p>{t("resourceLibrary.deleteNote")}</p>
        </div>
      ),
      okText: t("resourceLibrary.deleteCount", { count: eligibleIds.length }),
      cancelText: t("common.cancel"),
      okButtonProps: { danger: true, disabled: !eligibleIds.length },
      onOk: async () => {
        setBusy(true);
        try {
          onChange(
            await deleteLocalConfigResources({
              kind,
              ids: eligibleIds,
              expectedRevision: state.revision,
            }),
          );
          setSelected([]);
        } catch {
          void message.error(t("resourceLibrary.deleteFailed"));
          throw new Error("resource deletion rejected");
        } finally {
          setBusy(false);
        }
      },
    });
  };
  const createButton = (
    <Button
      disabled={busy}
      icon={<PlusOutlined />}
      onClick={
        kind === "rules" ? () => navigate("/config/rule-sets/new") : undefined
      }
    >
      {t(
        kind === "groups"
          ? "localConfig.resources.groups.create"
          : "localConfig.resources.ruleSets.create",
      )}
    </Button>
  );
  const create = (
    <div className="resource-library-create">
      {kind === "groups" ? (
        <Dropdown
          trigger={["click"]}
          menu={{
            items: ["rule", "selector"].map((value) => ({
              key: value,
              label: t(`localConfig.resources.groups.kinds.${value}`),
            })),
            onClick: ({ key }) => navigate(`/config/groups/new/${key}`),
          }}
        >
          {createButton}
        </Dropdown>
      ) : (
        createButton
      )}
      <FeatureHelp
        compact
        translationBase={
          kind === "groups"
            ? "localConfig.fieldHelp.local-strategy-groups"
            : "localConfig.fieldHelp.local-rule-sets"
        }
      />
      <FeatureHelp compact translationBase="offlineConfig.help.activation" />
    </div>
  );
  return (
    <div className="config-resource-panel">
      <ResourceLibraryToolbar
        kind={kind}
        state={state}
        filters={filters}
        onFilter={changeFilter}
        busy={busy}
        shown={visible.length}
        total={items.length}
        selected={selected.length}
        selecting={selecting}
        onSelecting={() => {
          setSelecting(!selecting);
          setSelected([]);
        }}
        onSelectAll={(checked) =>
          setSelected(checked ? visible.map((item) => item.id) : [])
        }
        onClear={() => setSelected([])}
        onDelete={() => confirmDelete(selected)}
        onReload={() => {
          void reload();
        }}
        create={create}
      />
      <div className="config-resource-card-grid">
        {visible.map((item) => {
          const presentation = resourcePresentation(item);
          const group =
            kind === "groups" ? (item as StrategyGroupResource) : null;
          const rule = kind === "rules" ? (item as RuleSetResource) : null;
          const fullName = group
            ? strategyGroupDisplayName(group)
            : resourceDisplayName(item);
          const wrongPlatform =
            rule?.platform &&
            state.platform &&
            rule.platform !== state.platform;
          const parentDisabled =
            rule &&
            resourceOwners(state, rule.id).some((parent) => parent.disabled);
          const checked = selected.includes(item.id);
          return (
            <Card
              key={item.id}
              variant="borderless"
              className={`config-resource-card resource-library-card${checked ? " is-selected" : ""}`}
            >
              <div className="config-resource-card-heading">
                {selecting ? (
                  <Checkbox
                    disabled={busy}
                    aria-label={t("resourceLibrary.selectItem", {
                      name: fullName,
                    })}
                    checked={checked}
                    onChange={(event) =>
                      setSelected(
                        event.target.checked
                          ? [...selected, item.id]
                          : selected.filter((id) => id !== item.id),
                      )
                    }
                  />
                ) : null}
                <span className="config-resource-card-icon" aria-hidden="true">
                  {presentation.emoji ? (
                    <FlagText>{presentation.emoji}</FlagText>
                  ) : group ? (
                    group.kind === "selector" ? (
                      <ShareAltOutlined />
                    ) : (
                      <FileTextOutlined />
                    )
                  ) : (
                    <GlobalOutlined />
                  )}
                </span>
                <Tooltip title={<FlagText>{fullName}</FlagText>}>
                  <strong className="resource-library-card-title">
                    <FlagText>{presentation.displayName}</FlagText>
                  </strong>
                </Tooltip>
                <Dropdown
                  trigger={["click"]}
                  menu={{
                    items: [
                      {
                        key: "edit",
                        icon: <EditOutlined />,
                        label: t("common.edit"),
                      },
                      {
                        key: "delete",
                        danger: true,
                        icon: <DeleteOutlined />,
                        label: t("common.delete"),
                      },
                    ],
                    onClick: ({ key }) => {
                      if (key === "edit")
                        navigate(
                          `/config/${kind === "groups" ? "groups" : "rule-sets"}/${item.id}/edit`,
                        );
                      if (key === "delete") confirmDelete([item.id]);
                    },
                  }}
                  disabled={busy}
                >
                  <Button
                    aria-label={t("localConfig.resources.actions", {
                      name: fullName,
                    })}
                    icon={<MoreOutlined />}
                    type="text"
                    disabled={busy}
                  />
                </Dropdown>
              </div>
              <div className="config-resource-tags">
                {group ? (
                  <>
                    <Tag>
                      {t(`localConfig.resources.groups.kinds.${group.kind}`)}
                    </Tag>
                    <Tag>
                      {group.kind === "selector"
                        ? group.type
                        : policyLabel(group, t)}
                    </Tag>
                    {group.autoSelect ? <Tag>Auto</Tag> : null}
                    <Tag>
                      {t("localConfig.resources.groups.ruleSetCount", {
                        count: group.ruleSetReferences.length,
                      })}
                    </Tag>
                    <Tag>
                      {group.ruleOutput === "inline" ? "inline" : "RULE-SET"}
                    </Tag>
                  </>
                ) : null}
                {rule ? (
                  <>
                    <Tag>{rule.sourceType}</Tag>
                    <Tag>{rule.behavior}</Tag>
                    {rule.platform ? (
                      <Tag>{t(`offlineConfig.platforms.${rule.platform}`)}</Tag>
                    ) : null}
                    <Tag>
                      {rule.sourceType === "inline"
                        ? rule.payload.length
                        : rule.format}
                    </Tag>
                    {rule.noResolve ? <Tag>no-resolve</Tag> : null}
                  </>
                ) : null}
              </div>
              <div className="config-resource-card-footer">
                {wrongPlatform ? (
                  <span>{t("resourceLibrary.notApplicable")}</span>
                ) : parentDisabled ? (
                  <span>{t("resourceLibrary.parentDisabled")}</span>
                ) : (
                  <time dateTime={item.updatedAt}>
                    {formatDate(item.updatedAt, i18n.language)}
                  </time>
                )}
                <ResourceActivationControl
                  name={fullName}
                  disabled={item.disabled}
                  busy={busy}
                  onChange={(enabled) => {
                    void toggle(item, enabled);
                  }}
                />
              </div>
            </Card>
          );
        })}
      </div>
      {!visible.length ? (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description={t("resourceLibrary.empty")}
        />
      ) : null}
    </div>
  );
}
