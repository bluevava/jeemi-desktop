import {
  CloseOutlined,
  ExperimentOutlined,
  SaveOutlined,
} from "@ant-design/icons";
import { Alert, App, Button, Card, Input, Modal, Spin, Tag } from "antd";
import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router-dom";

import { useUnsavedChangesGuard } from "../../../app/navigationGuard/NavigationGuardContext";
import { FeatureHelp } from "../../../components/help/FeatureHelp";
import { errorText } from "../../../lib/errorText";
import {
  getLocalScript,
  getSubscriptionState,
  saveLocalScript,
  testLocalScript,
} from "../../../services/appBridge";
import type {
  LocalScript,
  LocalScriptTestResult,
  SaveLocalScriptInput,
} from "../../../types/localScript";
import { isScriptURL, scriptEditorInput } from "../scriptSource";

const defaultScript = `const main = (config) => {
  return config;
};
`;

const emptyDraft: SaveLocalScriptInput = {
  id: "",
  name: "",
  description: "",
  contents: defaultScript,
};

export function LocalScriptEditorPage() {
  const { t } = useTranslation();
  const { modal } = App.useApp();
  const navigate = useNavigate();
  const { id } = useParams<{ id: string }>();
  const [draft, setDraft] = useState<SaveLocalScriptInput>(emptyDraft);
  const [savedScript, setSavedScript] = useState<LocalScript | null>(null);
  const [initialSignature, setInitialSignature] = useState("");
  const [subscriptionID, setSubscriptionID] = useState("");
  const [subscriptionName, setSubscriptionName] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<"" | "save" | "test">("");
  const [error, setError] = useState("");
  const [testResult, setTestResult] = useState<LocalScriptTestResult | null>(
    null,
  );
  const signature = useMemo(() => JSON.stringify(draft), [draft]);
  const urlSource = isScriptURL(draft.contents);
  const dirty = initialSignature !== "" && signature !== initialSignature;
  const navigationGuard = useUnsavedChangesGuard(
    Boolean(busy) || dirty,
    t("localScript.editor.unsavedPrompt"),
  );

  useEffect(() => {
    let active = true;
    Promise.all([
      id ? getLocalScript(id) : Promise.resolve(undefined),
      getSubscriptionState().catch(() => ({
        selectedSubscriptionId: "",
        subscriptions: [],
      })),
    ])
      .then(([script, subscriptions]) => {
        if (!active) return;
        const next = script ? scriptEditorInput(script) : emptyDraft;
        setSavedScript(script ?? null);
        setDraft(next);
        setInitialSignature(JSON.stringify(next));
        setSubscriptionID(subscriptions.selectedSubscriptionId);
        setSubscriptionName(
          subscriptions.subscriptions.find(
            (item) => item.id === subscriptions.selectedSubscriptionId,
          )?.name ?? "",
        );
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
  }, [id]);

  const cancel = () => {
    if (!navigationGuard.canLeave()) return;
    navigationGuard.clear();
    navigate("/config?section=scripts");
  };

  const runTest = async () => {
    if (!subscriptionID) return;
    setBusy("test");
    setError("");
    try {
      setTestResult(
        await testLocalScript({ ...draft, subscriptionId: subscriptionID }),
      );
    } catch (cause) {
      setError(errorText(cause, t("common.unknownError")));
    } finally {
      setBusy("");
    }
  };

  const save = async () => {
    if (!draft.name.trim()) {
      setError(t("localScript.errors.nameRequired"));
      return;
    }
    setBusy("save");
    setError("");
    try {
      const saved = await saveLocalScript(draft);
      setDraft(scriptEditorInput(saved));
      navigationGuard.clear();
      navigate("/config?section=scripts");
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
          setDraft(JSON.parse(initialSignature) as SaveLocalScriptInput);
          setError("");
        },
      });
    } finally {
      setBusy("");
    }
  };

  if (loading) {
    return (
      <div className="local-config-loading">
        <Spin />
        <span>{t("localScript.editor.loading")}</span>
      </div>
    );
  }

  return (
    <div
      className="page-stack local-script-editor-page"
      inert={Boolean(busy)}
      aria-busy={Boolean(busy)}
    >
      {error ? (
        <Alert
          closable
          description={<span className="local-script-error-text">{error}</span>}
          onClose={() => setError("")}
          showIcon
          type="error"
        />
      ) : null}
      <Card className="local-config-metadata" variant="borderless">
        <div className="local-config-metadata-grid">
          <label>
            <span>{t("localScript.editor.name")}</span>
            <Input
              maxLength={80}
              onChange={(event) =>
                setDraft({ ...draft, name: event.target.value })
              }
              placeholder={t("localScript.editor.namePlaceholder")}
              value={draft.name}
            />
          </label>
          <label className="local-config-description-field">
            <span>{t("localScript.editor.description")}</span>
            <Input
              maxLength={500}
              onChange={(event) =>
                setDraft({ ...draft, description: event.target.value })
              }
              placeholder={t("localScript.editor.descriptionPlaceholder")}
              value={draft.description}
            />
          </label>
        </div>
      </Card>
      <Card className="local-script-code-card" variant="borderless">
        <div className="local-script-code-heading">
          <div>
            <span className="config-resource-title-line">
              <strong>{t("localScript.editor.source")}</strong>
              <FeatureHelp compact topic="localScript" />
              {urlSource ? <Tag>{t("localScript.editor.urlSource")}</Tag> : null}
            </span>
          </div>
          {subscriptionID ? (
            <Tag>
              {t("localScript.editor.testTarget", { name: subscriptionName })}
            </Tag>
          ) : null}
        </div>
        <Input.TextArea
          aria-label={t("localScript.editor.source")}
          autoCapitalize="off"
          autoCorrect="off"
          className={`local-script-source${urlSource ? " is-url" : ""}`}
          onChange={(event) =>
            setDraft({ ...draft, contents: event.target.value })
          }
          placeholder={t("localScript.editor.sourcePlaceholder")}
          rows={urlSource ? 4 : 22}
          spellCheck={false}
          value={draft.contents}
        />
        {urlSource ? <p className="local-script-source-hint">{t("localScript.editor.urlHint")}</p> : null}
        {savedScript?.sourceUrl && draft.contents.trim() === savedScript.sourceUrl ? (
          <details className="local-script-cached-source">
            <summary>{t("localScript.editor.cachedSource")}</summary>
            <pre>{savedScript.contents}</pre>
          </details>
        ) : null}
        {!subscriptionID ? (
          <Alert
            description={t("localScript.editor.noTestSubscription")}
            showIcon
            type="info"
          />
        ) : null}
      </Card>
      <nav
        aria-label={t("localScript.editor.actionBar")}
        className="local-config-action-wrap"
      >
        <div className="local-config-action-bar">
          <Button
            disabled={busy !== ""}
            icon={<CloseOutlined />}
            onClick={cancel}
          >
            {t("common.cancel")}
          </Button>
          <Button
            disabled={!subscriptionID || busy !== ""}
            icon={<ExperimentOutlined />}
            loading={busy === "test"}
            onClick={() => void runTest()}
          >
            {t("localScript.editor.test")}
          </Button>
          <Button
            disabled={busy !== ""}
            icon={<SaveOutlined />}
            loading={busy === "save"}
            onClick={() => void save()}
            type="primary"
          >
            {t("localScript.editor.save")}
          </Button>
        </div>
      </nav>
      <Modal
        footer={null}
        onCancel={() => setTestResult(null)}
        open={testResult !== null}
        title={t("localScript.test.title")}
        width={900}
      >
        {testResult ? (
          <div className="local-script-test-result">
            <div className="local-script-test-meta">
              <Tag color="success">{t("localScript.test.staticValidated")}</Tag>
              <Tag color={testResult.coreValidated ? "success" : "processing"}>
                {t(
                  testResult.coreValidated
                    ? "localScript.test.coreValidated"
                    : "localScript.test.corePending",
                )}
              </Tag>
              <span>{testResult.subscriptionName}</span>
            </div>
            <pre>{testResult.contents}</pre>
          </div>
        ) : null}
      </Modal>
    </div>
  );
}
