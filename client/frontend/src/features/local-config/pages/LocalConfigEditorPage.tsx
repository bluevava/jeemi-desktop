import { useEffect, useMemo, useState, type Key, type ReactNode } from "react";
import {
  CheckOutlined,
  CloseOutlined,
  EyeInvisibleOutlined,
  EyeOutlined,
  FileSearchOutlined,
  LockOutlined,
  SaveOutlined,
} from "@ant-design/icons";
import {
  Alert,
  App,
  Collapse,
  ConfigProvider,
  Button,
  Card,
  Empty,
  Input,
  InputNumber,
  Modal,
  Select,
  Spin,
  Switch,
  Tag,
  Tree,
} from "antd";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router-dom";

import { useUnsavedChangesGuard } from "../../../app/navigationGuard/NavigationGuardContext";
import { errorText } from "../../../lib/errorText";
import { FeatureHelp } from "../../../components/help/FeatureHelp";
import {
  getConfigCatalog,
  getLocalConfig,
  getLocalConfigResources,
  previewLocalConfig,
  saveLocalConfig,
} from "../../../services/appBridge";
import type {
  ConfigCatalog,
  ConfigCatalogField,
  LocalConfigResourceState,
  LocalConfigField,
  LocalConfigPreview,
} from "../../../types/localConfig";
import {
  LocalConfigMatchEditor,
  LocalConfigRulesEditor,
} from "../components/LocalConfigResourcePlanEditor";
import {
  createLocalConfigDraft,
  configFieldDisplayName,
  configFieldTreeSegments,
  draftSignature,
  findDraftField,
  setFieldEnabled,
  shouldShowCatalogField,
  updateDraftField,
  type LocalConfigDraft,
} from "../model";

const virtualRulesPath = "virtual:rules.rules";
const virtualMatchPath = "virtual:rules.MATCH";

