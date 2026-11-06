import { describe, expect, it } from "vitest";
import {
  editRulePayload,
  parseRulePayload,
  splitClassicalRule,
} from "./rulePayload";

describe("visual rule payload editing", () => {
  it("preserves comments, ordering and literal Windows paths through edits", () => {
    const source =
      "# browser rules\npayload:\n  # explicit app\n  - 'PROCESS-PATH,C:\\Browser\\browser.exe' # keep this\n  - DOMAIN-SUFFIX,example.com\n";
    const edited = editRulePayload(source, {
      type: "edit",
      index: 1,
      value: "DOMAIN-SUFFIX,other.example",
    });
    expect(edited).toContain("# browser rules");
    expect(edited).toContain("# explicit app");
    expect(edited).toContain("# keep this");
    expect(parseRulePayload(edited).rules).toEqual([
      "PROCESS-PATH,C:\\Browser\\browser.exe",
      "DOMAIN-SUFFIX,other.example",
    ]);
    const added = editRulePayload(edited, {
      type: "add",
      value: "PROCESS-PATH,C:\\Bob's App: #1\\browser.exe",
    });
    const moved = editRulePayload(added, { type: "move", index: 2, to: 0 });
    const deleted = editRulePayload(moved, { type: "delete", index: 2 });
    expect(parseRulePayload(deleted).rules).toEqual([
      "PROCESS-PATH,C:\\Bob's App: #1\\browser.exe",
      "PROCESS-PATH,C:\\Browser\\browser.exe",
    ]);
  });
  it.each([
    "payload: &rules [one]\n",
    "payload: [*alias]\n",
    "payload: [123]\n",
    "payload: [{type: DOMAIN}]",
    "payload: [one]\nother: true",
    "payload: [one]\n---\npayload: [two]",
    "payload: ['one\n\n two']",
  ])("rejects unsupported YAML without rewriting the source: %s", (source) => {
    expect(() => parseRulePayload(source)).toThrow();
    expect(() =>
      editRulePayload(source, { type: "add", value: "example.com" }),
    ).toThrow();
  });
  it("allows an empty draft and preserves comma-rich expressions", () => {
    expect(parseRulePayload("").rules).toEqual([]);
    const rule = "AND,((PROCESS-NAME,app.exe),(DOMAIN-SUFFIX,example.com))";
    expect(splitClassicalRule(rule)).toEqual({
      type: "AND",
      value: "((PROCESS-NAME,app.exe),(DOMAIN-SUFFIX,example.com))",
    });
    expect(
      parseRulePayload(editRulePayload("", { type: "add", value: rule })).rules,
    ).toEqual([rule]);
    expect(() =>
      editRulePayload("payload: [one]", {
        type: "edit",
        index: 3,
        value: "two",
      }),
    ).toThrow();
  });
  it("edits a large offline baseline without losing entries", () => {
    const source = `payload:\n${Array.from({ length: 10000 }, (_, i) => `  - '+.site${i}.example'`).join("\n")}\n`;
    const edited = editRulePayload(source, {
      type: "edit",
      index: 9999,
      value: "+.changed.example",
    });
    const { rules } = parseRulePayload(edited);
    expect(rules).toHaveLength(10000);
    expect(rules[0]).toBe("+.site0.example");
    expect(rules[9999]).toBe("+.changed.example");
  });
});
