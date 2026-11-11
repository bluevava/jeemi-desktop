import { describe, expect, it } from "vitest";
import {
  EMOJI_CATEGORIES,
  EMOJI_OPTIONS,
  filterEmojiOptions,
} from "./emojiCatalog";

describe("shared Emoji catalog", () => {
  it("keeps complete, unique, bilingual options", () => {
    expect(EMOJI_OPTIONS).toHaveLength(150);
    expect(new Set(EMOJI_OPTIONS.map((item) => item.value)).size).toBe(150);
    expect(
      EMOJI_OPTIONS.every((item) => item.zh && item.en && item.value),
    ).toBe(true);
    expect(EMOJI_CATEGORIES.map((item) => item.id)).toEqual([
      "network",
      "communication",
      "media",
      "devices",
      "nature",
    ]);
  });
  it("matches Chinese and English keywords inside the selected category", () => {
    expect(
      filterEmojiOptions("", "AI").some((item) => item.value === "🤖"),
    ).toBe(true);
    expect(
      filterEmojiOptions("communication", "智能").map((item) => item.value),
    ).toEqual(["🤖", "🧠"]);
    expect(filterEmojiOptions("media", "智能")).toEqual([]);
    expect(filterEmojiOptions("", "🧩").map((item) => item.value)).toEqual([
      "🧩",
    ]);
  });
});