export function LocalConfigEditorPage() {
  const { t } = useTranslation();
  const { modal } = App.useApp();
  const navigate = useNavigate();
  const { id } = useParams<{ id: string }>();
  const [catalog, setCatalog] = useState<ConfigCatalog | null>(null);
  const [resources, setResources] = useState<LocalConfigResourceState | null>(
    null,
  );
  const [draft, setDraft] = useState<LocalConfigDraft>(
    createLocalConfigDraft(),
  );
  const [initialSignature, setInitialSignature] = useState("");
  const [selectedPath, setSelectedPath] = useState("");
  const [search, setSearch] = useState("");
  const [onlyEnabled, setOnlyEnabled] = useState(false);
  const [showLocked, setShowLocked] = useState(false);
  const [expandedKeys, setExpandedKeys] = useState<Key[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [errorKey, setErrorKey] = useState("");
  const [errorDetail, setErrorDetail] = useState("");
  const [preview, setPreview] = useState<LocalConfigPreview | null>(null);
  const [previewRevealed, setPreviewRevealed] = useState(false);
  const dirty =
    initialSignature !== "" && draftSignature(draft) !== initialSignature;
  const navigationGuard = useUnsavedChangesGuard(
    busy || dirty,
    t("localConfig.editor.unsavedPrompt"),
  );

  useEffect(() => {
    let active = true;
    const load = async () => {
      try {
        const [nextCatalog, nextResources, config] = await Promise.all([
          getConfigCatalog(),
          getLocalConfigResources(),
          id ? getLocalConfig(id) : Promise.resolve(undefined),
        ]);
        if (!active) {
          return;
        }
        const nextDraft = createLocalConfigDraft(config);
        setCatalog(nextCatalog);
        setResources(nextResources);
        setDraft(nextDraft);
        setInitialSignature(draftSignature(nextDraft));
        setSelectedPath(
          nextDraft.fields[0]?.path ?? firstEditablePath(nextCatalog),
        );
      } catch {
        if (active) {
          setErrorKey("localConfig.errors.load");
        }
      } finally {
        if (active) {
          setLoading(false);
        }
      }
    };
    void load();
    return () => {
      active = false;
    };
  }, [id]);

  const fields = useMemo(
    () => catalog?.categories.flatMap((category) => category.fields) ?? [],
    [catalog],
  );
  const definition = fields.find((field) => field.path === selectedPath);
  const draftField = findDraftField(draft, selectedPath);
  const enabledPaths = useMemo(
    () => new Set(draft.fields.map((field) => field.path)),
    [draft.fields],
  );
  const treeData = useMemo(() => {
    if (!catalog) {
      return [];
    }
    const needle = search.trim().toLocaleLowerCase();
    return catalog.categories
      .map((category) => {
        const categoryLabel = t(`localConfig.categories.${category.id}`);
        const visibleFields = category.fields.filter((field) => {
          if (
            !shouldShowCatalogField(
              field,
              enabledPaths,
              onlyEnabled,
              showLocked,
            )
          ) {
            return false;
          }
          return (
            needle === "" ||
            configFieldDisplayName(field.path)
              .toLocaleLowerCase()
              .includes(needle) ||
            field.id.toLocaleLowerCase().includes(needle) ||
            categoryLabel.toLocaleLowerCase().includes(needle)
          );
        });
        const virtualChildren =
          category.id === "routing"
            ? buildVirtualRuleNodes(
                needle,
                onlyEnabled,
                draft.resourcePlan.strategyGroupIds.length > 0 ||
                  draft.resourcePlan.defaultProxySelectorId !== "",
                showLocked,
              )
            : [];
        return {
          key: `category:${category.id}`,
          selectable: false,
          checkable: false,
          title: categoryLabel,
          children: [
            ...buildFieldTree(
              category.id,
              visibleFields,
              t("localConfig.editor.allProxyNodes"),
            ),
            ...virtualChildren,
          ],
        };
      })
      .filter((category) => category.children.length > 0);
  }, [
    catalog,
    draft.resourcePlan,
    enabledPaths,
    onlyEnabled,
    search,
    showLocked,
    t,
  ]);

  const visiblePaths = useMemo(
    () => new Set(collectSelectablePaths(treeData)),
    [treeData],
  );

  const firstVisiblePath = useMemo(
    () => firstSelectablePath(treeData),
    [treeData],
  );

  useEffect(() => {
    if (selectedPath && visiblePaths.has(selectedPath)) {
      return;
    }
    if (selectedPath !== firstVisiblePath) {
      setSelectedPath(firstVisiblePath);
    }
  }, [firstVisiblePath, selectedPath, visiblePaths]);

  const handleTreeCheck = (checkedValue: unknown) => {
    if (!catalog) {
      return;
    }
    const rawKeys = Array.isArray(checkedValue)
      ? checkedValue
      : ((checkedValue as { checked?: Key[] }).checked ?? []);
    const checkedPaths = new Set(
      rawKeys
        .map(String)
        .filter((key) => key.startsWith("field:"))
        .map((key) => key.replace("field:", "")),
    );
    setDraft((current) => {
      let next = current;
      for (const field of fields) {
        if (!visiblePaths.has(field.path) || field.locked) {
          continue;
        }
        next = setFieldEnabled(next, field, checkedPaths.has(field.path));
      }
      return next;
    });
  };

  const cancel = () => {
    if (navigationGuard.canLeave()) {
      navigationGuard.clear();
      navigate("/config?section=configs");
    }
  };

  const showPreview = async () => {
    setBusy(true);
    setErrorKey("");
    setErrorDetail("");
    try {
      setPreviewRevealed(false);
      setPreview(await previewLocalConfig(draft));
    } catch {
      setErrorKey("localConfig.errors.preview");
    } finally {
      setBusy(false);
    }
  };

  const save = async () => {
    setErrorDetail("");
    if (!draft.name.trim()) {
      setErrorKey("localConfig.errors.nameRequired");
      return;
    }
    setBusy(true);
    setErrorKey("");
    try {
      const saved = await saveLocalConfig(draft);
      const savedDraft = createLocalConfigDraft(saved);
      setDraft(savedDraft);
      setInitialSignature(draftSignature(savedDraft));
      navigationGuard.clear();
      navigate("/config?section=configs");
    } catch (cause) {
      const message = errorText(cause, t("localConfig.errors.save"));
      setErrorKey("localConfig.errors.save");
      setErrorDetail(message);
      modal.confirm({
        keyboard: false,
        title: t("localConfig.redesign.validationFailed"),
        content: message,
        okText: t("localConfig.redesign.keepEditing"),
        cancelText: t("localConfig.redesign.revertDraft"),
        onCancel: () => {
          setDraft(JSON.parse(initialSignature) as LocalConfigDraft);
          setErrorKey("");
          setErrorDetail("");
        },
      });
    } finally {
      setBusy(false);
    }
  };

  if (loading) {
    return (
      <div className="local-config-loading">
        <Spin />
        <span>{t("localConfig.editor.loading")}</span>
      </div>
    );
  }

  if (!catalog || !resources) {
    return (
      <Alert
        description={t(errorKey || "localConfig.errors.backendUnavailable")}
        showIcon
        type="error"
      />
    );
  }

  const category = catalog.categories.find((item) =>
    item.fields.some((field) => field.path === selectedPath),
  );

  return (
    <div
      className="page-stack local-config-editor-page"
      inert={busy}
      aria-busy={busy}
    >
      {errorKey ? (
        <Alert
          closable
          description={errorDetail || t(errorKey)}
          onClose={() => {
            setErrorKey("");
            setErrorDetail("");
          }}
          showIcon
          type="error"
        />
      ) : null}

      <ConfigProvider componentDisabled={busy}>
        <Card
          className="feature-card local-config-metadata"
          variant="borderless"
        >
          <div className="local-config-metadata-grid">
            <label>
              <span>{t("localConfig.editor.name")}</span>
              <Input
                maxLength={80}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    name: event.target.value,
                  }))
                }
                placeholder={t("localConfig.editor.namePlaceholder")}
                value={draft.name}
              />
            </label>
            <label className="local-config-description-field">
              <span>{t("localConfig.editor.description")}</span>
              <Input
                maxLength={500}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    description: event.target.value,
                  }))
                }
                placeholder={t("localConfig.editor.descriptionPlaceholder")}
                value={draft.description}
              />
            </label>
          </div>
        </Card>

        <Collapse
          className="local-config-sections"
          defaultActiveKey={["rules"]}
          items={[
            {
              key: "rules",
              label: (
                <span className="feature-title-with-help">
                  {t("localConfig.redesign.rulesConfig")}
                  <span onClick={(event) => event.stopPropagation()}>
                    <FeatureHelp
                      compact
                      translationBase="localConfig.fieldHelp.rules"
                    />
                  </span>
                </span>
              ),
              children: (
                <LocalConfigRulesEditor
                  resources={resources}
                  value={draft.resourcePlan}
                  onChange={(resourcePlan) =>
                    setDraft((current) => ({ ...current, resourcePlan }))
                  }
                />
              ),
            },
            {
              key: "fields",
              label: t("localConfig.redesign.customFields"),
              children: (
                <div className="local-config-workbench">
                  <Card
                    className="feature-card config-tree-card"
                    variant="borderless"
                  >
                    <div className="config-tree-toolbar">
                      <Input.Search
                        allowClear
                        onChange={(event) => setSearch(event.target.value)}
                        placeholder={t("localConfig.editor.search")}
                        value={search}
                      />
                      <div className="config-tree-filter-row">
                        <label>
                          <Switch
                            checked={onlyEnabled}
                            onChange={setOnlyEnabled}
                            size="small"
                          />
                          <span>{t("localConfig.editor.onlyEnabled")}</span>
                        </label>
                        <label>
                          <Switch
                            checked={showLocked}
                            onChange={setShowLocked}
                            size="small"
                          />
                          <span>{t("localConfig.editor.showLocked")}</span>
                        </label>
                      </div>
                    </div>
                    {treeData.length > 0 ? (
                      <Tree
                        checkable
                        checkedKeys={draft.fields.map(
                          (field) => `field:${field.path}`,
                        )}
                        expandedKeys={
                          search.trim()
                            ? collectExpandableKeys(treeData)
                            : expandedKeys
                        }
                        onCheck={handleTreeCheck}
                        onExpand={(keys) => {
                          if (!search.trim()) {
                            setExpandedKeys(keys);
                          }
                        }}
                        onSelect={(keys) => {
                          const key = String(keys[0] ?? "");
                          if (key.startsWith("field:")) {
                            setSelectedPath(key.replace("field:", ""));
                          } else if (key.startsWith("virtual:")) {
                            setSelectedPath(key);
                          }
                        }}
                        selectedKeys={
                          selectedPath
                            ? [
                                selectedPath.startsWith("virtual:")
                                  ? selectedPath
                                  : `field:${selectedPath}`,
                              ]
                            : []
                        }
                        treeData={treeData}
                      />
                    ) : (
                      <Empty
                        description={t("localConfig.editor.noSearchResults")}
                        image={Empty.PRESENTED_IMAGE_SIMPLE}
                      />
                    )}
                  </Card>

                  <Card
                    className="feature-card config-field-card"
                    variant="borderless"
                  >
                    {selectedPath === virtualRulesPath ? (
                      <Alert
                        showIcon
                        type="info"
                        title={t("localConfig.redesign.rulesLocked")}
                      />
                    ) : selectedPath === virtualMatchPath ? (
                      <LocalConfigMatchEditor />
                    ) : definition ? (
                      <ConfigFieldEditor
                        definition={definition}
                        documentationURL={
                          definition.documentationUrl ||
                          category?.documentationUrl ||
                          ""
                        }
                        field={draftField}
                        onEnable={(enabled) =>
                          setDraft((current) =>
                            setFieldEnabled(current, definition, enabled),
                          )
                        }
                        onUpdate={(update) =>
                          setDraft((current) =>
                            updateDraftField(current, definition.path, update),
                          )
                        }
                      />
                    ) : (
                      <Empty
                        description={t("localConfig.editor.selectField")}
                        image={Empty.PRESENTED_IMAGE_SIMPLE}
                      />
                    )}
                  </Card>
                </div>
              ),
            },
          ]}
        />
      </ConfigProvider>

      <nav
        aria-label={t("localConfig.editor.actionBar")}
        className="local-config-action-wrap"
      >
        <div className="local-config-action-bar">
          <Button
            disabled={busy}
            icon={<CloseOutlined />}
            onClick={cancel}
            size="large"
          >
            {t("common.cancel")}
          </Button>
          <Button
            disabled={busy}
            icon={<FileSearchOutlined />}
            onClick={() => void showPreview()}
            size="large"
          >
            {t("localConfig.editor.preview")}
          </Button>
          <Button
            icon={busy ? <CheckOutlined /> : <SaveOutlined />}
            loading={busy}
            onClick={() => void save()}
            size="large"
            type="primary"
          >
            {t("localConfig.editor.save")}
          </Button>
        </div>
      </nav>

      <Modal
        footer={null}
        onCancel={() => {
          setPreview(null);
          setPreviewRevealed(false);
        }}
        open={preview !== null}
        title={
          <span className="feature-title-with-help">
            {t("localConfig.preview.title")}
            <FeatureHelp compact topic="localConfigPreview" />
          </span>
        }
        width={760}
      >
        {preview ? (
          <div className="local-config-preview">
            <div className="local-config-preview-meta">
              <Tag>{preview.schemaVersion}</Tag>
              <span>
                {t("localConfig.preview.fieldCount", {
                  count: preview.enabledFieldCount,
                })}
              </span>
              {preview.proxyOverrideCount > 0 ? (
                <Tag>
                  {t("localConfig.preview.proxyOverrideCount", {
                    count: preview.proxyOverrideCount,
                  })}
                </Tag>
              ) : null}
              <Button
                icon={
                  previewRevealed ? <EyeInvisibleOutlined /> : <EyeOutlined />
                }
                onClick={() => setPreviewRevealed((current) => !current)}
                size="small"
                type="text"
              >
                {t(
                  previewRevealed
                    ? "localConfig.editor.hideSensitive"
                    : "localConfig.editor.revealSensitive",
                )}
              </Button>
            </div>
            {preview.proxyOverrideCount > 0 ? (
              <div className="local-config-transform-preview">
                <strong>{t("localConfig.preview.proxyOverrides")}</strong>
                {preview.fields
                  .filter((field) => field.path.includes("/*/"))
                  .map((field) => (
                    <code key={field.path}>
                      {configFieldDisplayName(field.path)}: {field.valueYaml}
                    </code>
                  ))}
              </div>
            ) : null}
            <pre>
              {previewRevealed
                ? preview.overlayYaml
                : preview.redactedOverlayYaml}
            </pre>
          </div>
        ) : null}
      </Modal>
    </div>
  );
}

