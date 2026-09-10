import { DeleteOutlined, EditOutlined, EyeOutlined, MoreOutlined, PlusOutlined, ReloadOutlined } from "@ant-design/icons";
import { Button, Collapse, Dropdown, Empty, Tag, Tooltip } from "antd";
import { useTranslation } from "react-i18next";
import type { ChainProxyGroup } from "../../types/chainProxy";
import { normalizationMessage } from "../subscription/normalization";

export function ChainProxyGroupCard({ group, expanded, disabled, onExpand, onAdd, onEdit, onRefresh, onDelete, onNode, onSource }: {
  group: ChainProxyGroup; expanded: boolean; disabled: boolean;
  onExpand: (expanded: boolean) => void; onAdd: () => void; onEdit: () => void; onRefresh: () => void;
  onDelete: (kind: "group" | "source" | "node", id: string, name: string) => void;
  onNode: (id: string, readOnly: boolean) => void; onSource: (id: string) => void;
}) {
  const { t, i18n } = useTranslation();
  const nodes = (sourceId: string) => <div className="chain-proxy-node-grid">
    {group.nodes.filter(node => node.sourceId === sourceId).map(node => <article className="chain-proxy-node" key={node.id}>
      <strong title={node.name}>{node.name}</strong>
      <div className="chain-proxy-node-footer"><span>{node.type.toUpperCase()}</span><span>
        <Tooltip title={t(`chainProxy.${sourceId ? "viewNode" : "editNode"}`)}><Button type="text" size="small" disabled={disabled}
          aria-label={t(`chainProxy.${sourceId ? "viewNode" : "editNode"}`)} icon={sourceId ? <EyeOutlined /> : <EditOutlined />} onClick={() => onNode(node.id, !!sourceId)} /></Tooltip>
        {!sourceId && <Tooltip title={t("chainProxy.deleteNode")}><Button type="text" size="small" disabled={disabled} aria-label={t("chainProxy.deleteNode")}
          icon={<DeleteOutlined />} onClick={() => onDelete("node", node.id, node.name)} /></Tooltip>}
      </span></div>
    </article>)}
  </div>;
  return <Collapse className="chain-proxy-group" activeKey={expanded ? [group.id] : []} onChange={keys => onExpand(keys.includes(group.id))} items={[{
    key: group.id,
    label: <span className="chain-proxy-group-title"><strong>{group.name}</strong><Tag>{t(`chainProxy.${group.kind}`)}</Tag><span>{t("chainProxy.nodeCount", { count: group.nodes.length })}</span></span>,
    extra: <span className="chain-proxy-actions" onClick={event => event.stopPropagation()} onKeyDown={event => event.stopPropagation()}>
      <Tooltip title={t(`chainProxy.${group.kind === "manual" ? "addNodes" : "addSource"}`)}><Button type="text" size="small" disabled={disabled} icon={<PlusOutlined />} onClick={onAdd}
        aria-label={t(`chainProxy.${group.kind === "manual" ? "addNodes" : "addSource"}`)} /></Tooltip>
      {group.kind === "subscription" && <Tooltip title={t("chainProxy.refresh")}><Button type="text" size="small" disabled={disabled || !group.sources.length} icon={<ReloadOutlined />} onClick={onRefresh} aria-label={t("chainProxy.refresh")} /></Tooltip>}
      <Dropdown trigger={["click"]} disabled={disabled} menu={{ items: [
        { key: "edit", icon: <EditOutlined />, label: t("chainProxy.editGroup") },
        { key: "delete", danger: true, icon: <DeleteOutlined />, label: t("chainProxy.deleteGroup") },
      ], onClick: ({ key }) => key === "edit" ? onEdit() : onDelete("group", group.id, group.name) }}>
        <Button type="text" size="small" disabled={disabled} icon={<MoreOutlined />} aria-label={t("chainProxy.groupMenu", { name: group.name })} />
      </Dropdown>
    </span>,
    children: <div className="page-stack">
      <div className="chain-proxy-filters chain-proxy-filter-summary">
        <div><span>{t("chainProxy.selectorFilter")}</span><code>{group.selectorFilter || t("chainProxy.matchDefault")}</code></div>
        <div><span>{t("chainProxy.nodeFilter")}</span><code>{group.nodeFilter || t("chainProxy.allNodes")}</code></div>
      </div>
      {group.kind === "manual" && nodes("")}
      {group.sources.map((source, index) => <section className="chain-proxy-source" key={source.id}>
        <div className="chain-proxy-source-title"><strong>{t("chainProxy.sourceLabel", { index: index + 1, label: source.label })}</strong><span>{t("chainProxy.nodeCount", { count: source.nodeCount })}</span>
          <span className="chain-proxy-actions">
            <Tooltip title={t("chainProxy.editSource")}><Button disabled={disabled} type="text" size="small" icon={<EditOutlined />} onClick={() => onSource(source.id)} aria-label={t("chainProxy.editSource")} /></Tooltip>
            <Tooltip title={t("chainProxy.deleteSource")}><Button disabled={disabled} type="text" size="small" icon={<DeleteOutlined />} onClick={() => onDelete("source", source.id, source.label)} aria-label={t("chainProxy.deleteSource")} /></Tooltip>
          </span>
        </div>
        <div className="chain-proxy-source-meta"><span>{t("chainProxy.landingFilter")}: {source.filter || t("chainProxy.allNodes")}</span><time dateTime={source.updatedAt}>{t("chainProxy.updatedAt", { time: new Date(source.updatedAt).toLocaleString(i18n.language) })}</time></div>
        {!!source.report.diagnostics?.length && <details><summary>{t("chainProxy.normalizationDetails")}</summary>{source.report.diagnostics.map((item, index) => <p key={index}>{item.code === "chain_dialer_replaced" ? t("chainProxy.dialerReplaced") : normalizationMessage(t, item)}</p>)}</details>}
        {nodes(source.id)}
        {!source.nodeCount && <p className="chain-proxy-muted">{t("chainProxy.sourceEmpty")}</p>}
      </section>)}
      {!group.nodes.length && !group.sources.length && <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("chainProxy.emptyGroup")} />}
    </div>,
  }]} />;
}
