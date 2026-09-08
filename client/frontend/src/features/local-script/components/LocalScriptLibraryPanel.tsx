import {
  CodeOutlined,
  DeleteOutlined,
  EditOutlined,
  ExportOutlined,
  ImportOutlined,
  MoreOutlined,
} from "@ant-design/icons";
import { App, Button, Card, Dropdown } from "antd";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import { FeatureHelp } from "../../../components/help/FeatureHelp";
import { LibraryCreateCard } from "../../../components/layout/LibraryCreateCard";
import { deleteLocalScript } from "../../../services/appBridge";
import type {
  LocalScriptState,
  LocalScriptSummary,
} from "../../../types/localScript";

interface LocalScriptLibraryPanelProps {
  busyID: string | null;
  onBusyChange: (id: string | null) => void;
  onError: (error: unknown) => void;
  onStateChange: (state: LocalScriptState) => void;
  onExport: (id: string) => void;
  onImport: (id: string) => void;
  state: LocalScriptState;
}

export function LocalScriptLibraryPanel({
  busyID,
  onBusyChange,
  onError,
  onStateChange,
  onExport,
  onImport,
  state,
}: LocalScriptLibraryPanelProps) {
  const { t, i18n } = useTranslation();
  const { modal } = App.useApp();
  const navigate = useNavigate();

  const confirmDelete = (script: LocalScriptSummary) => {
    modal.confirm({
      title: t("localScript.list.deleteTitle"),
      content: t("localScript.list.deleteDescription", { name: script.name }),
      okText: t("localScript.list.delete"),
      cancelText: t("common.cancel"),
      okButtonProps: { danger: true },
      onOk: async () => {
        onBusyChange(script.id);
        try {
          onStateChange(await deleteLocalScript(script.id));
        } catch (error) {
          onError(error);
          throw error;
        } finally {
          onBusyChange(null);
        }
      },
    });
  };

  return (
    <div className="local-config-card-grid local-script-card-grid">
      {state.scripts.map((script) => (
        <Card
          className="local-config-card local-script-card"
          key={script.id}
          loading={busyID === script.id}
          variant="borderless"
        >
          <div className="local-config-card-heading">
            <div className="local-config-card-identity">
              <span className="local-config-card-icon">
                <CodeOutlined />
              </span>
              <span className="local-config-card-copy">
                <strong title={script.name}>{script.name}</strong>
                <small title={script.description}>
                  {script.description || t("localScript.list.noDescription")}
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
                    label: t("localScript.list.edit"),
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
                    label: t("localScript.list.delete"),
                  },
                ],
                onClick: ({ key }) => {
                  if (key === "edit") {
                    navigate(`/config/scripts/${script.id}/edit`);
                  } else if (key === "delete") {
                    confirmDelete(script);
                  } else if (key === "export") {
                    onExport(script.id);
                  } else if (key === "import") {
                    onImport(script.id);
                  }
                },
              }}
              trigger={["click"]}
            >
              <Button
                aria-label={t("localScript.list.menu", { name: script.name })}
                icon={<MoreOutlined />}
                type="text"
              />
            </Dropdown>
          </div>
          <div className="local-config-card-facts">
            <span>
              <strong>{script.lineCount}</strong>
              {t("localScript.list.lines")}
            </span>
            <span>
              <strong>{formatSize(script.sizeBytes, i18n.language)}</strong>
              {t("localScript.list.size")}
            </span>
          </div>
          <time dateTime={script.updatedAt}>
            {t("localScript.list.updatedAt")}
            {" · "}
            {new Intl.DateTimeFormat(i18n.language, {
              dateStyle: "short",
              timeStyle: "short",
            }).format(new Date(script.updatedAt))}
          </time>
        </Card>
      ))}
      <LibraryCreateCard
        className="local-config-create-card"
        help={<FeatureHelp compact topic="localScript" />}
        onCreate={() => navigate("/config/scripts/new")}
        title={t("localScript.list.create")}
      />
    </div>
  );
}

function formatSize(bytes: number, locale: string): string {
  if (bytes < 1024) return `${bytes} B`;
  return `${new Intl.NumberFormat(locale, {
    maximumFractionDigits: 1,
  }).format(bytes / 1024)} KiB`;
}
