import { useMemo, useState } from "react";
import { Alert, Button, Input, Modal, Select, Space, Table, Tabs } from "antd";
import {
  ArrowDownOutlined,
  ArrowUpOutlined,
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
} from "@ant-design/icons";
import { useTranslation } from "react-i18next";
import {
  editRulePayload,
  parseRulePayload,
  splitClassicalRule,
  type RulePayloadEdit,
} from "../rulePayload";
import type { RuleSetResource } from "../../../types/localConfig";

const types = [
  "DOMAIN",
  "DOMAIN-SUFFIX",
  "DOMAIN-KEYWORD",
  "DOMAIN-WILDCARD",
  "DOMAIN-REGEX",
  "IP-CIDR",
  "IP-CIDR6",
  "GEOSITE",
  "GEOIP",
  "PROCESS-NAME",
  "PROCESS-PATH",
  "PROCESS-NAME-REGEX",
  "PROCESS-PATH-REGEX",
  "DST-PORT",
  "NETWORK",
  "AND",
  "OR",
  "NOT",
];

export function RulePayloadEditor({
  value,
  behavior,
  onChange,
}: {
  value: string;
  behavior: RuleSetResource["behavior"];
  onChange: (value: string) => void;
}) {
  const { t } = useTranslation();
  const [search, setSearch] = useState("");
  const [editing, setEditing] = useState<{
    index?: number;
    type: string;
    value: string;
    source: string;
  } | null>(null);
  const [failed, setFailed] = useState(false);
  const parsed = useMemo(() => {
    try {
      return { rules: parseRulePayload(value).rules, invalid: false };
    } catch {
      return { rules: [], invalid: true };
    }
  }, [value]);
  const rows = useMemo(
    () =>
      parsed.rules
        .map((rule, index) => ({ rule, index }))
        .filter(({ rule }) =>
          rule.toLowerCase().includes(search.toLowerCase()),
        ),
    [parsed.rules, search],
  );
  const mutate = (edit: RulePayloadEdit) => {
    try {
      onChange(editRulePayload(value, edit));
      setFailed(false);
      return true;
    } catch {
      setFailed(true);
      return false;
    }
  };
  const begin = (index?: number) => {
    const rule = index === undefined ? "" : parsed.rules[index];
    const parts =
      behavior === "classical"
        ? splitClassicalRule(rule)
        : { type: "", value: rule };
    setEditing({ index, ...parts, source: value });
    setFailed(false);
  };
  return (
    <>
      <Tabs
        items={[
          {
            key: "visual",
            label: t("offlineConfig.visual"),
            children: (
              <div className="config-resource-form">
                {parsed.invalid || failed ? (
                  <Alert
                    type="error"
                    title={t("offlineConfig.invalidPayload")}
                  />
                ) : null}
                <Space wrap>
                  <Input.Search
                    aria-label={t("offlineConfig.search")}
                    placeholder={t("offlineConfig.search")}
                    value={search}
                    onChange={(event) => setSearch(event.target.value)}
                    allowClear
                  />
                  <Button
                    icon={<PlusOutlined />}
                    disabled={parsed.invalid || parsed.rules.length >= 20000}
                    onClick={() => begin()}
                  >
                    {t("offlineConfig.addRule")}
                  </Button>
                  <span>
                    {t("offlineConfig.ruleCount", {
                      count: parsed.rules.length,
                    })}
                  </span>
                </Space>
                <Table
                  size="small"
                  rowKey="index"
                  dataSource={rows}
                  pagination={{ defaultPageSize: 30, showSizeChanger: false }}
                  scroll={{ x: 400 }}
                  columns={[
                    {
                      title: "#",
                      width: 54,
                      render: (_, row) => row.index + 1,
                    },
                    {
                      title: t("offlineConfig.rule"),
                      dataIndex: "rule",
                      render: (rule: string) => (
                        <span
                          style={{
                            overflowWrap: "anywhere",
                            whiteSpace: "pre-wrap",
                          }}
                        >
                          {rule}
                        </span>
                      ),
                    },
                    {
                      title: t("offlineConfig.actions"),
                      width: 148,
                      render: (_, row) => (
                        <Space size={0}>
                          <Button
                            type="text"
                            icon={<EditOutlined />}
                            aria-label={t("offlineConfig.editRule")}
                            onClick={() => begin(row.index)}
                          />
                          <Button
                            type="text"
                            icon={<ArrowUpOutlined />}
                            disabled={row.index === 0}
                            aria-label={t("offlineConfig.moveUp")}
                            onClick={() =>
                              mutate({
                                type: "move",
                                index: row.index,
                                to: row.index - 1,
                              })
                            }
                          />
                          <Button
                            type="text"
                            icon={<ArrowDownOutlined />}
                            disabled={row.index === parsed.rules.length - 1}
                            aria-label={t("offlineConfig.moveDown")}
                            onClick={() =>
                              mutate({
                                type: "move",
                                index: row.index,
                                to: row.index + 1,
                              })
                            }
                          />
                          <Button
                            danger
                            type="text"
                            icon={<DeleteOutlined />}
                            aria-label={t("offlineConfig.deleteRule")}
                            onClick={() =>
                              mutate({ type: "delete", index: row.index })
                            }
                          />
                        </Space>
                      ),
                    },
                  ]}
                />
              </div>
            ),
          },
          {
            key: "yaml",
            label: t("offlineConfig.source"),
            children: (
              <Input.TextArea
                className="config-yaml-input"
                aria-label={t("localConfig.resources.ruleSets.payload")}
                spellCheck={false}
                autoSize={{ minRows: 18, maxRows: 32 }}
                value={value}
                onChange={(event) => onChange(event.target.value)}
              />
            ),
          },
        ]}
      />
      <Modal
        open={editing !== null}
        title={t(
          editing?.index === undefined
            ? "offlineConfig.addRule"
            : "offlineConfig.editRule",
        )}
        onCancel={() => setEditing(null)}
        okText={t("common.save")}
        cancelText={t("common.cancel")}
        okButtonProps={{
          disabled: !editing?.value.trim() || editing.source !== value,
        }}
        onOk={() => {
          if (!editing || editing.source !== value) return;
          const rule =
            behavior === "classical"
              ? `${editing.type},${editing.value}`
              : editing.value;
          if (
            mutate(
              editing.index === undefined
                ? { type: "add", value: rule }
                : { type: "edit", index: editing.index, value: rule },
            )
          )
            setEditing(null);
        }}
      >
        <div className="config-resource-form">
          {behavior === "classical" ? (
            <label>
              <span>{t("offlineConfig.ruleType")}</span>
              <Select
                value={editing?.type}
                options={[
                  ...new Set([...types, editing?.type || "DOMAIN-SUFFIX"]),
                ].map((type) => ({ value: type }))}
                onChange={(type) => editing && setEditing({ ...editing, type })}
              />
            </label>
          ) : null}
          <label>
            <span>{t("offlineConfig.matchValue")}</span>
            <Input
              aria-label={t("offlineConfig.matchValue")}
              value={editing?.value || ""}
              onChange={(event) =>
                editing && setEditing({ ...editing, value: event.target.value })
              }
            />
          </label>
          <small>{t(`offlineConfig.hints.${behavior}`)}</small>
          {failed ? (
            <Alert type="error" title={t("offlineConfig.invalidPayload")} />
          ) : null}
        </div>
      </Modal>
    </>
  );
}
