import { describe, expect, it } from "vitest";
import type { MihomoConnection } from "../../lib/mihomo/client";
import type { RuleSetResource } from "../../types/localConfig";
import {
  canAppendRule,
  connectionRuleSeed,
  unnamedRuleSetName,
} from "./ruleEntryModel";

const connection: MihomoConnection = {
  id: "sample",
  metadata: {
    network: "tcp",
    type: "Tun",
    host: "api.anthropic.com",
    sourceIP: "127.0.0.1",
    sourcePort: "50000",
    destinationIP: "20.203.144.245",
    destinationPort: "443",
    process: "msedge.exe",
    processPath:
      "C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe",
  },
  upload: 0,
  download: 0,
  start: "2026-09-08T02:00:00Z",
  rule: "Match",
  rulePayload: "",
  chains: ["DIRECT"],
  providerChains: [],
};

describe("connection rule entry defaults", () => {
  it("prefills the target without a port and keeps the other matching values available", () => {
    const seed = connectionRuleSeed(connection, "target");
    expect(seed.matchType).toBe("domain");
    expect(seed.values.domain).toBe("api.anthropic.com");
    expect(seed.values.ip).toBe("20.203.144.245");
    expect(seed.values.processPath).toBe(connection.metadata.processPath);
    expect(seed.caseInsensitive).toBe(true);
  });

  it("uses IP matching for an absent domain or a literal IPv6 host", () => {
    const withoutDomain = {
      ...connection,
      metadata: { ...connection.metadata, host: "" },
    };
    expect(connectionRuleSeed(withoutDomain, "target").matchType).toBe("ip");
    const ipv6 = connectionRuleSeed(
      {
        ...connection,
        metadata: {
          ...connection.metadata,
          host: "2001:db8::1",
          destinationIP: "",
        },
      },
      "target",
    );
    expect(ipv6.matchType).toBe("ip");
    expect(ipv6.values.ip).toBe("2001:db8::1");
    expect(ipv6.values.domain).toBe("");
  });

  it("retains full literal paths and uses the process basename when needed", () => {
    const path = "/Applications/Example.app/Contents/MacOS/Example";
    const sample = {
      ...connection,
      metadata: { ...connection.metadata, process: "", processPath: path },
    };
    const seed = connectionRuleSeed(sample, "processPath");
    expect(seed.matchType).toBe("processPath");
    expect(seed.values.processPath).toBe(path);
    expect(seed.caseInsensitive).toBe(false);
    expect(connectionRuleSeed(sample, "processName").values.processName).toBe(
      "Example",
    );
  });

  it("chooses an unused draft name without overwriting an existing definition", () => {
    const sets = [
      { name: "未命名1" },
      { name: "未命名3" },
    ] as RuleSetResource[];
    expect(unnamedRuleSetName(sets, "未命名")).toBe("未命名2");
    expect(unnamedRuleSetName(sets, "Unnamed")).toBe("Unnamed1");
  });

  it("only offers compatible local destinations", () => {
    const set = (behavior: string, sourceType = "inline") =>
      ({ behavior, sourceType }) as RuleSetResource;
    expect(canAppendRule(set("classical"), "processPath")).toBe(true);
    expect(canAppendRule(set("classical", "http"), "processPath")).toBe(false);
    expect(canAppendRule(set("domain"), "domain")).toBe(true);
    expect(canAppendRule(set("domain"), "ip")).toBe(false);
    expect(canAppendRule(set("ipcidr"), "ip")).toBe(true);
    expect(canAppendRule(set("ipcidr"), "processName")).toBe(false);
  });
});
