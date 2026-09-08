import { describe, expect, it } from "vitest";

import { formatYamlStringSequence } from "./runtimeYaml";

describe("formatYamlStringSequence", () => {
  it("formats common resolver and rule values as a YAML sequence", () => {
    expect(
      formatYamlStringSequence([
        "223.5.5.5",
        "https://dns.alidns.com/dns-query",
        "IP-CIDR,10.0.0.0/8,DIRECT,no-resolve",
      ]),
    ).toBe(
      "- 223.5.5.5\n- https://dns.alidns.com/dns-query\n- IP-CIDR,10.0.0.0/8,DIRECT,no-resolve",
    );
  });

  it("keeps an empty sequence editor blank and quotes implicit YAML scalars", () => {
    expect(formatYamlStringSequence([])).toBe("");
    expect(formatYamlStringSequence(["true", "value with spaces"])).toBe(
      '- "true"\n- "value with spaces"',
    );
  });
});
