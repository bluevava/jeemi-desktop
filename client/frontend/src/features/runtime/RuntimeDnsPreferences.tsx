import {
  EditOutlined,
  RedoOutlined,
  SafetyCertificateOutlined,
} from "@ant-design/icons";
import { Alert, Button, Input, Modal, Select, Switch, Tag } from "antd";
import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import type { TextAreaRef } from "antd/es/input/TextArea";

import { FeatureHelp } from "../../components/help/FeatureHelp";
import {
  getDefaultLANBypassRules,
  validateRuntimeYAMLFragment,
} from "../../services/appBridge";
import type {
  DnsEnhancedMode,
  RuntimePreferences,
  RuntimeYamlFragmentValidation,
  RuntimeYamlValidationIssue,
} from "../../types/runtime";
import {
  RuntimePreferenceItem,
  RuntimePreferenceSection,
} from "./RuntimePreferenceFields";
import { RuntimeYamlResourceEditor } from "./RuntimeYamlResourceEditor";
import { formatYamlStringSequence } from "./runtimeYaml";

const commonResolverOptions = [
  "114.114.114.114",
  "223.5.5.5",
  "https://dns.alidns.com/dns-query",
  "https://doh.pub/dns-query",
  "https://1.1.1.1/dns-query",
  "https://dns.google/dns-query",
].map((value) => ({ label: value, value }));

const commonFakeIPFilterOptions = [
  "+.lan",
  "+.local",
  "+.home.arpa",
  "+.localdomain",
  "+.localhost",
  "+.msftconnecttest.com",
  "+.msftncsi.com",
].map((value) => ({ label: value, value }));

