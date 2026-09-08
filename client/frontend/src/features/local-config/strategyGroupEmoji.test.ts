import { describe, expect, it } from "vitest";

import {
  DEFAULT_SELECTOR_TEST_URL,
  SELECTOR_TEST_URLS,
  STRATEGY_GROUP_EMOJIS,
} from "./strategyGroupEmoji";

describe("local selector presets", () => {
  it("keeps the controlled Emoji catalog unique", () => {
    expect(STRATEGY_GROUP_EMOJIS).toHaveLength(91);
    expect(new Set(STRATEGY_GROUP_EMOJIS).size).toBe(
      STRATEGY_GROUP_EMOJIS.length,
    );
    expect(STRATEGY_GROUP_EMOJIS).toContain("🐱");
    expect(STRATEGY_GROUP_EMOJIS).toContain("🛰️");
  });

  it("uses the requested editable test URL presets", () => {
    expect(DEFAULT_SELECTOR_TEST_URL).toBe(
      "http://www.google.com/generate_204",
    );
    expect(SELECTOR_TEST_URLS).toEqual([
      "http://www.google.com/generate_204",
      "https://www.google.com/generate_204",
      "http://connect.rom.miui.com/generate_204",
      "http://cp.cloudflare.com/generate_204",
      "https://time.tv.cctv.com/time.php",
    ]);
  });
});
