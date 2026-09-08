import { useEffect, useState, type ReactNode } from "react";
import { Button, Descriptions, Modal, Tag } from "antd";
import { useTranslation } from "react-i18next";

import type { MihomoConnection } from "../../lib/mihomo/client";
import { RuleDocumentationButton } from "../../components/help/RuleDocumentationButton";
import { ConnectionRuleSetDialog } from "./ConnectionRuleSetDialog";
import { connectionRuleSeed, type ConnectionRuleSeed } from "./ruleEntryModel";
import { connectionTarget } from "./connectionFilters";
import {
  connectionEndpoint,
  connectionMatchedRule,
  connectionOutbound,
  connectionProcessName,
  formatConnectionBytes,
} from "./connectionPresentation";

export interface ConnectionDetailsDialogProps {
  connection: MihomoConnection | null;
  state: "active" | "ended" | "stale";
  onClose: () => void;
}

export function ConnectionDetailsDialog({
  connection,
  state,
  onClose,
}: ConnectionDetailsDialogProps) {
  const { t, i18n } = useTranslation();
  const [ruleSeed, setRuleSeed] = useState<ConnectionRuleSeed | null>(null);
  useEffect(() => setRuleSeed(null), [connection?.id]);
  const metadata = connection?.metadata;
  const startedAt = Date.parse(connection?.start ?? "");
  const ruleField = (
    value: ReactNode,
    field: "target" | "processName" | "processPath",
  ) => {
    const seed = connection ? connectionRuleSeed(connection, field) : null;
    return (
      <div className="connection-rule-field">
        <div className="connection-rule-field-value">{value}</div>
        <Button
          size="small"
          disabled={!seed?.values[seed.matchType]}
          onClick={() => setRuleSeed(seed)}
        >
          {t("ruleSetEntry.add")}
        </Button>
      </div>
    );
  };
  return (
    <>
      <Modal
        className="connection-details-dialog"
        footer={null}
        onCancel={onClose}
        open={connection !== null}
        keyboard={!ruleSeed}
        maskClosable={!ruleSeed}
        closable={!ruleSeed}
        title={
          <div className="rule-documentation-heading">
            <span>{t("connections.details.title")}</span>
            <RuleDocumentationButton />
          </div>
        }
        width={760}
      >
        {connection && metadata && (
          <div className="connection-details-body">
            <Tag color={state === "active" ? "success" : "default"}>
              {t(`connections.details.states.${state}`)}
            </Tag>
            <Descriptions
              bordered
              column={{ xs: 1, sm: 2 }}
              size="small"
              items={[
                {
                  key: "target",
                  label: t("connections.details.target"),
                  children: ruleField(connectionTarget(connection), "target"),
                  span: "filled",
                },
                {
                  key: "process",
                  label: t("connections.process"),
                  children: ruleField(
                    connectionProcessName(connection) ||
                      t("connections.unknown"),
                    "processName",
                  ),
                  span: "filled",
                },
                {
                  key: "path",
                  label: t("connections.details.processPath"),
                  children: ruleField(
                    <pre className="connection-process-path">
                      {metadata.processPath ||
                        t("connections.details.pathUnavailable")}
                    </pre>,
                    "processPath",
                  ),
                  span: "filled",
                },
                {
                  key: "source",
                  label: t("connections.details.source"),
                  children: connectionEndpoint(
                    metadata.sourceIP,
                    metadata.sourcePort,
                  ),
                  span: "filled",
                },
                {
                  key: "destination",
                  label: t("connections.details.destination"),
                  children: connectionEndpoint(
                    metadata.destinationIP,
                    metadata.destinationPort,
                  ),
                  span: "filled",
                },
                {
                  key: "network",
                  label: t("connections.details.network"),
                  children: metadata.network || "—",
                },
                {
                  key: "inbound",
                  label: t("connections.details.inbound"),
                  children: metadata.type || "—",
                },
                {
                  key: "rule",
                  label: t("connections.rule"),
                  children: connectionMatchedRule(connection),
                  span: "filled",
                },
                {
                  key: "ruleType",
                  label: t("connections.details.ruleType"),
                  children: connection.rule || "—",
                },
                {
                  key: "rulePayload",
                  label: t("connections.details.rulePayload"),
                  children: connection.rulePayload || "—",
                },
                {
                  key: "outbound",
                  label: t("connections.outbound"),
                  children: connectionOutbound(connection),
                  span: "filled",
                },
                {
                  key: "chain",
                  label: t("connections.details.chain"),
                  children: connection.chains.join(" → ") || "—",
                  span: "filled",
                },
                {
                  key: "upload",
                  label: t("connections.details.upload"),
                  children: formatConnectionBytes(
                    connection.upload,
                    i18n.language,
                  ),
                },
                {
                  key: "download",
                  label: t("connections.details.download"),
                  children: formatConnectionBytes(
                    connection.download,
                    i18n.language,
                  ),
                },
                {
                  key: "start",
                  label: t("connections.details.startedAt"),
                  children: Number.isFinite(startedAt)
                    ? new Date(startedAt).toLocaleString(i18n.language)
                    : "—",
                  span: "filled",
                },
                {
                  key: "id",
                  label: t("connections.details.id"),
                  children: connection.id,
                  span: "filled",
                },
              ]}
            />
          </div>
        )}
      </Modal>
      {connection && ruleSeed && (
        <ConnectionRuleSetDialog
          seed={ruleSeed}
          onClose={() => setRuleSeed(null)}
        />
      )}
    </>
  );
}
