import {
  ArrowDownOutlined,
  CheckOutlined,
  CopyOutlined,
  DeleteOutlined,
  FileTextOutlined,
  PauseOutlined,
  PlayCircleOutlined,
  SearchOutlined,
} from "@ant-design/icons";
import {
  Alert,
  Button,
  Empty,
  Input,
  Segmented,
  Select,
  Spin,
  Tag,
  Tooltip,
} from "antd";
import type { PropsWithChildren, ReactNode, UIEvent } from "react";
import {
  useCallback,
  useDeferredValue,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useTranslation } from "react-i18next";
import { Navigate } from "react-router-dom";

import { isLogPageAvailable } from "../../app/navigation";
import { useRuntimeStatus } from "../../app/runtime/RuntimeStatusContext";
import { useWindowActivity } from "../../app/runtime/WindowActivityContext";
import { FeatureHelp } from "../../components/help/FeatureHelp";
import {
  subscribeLogs,
  type MihomoLogEntry,
  type MihomoLogLevel,
  type MihomoStreamState,
} from "../../lib/mihomo/client";
import {
  appendBoundedLogs,
  filterMihomoLogs,
  formatMihomoLogContent,
  mihomoLogBufferLimit,
  type MihomoLogLevelFilter,
} from "./logFilters";

const mihomoLogLevels: readonly MihomoLogLevel[] = [
  "error",
  "warning",
  "info",
  "debug",
];
const displayLogLevels: readonly MihomoLogLevelFilter[] = [
  "all",
  ...mihomoLogLevels,
];
const flushDelayMilliseconds = 100;

type LogStreamState = MihomoStreamState | "offline" | "paused";
type DisplayLogEntry = MihomoLogEntry & { id: number };

export function LogsRouteGuard({ children }: PropsWithChildren) {
  const { preferencesLoading, runtimePreferences } = useRuntimeStatus();
  if (preferencesLoading) {
    return (
      <div className="logs-route-loading">
        <Spin />
      </div>
    );
  }
  if (!isLogPageAvailable(runtimePreferences?.logLevel)) {
    return <Navigate replace to="/home" />;
  }
  return children;
}

