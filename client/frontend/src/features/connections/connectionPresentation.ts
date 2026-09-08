import type { MihomoConnection } from "../../lib/mihomo/client";

// /connections returns RuleType.String(), not the spelling used in YAML.
// Keep unknown future types intact instead of guessing their syntax.
const ruleTypes: Readonly<Record<string, string>> = {
  DOMAIN: "DOMAIN",
  DOMAINSUFFIX: "DOMAIN-SUFFIX",
  DOMAINKEYWORD: "DOMAIN-KEYWORD",
  DOMAINREGEX: "DOMAIN-REGEX",
  DOMAINWILDCARD: "DOMAIN-WILDCARD",
  GEOSITE: "GEOSITE",
  GEOIP: "GEOIP",
  SRCGEOIP: "SRC-GEOIP",
  IPASN: "IP-ASN",
  SRCIPASN: "SRC-IP-ASN",
  IPCIDR: "IP-CIDR",
  SRCIPCIDR: "SRC-IP-CIDR",
  IPSUFFIX: "IP-SUFFIX",
  SRCIPSUFFIX: "SRC-IP-SUFFIX",
  SRCPORT: "SRC-PORT",
  DSTPORT: "DST-PORT",
  INPORT: "IN-PORT",
  DSCP: "DSCP",
  INUSER: "IN-USER",
  INNAME: "IN-NAME",
  INTYPE: "IN-TYPE",
  PROCESSNAME: "PROCESS-NAME",
  PROCESSPATH: "PROCESS-PATH",
  PROCESSNAMEREGEX: "PROCESS-NAME-REGEX",
  PROCESSPATHREGEX: "PROCESS-PATH-REGEX",
  PROCESSNAMEWILDCARD: "PROCESS-NAME-WILDCARD",
  PROCESSPATHWILDCARD: "PROCESS-PATH-WILDCARD",
  REMATCHNAME: "REMATCH-NAME",
  NETWORK: "NETWORK",
  UID: "UID",
  SUBRULES: "SUB-RULE",
  AND: "AND",
  OR: "OR",
  NOT: "NOT",
};

export function connectionMatchedRule(connection: MihomoConnection): string {
  const rule = connection.rule.trim();
  const type = rule.replaceAll("-", "").toUpperCase();
  if (type === "MATCH" || type === "FINAL") return "MATCH";
  if (type === "RULESET") {
    // providerChains describes proxy providers, never the matched rule set.
    return connection.rulePayload || "RULE-SET";
  }
  if (!rule) return "—"; // Global/direct mode may not match a rule at all.
  if (!connection.rulePayload) return "inline";
  // The API omits the original target and flags such as no-resolve.
  return `${ruleTypes[type] ?? rule},${connection.rulePayload}`;
}

export function connectionOutbound(connection: MihomoConnection): string {
  // mihomo Chain.Last() is chains[0]; later entries are its selectors.
  return connection.chains[0] || "—";
}

export function connectionProcessName(connection: MihomoConnection): string {
  return connection.metadata.process ||
    connection.metadata.processPath.split(/[\\/]/).at(-1) || "";
}

export function connectionEndpoint(host: string, port: string): string {
  const address = host.includes(":") && !host.startsWith("[") ? `[${host}]` : host;
  return port ? `${address || "—"}:${port}` : address || "—";
}

export function formatConnectionBytes(bytes: number, locale: string): string {
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let value = Math.max(0, bytes);
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${new Intl.NumberFormat(locale, {
    maximumFractionDigits: value >= 100 || unit === 0 ? 0 : 1,
  }).format(value)} ${units[unit]}`;
}
