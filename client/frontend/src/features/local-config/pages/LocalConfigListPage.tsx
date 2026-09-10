import { useEffect, useState } from "react";
import {
  DeleteOutlined,
  EditOutlined,
  ExportOutlined,
  ImportOutlined,
  FileTextOutlined,
  MoreOutlined,
} from "@ant-design/icons";
import { Alert, App, Button, Card, Dropdown, Spin, Tabs, Tag } from "antd";
import { useTranslation } from "react-i18next";
import { useNavigate, useSearchParams } from "react-router-dom";

import { FeatureHelp } from "../../../components/help/FeatureHelp";
import { LibraryCreateCard } from "../../../components/layout/LibraryCreateCard";
import {
  deleteLocalConfig,
  getLocalConfigResources,
  getLocalConfigState,
  getLocalScriptState,
} from "../../../services/appBridge";
import type {
  LocalConfigResourceState,
  LocalConfigState,
  LocalConfigSummary,
} from "../../../types/localConfig";
import type { LocalScriptState } from "../../../types/localScript";
import { LocalScriptLibraryPanel } from "../../local-script/components/LocalScriptLibraryPanel";
import {
  RuleSetLibraryPanel,
  StrategyGroupLibraryPanel,
} from "../components/LocalConfigResourcePanels";
import { useLocalPackageTransfer } from "../components/useLocalPackageTransfer";
import { useNavigationGuard } from "../../../app/navigationGuard/NavigationGuardContext";
import { ChainProxyWorkspace } from "../../chain-proxy/ChainProxyWorkspace";
import { isConfigSection, useConfigPageState } from "../ConfigPageStateContext";

