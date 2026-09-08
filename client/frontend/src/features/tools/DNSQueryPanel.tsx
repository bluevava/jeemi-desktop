import { CloseOutlined, DownOutlined, SearchOutlined } from "@ant-design/icons";
import { AutoComplete, Button, Input, Switch } from "antd";
import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { useRuntimeStatus } from "../../app/runtime/RuntimeStatusContext";
import { useWindowActivity } from "../../app/runtime/WindowActivityContext";
import { cancelDNSQuery, dnsErrorCode, queryDNS } from "../../services/dnsQueryBridge";
import { DNSResultCard } from "./DNSResultCard";
import { useDNSQueryPreferences } from "./DNSQueryPreferencesContext";
import { dnsServerPresets } from "./dnsServerPresets";
import { useToolsPageStateField } from "./ToolsPageStateContext";

export function DNSQueryPanel() {
  const { t, i18n } = useTranslation();
  const { runtime } = useRuntimeStatus();
  const { isWindowVisible } = useWindowActivity();
  const { customDNS, setCustomDNS, loading: preferencesLoading, error: preferencesError } = useDNSQueryPreferences();
  const [domain, setDomain] = useToolsPageStateField("domain");
  const [servers, setServers] = useToolsPageStateField("servers");
  const [customProxy, setCustomProxy] = useToolsPageStateField("customProxy");
  const [busy, setBusy] = useState(false);
  const [response, setResponse] = useToolsPageStateField("response");
  const [error, setError] = useToolsPageStateField("error");
  const activeID = useRef("");
  const proxyReady = runtime?.mihomo.state === "running" && runtime.mihomo.controllerReady;

  const cancel = useCallback(() => {
    const id = activeID.current;
    if (!id) return;
    activeID.current = "";
    setBusy(false);
    setError("cancelled");
    void cancelDNSQuery(id).catch(() => undefined);
  }, [setError]);

  useEffect(() => {
    if (!isWindowVisible) cancel();
  }, [cancel, isWindowVisible]);
  useEffect(() => () => cancel(), [cancel]);

  const submit = async () => {
    if (activeID.current || preferencesLoading || !domain.trim() || !isWindowVisible) return;
    const id = Array.from(crypto.getRandomValues(new Uint8Array(16)),
      (byte) => byte.toString(16).padStart(2, "0")).join("");
    activeID.current = id;
    setBusy(true);
    setError("");
    try {
      const result = await queryDNS({ id, domain, proxyDNS: servers.proxy, directDNS: servers.direct, customDNS, customProxy });
      if (activeID.current === id) {
        setDomain(result.domain);
        setCustomDNS(customDNS.trim());
        setResponse(result);
      }
    } catch (caught) {
      if (activeID.current === id) setError(dnsErrorCode(caught));
    } finally {
      if (activeID.current === id) {
        activeID.current = "";
        setBusy(false);
      }
    }
  };

  return (
    <div className="dns-query-panel">
      <form className="dns-query-domain" onSubmit={(event) => { event.preventDefault(); if (!busy) void submit(); }}>
        <Input aria-label={t("dnsQuery.domain")} autoComplete="off" disabled={busy} maxLength={8192} onChange={(event) => setDomain(event.target.value)} placeholder={t("dnsQuery.domainPlaceholder")} spellCheck={false} value={domain} />
        {busy ? <Button icon={<CloseOutlined />} onClick={cancel}>{t("dnsQuery.cancel")}</Button> : (
          <Button disabled={preferencesLoading || !domain.trim() || !isWindowVisible} htmlType="submit" icon={<SearchOutlined />} type="primary">{t("dnsQuery.query")}</Button>
        )}
      </form>
      <div className="dns-query-servers">
        {(["proxy", "direct", "custom"] as const).map((kind) => (
          <div className="dns-server-field" key={kind}>
            <div className="dns-server-label">
              <label htmlFor={`dns-server-${kind}`}>{t(`dnsQuery.${kind}`)}</label>
              {kind === "custom" ? <Switch aria-label={t("dnsQuery.customRoute")} checked={customProxy} checkedChildren={t("dnsQuery.customProxy")} disabled={busy} onChange={setCustomProxy} size="small" unCheckedChildren={t("dnsQuery.customDirect")} /> : null}
            </div>
            <AutoComplete
              allowClear
              disabled={busy || kind === "custom" && preferencesLoading}
              filterOption={false}
              id={`dns-server-${kind}`}
              onChange={(value) => {
                if (kind === "custom") setCustomDNS(value);
                else setServers((current) => ({ ...current, [kind]: value }));
              }}
              options={dnsServerPresets}
              placeholder={t("dnsQuery.serverPlaceholder")}
              value={kind === "custom" ? customDNS : servers[kind]}
            >
              <Input aria-label={t(`dnsQuery.${kind}`)} maxLength={2048} spellCheck={false} suffix={<DownOutlined aria-hidden />} />
            </AutoComplete>
          </div>
        ))}
      </div>
      {preferencesError && !response && !error ? <p className="dns-query-error" role="alert">{t(`dnsQuery.errors.${preferencesError}`)}</p> : null}
      {error ? <p className="dns-query-error" role="alert">{t(`dnsQuery.errors.${error}`)}</p> : null}
      {response && !busy ? <p className="dns-query-stamp">{t("dnsQuery.resultFor", { domain: response.domain, time: new Date(response.queriedAt).toLocaleString(i18n.language) })}</p> : null}
      <div aria-live="polite" className="dns-query-results">
        {(["proxy", "direct", "custom"] as const).map((kind) => (
          <DNSResultCard key={kind} kind={kind} pending={busy} proxyUnavailable={!proxyReady && (kind === "proxy" || kind === "custom" && customProxy)} result={busy ? undefined : response?.[kind]} />
        ))}
      </div>
    </div>
  );
}
