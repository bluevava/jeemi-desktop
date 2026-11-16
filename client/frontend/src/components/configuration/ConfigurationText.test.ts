import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import { ConfigurationText } from "./ConfigurationText";
import { findConfigurationMatches } from "./textSearch";

vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: (key: string) => key }) }));

const render = (contents: string, query: string, activeMatch = 0) => renderToStaticMarkup(createElement(
  ConfigurationText, { contents, matches: findConfigurationMatches(contents, query), activeMatch },
));

describe("read-only highlighted configuration", () => {
  it("highlights results and preserves the exact escaped source text", () => {
    const text = "# 🇭🇰 香港\nproxies:\n  - name: '<script>alert(1)</script>'\n    path: /a<b\n# PATH";
    const html = render(text, "path", 1);
    expect(html.match(/<mark/g)).toHaveLength(2);
    expect(html).toContain('<mark class="is-current">PATH</mark>');
    expect(html).not.toContain("<script>");
    expect(html.replace(/<mark[^>]*>|<\/mark>/g, "")).toBe(render(text, ""));
  });

  it("can highlight the final result of a large file without creating a huge DOM", () => {
    const text = "node: same\n".repeat(10000);
    const html = render(text, "same", 9999);
    expect(html.match(/<mark/g)).toHaveLength(1);
    expect(html).toContain('<mark class="is-current">same</mark>\n</pre>');
    expect(html.replace(/<mark[^>]*>|<\/mark>/g, "")).toBe(render(text, ""));
  });

  it("does not expose hidden fields that are absent from the provided preview", () => {
    const redacted = 'password: "***"\n';
    expect(render(redacted, "secret")).not.toContain("<mark");
    expect(render(redacted, "***")).toContain('<mark class="is-current">***</mark>');
  });
});
