import { CheckCircleFilled, ExclamationCircleOutlined, SafetyCertificateOutlined } from "@ant-design/icons";
import { Button, Modal, Spin, Tag } from "antd";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useWindowActivity } from "../../app/runtime/WindowActivityContext";
import { FeatureHelp } from "../../components/help/FeatureHelp";
import { cancelCoreAuthorization, getProxyAuthorization, openProxyAuthorizationSettings, setupProxyAuthorization } from "../../services/appBridge";
import { platformFailureMessageKey } from "../runtime/runtimePresentation";
import { AuthorizationFlow, idleAuthorization, type AuthorizationView } from "./authorizationFlow";

export function useProxyAuthorization() {
  const [view, setView] = useState<AuthorizationView>(idleAuthorization);
  const [flow] = useState(() => new AuthorizationFlow({ inspect: getProxyAuthorization, setup: setupProxyAuthorization, settings: openProxyAuthorizationSettings, cancel: cancelCoreAuthorization }, setView));
  const { isWindowVisible } = useWindowActivity();
  useEffect(() => () => flow.cancel(), [flow]);
  useEffect(() => {
    if (!view.open || view.busy || !isWindowVisible) return;
    const check = () => { void flow.check(); };
    window.addEventListener("focus", check);
    const timer = window.setInterval(check, 5000);
    return () => { window.removeEventListener("focus", check); window.clearInterval(timer); };
  }, [flow, view.open, view.busy, isWindowVisible]);
  return { flow, dialog: <ProxyAuthorizationDialog view={view} flow={flow} /> };
}

function ProxyAuthorizationDialog({ view, flow }: { view: AuthorizationView; flow: AuthorizationFlow }) {
  const { t } = useTranslation();
  const state = view.status;
  const reasonKey = state?.platform === "darwin"
    ? platformFailureMessageKey(`macos_network_${state.code}`)
    : state?.code === "linux_core_permission_required" ? "authorization.linuxRequired" : platformFailureMessageKey(state?.code ?? "");
  return <Modal open={view.open} centered width={560} className="proxy-authorization-dialog" onCancel={() => flow.cancel()} mask={{ closable: false }}
    title={<div className="authorization-title"><span className="authorization-icon"><SafetyCertificateOutlined /></span><span>{t("authorization.title")}</span><FeatureHelp compact topic="proxyAuthorization" /></div>}
    footer={<div className="authorization-footer"><Button disabled={view.busy} onClick={() => void flow.check()}>{t("authorization.check")}</Button><div><Button onClick={() => flow.cancel()}>{t("common.cancel")}</Button>{state?.action && <Button type="primary" loading={view.busy} onClick={() => void flow.perform()}>{t(`authorization.actions.${state.action}`)}</Button>}</div></div>}>
    <div className="authorization-body">
      {state && <div className="authorization-tags"><Tag>{state.platform === "darwin" ? "macOS" : state.platform === "windows" ? "Windows" : "Linux"}</Tag><Tag color={state.action === "repair" ? "warning" : undefined}>{t(`authorization.states.${state.action || "blocked"}`)}</Tag>{state.localTest && <Tag>{t("authorization.localTest")}</Tag>}</div>}
      <p className="authorization-reason" role="status">{t(view.errorKey ?? reasonKey ?? "authorization.inspectionFailed")}</p>
      <div className="authorization-checks">{state?.steps.map(step => <div className="authorization-check" key={step.id}><span>{t(state.platform === "darwin" ? `macNetwork.steps.${step.id}` : `authorization.steps.${step.id}`)}</span><span className={step.complete ? "authorization-check-ready" : "authorization-check-pending"}>{step.complete ? <CheckCircleFilled /> : <ExclamationCircleOutlined />}{t(step.complete ? "authorization.passed" : "authorization.pending")}</span></div>)}</div>
      {view.busy && <div className="authorization-working" role="status"><Spin size="small" />{t(state?.platform === "windows" ? "authorization.workingWindows" : "authorization.working")}</div>}
    </div>
  </Modal>;
}
