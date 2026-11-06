import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { FlagText } from "./FlagText";
import { flagSelectRenderers } from "./flagSelectRenderers";

describe("FlagText", () => {
  it("uses bundled SVGs and retains the original characters for copying and accessibility", () => {
    const html = renderToStaticMarkup(createElement(FlagText, { children: "🇹🇼Pro.ISP.tw.2x 🇯🇵" }));
    expect(html).toContain("flags/tw.svg");
    expect(html).toContain("flags/jp.svg");
    expect(html).toContain('<span class="flag-emoji-source">🇹🇼</span>');
    expect(html.replace(/<[^>]*>/gu, "")).toBe("🇹🇼Pro.ISP.tw.2x 🇯🇵");
    expect(html).not.toContain("https://");
    expect(html).not.toContain('aria-hidden="true"');
  });

  it("preserves plain text and escapes markup from names", () => {
    expect(renderToStaticMarkup(createElement(FlagText, { children: "JP 🐶 ordinary name" })))
      .toBe("JP 🐶 ordinary name");
    const html = renderToStaticMarkup(createElement(FlagText, { children: "🇺🇸 <script>alert(1)</script>" }));
    expect(html).toContain("&lt;script&gt;");
    expect(html).not.toContain("<script>");
  });

  it("renders selected labels without modifying the values and search labels", () => {
    const option = { label: "🇹🇼 Taiwan", value: "selector:🇹🇼 Taiwan" };
    const rendered = flagSelectRenderers.labelRender(option);
    expect(renderToStaticMarkup(createElement("span", null, rendered))).toContain("flags/tw.svg");
    expect(option).toEqual({ label: "🇹🇼 Taiwan", value: "selector:🇹🇼 Taiwan" });
    expect(renderToStaticMarkup(createElement("span", null,
      flagSelectRenderers.labelRender({ label: undefined, value: "missing" }),
    ))).toBe("<span>missing</span>");
  });
});
