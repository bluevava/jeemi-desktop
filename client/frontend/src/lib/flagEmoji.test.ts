import { readFileSync, readdirSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { COUNTRY_REGION_CODES, countryFlag } from "./countryFlags";
import { flagEmojiCode, leadingFlagEmoji, splitFlagText } from "./flagEmoji";

const subdivision = (tag: string) => String.fromCodePoint(
  0x1f3f4, ...[...tag].map((character) => 0xe0000 + character.charCodeAt(0)), 0xe007f,
);

describe("flag Emoji rendering", () => {
  it("covers every country and region in the picker, including CN, TW and CQ", () => {
    expect(COUNTRY_REGION_CODES).toHaveLength(259);
    for (const code of COUNTRY_REGION_CODES) {
      expect(flagEmojiCode(countryFlag(code))).toBe(code.toLowerCase());
    }
    expect(flagEmojiCode("🇨🇳")).toBe("cn");
    expect(flagEmojiCode("🇹🇼")).toBe("tw");
    expect(flagEmojiCode("🇭🇰")).toBe("hk");
    expect(flagEmojiCode("🇲🇴")).toBe("mo");
    expect(flagEmojiCode("🇨🇶")).toBe("cq");
  });

  it("recognizes complete subdivision sequences instead of showing a black flag", () => {
    for (const [tag, code] of [["gbeng", "gb-eng"], ["gbsct", "gb-sct"], ["gbwls", "gb-wls"]]) {
      const emoji = subdivision(tag);
      expect(flagEmojiCode(emoji)).toBe(code);
      expect(leadingFlagEmoji(`${emoji} node`)).toBe(emoji);
      expect(splitFlagText(`node ${emoji}`)).toEqual([{ text: "node " }, { text: emoji, code }]);
    }
    expect(flagEmojiCode(subdivision("unknown"))).toBeUndefined();
  });

  it("renders multiple flags anywhere in text while retaining the exact original string", () => {
    const text = "🇯🇵Pro.ISP.jp.3x → 🇹🇼Pro.ISP.tw.2x\nUS 🇺🇸🇨🇳 end🇭🇰";
    const parts = splitFlagText(text);
    expect(parts.filter((part) => part.code).map((part) => part.code)).toEqual(["jp", "tw", "us", "cn", "hk"]);
    expect(parts.map((part) => part.text).join("")).toBe(text);
    expect(splitFlagText("🇯🇵️ node")[0]).toEqual({ text: "🇯🇵️", code: "jp" });
  });

  it("leaves ordinary Emoji, text, invalid codes and incomplete flags untouched", () => {
    for (const text of ["", "JP and TW", "🐶👩‍💻🏳️‍🌈🏴‍☠️", "🇿🇿", "🇯", "🏴", "../tw", subdivision("unknown")]) {
      expect(splitFlagText(text)).toEqual([{ text }]);
      expect(flagEmojiCode(text)).toBeUndefined();
    }
    expect(leadingFlagEmoji(" 🇯🇵 node")).toBe("");
    expect(leadingFlagEmoji("node 🇯🇵")).toBe("");
  });
});

describe("bundled flag assets", () => {
  it("ships self-contained SVGs for all recognized flags with their license notices", () => {
    const directory = new URL("../../public/flags/", import.meta.url);
    const codes = [...COUNTRY_REGION_CODES.map((code) => code.toLowerCase()), "gb-eng", "gb-sct", "gb-wls"];
    expect(readdirSync(directory).filter((file) => file.endsWith(".svg")).sort())
      .toEqual(codes.map((code) => `${code}.svg`).sort());
    for (const code of codes) {
      const svg = readFileSync(new URL(`${code}.svg`, directory), "utf8");
      expect(svg, code).toMatch(/<svg\b[^>]*viewBox=/u);
      expect(svg, code).not.toMatch(/<\s*(?:script|foreignObject|image)\b|\bon\w+\s*=|(?:href|src)\s*=\s*["'](?!#)|(?:javascript|data):/iu);
      expect(svg, code).not.toMatch(/url\(\s*["']?https?:/iu);
    }
    expect(readFileSync(new URL("LICENSE.flag-icons.txt", directory), "utf8")).toContain("Panayiotis Lipiridis");
    expect(readFileSync(new URL("LICENSE.twemoji.txt", directory), "utf8")).toContain("Attribution 4.0 International");
  });
});
