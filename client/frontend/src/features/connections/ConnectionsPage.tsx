import {
  CloseCircleOutlined,
  DeleteOutlined,
  LinkOutlined,
  SearchOutlined,
} from "@ant-design/icons";
import {
  Alert,
  App,
  Button,
  Empty,
  Input,
  Segmented,
  Spin,
  Tooltip,
} from "antd";
import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";

import { useRuntimeStatus } from "../../app/runtime/RuntimeStatusContext";
import { useWindowActivity } from "../../app/runtime/WindowActivityContext";
import { FeatureHelp } from "../../components/help/FeatureHelp";
import {
  closeConnections,
  closeConnection,
  type MihomoConnection,
  type MihomoConnectionsSnapshot,
  type MihomoStreamState,
} from "../../lib/mihomo/client";
import {
  connectionTarget,
  filterConnections,
  type ConnectionRouteFilter,
} from "./connectionFilters";
import { ConnectionDetailsDialog } from "./ConnectionDetailsDialog";
import { observeConnections } from "./connectionFeed";
import {
  connectionMatchedRule,
  connectionOutbound,
  connectionProcessName,
  formatConnectionBytes as formatBytes,
} from "./connectionPresentation";

type ConnectionStreamState = MihomoStreamState | "offline";

export function ConnectionsPage() {
  const { t, i18n } = useTranslation();
  const { modal } = App.useApp();
  const { liveDataReady, runtime } = useRuntimeStatus();
  const { isWindowVisible } = useWindowActivity();
  const [snapshot, setSnapshot] = useState<MihomoConnectionsSnapshot | null>(null);
  const [streamState, setStreamState] =
    useState<ConnectionStreamState>("offline");
  const [query, setQuery] = useState("");
  const [routeFilter, setRouteFilter] =
    useState<ConnectionRouteFilter>("all");
  const [closingID, setClosingID] = useState("");
  const [closeAllBusy, setCloseAllBusy] = useState(false);
  const [operationError, setOperationError] = useState(false);
  const [updatedAt, setUpdatedAt] = useState(0);
  const [inspectedConnection, setInspectedConnection] =
    useState<MihomoConnection | null>(null);
  const session = runtime?.mihomo.controllerSession ?? null;
  const runtimeReady =
    runtime?.mihomo.state === "running" &&
    runtime.mihomo.controllerReady &&
    session !== null;

  useEffect(() => {
    setSnapshot(null);
    setUpdatedAt(0);
    setOperationError(false);
    if (!isWindowVisible || !liveDataReady || !runtimeReady || !session) {
      setStreamState("offline");
      return () => undefined;
    }

    setStreamState("connecting");
    return observeConnections(session, {
      onMessage: (value) => {
        setSnapshot(value);
        setUpdatedAt(Date.now());
      },
      onStateChange: setStreamState,
    });
  }, [
    isWindowVisible,
    liveDataReady,
    runtime?.mihomo.generationId,
    runtimeReady,
    session?.id,
  ]);

  const connections = useMemo(() => {
    return filterConnections(snapshot?.connections ?? [], {
      query,
      route: routeFilter,
    });
  }, [query, routeFilter, snapshot]);
  const live = streamState === "live";

  useEffect(() => {
    // Preserve the latest observed details if the connection ends or the
    // stream disconnects while its dialog is open.
    setInspectedConnection((current) => current
      ? snapshot?.connections.find((connection) => connection.id === current.id) ?? current
      : null);
  }, [snapshot]);

  const closeOne = async (connection: MihomoConnection) => {
    if (!session || !runtimeReady || closingID || closeAllBusy) return;
    setClosingID(connection.id);
    setOperationError(false);
    try {
      await closeConnection(session, connection.id);
      setSnapshot((current) =>
        current
          ? {
              ...current,
              connections: current.connections.filter(
                (item) => item.id !== connection.id,
              ),
            }
          : current,
      );
    } catch {
      setOperationError(true);
    } finally {
      setClosingID("");
    }
  };

  const confirmCloseAll = () => {
    if (!session || !runtimeReady || !live || connections.length === 0) return;
    const targetIDs = connections.map((connection) => connection.id);
    const targetIDSet = new Set(targetIDs);
    modal.confirm({
      title: t("connections.closeAll.title"),
      content: t("connections.closeAll.description", {
        count: targetIDs.length,
      }),
      okText: t("connections.closeAll.confirm"),
      cancelText: t("common.cancel"),
      okButtonProps: { danger: true },
      onOk: async () => {
        setCloseAllBusy(true);
        setOperationError(false);
        try {
          await closeConnections(session, targetIDs);
          setSnapshot((current) =>
            current
              ? {
                  ...current,
                  connections: current.connections.filter(
                    (connection) => !targetIDSet.has(connection.id),
                  ),
                }
              : current,
          );
        } catch {
          setOperationError(true);
        } finally {
          setCloseAllBusy(false);
        }
      },
    });
  };

  return (
    <div className="page-stack connections-page">
      <section className="connections-panel">
        <div className="connections-toolbar">
          <div className="connections-heading">
            <span className="connections-heading-icon">
              <LinkOutlined />
            </span>
            <div>
              <span className="connections-title-line">
                <strong>{t("connections.title")}</strong>
                <FeatureHelp compact topic="connections" />
              </span>
              <small>
                {t(
                  connections.length === (snapshot?.connections.length ?? 0)
                    ? "connections.summary"
                    : "connections.summaryFiltered",
                  {
                    count: connections.length,
                    total: snapshot?.connections.length ?? 0,
                  },
                )}
              </small>
            </div>
          </div>
          <div className="connections-toolbar-actions">
            <Segmented
              aria-label={t("connections.filters.route")}
              onChange={(value) =>
                setRouteFilter(value as ConnectionRouteFilter)
              }
              options={[
                { label: t("connections.filters.all"), value: "all" },
                { label: t("connections.filters.direct"), value: "direct" },
                { label: t("connections.filters.proxy"), value: "proxy" },
              ]}
              value={routeFilter}
            />
            <Input
              allowClear
              aria-label={t("connections.search")}
              onChange={(event) => setQuery(event.target.value)}
              placeholder={t("connections.search")}
              prefix={<SearchOutlined />}
              value={query}
            />
            <Button
              danger
              disabled={
                !live || connections.length === 0 || Boolean(closingID)
              }
              icon={<DeleteOutlined />}
              loading={closeAllBusy}
              onClick={confirmCloseAll}
            >
              {t("connections.closeAll.button")}
            </Button>
          </div>
        </div>

        {operationError ? (
          <Alert
            closable
            description={t("connections.errors.close")}
            onClose={() => setOperationError(false)}
            showIcon
            type="error"
          />
        ) : null}

        {!runtimeReady ? (
          <Alert
            description={t("connections.requiresCore")}
            showIcon
            type="info"
          />
        ) : !live && snapshot ? (
          <Alert
            description={t("connections.stale", {
              time: updatedAt
                ? new Intl.DateTimeFormat(i18n.language, {
                    hour: "2-digit",
                    minute: "2-digit",
                    second: "2-digit",
                  }).format(updatedAt)
                : "—",
            })}
            showIcon
            type="warning"
          />
        ) : null}

        {!runtimeReady ? null : !snapshot ? (
          <div className="connections-loading">
            <Spin />
            <span>{t(streamState === "error" || streamState === "reconnecting"
              ? `connections.states.${streamState}`
              : "connections.loading")}</span>
          </div>
        ) : connections.length === 0 ? (
          <Empty
            description={t(
              query.trim() || routeFilter !== "all"
                ? "connections.emptySearch"
                : "connections.empty",
            )}
            image={Empty.PRESENTED_IMAGE_SIMPLE}
          />
        ) : (
          <div className={`connection-list${live ? "" : " stale"}`}>
            {connections.map((connection) => {
              const target = connectionTarget(connection);
              const process = connectionProcessName(connection);
              const matchedRule = connectionMatchedRule(connection);
              const outbound = connectionOutbound(connection);
              return (
                <article className="connection-item" key={connection.id}>
                  <div className="connection-target">
                    <button
                      aria-haspopup="dialog"
                      aria-label={t("connections.openDetails", { target })}
                      className="connection-details-trigger"
                      onClick={() => setInspectedConnection(connection)}
                      title={target}
                      type="button"
                    >
                      {target}
                    </button>
                    <small>
                      {connection.metadata.network || "—"}
                      {connection.metadata.type
                        ? ` · ${connection.metadata.type}`
                        : ""}
                    </small>
                  </div>
                  <div className="connection-detail connection-process">
                    <span>{t("connections.process")}</span>
                    {process ? (
                      <button
                        aria-haspopup="dialog"
                        aria-label={t("connections.viewProcessPath", { process })}
                        className="connection-details-trigger"
                        onClick={() => setInspectedConnection(connection)}
                        title={connection.metadata.processPath}
                        type="button"
                      >
                        {process}
                      </button>
                    ) : <strong>{t("connections.unknown")}</strong>}
                  </div>
                  <div className="connection-detail connection-rule">
                    <span>{t("connections.rule")}</span>
                    <strong title={matchedRule}>
                      {matchedRule}
                    </strong>
                  </div>
                  <div className="connection-detail connection-outbound">
                    <span>{t("connections.outbound")}</span>
                    <strong title={outbound}>
                      {outbound}
                    </strong>
                  </div>
                  <div className="connection-traffic">
                    <span>↑ {formatBytes(connection.upload, i18n.language)}</span>
                    <span>↓ {formatBytes(connection.download, i18n.language)}</span>
                  </div>
                  <Tooltip title={t("connections.closeOne")}>
                    <Button
                      aria-label={t("connections.closeOneNamed", {
                        target,
                      })}
                      className="connection-close"
                      danger
                      disabled={!live || closeAllBusy}
                      icon={<CloseCircleOutlined />}
                      loading={closingID === connection.id}
                      onClick={() => void closeOne(connection)}
                      type="text"
                    />
                  </Tooltip>
                </article>
              );
            })}
          </div>
        )}
      </section>
      <ConnectionDetailsDialog
        connection={inspectedConnection}
        onClose={() => setInspectedConnection(null)}
        state={!live ? "stale" : snapshot?.connections.some((connection) =>
          connection.id === inspectedConnection?.id) ? "active" : "ended"}
      />
    </div>
  );
}
