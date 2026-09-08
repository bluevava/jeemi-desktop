import { describe, expect, it } from "vitest";

import type { ConfigCatalogField } from "../../types/localConfig";
import { defaultLocalConfigResourcePlan } from "../../types/localConfig";
import {
  createLocalConfigDraft,
  configFieldDisplayName,
  configFieldTreeSegments,
  draftSignature,
  setFieldEnabled,
  shouldShowCatalogField,
  updateDraftField,
} from "./model";

const dnsField: ConfigCatalogField = {
  id: "dns-nameserver",
  path: "/dns/nameserver",
  kind: "sequence",
  editor: "yaml",
  options: [],
  exampleYaml: "- 1.1.1.1",
  strategies: ["prepend", "append", "replace"],
  defaultStrategy: "append",
  defaultConflictPolicy: "",
  locked: false,
  sensitive: false,
  documentationUrl: "",
  hidden: false,
  scope: "",
  targetPath: "",
  applicableTypes: [],
};

describe("local configuration draft", () => {
  it("adds catalog defaults once and removes disabled fields", () => {
    const empty = createLocalConfigDraft();
    expect(empty).toEqual({
      id: "",
      name: "",
      description: "",
      fields: [],
      resourcePlan: defaultLocalConfigResourcePlan(),
    });
    const enabled = setFieldEnabled(empty, dnsField, true);
    const enabledAgain = setFieldEnabled(enabled, dnsField, true);

    expect(enabled.fields).toEqual([
      {
        path: "/dns/nameserver",
        valueYaml: "- 1.1.1.1",
        strategy: "append",
        conflictPolicy: "",
      },
    ]);
    expect(enabledAgain.fields).toHaveLength(1);
    expect(setFieldEnabled(enabledAgain, dnsField, false).fields).toEqual([]);
  });

  it("tracks value and strategy changes without mutating the original", () => {
    const enabled = setFieldEnabled(createLocalConfigDraft(), dnsField, true);
    const updated = updateDraftField(enabled, dnsField.path, {
      strategy: "prepend",
      valueYaml: "- tls://dns.example",
    });

    expect(updated.fields[0].strategy).toBe("prepend");
    expect(enabled.fields[0].strategy).toBe("append");
    expect(draftSignature(updated)).not.toBe(draftSignature(enabled));
  });

  it("hides runtime-owned fields until the locked-field filter is enabled", () => {
    const enabledPaths = new Set([dnsField.path]);
    const lockedField: ConfigCatalogField = {
      ...dnsField,
      id: "runtime-secret",
      path: "/secret",
      locked: true,
    };

    expect(
      shouldShowCatalogField(lockedField, enabledPaths, false, false),
    ).toBe(false);
    expect(shouldShowCatalogField(lockedField, enabledPaths, true, true)).toBe(
      true,
    );
    expect(shouldShowCatalogField(dnsField, new Set(), true, false)).toBe(
      false,
    );
    expect(shouldShowCatalogField(dnsField, enabledPaths, true, false)).toBe(
      true,
    );
  });

  it("formats JSON-pointer fields as dotted editor titles and tree segments", () => {
    expect(configFieldDisplayName("/log-level")).toBe("log-level");
    expect(configFieldDisplayName("/dns/ipv6")).toBe("dns.ipv6");
    expect(configFieldDisplayName("/proxies/*/client-fingerprint")).toBe(
      "proxies.client-fingerprint",
    );
    expect(
      configFieldTreeSegments("sniffer", "/sniffer/sniff/HTTP/ports"),
    ).toEqual(["sniff", "HTTP", "ports"]);
  });

  it("keeps hidden fields invisible unless already enabled", () => {
    const hidden = { ...dnsField, hidden: true };
    expect(shouldShowCatalogField(hidden, new Set(), false, false)).toBe(false);
    expect(
      shouldShowCatalogField(hidden, new Set([hidden.path]), false, false),
    ).toBe(true);
  });
});
