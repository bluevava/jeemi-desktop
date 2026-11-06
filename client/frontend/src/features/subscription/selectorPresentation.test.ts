import { describe, expect, it } from "vitest";

import {
  selectorPresentation,
  shouldDisplaySelector,
} from "./selectorPresentation";

describe("selector visibility", () => {
  it("hides mihomo hidden groups until the local display switch is enabled", () => {
    expect(shouldDisplaySelector(true, false)).toBe(false);
    expect(shouldDisplaySelector(true, true)).toBe(true);
    expect(shouldDisplaySelector(false, false)).toBe(true);
  });
});

describe("selectorPresentation", () => {
  it("moves a leading pictographic emoji into the empty icon slot", () => {
    expect(selectorPresentation("✈️ Telegram", "")).toEqual({
      displayName: "Telegram",
      emoji: "✈️",
      iconUrl: null,
    });
  });

  it("keeps regional-indicator flags as one selector icon", () => {
    expect(selectorPresentation("🇯🇵 Japan", "")).toEqual({
      displayName: "Japan",
      emoji: "🇯🇵",
      iconUrl: null,
    });
  });

  it("gives an explicit remote icon precedence over the name emoji", () => {
    expect(
      selectorPresentation(
        "🐥 Jeeyio",
        "https://assets.example.com/groups/jeeyio.png",
      ),
    ).toEqual({
      displayName: "🐥 Jeeyio",
      emoji: "",
      iconUrl: "https://assets.example.com/groups/jeeyio.png",
    });
  });

  it("moves a complete subdivision flag into the icon slot", () => {
    const england = "\u{1f3f4}\u{e0067}\u{e0062}\u{e0065}\u{e006e}\u{e0067}\u{e007f}";
    expect(selectorPresentation(`${england} England`, "")).toEqual({
      displayName: "England", emoji: england, iconUrl: null,
    });
  });

  it("does not load local paths or executable URL schemes", () => {
    expect(selectorPresentation("Primary", "file:///tmp/icon.png")).toEqual({
      displayName: "Primary",
      emoji: "",
      iconUrl: null,
    });
    expect(selectorPresentation("Primary", "javascript:alert(1)").iconUrl).toBeNull();
  });
});
