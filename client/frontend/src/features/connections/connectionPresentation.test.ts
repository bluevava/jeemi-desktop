import { describe, expect, it } from "vitest";

import type { MihomoConnection } from "../../lib/mihomo/client";
import {
  connectionEndpoint,
  connectionMatchedRule,
  connectionOutbound,
  connectionProcessName,
} from "./connectionPresentation";

const sample: MihomoConnection = {
  id: "connection",
  metadata: {
    network: "tcp", type: "Tun", host: "example.com",
    sourceIP: "127.0.0.1", sourcePort: "50000",
    destinationIP: "203.0.113.1", destinationPort: "443",
    process: "Browser", processPath: "/Applications/Browser.app/Contents/MacOS/Browser",
  },
  upload: 0, download: 0, start: "2026-09-07T02:00:00Z",
  rule: "RuleSet", rulePayload: "🌐 应用规则",
  chains: ["🇯🇵 Node A", "Automatic", "Applications"],
  providerChains: ["Proxy subscription"],
};

describe("connection presentation", () => {
  it("uses the rule payload as the rule-provider name, not proxy providers", () => {
    expect(connectionMatchedRule(sample)).toBe("🌐 应用规则");
    expect(connectionMatchedRule({ ...sample, rulePayload: "" })).toBe("RULE-SET");
  });

  it("normalises final rules and displays available inline conditions", () => {
    expect(connectionMatchedRule({ ...sample, rule: "Match" })).toBe("MATCH");
    expect(connectionMatchedRule({ ...sample, rule: "FINAL" })).toBe("MATCH");
    expect(connectionMatchedRule({ ...sample, rule: "DomainSuffix", rulePayload: "example.com" }))
      .toBe("DOMAIN-SUFFIX,example.com");
    expect(connectionMatchedRule({ ...sample, rule: "IPCIDR", rulePayload: "10.0.0.0/8" }))
      .toBe("IP-CIDR,10.0.0.0/8");
    expect(connectionMatchedRule({ ...sample, rule: "AND", rulePayload: "((NETWORK,tcp),(DST-PORT,443))" }))
      .toBe("AND,((NETWORK,tcp),(DST-PORT,443))");
  });

  it("distinguishes incomplete inline details from a connection with no rule", () => {
    expect(connectionMatchedRule({ ...sample, rule: "Domain", rulePayload: "" })).toBe("inline");
    expect(connectionMatchedRule({ ...sample, rule: "", rulePayload: "" })).toBe("—");
    expect(connectionMatchedRule({ ...sample, rule: "FutureRule", rulePayload: "opaque" }))
      .toBe("FutureRule,opaque");
  });

  it("shows the actual first outbound even through nested selectors", () => {
    expect(connectionOutbound(sample)).toBe("🇯🇵 Node A");
    expect(connectionOutbound({ ...sample, chains: ["DIRECT", "Applications"] })).toBe("DIRECT");
    expect(connectionOutbound({ ...sample, chains: ["REJECT", "Applications"] })).toBe("REJECT");
    expect(connectionOutbound({ ...sample, chains: [] })).toBe("—");
  });

  it("uses a reported process name or derives one from a returned platform path", () => {
    expect(connectionProcessName(sample)).toBe("Browser");
    expect(connectionProcessName({ ...sample, metadata: { ...sample.metadata, process: "" } })).toBe("Browser");
    expect(connectionProcessName({ ...sample, metadata: { ...sample.metadata, process: "", processPath: "C:\\Program Files\\Browser\\browser.exe" } }))
      .toBe("browser.exe");
    expect(connectionProcessName({ ...sample, metadata: { ...sample.metadata, process: "", processPath: "" } })).toBe("");
  });

  it("keeps IPv6 address and port boundaries unambiguous", () => {
    expect(connectionEndpoint("2001:db8::1", "443")).toBe("[2001:db8::1]:443");
    expect(connectionEndpoint("[2001:db8::1]", "443")).toBe("[2001:db8::1]:443");
    expect(connectionEndpoint("127.0.0.1", "50000")).toBe("127.0.0.1:50000");
  });
});