interface ConfigFieldEditorProps {
  definition: ConfigCatalogField;
  documentationURL: string;
  field?: LocalConfigField;
  onEnable: (enabled: boolean) => void;
  onUpdate: (update: Partial<Omit<LocalConfigField, "path">>) => void;
}

function ConfigFieldEditor({
  definition,
  documentationURL,
  field,
  onEnable,
  onUpdate,
}: ConfigFieldEditorProps) {
  const { t } = useTranslation();
  const [revealed, setRevealed] = useState(false);
  const enabled = field !== undefined;
  const displayName = configFieldDisplayName(definition.path);

  return (
    <div className="config-field-editor">
      <div className="config-field-heading">
        <div>
          <div className="config-field-title-line">
            <code>{displayName}</code>
            {definition.locked ? (
              <Tag icon={<LockOutlined />}>
                {t("localConfig.editor.locked")}
              </Tag>
            ) : null}
            {definition.scope === "all_proxies" ? (
              <Tag>{t("localConfig.editor.proxyFieldOverride")}</Tag>
            ) : null}
            <FeatureHelp
              compact
              fallbackBase="localConfig.fieldHelp.generic"
              translationBase={`localConfig.fieldHelp.${definition.id}`}
              values={{ field: displayName }}
            />
          </div>
          <span>{t(`localConfig.kinds.${definition.kind}`)}</span>
        </div>
        {documentationURL ? (
          <Button
            href={documentationURL}
            rel="noreferrer"
            target="_blank"
            type="link"
          >
            {t("localConfig.editor.officialDocs")}
          </Button>
        ) : null}
      </div>

      {definition.locked ? (
        <Alert
          description={t("localConfig.editor.lockedDescription")}
          message={t("localConfig.editor.lockedTitle")}
          showIcon
          type="warning"
        />
      ) : (
        <>
          <div className="config-field-output-row">
            <div>
              <strong>{t("localConfig.editor.output")}</strong>
            </div>
            <Switch
              aria-label={t("localConfig.editor.output")}
              checked={enabled}
              onChange={onEnable}
            />
          </div>

          {enabled && field ? (
            <>
              <label className="config-field-control">
                <span>{t("localConfig.editor.mergeStrategy")}</span>
                <Select
                  onChange={(strategy) =>
                    onUpdate({
                      strategy,
                      conflictPolicy:
                        strategy === "merge_by_name"
                          ? field.conflictPolicy || "error"
                          : "",
                    })
                  }
                  options={definition.strategies.map((strategy) => ({
                    label: t(`localConfig.strategies.${strategy}`),
                    value: strategy,
                  }))}
                  value={field.strategy}
                />
              </label>

              {field.strategy === "merge_by_name" ? (
                <label className="config-field-control">
                  <span>{t("localConfig.editor.conflictPolicy")}</span>
                  <Select
                    onChange={(conflictPolicy) => onUpdate({ conflictPolicy })}
                    options={[
                      {
                        label: t("localConfig.conflicts.error"),
                        value: "error",
                      },
                      {
                        label: t("localConfig.conflicts.use_local"),
                        value: "use_local",
                      },
                    ]}
                    value={field.conflictPolicy || "error"}
                  />
                </label>
              ) : null}

              <div className="config-field-value-heading">
                <span>{t("localConfig.editor.localValue")}</span>
                {definition.sensitive ? (
                  <Button
                    icon={revealed ? <EyeInvisibleOutlined /> : <EyeOutlined />}
                    onClick={() => setRevealed((current) => !current)}
                    size="small"
                    type="text"
                  >
                    {t(
                      revealed
                        ? "localConfig.editor.hideSensitive"
                        : "localConfig.editor.revealSensitive",
                    )}
                  </Button>
                ) : null}
              </div>

              {definition.sensitive && !revealed ? (
                <div className="config-sensitive-placeholder">
                  <EyeInvisibleOutlined />
                  <span>{t("localConfig.editor.sensitiveHidden")}</span>
                </div>
              ) : (
                <FieldValueInput
                  definition={definition}
                  field={field}
                  onChange={(valueYaml) => onUpdate({ valueYaml })}
                />
              )}
            </>
          ) : (
            <Empty
              description={t("localConfig.editor.enableToEdit")}
              image={Empty.PRESENTED_IMAGE_SIMPLE}
            />
          )}
        </>
      )}
    </div>
  );
}

