import { PlusOutlined } from "@ant-design/icons";
import { Button, Dropdown, Input, Modal, Switch, Tooltip } from "antd";
import type { MenuProps } from "antd";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import type { TextAreaRef } from "antd/es/input/TextArea";

import { FeatureHelp, type HelpTopic } from "../../components/help/FeatureHelp";
import { validateRuntimeYAMLFragment } from "../../services/appBridge";
import type {
  RuntimeMergeMode,
  RuntimeYamlField,
  RuntimeYamlFragmentValidation,
  RuntimeYamlValidationIssue,
} from "../../types/runtime";
import { RuntimeMergeControl } from "./RuntimePreferenceFields";
import { formatYamlStringSequence } from "./runtimeYaml";

interface RuntimeYamlResourceEditorProps {
  enabled?: boolean;
  field: RuntimeYamlField;
  helpTopic: HelpTopic;
  label: string;
  mergeMode?: RuntimeMergeMode;
  onCommit: (result: RuntimeYamlFragmentValidation) => void;
  onEnabledChange?: (enabled: boolean) => void;
  onMergeModeChange?: (mode: RuntimeMergeMode) => void;
  placeholder: string;
  presets?: Array<{ label: string; value: string }>;
  sourceText: string;
}

export function RuntimeYamlResourceEditor({
  enabled = true,
  field,
  helpTopic,
  label,
  mergeMode,
  onCommit,
  onEnabledChange,
  onMergeModeChange,
  placeholder,
  presets = [],
  sourceText,
}: RuntimeYamlResourceEditorProps) {
  const { t } = useTranslation();
  const [contents, setContents] = useState(sourceText);
  const [issue, setIssue] = useState<RuntimeYamlValidationIssue | null>(null);
  const [validating, setValidating] = useState(false);
  const committedText = useRef(sourceText);
  const presetActionStarted = useRef(false);
  const suppressNextBlur = useRef(false);
  const textAreaRef = useRef<TextAreaRef>(null);

  const preserveEditorCommitForAction = () => {
    // Let buttons, switches, and Segmented keep their native pointer/default
    // behavior. We only suppress the TextArea blur commit because the chosen
    // action validates and commits the same draft explicitly.
    suppressNextBlur.current = true;
    window.setTimeout(() => {
      suppressNextBlur.current = false;
    }, 0);
  };

  useEffect(() => {
    if (sourceText === committedText.current) return;
    committedText.current = sourceText;
    setContents(sourceText);
  }, [sourceText]);

  const validate = async (
    nextContents: string,
  ): Promise<RuntimeYamlFragmentValidation | null> => {
    setValidating(true);
    try {
      const result = await validateRuntimeYAMLFragment({
        contents: nextContents,
        field,
      });
      if (!result.valid) {
        setIssue(
          result.issue ?? {
            code: "runtime_yaml_invalid",
            column: 0,
            line: 0,
            message: t("home.runtimePreferences.yamlError.unknown"),
          },
        );
        return null;
      }
      return result;
    } catch (error) {
      setIssue({
        code: "runtime_yaml_unavailable",
        column: 0,
        line: 0,
        message:
          error instanceof Error
            ? error.message
            : t("home.runtimePreferences.yamlError.unavailable"),
      });
      return null;
    } finally {
      setValidating(false);
    }
  };

  const commit = async (force = false): Promise<boolean> => {
    if (!force && contents === committedText.current) return true;
    const result = await validate(contents);
    if (!result) return false;
    committedText.current = result.normalizedYaml;
    setContents(result.normalizedYaml);
    onCommit(result);
    return true;
  };

  const handleEnabledChange = async (nextEnabled: boolean) => {
    if (!onEnabledChange) return;
    if (!(await commit(nextEnabled))) return;
    onEnabledChange(nextEnabled);
  };

  const handleMergeModeChange = async (nextMode: RuntimeMergeMode) => {
    if (!onMergeModeChange) return;
    if (!(await commit())) return;
    onMergeModeChange(nextMode);
  };

  const handlePreset = async (value: string) => {
    const current = await validate(contents);
    if (!current) return;
    if (current.values.includes(value)) {
      if (contents !== committedText.current) {
        committedText.current = current.normalizedYaml;
        setContents(current.normalizedYaml);
        onCommit(current);
      }
      requestAnimationFrame(() => textAreaRef.current?.focus());
      return;
    }
    const candidate = formatYamlStringSequence([...current.values, value]);
    const result = await validate(candidate);
    if (!result) return;
    committedText.current = result.normalizedYaml;
    setContents(result.normalizedYaml);
    onCommit(result);
    requestAnimationFrame(() => textAreaRef.current?.focus());
  };

  const menu: MenuProps = {
    items: presets.map((preset) => ({ key: preset.value, label: preset.label })),
    onClick: ({ key }) => {
      presetActionStarted.current = true;
      void handlePreset(key);
    },
  };

  const returnToEditor = () => {
    const targetLine = issue?.line ?? 0;
    setIssue(null);
    requestAnimationFrame(() => {
      textAreaRef.current?.focus();
      const textArea = textAreaRef.current?.resizableTextArea?.textArea;
      if (!textArea || targetLine < 1) return;
      const offset = contents
        .split("\n")
        .slice(0, targetLine - 1)
        .reduce((total, line) => total + line.length + 1, 0);
      textArea.setSelectionRange(offset, offset);
    });
  };

  const revertContents = () => {
    setContents(committedText.current);
    setIssue(null);
  };

  return (
    <section
      className={`runtime-dns-resource-editor${enabled ? "" : " disabled"}`}
    >
      <div className="runtime-dns-resource-heading">
        <div className="runtime-dns-resource-title">
          <strong>{label}</strong>
          <FeatureHelp compact topic={helpTopic} />
          {onEnabledChange ? (
            <span onMouseDownCapture={preserveEditorCommitForAction}>
              <Switch
                aria-label={t("home.runtimePreferences.resourceEnabledAria", {
                  resource: label,
                })}
                checked={enabled}
                loading={validating}
                onChange={(nextEnabled) => void handleEnabledChange(nextEnabled)}
                size="small"
              />
            </span>
          ) : null}
        </div>

        {enabled && (presets.length > 0 || (mergeMode && onMergeModeChange)) ? (
          <div className="runtime-dns-resource-actions">
            {presets.length > 0 ? (
              <span onMouseDownCapture={preserveEditorCommitForAction}>
                <Dropdown
                  menu={menu}
                  onOpenChange={(open) => {
                    if (open) {
                      presetActionStarted.current = false;
                      suppressNextBlur.current = true;
                      return;
                    }
                    window.setTimeout(() => {
                      suppressNextBlur.current = false;
                      if (!presetActionStarted.current) void commit();
                      presetActionStarted.current = false;
                    }, 0);
                  }}
                  placement="bottomRight"
                  trigger={["click"]}
                >
                  <Tooltip title={t("home.runtimePreferences.quickAdd")}>
                    <Button
                      aria-label={`${label} ${t("home.runtimePreferences.quickAdd")}`}
                      icon={<PlusOutlined />}
                      loading={validating}
                      size="small"
                      type="text"
                    />
                  </Tooltip>
                </Dropdown>
              </span>
            ) : null}
            {mergeMode && onMergeModeChange ? (
              <span onMouseDownCapture={preserveEditorCommitForAction}>
                <RuntimeMergeControl
                  ariaLabel={`${label} ${t("home.runtimePreferences.mergeMode")}`}
                  disabled={validating}
                  onChange={(nextMode) => void handleMergeModeChange(nextMode)}
                  value={mergeMode}
                />
              </span>
            ) : null}
          </div>
        ) : null}
      </div>

      {enabled ? (
        <Input.TextArea
          aria-label={label}
          autoSize={{ minRows: 4, maxRows: 10 }}
          onBlur={() => {
            if (suppressNextBlur.current) return;
            void commit();
          }}
          onChange={(event) => setContents(event.target.value)}
          placeholder={placeholder}
          ref={textAreaRef}
          value={contents}
        />
      ) : null}

      <Modal
        centered
        closable={false}
        footer={[
          <Button key="revert" onClick={revertContents}>
            {t("home.runtimePreferences.yamlError.revert")}
          </Button>,
          <Button key="continue" onClick={returnToEditor} type="primary">
            {t("home.runtimePreferences.yamlError.continue")}
          </Button>,
        ]}
        mask={{ closable: false }}
        open={issue !== null}
        title={t("home.runtimePreferences.yamlError.title", { resource: label })}
      >
        {issue ? (
          <div className="runtime-yaml-error">
            {issue.line > 0 ? (
              <strong>
                {t("home.runtimePreferences.yamlError.location", {
                  column: issue.column || 1,
                  line: issue.line,
                })}
              </strong>
            ) : null}
            <pre>{issue.message}</pre>
          </div>
        ) : null}
      </Modal>
    </section>
  );
}
