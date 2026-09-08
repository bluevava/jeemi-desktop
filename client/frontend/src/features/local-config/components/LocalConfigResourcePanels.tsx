import { useState } from "react";
import {
  DeleteOutlined,
  EditOutlined,
  FileTextOutlined,
  GlobalOutlined,
  MoreOutlined,
  ShareAltOutlined,
} from "@ant-design/icons";
import { App, Button, Card, Dropdown, Tag } from "antd";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { FeatureHelp } from "../../../components/help/FeatureHelp";
import { LibraryCreateCard } from "../../../components/layout/LibraryCreateCard";
import {
  deleteRuleSet,
  deleteStrategyGroup,
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
interface ResourcePanelProps {
  state: LocalConfigResourceState;
  onChange: (state: LocalConfigResourceState) => void;
  onError: () => void;
}

export function StrategyGroupLibraryPanel({
  state,
  onChange,
  onError,
}: ResourcePanelProps) {
  const { t, i18n } = useTranslation();
  const { modal } = App.useApp();
  const navigate = useNavigate();
  const [busy, setBusy] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const confirmDelete = (item: StrategyGroupResource) => {
    modal.confirm({
      title: t("localConfig.resources.groups.deleteTitle"),
      content: t("localConfig.resources.groups.deleteDescription", {
        name: strategyGroupDisplayName(item),
      }),
      okText: t("common.delete"),
      cancelText: t("common.cancel"),
      okButtonProps: { danger: true },
      onOk: async () => {
        setBusy(true);
        try {
          onChange(await deleteStrategyGroup(item.id));
        } catch {
          onError();
          throw new Error("delete strategy group failed");
        } finally {
          setBusy(false);
        }
      },
    });
  };

  return (
    <div className="config-resource-panel">
      <div className="config-resource-card-grid">
        {state.strategyGroups.map((item) => (
          <Card
            className="config-resource-card"
            key={item.id}
            variant="borderless"
          >
            <div className="config-resource-card-heading">
              <span className="config-resource-card-icon">
                {item.kind === "selector" ? (
                  <ShareAltOutlined />
                ) : (
                  <FileTextOutlined />
                )}
              </span>
              <div>
                <strong>{strategyGroupDisplayName(item)}</strong>
                <small>
                  {item.description || t("localConfig.list.noDescription")}
                </small>
              </div>
              <Dropdown
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
                    if (key === "edit") {
                      navigate(`/config/groups/${item.id}/edit`);
                    } else {
                      confirmDelete(item);
                    }
                  },
                }}
                trigger={["click"]}
              >
                <Button
                  aria-label={t("localConfig.resources.actions", {
                    name: strategyGroupDisplayName(item),
                  })}
                  icon={<MoreOutlined />}
                  loading={busy}
                  type="text"
                />
              </Dropdown>
            </div>
            <div className="config-resource-tags">
              <Tag>{t(`localConfig.resources.groups.kinds.${item.kind}`)}</Tag>
              <Tag>
                {item.kind === "selector" ? item.type : policyLabel(item, t)}
              </Tag>
              <Tag>
                {t("localConfig.resources.groups.ruleSetCount", {
                  count: item.ruleSetReferences.length,
                })}
              </Tag>
              <Tag>{item.ruleOutput === "inline" ? "inline" : "RULE-SET"}</Tag>
            </div>
            <time dateTime={item.updatedAt}>
              {formatDate(item.updatedAt, i18n.language)}
            </time>
          </Card>
        ))}
        <Dropdown
          open={createOpen}
          onOpenChange={setCreateOpen}
          trigger={[]}
          menu={{
            items: ["rule", "selector"].map((kind) => ({
              key: kind,
              label: t(`localConfig.resources.groups.kinds.${kind}`),
            })),
            onClick: ({ key }) => {
              setCreateOpen(false);
              navigate(`/config/groups/new/${key}`);
            },
          }}
        >
          <div className="resource-create-menu-anchor">
            <LibraryCreateCard
              className="config-resource-create-card"
              help={
                <FeatureHelp
                  compact
                  translationBase="localConfig.fieldHelp.local-strategy-groups"
                />
              }
              onCreate={() => {
                setCreateOpen(true);
              }}
              title={t("localConfig.resources.groups.create")}
            />
          </div>
        </Dropdown>
      </div>
    </div>
  );
}

export function RuleSetLibraryPanel({
  state,
  onChange,
  onError,
}: ResourcePanelProps) {
  const { t, i18n } = useTranslation();
  const { modal } = App.useApp();
  const navigate = useNavigate();
  const [busy, setBusy] = useState(false);
  const confirmDelete = (item: RuleSetResource) => {
    modal.confirm({
      title: t("localConfig.resources.ruleSets.deleteTitle"),
      content: t("localConfig.resources.ruleSets.deleteDescription", {
        name: item.name,
      }),
      okText: t("common.delete"),
      cancelText: t("common.cancel"),
      okButtonProps: { danger: true },
      onOk: async () => {
        setBusy(true);
        try {
          onChange(await deleteRuleSet(item.id));
        } catch {
          onError();
          throw new Error("delete rule set failed");
        } finally {
          setBusy(false);
        }
      },
    });
  };

  return (
    <div className="config-resource-panel">
      <div className="config-resource-card-grid">
        {state.ruleSets.map((item) => (
          <Card
            className="config-resource-card"
            key={item.id}
            variant="borderless"
          >
            <div className="config-resource-card-heading">
              <span className="config-resource-card-icon">
                <GlobalOutlined />
              </span>
              <div>
                <strong>{item.name}</strong>
                <small>
                  {item.description || t("localConfig.list.noDescription")}
                </small>
              </div>
              <Dropdown
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
                    if (key === "edit") {
                      navigate(`/config/rule-sets/${item.id}/edit`);
                    } else {
                      confirmDelete(item);
                    }
                  },
                }}
                trigger={["click"]}
              >
                <Button
                  aria-label={t("localConfig.resources.actions", {
                    name: item.name,
                  })}
                  icon={<MoreOutlined />}
                  loading={busy}
                  type="text"
                />
              </Dropdown>
            </div>
            <div className="config-resource-tags">
              <Tag>{item.sourceType}</Tag>
              <Tag>{item.behavior}</Tag>
              <Tag>
                {item.sourceType === "inline"
                  ? `${item.payload.length}`
                  : item.format}
              </Tag>
              {item.noResolve ? <Tag>no-resolve</Tag> : null}
            </div>
            <time dateTime={item.updatedAt}>
              {formatDate(item.updatedAt, i18n.language)}
            </time>
          </Card>
        ))}
        <LibraryCreateCard
          className="config-resource-create-card"
          help={
            <FeatureHelp
              compact
              translationBase="localConfig.fieldHelp.local-rule-sets"
            />
          }
          onCreate={() => {
            navigate("/config/rule-sets/new");
          }}
          title={t("localConfig.resources.ruleSets.create")}
        />
      </div>
    </div>
  );
}