function FieldValueInput({
  definition,
  field,
  onChange,
}: {
  definition: ConfigCatalogField;
  field: LocalConfigField;
  onChange: (value: string) => void;
}) {
  if (definition.editor === "boolean") {
    return (
      <Switch
        checked={field.valueYaml.trim() === "true"}
        checkedChildren="true"
        onChange={(checked) => onChange(checked ? "true" : "false")}
        unCheckedChildren="false"
      />
    );
  }
  if (definition.editor === "number") {
    const numericValue = Number(field.valueYaml);
    return (
      <InputNumber
        onChange={(value) => onChange(value === null ? "" : String(value))}
        value={Number.isFinite(numericValue) ? numericValue : null}
      />
    );
  }
  if (definition.editor === "enum") {
    return (
      <Select
        onChange={onChange}
        options={definition.options.map((option) => ({
          label: option,
          value: option,
        }))}
        value={scalarDisplayValue(field.valueYaml)}
      />
    );
  }
  if (definition.editor === "string") {
    return (
      <Input
        onChange={(event) => onChange(JSON.stringify(event.target.value))}
        value={scalarDisplayValue(field.valueYaml)}
      />
    );
  }
  return (
    <Input.TextArea
      autoSize={{ minRows: 10, maxRows: 24 }}
      className="config-yaml-input"
      onChange={(event) => onChange(event.target.value)}
      spellCheck={false}
      value={field.valueYaml}
    />
  );
}

