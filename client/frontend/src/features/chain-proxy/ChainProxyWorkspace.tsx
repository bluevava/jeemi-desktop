import { useCallback, useEffect, useRef, useState } from "react";
import { PlusOutlined, ReloadOutlined } from "@ant-design/icons";
import { Alert, App, Button, Dropdown, Empty, Spin } from "antd";
import { useTranslation } from "react-i18next";
import { useUnsavedChangesGuard } from "../../app/navigationGuard/NavigationGuardContext";
import { useProxyOverview } from "../../app/runtime/ProxyOverviewContext";
import { FeatureHelp } from "../../components/help/FeatureHelp";
import { getSubscriptionState } from "../../services/appBridge";
import { cancelChainProxyRefresh, deleteChainProxyItem, getChainProxyNodeText, getChainProxySource, getChainProxyState, importChainProxyNodes, refreshChainProxySources, saveChainProxyGroup, saveChainProxySource } from "../../services/chainProxyBridge";
import type { ChainProxyGroup, ChainProxyResult, ChainProxyState } from "../../types/chainProxy";
import { normalizationMessage } from "../subscription/normalization";
import { useConfigPageState } from "../local-config/ConfigPageStateContext";
import { ChainProxyEditors, type ChainEditor } from "./ChainProxyEditors";
import { ChainProxyGroupCard } from "./ChainProxyGroupCard";
import { chainProxyErrorKey } from "./model";
import "../../styles/chain-proxy.css";