export function RuntimeDnsPreferences({
  draft,
  updateDraft,
}: {
  draft: RuntimePreferences;
  updateDraft: (update: Partial<RuntimePreferences>) => void;
}) {
  const { t } = useTranslation();

  return (
    <>
      <RuntimePreferenceSection
        helpTopic="runtimeDns"
        title={t("home.runtimePreferences.sections.dns")}
      >
        <div className="runtime-preferences-grid">
          <RuntimePreferenceItem title={t("home.runtimePreferences.dnsEnabled")}>
          <Switch
            aria-label={t("home.runtimePreferences.dnsEnabled")}
            checked={draft.dnsEnabled}
              checkedChildren={t("runtime.on")}
              onChange={(dnsEnabled) => updateDraft({ dnsEnabled })}
              unCheckedChildren={t("runtime.off")}
            />
          </RuntimePreferenceItem>

          <RuntimePreferenceItem title={t("home.runtimePreferences.dnsIpv6")}>
          <Switch
            aria-label={t("home.runtimePreferences.dnsIpv6")}
            checked={draft.dnsIpv6}
              checkedChildren={t("runtime.on")}
              onChange={(dnsIpv6) => updateDraft({ dnsIpv6 })}
              unCheckedChildren={t("runtime.off")}
            />
          </RuntimePreferenceItem>

          <RuntimePreferenceItem title={t("home.runtimePreferences.dnsListen")}>
            <Input
              aria-label={t("home.runtimePreferences.dnsListen")}
              onChange={(event) =>
                updateDraft({ dnsListen: event.target.value })
              }
              placeholder="127.0.0.1:1053"
              value={draft.dnsListen}
            />
          </RuntimePreferenceItem>

          <RuntimePreferenceItem
            title={t("home.runtimePreferences.dnsEnhancedMode")}
          >
            <Select
              aria-label={t("home.runtimePreferences.dnsEnhancedMode")}
              onChange={(dnsEnhancedMode: DnsEnhancedMode) =>
                updateDraft({ dnsEnhancedMode })
              }
              options={(["fake-ip", "redir-host"] as const).map((value) => ({
                label: t(
                  `home.runtimePreferences.dnsEnhancedModes.${value}`,
                ),
                value,
              }))}
              value={draft.dnsEnhancedMode}
            />
          </RuntimePreferenceItem>

          <RuntimePreferenceItem
            title={t("home.runtimePreferences.dnsFakeIpRange")}
          >
            <Input
              aria-label={t("home.runtimePreferences.dnsFakeIpRange")}
              onChange={(event) =>
                updateDraft({ dnsFakeIpRange: event.target.value })
              }
              value={draft.dnsFakeIpRange}
            />
          </RuntimePreferenceItem>

          <RuntimePreferenceItem
            title={t("home.runtimePreferences.dnsFakeIpRange6")}
          >
            <Input
              aria-label={t("home.runtimePreferences.dnsFakeIpRange6")}
              onChange={(event) =>
                updateDraft({ dnsFakeIpRange6: event.target.value })
              }
              value={draft.dnsFakeIpRange6}
            />
          </RuntimePreferenceItem>
        </div>
      </RuntimePreferenceSection>

      <RuntimePreferenceSection
        helpTopic="runtimeDnsMerge"
        title={t("home.runtimePreferences.sections.dnsResources")}
      >
        <div className="runtime-dns-resources-grid">
          <RuntimeYamlResourceEditor
            enabled={draft.dnsNameserverEnabled}
            field="dns.nameserver"
            helpTopic="runtimeDnsResolvers"
            label={t("home.runtimePreferences.dnsNameservers")}
            mergeMode={draft.dnsNameserverMerge}
            onCommit={(result) =>
              updateDraft({ dnsNameservers: result.values })
            }
            onEnabledChange={(dnsNameserverEnabled) =>
              updateDraft({ dnsNameserverEnabled })
            }
            onMergeModeChange={(dnsNameserverMerge) =>
              updateDraft({ dnsNameserverMerge })
            }
            placeholder={t("home.runtimePreferences.resolverPlaceholder")}
            presets={commonResolverOptions}
            sourceText={formatYamlStringSequence(draft.dnsNameservers)}
          />

          <RuntimeYamlResourceEditor
            enabled={draft.dnsFakeIpFilterEnabled}
            field="dns.fake-ip-filter"
            helpTopic="runtimeFakeIpFilter"
            label={t("home.runtimePreferences.dnsFakeIpFilter")}
            mergeMode={draft.dnsFakeIpFilterMerge}
            onCommit={(result) =>
              updateDraft({ dnsFakeIpFilter: result.values })
            }
            onEnabledChange={(dnsFakeIpFilterEnabled) =>
              updateDraft({ dnsFakeIpFilterEnabled })
            }
            onMergeModeChange={(dnsFakeIpFilterMerge) =>
              updateDraft({ dnsFakeIpFilterMerge })
            }
            placeholder={t("home.runtimePreferences.filterPlaceholder")}
            presets={commonFakeIPFilterOptions}
            sourceText={formatYamlStringSequence(draft.dnsFakeIpFilter)}
          />

          <RuntimeYamlResourceEditor
            enabled={draft.dnsProxyServerNameserverEnabled}
            field="dns.proxy-server-nameserver"
            helpTopic="runtimeDnsResolvers"
            label={t("home.runtimePreferences.dnsProxyServerNameservers")}
            mergeMode={draft.dnsProxyServerNameserverMerge}
            onCommit={(result) =>
              updateDraft({ dnsProxyServerNameservers: result.values })
            }
            onEnabledChange={(dnsProxyServerNameserverEnabled) =>
              updateDraft({ dnsProxyServerNameserverEnabled })
            }
            onMergeModeChange={(dnsProxyServerNameserverMerge) =>
              updateDraft({ dnsProxyServerNameserverMerge })
            }
            placeholder={t("home.runtimePreferences.resolverPlaceholder")}
            presets={commonResolverOptions}
            sourceText={formatYamlStringSequence(
              draft.dnsProxyServerNameservers,
            )}
          />

          <RuntimeYamlResourceEditor
            enabled={draft.dnsNameserverPolicyEnabled}
            field="dns.nameserver-policy"
            helpTopic="runtimeDnsPolicies"
            label={t("home.runtimePreferences.dnsNameserverPolicy")}
            mergeMode={draft.dnsNameserverPolicyMerge}
            onCommit={(result) =>
              updateDraft({ dnsNameserverPolicyYaml: result.normalizedYaml })
            }
            onEnabledChange={(dnsNameserverPolicyEnabled) =>
              updateDraft({ dnsNameserverPolicyEnabled })
            }
            onMergeModeChange={(dnsNameserverPolicyMerge) =>
              updateDraft({ dnsNameserverPolicyMerge })
            }
            placeholder={t("home.runtimePreferences.policyPlaceholder")}
            sourceText={draft.dnsNameserverPolicyYaml}
          />

          <RuntimeYamlResourceEditor
            enabled={draft.dnsProxyServerNameserverPolicyEnabled}
            field="dns.proxy-server-nameserver-policy"
            helpTopic="runtimeDnsPolicies"
            label={t(
              "home.runtimePreferences.dnsProxyServerNameserverPolicy",
            )}
            mergeMode={draft.dnsProxyServerNameserverPolicyMerge}
            onCommit={(result) =>
              updateDraft({
                dnsProxyServerNameserverPolicyYaml: result.normalizedYaml,
              })
            }
            onEnabledChange={(dnsProxyServerNameserverPolicyEnabled) =>
              updateDraft({ dnsProxyServerNameserverPolicyEnabled })
            }
            onMergeModeChange={(dnsProxyServerNameserverPolicyMerge) =>
              updateDraft({ dnsProxyServerNameserverPolicyMerge })
            }
            placeholder={t("home.runtimePreferences.policyPlaceholder")}
            sourceText={draft.dnsProxyServerNameserverPolicyYaml}
          />

          <RuntimeYamlResourceEditor
            enabled={draft.dnsUseHosts}
            field="hosts"
            helpTopic="runtimeHosts"
            label={t("home.runtimePreferences.hosts")}
            mergeMode={draft.hostsMerge}
            onCommit={(result) =>
              updateDraft({ hostsYaml: result.normalizedYaml })
            }
            onEnabledChange={(dnsUseHosts) => updateDraft({ dnsUseHosts })}
            onMergeModeChange={(hostsMerge) => updateDraft({ hostsMerge })}
            placeholder={t("home.runtimePreferences.hostsPlaceholder")}
            sourceText={draft.hostsYaml}
          />
        </div>
      </RuntimePreferenceSection>

      <LANBypassEditor draft={draft} updateDraft={updateDraft} />
    </>
  );
}

