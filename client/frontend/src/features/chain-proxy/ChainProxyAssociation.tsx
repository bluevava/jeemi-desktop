import { useEffect, useState } from "react";
import { Alert, Form, Modal, Select, Spin, Tag } from "antd";
import { useTranslation } from "react-i18next";
import { useUnsavedChangesGuard } from "../../app/navigationGuard/NavigationGuardContext";
import { FeatureHelp } from "../../components/help/FeatureHelp";
import { getChainProxyState, setSubscriptionChainProxyGroups } from "../../services/chainProxyBridge";
import type { ChainProxyState } from "../../types/chainProxy";
import type { SubscriptionState, SubscriptionSummary } from "../../types/subscription";
import { chainProxyErrorKey } from "./model";

export function ChainProxyAssociation({ target, onClose, onSaved }: { target: SubscriptionSummary; onClose: () => void; onSaved: (state: SubscriptionState) => void }) {
  const { t } = useTranslation();
  const [state, setState] = useState<ChainProxyState | null>(null);
  const [selected, setSelected] = useState(target.chainProxyGroupIds ?? []);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  useUnsavedChangesGuard(busy || JSON.stringify(selected) !== JSON.stringify(target.chainProxyGroupIds ?? []), t("chainProxy.dirty"));
  useEffect(() => {
    let active = true;
    void getChainProxyState().then(next => { if (active) setState(next); }).catch(cause => { if (active) setError(t(chainProxyErrorKey(cause))); });
    return () => { active = false; };
  }, [t]);
  const save = async () => {
    if (!state || busy) return;
    setBusy(true); setError("");
    try { onSaved(await setSubscriptionChainProxyGroups(target.id, selected, target.chainProxyRevision, state.revision)); onClose(); }
    catch (cause) { setError(t(chainProxyErrorKey(cause))); }
    finally { setBusy(false); }
  };
  const missing = selected.filter(id => !state?.groups.some(g => g.id === id));
  return <Modal open width={720} title={t("chainProxy.associationTitle", { name: target.name })} onCancel={() => !busy && onClose()} onOk={() => void save()}
    styles={{ body: { maxHeight: "calc(100dvh - 240px)", overflowY: "auto" } }}
    confirmLoading={busy} okButtonProps={{ disabled: !state }} cancelButtonProps={{ disabled: busy }} closable={!busy} okText={t("common.save")} cancelText={t("common.cancel")}>
    <div className="chain-proxy-association">
      {error && <Alert showIcon type="error" title={error} />}
      {!state && !error && <Spin />}
      {state && <>
        <Form layout="vertical"><Form.Item label={<>{t("chainProxy.associationGroups")}<FeatureHelp topic="chainProxy" compact /></>}>
          <Select aria-label={t("chainProxy.associationGroups")} mode="multiple" allowClear disabled={busy} value={selected} onChange={setSelected} optionFilterProp="label" placeholder={t("chainProxy.associationPlaceholder")}
            options={[...state.groups.map(g => ({ value: g.id, label: `${g.name} (${g.nodes.length})` })), ...missing.map(id => ({ value: id, label: t("chainProxy.missingGroup"), disabled: true }))]} />
        </Form.Item></Form>
        <p>{t("chainProxy.linkedNodes")}</p>
        {!state.groups.length && <Alert type="info" title={t("chainProxy.noGroups")} />}
        {state.groups.filter(g => selected.includes(g.id)).map(group => <div className="chain-proxy-association-group" key={group.id}>
          <strong>{group.name}</strong><div>{group.nodes.map(node => <Tag key={node.id}>{node.name}</Tag>)}</div>
        </div>)}
      </>}
    </div>
  </Modal>;
}
