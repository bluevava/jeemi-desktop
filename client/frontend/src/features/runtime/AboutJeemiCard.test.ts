import { renderToStaticMarkup } from "react-dom/server";
import { createElement } from "react";
import { MemoryRouter } from "react-router-dom";
import { App } from "antd";
import { describe, expect, it, vi } from "vitest";

const fixture = vi.hoisted(() => ({ present: false, health: "not_installed" }));
vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: (key: string) => key }) }));
vi.mock("../../app/runtime/RuntimeStatusContext", () => ({ useRuntimeStatus: () => ({
  bootstrap: { app: { version: "1.2.3" } }, runtime: { core: { version: "v1.19.0" }, platform: { os: "windows", architecture: "amd64" } },
  removeAuthorization: vi.fn(), actionBusy: false, removingAuthorization: false,
}) }));
vi.mock("./useAuthorizationHelperStatus", () => ({ useAuthorizationHelperStatus: () => fixture }));
vi.mock("../update/JeemiUpdateProvider", () => ({ useJeemiUpdate: () => ({ checking: false, busy: false, check: vi.fn() }) }));

import { AboutJeemiCard } from "./AboutJeemiCard";

const render = () => renderToStaticMarkup(createElement(App, null, createElement(MemoryRouter, null, createElement(AboutJeemiCard))));

describe("About card actions", () => {
  it("shows version actions and omits uninstall when the helper is absent", () => {
    fixture.present = false;
    fixture.health = "not_installed";
    const html = render();
    expect(html).toContain("appUpdate.check");
    expect(html).toContain("home.about.manageCore");
    expect(html).toContain("home.about.helperStates.not_installed");
    expect(html).not.toContain("home.about.removeHelper");
    expect(html).not.toContain("question-circle");
    expect(html).toContain("windows / amd64");
  });
  it("keeps uninstall available for installed but unhealthy helpers", () => {
    for (const health of ["ready", "failed", "update"]) {
      fixture.present = true;
      fixture.health = health;
      const html = render();
      expect(html).toContain("home.about.removeHelper");
      expect(html).toContain(`home.about.helperStates.${health}`);
    }
  });
});
