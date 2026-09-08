import { Spin } from "antd";
import { useTranslation } from "react-i18next";

import { dnsErrorCode } from "../../services/dnsQueryBridge";
import type { DNSQueryResult } from "../../types/dnsQuery";

interface Props {
  kind: "proxy" | "direct" | "custom";
  result?: DNSQueryResult;
  pending: boolean;
  proxyUnavailable: boolean;
}

export function DNSResultCard({ kind, result, pending, proxyUnavailable }: Props) {
  const { t } = useTranslation();
  const unavailable = pending && proxyUnavailable;
  return (
    <section className="dns-result-card" aria-label={t(`dnsQuery.${kind}`)}>
      <div className="dns-result-heading">
        <h3>{t(`dnsQuery.${kind}`)}</h3>
        {pending && !unavailable ? <Spin size="small" /> : null}
        {result ? <span className="dns-result-route">{t(result.route === "proxy" ? "dnsQuery.customProxy" : "dnsQuery.customDirect")}</span> : null}
      </div>
      {unavailable ? <p className="dns-query-error">{t("dnsQuery.errors.proxy_invalid")}</p> : pending ? (
        <p className="dns-result-empty">{t("dnsQuery.querying")}</p>
      ) : result ? (
        <>
          <div className="dns-result-server">{result.server}</div>
          {result.node ? <div className="dns-result-node">{t("dnsQuery.selectedNode", { selector: result.selector, node: result.node })}</div> : null}
          <div className="dns-result-status" data-status={result.status}>
            <span>{t(`dnsQuery.states.${result.status}`)}</span>
            <span>{t("dnsQuery.duration", { duration: result.durationMS })}</span>
          </div>
          {result.error ? <p className="dns-query-error">{t(`dnsQuery.errors.${dnsErrorCode(result.error)}`)}</p> : null}
          {result.records.map((record) => (
            <div className="dns-result-family" key={record.type}>
              <h4>{record.type}</h4>
              {record.addresses.length ? (
                <ul>{record.addresses.map((ip) => <li key={ip}><code>{ip}</code></li>)}</ul>
              ) : null}
              {record.error ? <p className="dns-query-error">{t(`dnsQuery.errors.${dnsErrorCode(record.error)}`)}</p> : !record.addresses.length ? (
                <p className="dns-result-empty">{t("dnsQuery.noRecords")}</p>
              ) : null}
            </div>
          ))}
        </>
      ) : <p className="dns-result-empty">{t("dnsQuery.empty")}</p>}
    </section>
  );
}