export function LocalConfigListPage() {
  const { t, i18n } = useTranslation();
  const { modal } = App.useApp();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const guard = useNavigationGuard();
  const { section, setSection } = useConfigPageState();
  const requestedTab = searchParams.get("section") ?? section;
  const selectedTab = isConfigSection(requestedTab) ? requestedTab : "scripts";
  useEffect(() => setSection(selectedTab), [selectedTab, setSection]);
  const [state, setState] = useState<LocalConfigState | null>(null);
  const [resources, setResources] = useState<LocalConfigResourceState | null>(
    null,
  );
  const [scripts, setScripts] = useState<LocalScriptState | null>(null);
  const [loading, setLoading] = useState(true);
  const [busyID, setBusyID] = useState<string | null>(null);
  const [error, setError] = useState(false);

  const transfer = useLocalPackageTransfer(setBusyID, async () => {
    const results = await Promise.allSettled([
      getLocalConfigState(),
      getLocalConfigResources(),
      getLocalScriptState(),
    ]);
    const [configsResult, resourcesResult, scriptsResult] = results;
    if (configsResult.status === "fulfilled") setState(configsResult.value);
    if (resourcesResult.status === "fulfilled")
      setResources(resourcesResult.value);
    if (scriptsResult.status === "fulfilled") setScripts(scriptsResult.value);
    setError(results.some((result) => result.status === "rejected"));
  });

  useEffect(() => {
    let active = true;
    Promise.allSettled([
      getLocalConfigState(),
      getLocalConfigResources(),
      getLocalScriptState(),
    ])
      .then(([nextState, nextResources, nextScripts]) => {
        if (active) {
          setState(
            nextState.status === "fulfilled"
              ? nextState.value
              : { directory: "", configs: [] },
          );
          setResources(
            nextResources.status === "fulfilled"
              ? nextResources.value
              : {
                  directory: "",
                  revision: 0,
                  updatedAt: "",
                  strategyGroups: [],
                  ruleSets: [],
                },
          );
          setScripts(
            nextScripts.status === "fulfilled"
              ? nextScripts.value
              : { directory: "", scripts: [] },
          );
          if (
            [nextState, nextResources, nextScripts].some(
              (item) => item.status === "rejected",
            )
          )
            setError(true);
        }
      })
      .catch(() => {
        if (active) {
          setError(true);
        }
      })
      .finally(() => {
        if (active) {
          setLoading(false);
        }
      });
    return () => {
      active = false;
    };
  }, []);

  const confirmDelete = (config: LocalConfigSummary) => {
    modal.confirm({
      title: t("localConfig.list.deleteTitle"),
      content: t("localConfig.list.deleteDescription", { name: config.name }),
      okText: t("localConfig.list.delete"),
      cancelText: t("common.cancel"),
      okButtonProps: { danger: true },
      onOk: async () => {
        setBusyID(config.id);
        setError(false);
        try {
          setState(await deleteLocalConfig(config.id));
        } catch {
          setError(true);
          throw new Error("delete local configuration failed");
        } finally {
          setBusyID(null);
        }
      },
    });
  };

  if (loading) {
    return (
      <div className="local-config-loading">
        <Spin />
        <span>{t("localConfig.list.loading")}</span>
      </div>
    );
  }

  if (!state || !resources || !scripts) {
    return (
      <Alert
        description={t("localConfig.errors.backendUnavailable")}
        showIcon
        type="error"
      />
    );
  }

  return (
    <div className="page-stack local-config-list-page">
      {transfer.dialog}
      {error ? (
        <Alert
          closable
          description={t("localConfig.errors.operation")}
          onClose={() => setError(false)}
          showIcon
          type="error"
        />
      ) : null}

      <Tabs
        activeKey={selectedTab}
        onChange={(nextSection) => {
          if (guard.canLeave() && isConfigSection(nextSection)) {
            setSection(nextSection);
            setSearchParams({ section: nextSection }, { replace: true });
          }
        }}
        className="local-config-library-tabs"
        items={[
          {
            key: "scripts",
            label: t("localScript.tabs.scripts", {
              count: scripts.scripts.length,
            }),
            children: (
              <LocalScriptLibraryPanel
                busyID={busyID}
                onBusyChange={setBusyID}
                onError={() => setError(true)}
                onStateChange={setScripts}
                onExport={(id) =>
                  void transfer.exportPackage("local-script", id)
                }
                onImport={(id) =>
                  void transfer.importPackage("local-script", id)
                }
                state={scripts}
              />
            ),
          },
          {
            key: "configs",
            label: t("localConfig.resources.tabs.configs", {
              count: state.configs.length,
            }),
            children: (
              <div className="local-config-card-grid">
                {state.configs.map((config) => (
                  <Card
                    className="local-config-card"
                    key={config.id}
                    loading={busyID === config.id}
                    variant="borderless"
                  >
                    <div className="local-config-card-heading">
                      <div className="local-config-card-identity">
                        <span className="local-config-card-icon">
                          <FileTextOutlined />
                        </span>
                        <span className="local-config-card-copy">
                          <strong title={config.name}>{config.name}</strong>
                          <small title={config.description}>
                            {config.description ||
                              t("localConfig.list.noDescription")}
                          </small>
                        </span>
                      </div>
                      <Dropdown
                        disabled={busyID !== null}
                        menu={{
                          items: [
                            {
                              key: "edit",
                              icon: <EditOutlined />,
                              label: t("localConfig.list.edit"),
                            },
                            {
                              key: "export",
                              icon: <ExportOutlined />,
                              label: t("localPackage.export"),
                            },
                            {
                              key: "import",
                              icon: <ImportOutlined />,
                              label: t("localPackage.import"),
                            },
                            {
                              key: "delete",
                              danger: true,
                              icon: <DeleteOutlined />,
                              label: t("localConfig.list.delete"),
                            },
                          ],
                          onClick: ({ key }) => {
                            if (key === "edit") {
                              navigate(`/config/${config.id}/edit`);
                            } else if (key === "delete") {
                              confirmDelete(config);
                            } else if (key === "export") {
                              void transfer.exportPackage(
                                "local-config",
                                config.id,
                              );
                            } else if (key === "import") {
                              void transfer.importPackage(
                                "local-config",
                                config.id,
                              );
                            }
                          },
                        }}
                        trigger={["click"]}
                      >
                        <Button
                          aria-label={t("localConfig.list.menu", {
                            name: config.name,
                          })}
                          icon={<MoreOutlined />}
                          type="text"
                        />
                      </Dropdown>
                    </div>

                    {config.staticValidationStatus === "references_invalid" ? (
                      <Tag color="warning">
                        {t("localConfig.redesign.repairReferences")}
                      </Tag>
                    ) : null}
                    <div className="local-config-card-facts">
                      <span>
                        <strong>{config.enabledFieldCount}</strong>
                        {t("localConfig.list.enabledFields")}
                      </span>
                      <span>
                        <strong>{config.ruleProviderCount}</strong>
                        {t("localConfig.list.ruleProviders")}
                      </span>
                      <span>
                        <strong>{config.strategyGroupCount}</strong>
                        {t("localConfig.list.strategyGroups")}
                      </span>
                    </div>

                    <time dateTime={config.updatedAt}>
                      {t("localConfig.list.updatedAt")}
                      {" · "}
                      {new Intl.DateTimeFormat(i18n.language, {
                        dateStyle: "short",
                        timeStyle: "short",
                      }).format(new Date(config.updatedAt))}
                    </time>
                  </Card>
                ))}

                <LibraryCreateCard
                  className="local-config-create-card"
                  help={<FeatureHelp compact topic="localConfig" />}
                  onCreate={() => navigate("/config/new")}
                  title={t("localConfig.list.create")}
                />
              </div>
            ),
          },
          {
            key: "groups",
            label: t("localConfig.resources.tabs.groups", {
              count: resources.strategyGroups.length,
            }),
            children: (
              <StrategyGroupLibraryPanel
                onChange={setResources}
                onError={() => setError(true)}
                state={resources}
              />
            ),
          },
          {
            key: "ruleSets",
            label: t("localConfig.resources.tabs.ruleSets", {
              count: resources.ruleSets.length,
            }),
            children: (
              <RuleSetLibraryPanel
                onChange={setResources}
                onError={() => setError(true)}
                state={resources}
              />
            ),
          },
          {
            key: "chains",
            label: t("chainProxy.tab"),
            children: <ChainProxyWorkspace />,
            destroyOnHidden: true,
          },
        ]}
      />
    </div>
  );
}
