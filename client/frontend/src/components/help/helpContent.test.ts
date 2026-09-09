import { createInstance } from "i18next";
import { describe, expect, it } from "vitest";
import { resources } from "../../i18n/resources";
import { helpSections, isHelpContent, resolveHelpContent } from "./helpContent";

async function translator(language = "zh-CN") {
  const i18n = createInstance();
  await i18n.init({ resources, lng: language, fallbackLng: "zh-CN", interpolation: { escapeValue: false } });
  return i18n;
}

describe("feature help content", () => {
  it("keeps warnings and examples together in the third section", async () => {
    const { t } = await translator();
    const content = resolveHelpContent(t, "help.topics.subscriptionNodeSearch");
    const sections = helpSections(content);

    expect(sections.map(({ key }) => key)).toEqual(["purpose", "scenarios", "cautionsAndExample"]);
    expect(sections[2].paragraphs).toEqual([content.cautions, content.example]);
    expect(sections[2].caution).toBe(true);
    expect(content.cautions).toContain("| 或 → & 与 → ! 排除");
    expect(content.example).toContain("hk & gm & !ev | jp");
  });

  it("does not add a fallback warning to an example-only topic", async () => {
    const { t } = await translator();
    const content = resolveHelpContent(t, "help.topics.emptySettings", "help.topics.appUpdate");
    const sections = helpSections(content);

    expect(content.cautions).toBeUndefined();
    expect(sections.map(({ key }) => key)).toEqual(["purpose", "scenarios", "example"]);
    expect(sections[2].caution).toBe(false);
    expect(sections[2].paragraphs).toEqual([content.example]);
    expect(t("help.section.example")).toBe("示例");
  });

  it("uses notes without an empty example when only operational impacts apply", async () => {
    const { t } = await translator("en-US");
    const content = resolveHelpContent(t, "help.topics.appUpdate");
    const sections = helpSections(content);

    expect(sections.map(({ key }) => key)).toEqual(["purpose", "scenarios", "cautions"]);
    expect(sections[2].paragraphs).toEqual([content.cautions]);
    expect(t("help.section.cautions")).toBe("Notes");
  });

  it("resolves dynamic field fallback as a whole topic with interpolated values", async () => {
    const { t } = await translator();
    const content = resolveHelpContent(t, "localConfig.fieldHelp.unknown", "localConfig.fieldHelp.generic", { field: "new-field" });

    expect(content.title).toBe("new-field 字段");
    expect(content.purpose).toContain("new-field");
    expect(content.purpose).not.toContain("{{field}}");
    expect(helpSections(content)).toHaveLength(3);
  });

  it("keeps imported help and language changes working", async () => {
    const i18n = await translator();
    expect(resolveHelpContent(i18n.t, "subscription.normalization.help").title).toBe("订阅格式转换");
    await i18n.changeLanguage("en-US");
    expect(resolveHelpContent(i18n.t, "subscription.normalization.help").title).toBe("Subscription conversion");
    expect(resolveHelpContent(i18n.t, "help.topics.emptySettings").example).toContain("Home settings");
  });

  it("falls back to the generic default for unknown or incomplete topics", async () => {
    const { t } = await translator();
    const expected = resolveHelpContent(t, "help.topics.localConfigField");
    expect(resolveHelpContent(t, "missing-topic", "missing-fallback")).toEqual(expected);
    expect(resolveHelpContent(t, "help.section")).toEqual(expected);
  });

  it("requires a meaningful third section and rejects empty or malformed content", () => {
    const base = { title: "Topic", purpose: "Function", scenarios: "Situation" };
    expect(isHelpContent({ ...base, example: "Example" })).toBe(true);
    expect(isHelpContent({ ...base, cautions: "Impact" })).toBe(true);
    for (const invalid of [null, "missing", base, { ...base, cautions: " " }, { ...base, example: 1 }, { ...base, cautions: "Impact", example: "" }]) {
      expect(isHelpContent(invalid)).toBe(false);
    }
  });
});