function scalarDisplayValue(value: string): string {
  const trimmed = value.trim();
  if (trimmed.startsWith('"') && trimmed.endsWith('"')) {
    try {
      return JSON.parse(trimmed) as string;
    } catch {
      return trimmed;
    }
  }
  return trimmed;
}

function firstEditablePath(catalog: ConfigCatalog): string {
  return (
    catalog.categories
      .flatMap((category) => category.fields)
      .find((field) => !field.locked && !field.hidden)?.path ?? ""
  );
}

interface ConfigTreeNode {
  key: string;
  title: ReactNode;
  selectable?: boolean;
  checkable?: boolean;
  disableCheckbox?: boolean;
  children?: ConfigTreeNode[];
}

function buildFieldTree(
  categoryID: string,
  fields: ConfigCatalogField[],
  allProxyNodesLabel: string,
): ConfigTreeNode[] {
  const roots: ConfigTreeNode[] = [];
  for (const field of fields) {
    const segments = configFieldTreeSegments(categoryID, field.path);
    let siblings = roots;
    let branchPath = categoryID;
    for (const segment of segments.slice(0, -1)) {
      branchPath += `/${segment}`;
      const branchKey = `branch:${branchPath}`;
      let branch = siblings.find((node) => node.key === branchKey);
      if (!branch) {
        branch = {
          key: branchKey,
          title: segment === "*" ? allProxyNodesLabel : segment,
          selectable: false,
          checkable: false,
          children: [],
        };
        siblings.push(branch);
      }
      siblings = branch.children ?? [];
    }
    const label = segments.at(-1) ?? configFieldDisplayName(field.path);
    siblings.push({
      key: `field:${field.path}`,
      disableCheckbox: field.locked,
      title: (
        <span className="config-tree-field-title">
          <span>{label}</span>
          {field.locked ? <LockOutlined /> : null}
        </span>
      ),
    });
  }
  return roots;
}

