import { DownOutlined, SearchOutlined, UpOutlined } from "@ant-design/icons";
import { Alert, Button, Input, Modal, Spin, Tooltip, type InputRef } from "antd";
import { useEffect, useMemo, useRef, useState, type KeyboardEvent, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { ConfigurationText } from "./ConfigurationText";
import { configurationSearchAction, findConfigurationMatches, nextConfigurationMatch } from "./textSearch";

interface ConfigurationViewerProps {
  open: boolean;
  title: ReactNode;
  contents: string;
  onClose: () => void;
  documentKey?: string;
  loading?: boolean;
  loadingLabel?: string;
  error?: string;
  metadata?: ReactNode;
  details?: ReactNode;
}

// This viewer owns only presentation and search. Callers retain their existing
// read-only Go APIs and pass the exact source or already-sanitized preview text.
export function ConfigurationViewer({
  open, title, contents, onClose, documentKey = "", loading = false,
  loadingLabel, error, metadata, details,
}: ConfigurationViewerProps) {
  const { t } = useTranslation();
  const [query, setQuery] = useState("");
  const [position, setPosition] = useState<{ matches: Uint32Array; index: number } | null>(null);
  const input = useRef<InputRef>(null);
  const searchable = open && !loading && !error ? contents : "";
  const matches = useMemo(() => findConfigurationMatches(searchable, query), [searchable, query]);
  const count = matches.length / 2;
  const activeMatch = count ? (position?.matches === matches ? position.index : 0) : -1;

  useEffect(() => {
    setQuery("");
    setPosition(null);
  }, [open, documentKey]);

  const move = (direction: 1 | -1) => {
    setPosition((current) => ({
      matches,
      index: nextConfigurationMatch(current?.matches === matches ? current.index : activeMatch, count, direction),
    }));
  };
  const focusSearch = () => input.current?.focus({ cursor: "all", preventScroll: true });
  const handleKey = (event: KeyboardEvent<HTMLDivElement>) => {
    if (!open) return;
    const action = configurationSearchAction(
      { ...event, isComposing: event.nativeEvent.isComposing },
      event.target === input.current?.input,
      query.length > 0,
    );
    if (!action) return;
    event.preventDefault();
    event.stopPropagation();
    if (action === "focus") focusSearch();
    else if (action === "clear") { setQuery(""); focusSearch(); }
    else move(action === "next" ? 1 : -1);
  };

  return <Modal
    afterOpenChange={(visible) => { if (visible) focusSearch(); }}
    centered
    className="configuration-viewer-modal"
    classNames={{ header: "configuration-viewer-header", body: "configuration-viewer-body" }}
    destroyOnHidden
    footer={null}
    modalRender={(dialog) => <div onKeyDownCapture={handleKey}>{dialog}</div>}
    onCancel={onClose}
    open={open}
    title={<div className="configuration-viewer-titlebar">
      <div className="configuration-viewer-title">{title}</div>
      <div aria-label={t("configurationViewer.search")} className="configuration-viewer-search" role="search">
        <Input
          allowClear
          aria-keyshortcuts="Control+f Meta+f"
          aria-label={t("configurationViewer.search")}
          autoComplete="off"
          disabled={loading || !!error}
          onChange={(event) => setQuery(event.target.value)}
          placeholder={t("configurationViewer.placeholder")}
          prefix={<SearchOutlined />}
          ref={input}
          size="small"
          spellCheck={false}
          value={query}
        />
        <span
          aria-label={count ? t("configurationViewer.matches", { current: activeMatch + 1, total: count }) : t("configurationViewer.noMatches")}
          aria-live="polite"
          className={`configuration-viewer-count${query && count === 0 ? " is-empty" : ""}`}
          role="status"
        >{activeMatch + 1} / {count}</span>
        <Tooltip title={t("configurationViewer.previous")}>
          <Button aria-label={t("configurationViewer.previous")} disabled={!count} icon={<UpOutlined />} onClick={() => move(-1)} size="small" type="text" />
        </Tooltip>
        <Tooltip title={t("configurationViewer.next")}>
          <Button aria-label={t("configurationViewer.next")} disabled={!count} icon={<DownOutlined />} onClick={() => move(1)} size="small" type="text" />
        </Tooltip>
      </div>
    </div>}
    width={1040}
  >
    {loading ? <div className="configuration-viewer-loading">
      <Spin /><span>{loadingLabel || t("configurationViewer.loading")}</span>
    </div> : error ? <Alert description={error} showIcon type="error" /> : <>
      {metadata ? <div className="configuration-viewer-meta">{metadata}</div> : null}
      {details}
      <ConfigurationText contents={contents} matches={matches} activeMatch={activeMatch} />
    </>}
  </Modal>;
}