function LANBypassEditor({
  draft,
  updateDraft,
}: {
  draft: RuntimePreferences;
  updateDraft: (update: Partial<RuntimePreferences>) => void;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [contents, setContents] = useState("");
  const [issue, setIssue] = useState<RuntimeYamlValidationIssue | null>(null);
  const [validating, setValidating] = useState(false);
  const [restoring, setRestoring] = useState(false);
  const textAreaRef = useRef<TextAreaRef>(null);

  const showEditor = () => {
    setContents(formatYamlStringSequence(draft.lanBypassRules));
    setIssue(null);
    setOpen(true);
    requestAnimationFrame(() => textAreaRef.current?.focus());
  };

  const restoreDefaults = async () => {
    setRestoring(true);
    try {
      const defaults = await getDefaultLANBypassRules();
      setContents(formatYamlStringSequence(defaults));
      setIssue(null);
      requestAnimationFrame(() => textAreaRef.current?.focus());
    } catch (error) {
      setIssue({
        code: "runtime_defaults_unavailable",
        column: 0,
        line: 0,
        message:
          error instanceof Error
            ? error.message
            : t("home.runtimePreferences.lanEditor.restoreError"),
      });
    } finally {
      setRestoring(false);
    }
  };

  const save = async () => {
    setValidating(true);
    try {
      const result: RuntimeYamlFragmentValidation =
        await validateRuntimeYAMLFragment({
          contents,
          field: "rules.lan-bypass",
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
        requestAnimationFrame(() => textAreaRef.current?.focus());
        return;
      }
      updateDraft({ lanBypassRules: result.values });
      setOpen(false);
      setIssue(null);
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
    } finally {
      setValidating(false);
    }
  };

  return (
    <>
      <div className="runtime-lan-protection">
        <SafetyCertificateOutlined />
        <div>
          <strong>{t("home.runtimePreferences.lanBypass")}</strong>
          <span>
            {t("home.runtimePreferences.lanBypassValue", {
              count: draft.lanBypassRules.length,
            })}
          </span>
        </div>
        <Tag variant="filled">{t("home.runtimePreferences.direct")}</Tag>
        <Button icon={<EditOutlined />} onClick={showEditor} size="small">
          {t("home.runtimePreferences.lanEditor.edit")}
        </Button>
        <FeatureHelp compact topic="runtimeLanBypass" />
      </div>

      <Modal
        centered
        footer={
          <div className="runtime-lan-editor-footer">
            <Button
              icon={<RedoOutlined />}
              loading={restoring}
              onClick={() => void restoreDefaults()}
            >
              {t("home.runtimePreferences.lanEditor.restoreDefaults")}
            </Button>
            <div>
              <Button onClick={() => setOpen(false)}>
                {t("common.cancel")}
              </Button>
              <Button
                loading={validating}
                onClick={() => void save()}
                type="primary"
              >
                {t("common.save")}
              </Button>
            </div>
          </div>
        }
        mask={{ closable: false }}
        onCancel={() => setOpen(false)}
        open={open}
        title={
          <span className="feature-title-with-help">
            {t("home.runtimePreferences.lanEditor.title")}
            <FeatureHelp compact topic="runtimeLanBypass" />
          </span>
        }
        width={720}
      >
        {issue ? (
          <Alert
            description={issue.message}
            message={
              issue.line > 0
                ? t("home.runtimePreferences.yamlError.location", {
                    column: issue.column || 1,
                    line: issue.line,
                  })
                : t("home.runtimePreferences.yamlError.unknown")
            }
            showIcon
            type="error"
          />
        ) : null}
        <Input.TextArea
          aria-label={t("home.runtimePreferences.lanEditor.title")}
          autoSize={{ minRows: 12, maxRows: 20 }}
          className="runtime-lan-editor-textarea"
          onChange={(event) => {
            setContents(event.target.value);
            setIssue(null);
          }}
          placeholder="- IP-CIDR,10.0.0.0/8,DIRECT,no-resolve"
          ref={textAreaRef}
          value={contents}
        />
      </Modal>
    </>
  );
}
