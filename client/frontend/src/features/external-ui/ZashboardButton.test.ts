import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { createElement } from "react";
import { ZashboardButton } from "./ZashboardButton";

const fixture = vi.hoisted(() => ({ enabled: false, running: false }));
vi.mock("../../app/runtime/RuntimeStatusContext", () => ({
  useRuntimeStatus: () => ({
    runtimePreferences: { externalUIEnabled: fixture.enabled },
    runtime: { mihomo: { state: fixture.running ? "running" : "stopped", controllerReady: fixture.running,
      controllerSession: { baseUrl: "http://127.0.0.1:12345", secret: "PRIVATE_SESSION_FIXTURE" } } },
  }),
}));
vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: (key: string) => key }) }));

describe("zashboard browser entry", () => {
  beforeEach(() => { fixture.enabled = false; fixture.running = false; });
  it("stays hidden by default", () => {
    expect(renderToStaticMarkup(createElement(ZashboardButton))).toBe("");
  });
  it("shows a disabled entry while the core is stopped", () => {
    fixture.enabled = true;
    const html = renderToStaticMarkup(createElement(ZashboardButton));
    expect(html).toContain('aria-label="externalUI.open"');
    expect(html).toContain('disabled=""');
  });
  it("lets Go open the current session without placing its secret in the DOM", () => {
    fixture.enabled = true; fixture.running = true;
    const html = renderToStaticMarkup(createElement(ZashboardButton));
    expect(html).not.toContain('disabled=""');
    expect(html).toContain("zashboard.ico");
    expect(html).not.toContain("PRIVATE_SESSION_FIXTURE");
    expect(html).not.toContain("127.0.0.1:12345");
  });
});