export function ChainProxyWorkspace() {
  const { t } = useTranslation();
  const { modal } = App.useApp();
  const { syncSubscriptionState } = useProxyOverview();
  const [state, setState] = useState<ChainProxyState | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [editorError, setEditorError] = useState("");
  const [editor, setEditor] = useState<ChainEditor | null>(null);
  const [lastResult, setLastResult] = useState<ChainProxyResult | null>(null);
  const { expandedChainGroups: expanded, setExpandedChainGroups: setExpanded } = useConfigPageState();
  const alive = useRef(true);
  const network = useRef(false);
  useUnsavedChangesGuard(!!busy || !!editor && !(editor.kind === "text" && editor.readOnly), t("chainProxy.dirty"));
  const load = useCallback(async () => {
    setLoading(true);
    try { const next = await getChainProxyState(); if (alive.current) { setState(next); setError(""); } }
    catch (cause) { if (alive.current) setError(t(chainProxyErrorKey(cause))); }
    finally { if (alive.current) setLoading(false); }
  }, [t]);
  useEffect(() => {
    alive.current = true;
    void load();
    return () => { alive.current = false; if (network.current) void cancelChainProxyRefresh().catch(() => undefined); };
  }, [load]);

  const run = async (key: string, action: () => Promise<ChainProxyResult>, closeEditor = false) => {
    if (busy) return;
    setBusy(key); setError(""); setEditorError("");
    network.current = key === "refresh" || key === "source";
    try {
      const result = await action();
      if (!alive.current) return;
      const added = result.state.groups.filter(group => !state?.groups.some(previous => previous.id === group.id)).map(group => group.id);
      if (added.length) setExpanded(current => [...new Set([...current, ...added])]);
      setState(result.state); setLastResult(result);
      if (closeEditor) setEditor(null);
      try {
        const subscriptions = await getSubscriptionState();
        if (alive.current) syncSubscriptionState(subscriptions);
      } catch { if (alive.current) setError(t("subscription.errors.load")); }
    } catch (cause) {
      if (alive.current) (closeEditor ? setEditorError : setError)(t(chainProxyErrorKey(cause)));
    } finally { network.current = false; if (alive.current) setBusy(""); }
  };
  const openGroup = (kind: "manual" | "subscription", group?: ChainProxyGroup) => {
    if (!state) return;
    setEditorError(""); setEditor({ kind: "group", input: { revision: state.revision, id: group?.id ?? "", kind,
      name: group?.name ?? "", selectorFilter: group?.selectorFilter ?? "", nodeFilter: group?.nodeFilter ?? "" } });
  };
  const add = (group: ChainProxyGroup) => {
    if (!state) return;
    setEditorError("");
    setEditor(group.kind === "manual"
      ? { kind: "text", input: { revision: state.revision, groupId: group.id, nodeId: "", contents: "" } }
      : { kind: "source", input: { revision: state.revision, groupId: group.id, id: "", url: "", filter: "" } });
  };
  const openNode = async (groupId: string, nodeId: string, readOnly: boolean) => {
    if (!state || busy) return;
    setBusy("read"); setEditorError("");
    try { const contents = await getChainProxyNodeText(groupId, nodeId); if (alive.current) setEditor({ kind: "text", readOnly, input: { revision: state.revision, groupId, nodeId, contents } }); }
    catch (cause) { if (alive.current) setError(t(chainProxyErrorKey(cause))); }
    finally { if (alive.current) setBusy(""); }
  };
  const openSource = async (groupId: string, id: string) => {
    if (busy) return;
    setBusy("read"); setEditorError("");
    try { const input = await getChainProxySource(groupId, id); if (alive.current) setEditor({ kind: "source", input }); }
    catch (cause) { if (alive.current) setError(t(chainProxyErrorKey(cause))); }
    finally { if (alive.current) setBusy(""); }
  };
  const save = () => {
    if (!editor) return;
    if (editor.kind === "group") void run("group", () => saveChainProxyGroup(editor.input), true);
    else if (editor.kind === "source") void run("source", () => saveChainProxySource(editor.input), true);
    else void run("text", () => importChainProxyNodes(editor.input), true);
  };
  const remove = (groupId: string, kind: "group" | "node" | "source", id: string, name: string) => {
    if (!state) return;
    const revision = state.revision;
    modal.confirm({ title: t("chainProxy.deleteConfirm", { name }), content: t("chainProxy.deleteImpact"), okText: t("common.delete"), cancelText: t("common.cancel"), okButtonProps: { danger: true },
      onOk: () => run("delete", () => deleteChainProxyItem(groupId, kind, id, revision)) });
  };
  const disabled = !!busy || loading;
  return <div className="page-stack chain-proxy-workspace">
    <div className="chain-proxy-toolbar"><span className="chain-proxy-actions">
      <Dropdown disabled={disabled || !state} trigger={["click"]} menu={{ items: [{ key: "manual", label: t("chainProxy.manual") }, { key: "subscription", label: t("chainProxy.subscription") }], onClick: ({ key }) => openGroup(key as "manual" | "subscription") }}>
        <Button icon={<PlusOutlined />} disabled={disabled || !state}>{t("chainProxy.addGroup")}</Button>
      </Dropdown><FeatureHelp topic="chainProxy" />
    </span><span className="chain-proxy-actions">
      {network.current ? <Button onClick={() => void cancelChainProxyRefresh().catch(cause => setError(t(chainProxyErrorKey(cause))))}>{t("chainProxy.cancelRefresh")}</Button> :
        <Button icon={<ReloadOutlined />} disabled={disabled || !state?.groups.some(g => g.sources.length)} onClick={() => state && void run("refresh", () => refreshChainProxySources("", state.revision))}>{t("chainProxy.refreshAll")}</Button>}
    </span></div>
    {error && <Alert type="error" showIcon title={error} action={<Button onClick={() => void load()} disabled={!!busy}>{t("chainProxy.loadRetry")}</Button>} />}
    {lastResult?.failures.length ? <Alert type="warning" showIcon title={t("chainProxy.refreshFailed")} description={<ul>{lastResult.failures.map((item, index) => {
      const group = state?.groups.find(g => g.id === item.groupId);
      const source = group?.sources.find(s => s.id === item.sourceId);
      return <li key={index}>{group?.name}{source ? ` · ${t("chainProxy.sourceLabel", { index: group!.sources.indexOf(source) + 1, label: source.label })}` : ""}: {t(chainProxyErrorKey(`chain_proxy:${item.code}`))}</li>;
    })}</ul>} /> : null}
    {lastResult?.report && <Alert type="info" closable onClose={() => setLastResult(null)} title={t("chainProxy.importResult", { count: lastResult.report.proxyCount, skipped: lastResult.report.skippedNodes })}
      description={lastResult.report.diagnostics.map((item, index) => <div key={index}>{item.code === "chain_dialer_replaced" ? t("chainProxy.dialerReplaced") : normalizationMessage(t, item)}</div>)} />}
    {loading && <Spin />}
    {!loading && state?.groups.length === 0 && <Empty description={t("chainProxy.empty")} image={Empty.PRESENTED_IMAGE_SIMPLE} />}
    {state?.groups.map(group => <ChainProxyGroupCard key={group.id} group={group} disabled={disabled} expanded={expanded.includes(group.id)}
      onExpand={open => setExpanded(current => open ? [...new Set([...current, group.id])] : current.filter(id => id !== group.id))}
      onAdd={() => add(group)} onEdit={() => openGroup(group.kind, group)} onRefresh={() => void run("refresh", () => refreshChainProxySources(group.id, state.revision))}
      onDelete={(kind, id, name) => remove(group.id, kind, id, name)} onNode={(id, readOnly) => void openNode(group.id, id, readOnly)} onSource={id => void openSource(group.id, id)} />)}
    <ChainProxyEditors editor={editor} onChange={setEditor} onClose={() => !busy && setEditor(null)} onSave={save}
      onCancelRequest={() => void cancelChainProxyRefresh().catch(cause => setEditorError(t(chainProxyErrorKey(cause))))} busy={!!busy} error={editorError} />
  </div>;
}
