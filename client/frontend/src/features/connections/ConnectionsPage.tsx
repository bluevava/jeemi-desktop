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
  Select,
  Spin,
  Tag,
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
  getConnections,
  getProxyRuntimeState,
  subscribeConnections,
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
  const [proxySelectors, setProxySelectors] = useState<string[]>([]);
  const [selectedProxySelectors, setSelectedProxySelectors] = useState<
    string[] | null
  >(null);
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
    setSelectedProxySelectors(null);
  }, [runtime?.mihomo.generationId, session?.id]);

  useEffect(() => {
    setSnapshot(null);
    setUpdatedAt(0);
    setOperationError(false);
    setProxySelectors([]);
    if (!isWindowVisible || !liveDataReady || !runtimeReady || !session) {
      setStreamState("offline");
      return () => undefined;
    }

    let active = true;
    setStreamState("connecting");
    void getConnections(session)
      .then((value) => {
        if (active) {
          setSnapshot(value);
          setUpdatedAt(Date.now());
        }
      })
      .catch(() => {
        if (active) setStreamState("error");
      });
    const refreshProxySelectors = async () => {
      try {
        const value = await getProxyRuntimeState(session);
        if (!active) return;
        setProxySelectors(value.selectorNames);
        setSelectedProxySelectors((current) =>
          current === null
            ? null
            : current.filter((name) => value.selectorNames.includes(name)),
        );
      } catch {
        // Keep the last known selector list during a transient controller
        // failure. The connection stream state still disables destructive
        // actions when the session is no longer live.
      }
    };
    void refreshProxySelectors();
    const selectorTimer = globalThis.setInterval(
      refreshProxySelectors,
      10000,
    );
    const unsubscribe = subscribeConnections(session, {
      onMessage: (value) => {
        if (!active) return;
        setSnapshot(value);
        setUpdatedAt(Date.now());
      },
      onStateChange: (value) => {
        if (active) setStreamState(value);
      },
    });
    return () => {
      active = false;
      globalThis.clearInterval(selectorTimer);
      unsubscribe();
    };
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
      selectedSelectors:
        selectedProxySelectors === null
          ? null
          : new Set(selectedProxySelectors),
    });
  }, [query, routeFilter, selectedProxySelectors, snapshot]);
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
                    upload: formatBytes(snapshot?.uploadTotal ?? 0, i18n.language),
                    download: formatBytes(snapshot?.downloadTotal ?? 0, i18n.language),
                  },
                )}
              </small>
            </div>
          </div>
          <div className="connections-toolbar-actions">
            <Tag color={live ? "success" : streamState === "offline" ? "default" : "processing"}>
              {t(`connections.states.${streamState}`)}
            </Tag>
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
            {routeFilter === "proxy" ? (
              <Select
                aria-label={t("connections.filters.selectors")}
                maxTagCount="responsive"
                mode="multiple"
                onChange={(values) =>
                  setSelectedProxySelectors(
                    values.length === proxySelectors.length ? null : values,
                  )
                }
                options={proxySelectors.map((name) => ({
                  label: name,
                  value: name,
                }))}
                placeholder={t("connections.filters.selectorsPlaceholder")}
                value={selectedProxySelectors ?? proxySelectors}
              />
            ) : null}
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
            <span>{t("connections.loading")}</span>
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
