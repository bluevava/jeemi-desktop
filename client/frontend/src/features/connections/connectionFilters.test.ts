import { describe, expect, it } from "vitest";

import type { MihomoConnection } from "../../lib/mihomo/client";
import { filterConnections, isDirectConnection } from "./connectionFilters";

const connection = (
  id: string,
  chains: string[],
  host: string,
): MihomoConnection => ({
  id,
  metadata: {
    network: "tcp",
    type: "HTTP",
    host,
    sourceIP: "127.0.0.1",
    sourcePort: "50000",
    destinationIP: "1.1.1.1",
    destinationPort: "443",
    process: "browser.exe",
    processPath: "C:/browser.exe",
  },
  upload: 0,
  download: 0,
  start: "",
  rule: "MATCH",
  rulePayload: "",
  chains,
  providerChains: [],
});

const items = [
  connection("direct", ["DIRECT"], "direct.example"),
  connection("proxy-a", ["Node A", "Selector A"], "a.example"),
  connection("proxy-b", ["Node B", "Selector B"], "b.example"),
];

describe("connection filters", () => {
  it("classifies the actual first outbound as direct", () => {
    expect(isDirectConnection(items[0])).toBe(true);
    expect(isDirectConnection(items[1])).toBe(false);
  });

  it("does not invent a direct or proxy route when the outbound is missing", () => {
    const unknown = connection("unknown", [], "unknown.example");
    expect(isDirectConnection(unknown)).toBe(false);
    for (const route of ["direct", "proxy"] as const) {
      expect(filterConnections([unknown], { query: "", route, selectedSelectors: null })).toEqual([]);
    }
    expect(filterConnections([unknown], { query: "", route: "all", selectedSelectors: null })).toEqual([unknown]);
  });

  it("filters direct and proxy connections independently", () => {
    expect(
      filterConnections(items, {
        query: "",
        route: "direct",
        selectedSelectors: null,
      }).map((item) => item.id),
    ).toEqual(["direct"]);
    expect(
      filterConnections(items, {
        query: "",
        route: "proxy",
        selectedSelectors: null,
      }).map((item) => item.id),
    ).toEqual(["proxy-a", "proxy-b"]);
  });

  it("applies selected proxy groups and text search together", () => {
    expect(
      filterConnections(items, {
        query: "a.example",
        route: "proxy",
        selectedSelectors: new Set(["Selector A"]),
      }).map((item) => item.id),
    ).toEqual(["proxy-a"]);
    expect(
      filterConnections(items, {
        query: "",
        route: "proxy",
        selectedSelectors: new Set(),
      }),
    ).toEqual([]);
  });

  it("searches displayed rule conditions and retains hidden selector-chain search", () => {
    const inline = { ...items[1], rule: "DomainSuffix", rulePayload: "example.com" };
    expect(filterConnections([inline], {
      query: "DOMAIN-SUFFIX,example.com", route: "all", selectedSelectors: null,
    })).toEqual([inline]);
    expect(filterConnections([inline], {
      query: "Selector A", route: "proxy", selectedSelectors: null,
    })).toEqual([inline]);
  });
});
