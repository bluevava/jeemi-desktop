import { createContext, useContext, useEffect, useRef, useState, type PropsWithChildren } from "react";
import { App, Button, Modal, Progress } from "antd";
import { useTranslation } from "react-i18next";

import { useWindowActivity } from "../../app/runtime/WindowActivityContext";
import { cancelJeemiUpdate, checkJeemiUpdates, dismissJeemiUpdateResult, getJeemiUpdateState, installJeemiUpdate, openJeemiReleases } from "../../services/appBridge";
import type { JeemiUpdateResult, JeemiUpdateState } from "../../types/appUpdate";
import { downloadPercent, updateErrorKey, updateInProgress } from "./updateState";

const initialState: JeemiUpdateState = { phase: "idle", version: "", downloaded: 0, total: 0, error: "", jobId: "" };
const UpdateContext = createContext<{ checking: boolean; busy: boolean; check: () => Promise<void> } | null>(null);

export function JeemiUpdateProvider({ children }: PropsWithChildren) {
  const { t } = useTranslation();
  const { message, modal } = App.useApp();
  const { isWindowVisible } = useWindowActivity();
  const [state, setState] = useState(initialState);
  const [checking, setChecking] = useState(false);
  const [installing, setInstalling] = useState(false);
  const [cancelling, setCancelling] = useState(false);
  const [candidate, setCandidate] = useState<JeemiUpdateResult | null>(null);
  const operation = useRef(false);
  const lastReceipt = useRef("");

  const showError = (error: unknown) => {
    modal.error({
      title: t("appUpdate.failed"),
      content: t(`appUpdate.errors.${updateErrorKey(error)}`),
      footer: (_, { OkBtn }) => <><Button onClick={() => { void openJeemiReleases().catch(() => message.error(t("home.about.repositoryError"))); }}>{t("appUpdate.releases")}</Button><OkBtn /></>,
    });
  };

  useEffect(() => {
    if (!isWindowVisible) return;
    let active = true;
    let reading = false;
    const refresh = async () => {
      if (reading) return;
      reading = true;
      try {
        const next = await getJeemiUpdateState();
        if (active) setState(next);
      } catch { /* A browser preview has no desktop updater. */ }
      finally { reading = false; }
    };
    void refresh();
    const timer = window.setInterval(() => void refresh(), 1000);
    return () => { active = false; window.clearInterval(timer); };
  }, [isWindowVisible]);

  useEffect(() => {
    if (operation.current || !state.jobId || lastReceipt.current === state.jobId || (state.phase !== "succeeded" && state.phase !== "failed")) return;
    lastReceipt.current = state.jobId;
    if (state.phase === "succeeded") void message.success(t("appUpdate.updated", { version: state.version }));
    else showError(state.error);
    void dismissJeemiUpdateResult(state.jobId).catch(() => undefined);
  }, [state, message, t]);

  const check = async () => {
    if (operation.current || updateInProgress(state.phase)) return;
    operation.current = true;
    setChecking(true);
    try {
      const result = await checkJeemiUpdates();
      if (result.updateAvailable) setCandidate(result);
      else void message.success(t("appUpdate.latest"));
    } catch (error) { showError(error); }
    finally { operation.current = false; setChecking(false); }
  };

  const install = async () => {
    if (!candidate || operation.current) return;
    operation.current = true;
    setInstalling(true);
    setCandidate(null);
    setState({ ...initialState, phase: "downloading", version: candidate.latestVersion });
    try { await installJeemiUpdate(candidate.latestVersion); }
    catch (error) {
      const code = updateErrorKey(error);
      if (code === "cancelled") void message.info(t("appUpdate.errors.cancelled"));
      else showError(error);
      try {
        const next = await getJeemiUpdateState();
        lastReceipt.current = next.jobId;
        setState(next);
      } catch { setState(initialState); }
    } finally { operation.current = false; setInstalling(false); setCancelling(false); }
  };

  const cancel = async () => {
    setCancelling(true);
    try { await cancelJeemiUpdate(); }
    catch (error) { setCancelling(false); showError(error); }
  };
  const progressOpen = installing || updateInProgress(state.phase);
  const checkingNow = checking || state.phase === "checking";
  const phase = updateInProgress(state.phase) ? state.phase : "downloading";
  return (
    <UpdateContext.Provider value={{ checking: checkingNow, busy: checkingNow || progressOpen || candidate !== null, check }}>
      {children}
      <Modal open={candidate !== null} title={t("appUpdate.available")} okText={t("appUpdate.install")} cancelText={t("common.cancel")} onOk={() => void install()} onCancel={() => setCandidate(null)}>
        <p>{t("appUpdate.versions", { current: candidate?.currentVersion, latest: candidate?.latestVersion })}</p>
        <p>{t("appUpdate.confirm")}</p>
      </Modal>
      <Modal open={progressOpen} title={t(`appUpdate.phases.${phase}`)} closable={false} maskClosable={false} keyboard={false} footer={phase === "restarting" ? null : <Button loading={cancelling} onClick={() => void cancel()}>{t("common.cancel")}</Button>}>
        <p>{t("appUpdate.progress", { version: state.version })}</p>
        <Progress percent={downloadPercent(state)} status="active" />
        <p>{t(`appUpdate.details.${phase}`)}</p>
      </Modal>
    </UpdateContext.Provider>
  );
}

export function useJeemiUpdate() {
  const value = useContext(UpdateContext);
  if (!value) throw new Error("JeemiUpdateProvider is unavailable");
  return value;
}
