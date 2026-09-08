import { describe, expect, it } from "vitest";
import { insertAtSelection } from "./textInsertion";
import { emptyRuleSet, emptyStrategyGroup } from "./resourceModel";
import { isLocalConfigEditorPath } from "../../app/navigation";

describe("resource page editing", () => {
  it("inserts a complete flag at a selected range without losing surrounding lines", () => {
    const result = insertAtSelection("first\n!test\nlast", "🇺🇸", 7, 11);
    expect(result.text).toBe("first\n!🇺🇸\nlast");
    expect(result.text.slice(result.caret)).toBe("\nlast");
  });
  it("preserves trailing blank lines and caret after a surrogate-pair flag", () => {
    const result = insertAtSelection("🇯🇵\n\n", "🇺🇸", 5, 5);
    expect(result.text).toBe("🇯🇵\n🇺🇸\n");
    expect(result.caret).toBe(9);
  });
  it("starts with classical rules and all subscription nodes for a new selector", () => {
    expect(emptyRuleSet().behavior).toBe("classical");
    expect(emptyStrategyGroup().filter).toEqual({
      proxyTypes: [],
      namePatterns: [],
    });
    expect(emptyStrategyGroup("rule").type).toBe("");
  });
  it("gives each resource a page editor with an isolated action bar", () => {
    for (const path of [
      "/config/groups/new/rule",
      "/config/groups/new/selector",
      "/config/rule-sets/new",
      `/config/groups/${"a".repeat(32)}/edit`,
      `/config/rule-sets/${"b".repeat(32)}/edit`,
    ])
      expect(isLocalConfigEditorPath(path)).toBe(true);
  });
});
