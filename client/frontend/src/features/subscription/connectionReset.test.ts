import { describe, expect, it } from "vitest";

import type { MihomoConnection } from "../../lib/mihomo/client";
import { connectionIDsForNodeSwitch } from "./connectionReset";

const connection = (id: string, chains: string[]): MihomoConnection => ({
  id,
  metadata: {
    network: "tcp",
    type: "HTTP",
    host: "example.com",
    sourceIP: "127.0.0.1",
    sourcePort: "50000",
    destinationIP: "192.0.2.1",
    destinationPort: "443",
    process: "",
    processPath: "",
  },
  upload: 0,
  download: 0,
  start: "",
  rule: "MATCH",
  rulePayload: "",
  chains,
  providerChains: [],
});

describe("connection reset after node switching", () => {
  const connections = [
    connection("a", ["🇯🇵 Tokyo", "Selector A"]),
    connection("b", ["🇩🇪 Berlin", "Selector B"]),
    connection("direct", ["DIRECT"]),
  ];

  it("keeps all connections when disabled", () => {
    expect(connectionIDsForNodeSwitch(connections, "off", "Selector A")).toEqual([]);
  });

  it("targets only chains containing the switched selector", () => {
    expect(
      connectionIDsForNodeSwitch(connections, "selector", "Selector A"),
    ).toEqual(["a"]);
  });

  it("targets every connection in all mode", () => {
    expect(connectionIDsForNodeSwitch(connections, "all", "Selector A")).toEqual([
      "a",
      "b",
      "direct",
    ]);
  });
});
