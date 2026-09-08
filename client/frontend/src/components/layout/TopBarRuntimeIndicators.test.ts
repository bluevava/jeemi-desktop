import { describe, expect, it } from "vitest";

import {
  formatTitleBarRate,
  leadingCountryFlag,
  resolveSelectedLeaf,
} from "./TopBarRuntimeIndicators";

describe("top-bar runtime indicators", () => {
  it("extracts only a leading regional-indicator flag", () => {
    expect(leadingCountryFlag("🇯🇵 Tokyo")).toBe("🇯🇵");
    expect(leadingCountryFlag(" 🇯🇵 Tokyo")).toBe("");
    expect(leadingCountryFlag("Tokyo 🇯🇵")).toBe("");
    expect(leadingCountryFlag("🐱 Tokyo")).toBe("");
  });

  it("resolves nested selector choices to the active leaf", () => {
    expect(
      resolveSelectedLeaf(
        "Main",
        { Main: "Automatic", Automatic: "🇩🇪 Berlin" },
        "Fallback",
      ),
    ).toBe("🇩🇪 Berlin");
  });

  it("formats compact title-bar rates without a per-second suffix", () => {
    expect(formatTitleBarRate(512, "en-US")).toBe("512 b");
    expect(formatTitleBarRate(1536, "en-US")).toBe("1.5 k");
    expect(formatTitleBarRate(2 * 1024 * 1024, "en-US")).toBe("2 m");
  });
});
