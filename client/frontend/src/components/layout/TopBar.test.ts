import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import { TopBar } from "./TopBar";

const fixture = vi.hoisted(() => ({ platform: undefined as string | undefined }));
vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: (key: string) => key }) }));
vi.mock("react-router-dom", () => ({
  useLocation: () => ({ pathname: "/home" }),
  useNavigate: () => vi.fn(),
}));
vi.mock("../../app/navigationGuard/NavigationGuardContext", () => ({
  useNavigationGuard: () => ({ canLeave: () => true }),
}));
vi.mock("../../app/runtime/RuntimeStatusContext", () => ({
  useRuntimeStatus: () => ({
    runtime: fixture.platform === undefined ? null : {
      platform: { os: fixture.platform }, core: { state: "ready" },
      mihomo: { state: "stopped", desiredRunning: false },
    },
    actionBusy: false, start: vi.fn(), restart: vi.fn(),
  }),
}));
vi.mock("../help/FeatureHelp", () => ({
  FeatureHelp: () => createElement("button", { "aria-label": "page-help" }),
}));
vi.mock("./TopBarRuntimeIndicators", () => ({
  TopBarRuntimeIndicators: () => createElement("span", { className: "top-bar-runtime-indicators" }, "traffic / exits"),
}));

afterEach(() => { fixture.platform = undefined; vi.unstubAllGlobals(); });

describe("platform title bar", () => {
  it.each([undefined, "darwin"])("reserves native buttons before and after macOS bootstrap (%s)", (platform) => {
    vi.stubGlobal("navigator", { userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 wails.io" });
    fixture.platform = platform;
    const markup = renderToStaticMarkup(createElement(TopBar));
    expect(markup).toContain('class="top-bar top-bar-native"');
    for (const action of ["minimise", "maximise", "close"]) {
      expect(markup).not.toContain(`aria-label="action.${action}"`);
    }
    expect(markup).not.toContain("window-control-divider");
    for (const sharedControl of ["top-bar-app-name", "page-help", "top-bar-page-title", "page-settings-button", "top-bar-runtime-indicators", "runtime-power-button"]) {
      expect(markup).toContain(sharedControl);
    }
  });

  it.each(["windows", "linux", "browser"])("keeps custom controls on %s using the backend platform", (platform) => {
    vi.stubGlobal("navigator", { userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)" });
    fixture.platform = platform;
    const markup = renderToStaticMarkup(createElement(TopBar));
    expect(markup).not.toContain("top-bar-native");
    for (const action of ["minimise", "maximise", "close"]) {
      expect(markup).toContain(`aria-label="action.${action}"`);
    }
  });
});
