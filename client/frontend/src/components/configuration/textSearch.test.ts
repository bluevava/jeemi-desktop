import { describe, expect, it } from "vitest";
import {
  configurationSearchAction,
  findConfigurationMatches,
  nextConfigurationMatch,
  visibleConfigurationMatches,
} from "./textSearch";

describe("configuration text search", () => {
  it("finds literal text regardless of case without interpreting regular expressions", () => {
    const text = "proxy: [Node.*] # [node.*]\npath: /a+b?$x\\c";
    expect([...findConfigurationMatches(text, "[node.*]")]).toEqual([7, 15, 18, 26]);
    const query = "/a+b?$x\\c";
    expect([...findConfigurationMatches(text, query)]).toEqual([text.indexOf(query), text.length]);
    expect(findConfigurationMatches(text, "[other.*]")).toHaveLength(0);
  });

  it("keeps original UTF-16 offsets for emoji and Unicode case folding", () => {
    const text = "İSS 🇭🇰 香港\nKey: value\nkey: next";
    const matches = findConfigurationMatches(text, "KEY");
    expect([...matches]).toEqual([text.indexOf("Key"), text.indexOf("Key") + 3, text.indexOf("key"), text.indexOf("key") + 3]);
    const flags = findConfigurationMatches(text, "🇭🇰");
    expect(text.slice(flags[0], flags[1])).toBe("🇭🇰");
    expect(findConfigurationMatches(text, "香港")).toHaveLength(2);
  });

  it("preserves meaningful whitespace and finds non-overlapping matches", () => {
    expect([...findConfigurationMatches("a  b\na  b", "  ")]).toEqual([1, 3, 6, 8]);
    expect([...findConfigurationMatches("aaaaa", "aa")]).toEqual([0, 2, 2, 4]);
    expect([...findConfigurationMatches("a\nb\na\nb", "a\nb")]).toEqual([0, 3, 4, 7]);
    expect(findConfigurationMatches("anything", "")).toHaveLength(0);
    expect(findConfigurationMatches("", "anything")).toHaveLength(0);
  });

  it("counts the full large document while bounding rendered highlights", () => {
    const text = "node: a\n".repeat(131072);
    const matches = findConfigurationMatches(text, "a");
    expect(matches.length / 2).toBe(131072);
    expect(visibleConfigurationMatches(matches, 131071)).toEqual([131071]);
    expect([...matches.slice(-2)]).toEqual([text.length - 2, text.length - 1]);
    expect(visibleConfigurationMatches(findConfigurationMatches("aaa", "a"), 1)).toEqual([0, 1, 2]);
  });

  it("wraps in both directions and handles empty and single-result documents", () => {
    expect(nextConfigurationMatch(2, 3, 1)).toBe(0);
    expect(nextConfigurationMatch(0, 3, -1)).toBe(2);
    expect(nextConfigurationMatch(-1, 3, 1)).toBe(0);
    expect(nextConfigurationMatch(-1, 3, -1)).toBe(2);
    expect(nextConfigurationMatch(0, 1, -1)).toBe(0);
    expect(nextConfigurationMatch(0, 0, 1)).toBe(-1);
  });
});

const key = (value: string, modifiers = {}) => ({
  key: value, ctrlKey: false, metaKey: false, altKey: false, shiftKey: false,
  isComposing: false, ...modifiers,
});

describe("viewer-scoped search shortcuts", () => {
  it("supports Windows/Linux and macOS find shortcuts", () => {
    expect(configurationSearchAction(key("f", { ctrlKey: true }), false, false)).toBe("focus");
    expect(configurationSearchAction(key("F", { metaKey: true }), false, true)).toBe("focus");
    expect(configurationSearchAction(key("f"), false, false)).toBeNull();
  });

  it("uses Enter only in search, and supports F3 and Cmd/Ctrl+G navigation", () => {
    expect(configurationSearchAction(key("Enter"), true, true)).toBe("next");
    expect(configurationSearchAction(key("Enter", { shiftKey: true }), true, true)).toBe("previous");
    expect(configurationSearchAction(key("Enter"), false, true)).toBeNull();
    expect(configurationSearchAction(key("F3"), false, true)).toBe("next");
    expect(configurationSearchAction(key("g", { metaKey: true, shiftKey: true }), false, true)).toBe("previous");
  });

  it("clears search first on Escape without consuming input-method composition", () => {
    expect(configurationSearchAction(key("Escape"), true, true)).toBe("clear");
    expect(configurationSearchAction(key("Escape"), true, false)).toBeNull();
    expect(configurationSearchAction(key("Enter", { isComposing: true }), true, true)).toBeNull();
    expect(configurationSearchAction(key("f", { ctrlKey: true, altKey: true }), true, true)).toBeNull();
  });
});
