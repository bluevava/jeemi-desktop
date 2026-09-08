import { useEffect, useState } from "react";
import { Alert, App, Button, ConfigProvider, Spin } from "antd";
import { CloseOutlined, SaveOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router-dom";
import { useUnsavedChangesGuard } from "../../../app/navigationGuard/NavigationGuardContext";
import { errorText } from "../../../lib/errorText";
import {
  getLocalConfigResources,
  saveRuleSet,
  saveStrategyGroup,
} from "../../../services/appBridge";
import type {
  LocalConfigResourceState,
  RuleSetResource,
  StrategyGroupResource,
} from "../../../types/localConfig";
import {
  canSaveRuleSet,
  canSaveStrategyGroup,
  cloneRuleSet,
  cloneStrategyGroup,
  emptyRuleSet,
  emptyStrategyGroup,
} from "../resourceModel";
import { StrategyGroupForm } from "../components/StrategyGroupForm";
import { RuleSetForm } from "../components/RuleSetForm";

export function LocalResourceEditorPage({
  resource,
  kind = "selector",
}: {
  resource: "group" | "rule-set";
  kind?: "rule" | "selector";
}) {
  const { id } = useParams<{ id: string }>();
  const { t } = useTranslation();
  const { modal } = App.useApp();
  const navigate = useNavigate();
  const [state, setState] = useState<LocalConfigResourceState | null>(null);
  const [draft, setDraft] = useState<
    StrategyGroupResource | RuleSetResource | null
  >(null);
  const [initial, setInitial] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const guard = useUnsavedChangesGuard(
    busy || Boolean(draft && initial && JSON.stringify(draft) !== initial),
    t("localConfig.editor.unsavedPrompt"),
  );
  useEffect(() => {
    let active = true;
    setLoading(true);
    void getLocalConfigResources()
      .then((next) => {
        if (!active) return;
        const item =
          resource === "group"
            ? next.strategyGroups.find((value) => value.id === id)
            : next.ruleSets.find((value) => value.id === id);
        if (id && !item) throw new Error(t("localConfig.errors.load"));
        const value = item
          ? resource === "group"
            ? cloneStrategyGroup(item as StrategyGroupResource)
            : cloneRuleSet(item as RuleSetResource)
          : resource === "group"
            ? emptyStrategyGroup(kind)
            : emptyRuleSet();
        setState(next);
        setDraft(value);
        setInitial(JSON.stringify(value));
      })
      .catch((cause) => {
        if (active) setError(errorText(cause, t("common.unknownError")));
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [id, resource, kind]);
  const cancel = () => {
    if (!busy && guard.canLeave()) {
      guard.clear();
      navigate(
        resource === "group"
          ? "/config?section=groups"
          : "/config?section=ruleSets",
      );
    }
  };
  const save = async () => {
    if (!draft || busy) return;
    setBusy(true);
    setError("");
    try {
      if (resource === "group")
        await saveStrategyGroup(draft as StrategyGroupResource);
      else await saveRuleSet(draft as RuleSetResource);
      guard.clear();
      navigate(
        resource === "group"
          ? "/config?section=groups"
          : "/config?section=ruleSets",
      );
    } catch (cause) {
      const message = errorText(cause, t("common.unknownError"));
      setError(message);
      modal.confirm({
        keyboard: false,
        title: t("localConfig.redesign.validationFailed"),
        content: message,
        okText: t("localConfig.redesign.keepEditing"),
        cancelText: t("localConfig.redesign.revertDraft"),
        onCancel: () => {
          setDraft(JSON.parse(initial) as typeof draft);
          setError("");
        },
      });
    } finally {
      setBusy(false);
    }
  };
  if (loading)
    return (
      <div className="local-config-loading">
        <Spin />
      </div>
    );
  if (!draft || !state)
    return (
      <Alert
        type="error"
        showIcon
        description={error || t("localConfig.errors.load")}
      />
    );
  const canSave =
    resource === "group"
      ? canSaveStrategyGroup(draft as StrategyGroupResource, state)
      : canSaveRuleSet(draft as RuleSetResource);
  return (
    <div
      className="page-stack local-resource-editor-page"
      inert={busy}
      aria-busy={busy}
    >
      {error ? (
        <Alert
          type="error"
          showIcon
          closable
          description={error}
          onClose={() => setError("")}
        />
      ) : null}
      <ConfigProvider componentDisabled={busy}>
        {resource === "group" ? (
          <StrategyGroupForm
            key={initial}
            draft={draft as StrategyGroupResource}
            setDraft={setDraft}
            state={state}
          />
        ) : (
          <RuleSetForm draft={draft as RuleSetResource} setDraft={setDraft} />
        )}
      </ConfigProvider>
      <nav
        className="local-config-action-wrap"
        aria-label={t("localConfig.editor.actionBar")}
      >
        <div className="local-config-action-bar">
          <Button
            size="large"
            disabled={busy}
            icon={<CloseOutlined />}
            onClick={cancel}
          >
            {t("common.cancel")}
          </Button>
          <Button
            size="large"
            type="primary"
            disabled={!canSave}
            loading={busy}
            icon={<SaveOutlined />}
            onClick={() => void save()}
          >
            {t("common.save")}
          </Button>
        </div>
      </nav>
    </div>
  );
}
