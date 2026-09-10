import { Alert, Button, Form, Input, Modal, Segmented } from "antd";
import { useTranslation } from "react-i18next";
import { FeatureHelp } from "../../components/help/FeatureHelp";
import type { ChainProxyGroupInput, ChainProxyImportInput, ChainProxySourceInput } from "../../types/chainProxy";

export type ChainEditor =
  | { kind: "group"; input: ChainProxyGroupInput }
  | { kind: "text"; input: ChainProxyImportInput; readOnly?: boolean }
  | { kind: "source"; input: ChainProxySourceInput };

export function ChainProxyEditors({ editor, onChange, onClose, onSave, onCancelRequest, busy, error }: {
  editor: ChainEditor | null; onChange: (editor: ChainEditor) => void;
  onClose: () => void; onSave: () => void; busy: boolean; error: string;
  onCancelRequest: () => void;
}) {
  const { t } = useTranslation();
  if (!editor) return null;
  const title = editor.kind === "group" ? (editor.input.id ? "editGroup" : "addGroup")
    : editor.kind === "source" ? (editor.input.id ? "editSource" : "addSource")
      : editor.readOnly ? "viewNode" : editor.input.nodeId ? "editNode" : "addNodes";
  const readOnly = editor.kind === "text" && editor.readOnly;
  return <Modal open width={760} title={t(`chainProxy.${title}`)} onCancel={onClose}
    styles={{ body: { maxHeight: "calc(100dvh - 240px)", overflowY: "auto" } }}
    onOk={onSave} confirmLoading={busy} closable={!busy} mask={{ closable: !busy }}
    cancelButtonProps={{ disabled: busy }} okText={t("common.save")} cancelText={t("common.cancel")}
    footer={readOnly ? null : editor.kind === "source" && busy ? <Button onClick={onCancelRequest}>{t("chainProxy.cancelRefresh")}</Button> : undefined}>
    <Form layout="vertical" disabled={busy}>
      {error && <Alert type="error" showIcon title={error} />}
      {editor.kind === "group" && <>
        <Form.Item label={t("chainProxy.name")} required><Input aria-label={t("chainProxy.name")} autoFocus maxLength={80} value={editor.input.name} onChange={e => onChange({ ...editor, input: { ...editor.input, name: e.target.value } })} /></Form.Item>
        <Form.Item label={t("chainProxy.kind")}><Segmented aria-label={t("chainProxy.kind")} disabled={busy || !!editor.input.id} value={editor.input.kind}
          options={[{ value: "manual", label: t("chainProxy.manual") }, { value: "subscription", label: t("chainProxy.subscription") }]}
          onChange={kind => onChange({ ...editor, input: { ...editor.input, kind: kind as "manual" | "subscription" } })} /></Form.Item>
        <div className="chain-proxy-filters">
          <Form.Item label={<>{t("chainProxy.selectorFilter")}<FeatureHelp topic="chainProxyFilter" compact /></>}>
            <Input aria-label={t("chainProxy.selectorFilter")} value={editor.input.selectorFilter} maxLength={512} placeholder={t("chainProxy.matchDefault")} onChange={e => onChange({ ...editor, input: { ...editor.input, selectorFilter: e.target.value } })} />
          </Form.Item>
          <Form.Item label={<>{t("chainProxy.nodeFilter")}<FeatureHelp topic="chainProxyFilter" compact /></>}>
            <Input aria-label={t("chainProxy.nodeFilter")} value={editor.input.nodeFilter} maxLength={512} placeholder={t("chainProxy.allNodes")} onChange={e => onChange({ ...editor, input: { ...editor.input, nodeFilter: e.target.value } })} />
          </Form.Item>
        </div>
      </>}
      {editor.kind === "source" && <>
        <Form.Item label={<>{t("chainProxy.url")}<FeatureHelp topic="chainProxyImport" compact /></>} required>
          <Input.Password aria-label={t("chainProxy.url")} autoFocus autoComplete="off" value={editor.input.url} maxLength={8192} onChange={e => onChange({ ...editor, input: { ...editor.input, url: e.target.value } })} />
        </Form.Item>
        <Form.Item label={<>{t("chainProxy.landingFilter")}<FeatureHelp topic="chainProxyFilter" compact /></>}>
          <Input aria-label={t("chainProxy.landingFilter")} maxLength={512} value={editor.input.filter} placeholder={t("chainProxy.allNodes")} onChange={e => onChange({ ...editor, input: { ...editor.input, filter: e.target.value } })} />
        </Form.Item>
      </>}
      {editor.kind === "text" && <Form.Item label={<>{t("chainProxy.text")}<FeatureHelp topic="chainProxyImport" compact /></>}>
        <Input.TextArea aria-label={t("chainProxy.text")} className="chain-proxy-text" spellCheck={false} autoFocus autoSize={{ minRows: 10, maxRows: 20 }} maxLength={4 * 1024 * 1024}
          readOnly={readOnly} value={editor.input.contents} placeholder={t("chainProxy.textPlaceholder")}
          onChange={e => onChange({ ...editor, input: { ...editor.input, contents: e.target.value } })} />
      </Form.Item>}
    </Form>
  </Modal>;
}