export function LogsPage() {
  const { t, i18n } = useTranslation();
  const {
    liveDataReady,
    preferencesBusy,
    runtime,
    runtimePreferences,
    setLogLevel,
  } = useRuntimeStatus();
  const { isWindowVisible } = useWindowActivity();
  const [logs, setLogs] = useState<DisplayLogEntry[]>([]);
  const [query, setQuery] = useState("");
  const deferredQuery = useDeferredValue(query);
  const [displayLevel, setDisplayLevel] =
    useState<MihomoLogLevelFilter>("all");
  const [streamState, setStreamState] =
    useState<LogStreamState>("offline");
  const [paused, setPaused] = useState(false);
  const [autoFollow, setAutoFollow] = useState(true);
  const [errorKey, setErrorKey] = useState("");
  const [copied, setCopied] = useState(false);
  const listRef = useRef<HTMLDivElement | null>(null);
  const pendingLogs = useRef<DisplayLogEntry[]>([]);
  const flushTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const copyTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const nextLogID = useRef(0);
  const controllerSession = runtime?.mihomo.controllerSession ?? null;
  const session = useMemo(
    () =>
      controllerSession
        ? {
            id: controllerSession.id,
            baseUrl: controllerSession.baseUrl,
            secret: controllerSession.secret,
          }
        : null,
    [
      controllerSession?.baseUrl,
      controllerSession?.id,
      controllerSession?.secret,
    ],
  );
  const configuredLevel = runtimePreferences?.logLevel;
  const activeLogLevel: MihomoLogLevel | null =
    configuredLevel && configuredLevel !== "silent" ? configuredLevel : null;
  const runtimeReady = Boolean(
    runtime?.mihomo.state === "running" &&
      runtime.mihomo.controllerReady &&
      session,
  );

  const discardPendingLogs = useCallback(() => {
    if (flushTimer.current !== null) {
      globalThis.clearTimeout(flushTimer.current);
      flushTimer.current = null;
    }
    pendingLogs.current = [];
  }, []);

  const flushPendingLogs = useCallback(() => {
    flushTimer.current = null;
    const batch = pendingLogs.current;
    pendingLogs.current = [];
    if (batch.length === 0) return;
    setLogs((current) =>
      appendBoundedLogs(current, batch, mihomoLogBufferLimit),
    );
  }, []);

  const enqueueLog = useCallback(
    (log: MihomoLogEntry) => {
      pendingLogs.current.push({ ...log, id: ++nextLogID.current });
      if (flushTimer.current === null) {
        flushTimer.current = globalThis.setTimeout(
          flushPendingLogs,
          flushDelayMilliseconds,
        );
      }
    },
    [flushPendingLogs],
  );

  useEffect(() => {
    return () => {
      discardPendingLogs();
      if (copyTimer.current !== null) globalThis.clearTimeout(copyTimer.current);
    };
  }, [discardPendingLogs]);

  useEffect(() => {
    discardPendingLogs();
    setLogs([]);
    setAutoFollow(true);
  }, [discardPendingLogs, runtime?.mihomo.generationId, session?.id]);

  useEffect(() => {
    discardPendingLogs();
    if (!runtimeReady || !session || !activeLogLevel) {
      setStreamState("offline");
      return () => undefined;
    }
    if (paused || !isWindowVisible || !liveDataReady) {
      setStreamState("paused");
      return () => undefined;
    }
    const unsubscribe = subscribeLogs(session, activeLogLevel, {
      onMessage: enqueueLog,
      onStateChange: setStreamState,
    });
    return () => {
      unsubscribe();
      discardPendingLogs();
    };
  }, [
    activeLogLevel,
    discardPendingLogs,
    enqueueLog,
    isWindowVisible,
    liveDataReady,
    paused,
    runtimeReady,
    session,
  ]);

  const filteredLogs = useMemo(
    () => filterMihomoLogs(logs, deferredQuery, displayLevel),
    [deferredQuery, displayLevel, logs],
  );

  useEffect(() => {
    if (!autoFollow || filteredLogs.length === 0) return;
    const frame = globalThis.requestAnimationFrame(() => {
      const list = listRef.current;
      if (list) list.scrollTop = list.scrollHeight;
    });
    return () => globalThis.cancelAnimationFrame(frame);
  }, [autoFollow, filteredLogs.length]);

  const changeLogLevel = async (level: MihomoLogLevel) => {
    if (preferencesBusy || level === activeLogLevel) return;
    setErrorKey("");
    try {
      await setLogLevel(level);
    } catch {
      setErrorKey("pages.logs.errors.level");
    }
  };

  const clearLogs = () => {
    discardPendingLogs();
    setLogs([]);
    setAutoFollow(true);
  };

  const copyVisibleLogs = async () => {
    if (filteredLogs.length === 0) return;
    const text = filteredLogs
      .map((log) => {
        const time = displayLogTime(log, i18n.language);
        return `${time} [${log.level.toLocaleUpperCase()}] ${formatMihomoLogContent(log)}`;
      })
      .join("\n");
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      if (copyTimer.current !== null) globalThis.clearTimeout(copyTimer.current);
      copyTimer.current = globalThis.setTimeout(() => setCopied(false), 1400);
    } catch {
      setErrorKey("pages.logs.errors.copy");
    }
  };

  const handleListScroll = (event: UIEvent<HTMLDivElement>) => {
    const target = event.currentTarget;
    const atEnd =
      target.scrollHeight - target.scrollTop - target.clientHeight <= 28;
    setAutoFollow(atEnd);
  };

  const scrollToEnd = () => {
    setAutoFollow(true);
    const list = listRef.current;
    if (list) list.scrollTop = list.scrollHeight;
  };

  return (
    <div className="page-stack logs-page">
      <section className="logs-panel">
        <div className="logs-heading-row">
          <div className="logs-heading">
            <span className="logs-heading-icon">
              <FileTextOutlined />
            </span>
            <div>
              <span className="logs-title-line">
                <strong>{t("pages.logs.title")}</strong>
                <FeatureHelp compact topic="logs" />
              </span>
              <small>
                {t("pages.logs.summary", {
                  visible: filteredLogs.length,
                  total: logs.length,
                  limit: mihomoLogBufferLimit,
                })}
              </small>
            </div>
          </div>
          <div className="logs-level-control">
            <Tag className={`logs-stream-state ${streamState}`}>
              {t(`pages.logs.states.${streamState}`)}
            </Tag>
            <Segmented
              aria-label={t("pages.logs.coreLevel")}
              disabled={preferencesBusy || !activeLogLevel}
              onChange={(value) =>
                void changeLogLevel(value as MihomoLogLevel)
              }
              options={mihomoLogLevels.map((level) => ({
                label: t(`home.runtimePreferences.logLevels.${level}`),
                value: level,
              }))}
              value={activeLogLevel ?? undefined}
            />
            {preferencesBusy ? <Spin size="small" /> : null}
          </div>
        </div>

        <div className="logs-toolbar">
          <Input
            allowClear
            aria-label={t("pages.logs.search")}
            onChange={(event) => setQuery(event.target.value)}
            placeholder={t("pages.logs.search")}
            prefix={<SearchOutlined />}
            value={query}
          />
          <Select
            aria-label={t("pages.logs.displayLevel")}
            onChange={(value: MihomoLogLevelFilter) => setDisplayLevel(value)}
            options={displayLogLevels.map((level) => ({
              label: t(`pages.logs.levels.${level}`),
              value: level,
            }))}
            popupMatchSelectWidth={false}
            value={displayLevel}
          />
          <Tooltip
            title={t(paused ? "pages.logs.actions.resume" : "pages.logs.actions.pause")}
          >
            <Button
              aria-label={t(
                paused ? "pages.logs.actions.resume" : "pages.logs.actions.pause",
              )}
              disabled={!runtimeReady}
              icon={paused ? <PlayCircleOutlined /> : <PauseOutlined />}
              onClick={() => setPaused((current) => !current)}
              type={paused ? "primary" : "default"}
            />
          </Tooltip>
          <Tooltip
            title={t(
              copied ? "pages.logs.actions.copied" : "pages.logs.actions.copy",
            )}
          >
            <Button
              aria-label={t("pages.logs.actions.copy")}
              disabled={filteredLogs.length === 0}
              icon={copied ? <CheckOutlined /> : <CopyOutlined />}
              onClick={() => void copyVisibleLogs()}
            />
          </Tooltip>
          <Tooltip title={t("pages.logs.actions.clear")}>
            <Button
              aria-label={t("pages.logs.actions.clear")}
              disabled={logs.length === 0}
              icon={<DeleteOutlined />}
              onClick={clearLogs}
            />
          </Tooltip>
        </div>

        {errorKey ? (
          <Alert
            closable
            description={t(errorKey)}
            onClose={() => setErrorKey("")}
            showIcon
            type="error"
          />
        ) : null}

        {!runtimeReady && logs.length === 0 ? (
          <Empty
            description={t("pages.logs.requiresCore")}
            image={Empty.PRESENTED_IMAGE_SIMPLE}
          />
        ) : filteredLogs.length === 0 ? (
          streamState === "connecting" || streamState === "reconnecting" ? (
            <div className="logs-loading">
              <Spin />
              <span>{t("pages.logs.connecting")}</span>
            </div>
          ) : (
            <Empty
              description={t(
                logs.length === 0
                  ? paused
                    ? "pages.logs.pausedEmpty"
                    : "pages.logs.empty"
                  : "pages.logs.noMatch",
              )}
              image={Empty.PRESENTED_IMAGE_SIMPLE}
            />
          )
        ) : (
          <div className="logs-list-shell">
            <div className="logs-list-header" aria-hidden="true">
              <span>{t("pages.logs.columns.time")}</span>
              <span>{t("pages.logs.columns.level")}</span>
              <span>{t("pages.logs.columns.content")}</span>
            </div>
            <div
              className="logs-list"
              onScroll={handleListScroll}
              ref={listRef}
            >
              {filteredLogs.map((log) => {
                const content = formatMihomoLogContent(log);
                return (
                  <article className="logs-row" key={log.id}>
                    <time title={formatReceivedAt(log.receivedAt, i18n.language)}>
                      {displayLogTime(log, i18n.language)}
                    </time>
                    <span className={`logs-level ${log.level}`}>
                      {t(`pages.logs.levels.${log.level}`)}
                    </span>
                    <pre className="logs-message">
                      {highlightText(content, deferredQuery)}
                    </pre>
                  </article>
                );
              })}
            </div>
            {!autoFollow ? (
              <Button
                className="logs-follow-button"
                icon={<ArrowDownOutlined />}
                onClick={scrollToEnd}
                size="small"
              >
                {t("pages.logs.actions.follow")}
              </Button>
            ) : null}
          </div>
        )}
      </section>
    </div>
  );
}

function displayLogTime(log: MihomoLogEntry, locale: string): string {
  if (log.time.trim()) return log.time.trim();
  return new Intl.DateTimeFormat(locale, {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  }).format(log.receivedAt);
}

function formatReceivedAt(timestamp: number, locale: string): string {
  return new Intl.DateTimeFormat(locale, {
    dateStyle: "medium",
    timeStyle: "medium",
  }).format(timestamp);
}

function highlightText(text: string, input: string): ReactNode {
  const query = input.trim();
  if (!query) return text;
  const normalizedText = text.toLocaleLowerCase();
  const normalizedQuery = query.toLocaleLowerCase();
  const parts: ReactNode[] = [];
  let cursor = 0;
  let match = normalizedText.indexOf(normalizedQuery);
  while (match >= 0) {
    if (match > cursor) parts.push(text.slice(cursor, match));
    parts.push(
      <mark key={`${match}-${parts.length}`}>
        {text.slice(match, match + query.length)}
      </mark>,
    );
    cursor = match + query.length;
    match = normalizedText.indexOf(normalizedQuery, cursor);
  }
  if (cursor < text.length) parts.push(text.slice(cursor));
  return parts.length > 0 ? parts : text;
}