function collectSelectablePaths(nodes: ConfigTreeNode[]): string[] {
  return nodes.flatMap((node) => [
    ...(node.key.startsWith("field:")
      ? [node.key.replace("field:", "")]
      : node.key.startsWith("virtual:")
        ? [node.key]
        : []),
    ...collectSelectablePaths(node.children ?? []),
  ]);
}

function firstSelectablePath(nodes: ConfigTreeNode[]): string {
  for (const node of nodes) {
    if (node.key.startsWith("field:")) {
      return node.key.replace("field:", "");
    }
    if (node.key.startsWith("virtual:")) {
      return node.key;
    }
    const nested = firstSelectablePath(node.children ?? []);
    if (nested) {
      return nested;
    }
  }
  return "";
}

function buildVirtualRuleNodes(
  needle: string,
  onlyEnabled: boolean,
  rulesEnabled: boolean,
  showLocked: boolean,
): ConfigTreeNode[] {
  const children = [
    {
      key: virtualRulesPath,
      label: "rules.rules",
      enabled: rulesEnabled,
      locked: true,
    },
    {
      key: virtualMatchPath,
      label: "rules.MATCH",
      enabled: false,
      locked: true,
    },
  ]
    .filter(
      (item) =>
        (item.locked ? showLocked : !onlyEnabled || item.enabled) &&
        (needle === "" || item.label.toLocaleLowerCase().includes(needle)),
    )
    .map((item) => ({
      key: item.key,
      checkable: false,
      title: (
        <span className="config-tree-field-title">
          <span>{item.label}</span>
          {item.locked ? <LockOutlined /> : null}
        </span>
      ),
    }));
  if (children.length === 0) {
    return [];
  }
  return [
    {
      key: "branch:routing/rules-structured",
      title: "rules",
      selectable: false,
      checkable: false,
      children,
    },
  ];
}

function collectExpandableKeys(nodes: ConfigTreeNode[]): Key[] {
  return nodes.flatMap((node) => [
    ...(node.children?.length ? [node.key] : []),
    ...collectExpandableKeys(node.children ?? []),
  ]);
}
