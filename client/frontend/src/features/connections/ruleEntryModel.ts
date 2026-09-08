import type { MihomoConnection } from "../../lib/mihomo/client";
import type { RuleSetResource } from "../../types/localConfig";
import type { RuleMatchType } from "../../types/ruleSetEntry";
import { connectionProcessName } from "./connectionPresentation";

export interface ConnectionRuleSeed {
  matchType: RuleMatchType;
  values: Record<RuleMatchType, string>;
  caseInsensitive: boolean;
}

export function connectionRuleSeed(
  connection: MihomoConnection,
  field: "target" | "processName" | "processPath",
): ConnectionRuleSeed {
  const { host, destinationIP, processPath } = connection.metadata;
  const hostIsIP = host.includes(":") || /^\d+\.\d+\.\d+\.\d+$/.test(host);
  const domain = hostIsIP ? "" : host;
  return {
    matchType: field === "target" ? (domain ? "domain" : "ip") : field,
    values: {
      domain,
      ip: destinationIP || (hostIsIP ? host : ""),
      processName: connectionProcessName(connection),
      processPath,
    },
    caseInsensitive: processPath.includes("\\"),
  };
}

export function unnamedRuleSetName(
  ruleSets: RuleSetResource[],
  prefix: string,
): string {
  const names = new Set(ruleSets.map((ruleSet) => ruleSet.name));
  let index = 1;
  while (names.has(`${prefix}${index}`)) index += 1;
  return `${prefix}${index}`;
}

export function canAppendRule(
  ruleSet: RuleSetResource,
  type: RuleMatchType,
): boolean {
  return (
    ruleSet.sourceType === "inline" &&
    (ruleSet.behavior === "classical" ||
      (ruleSet.behavior === "domain" && type === "domain") ||
      (ruleSet.behavior === "ipcidr" && type === "ip"))
  );
}
