import { describe, expect, it } from "vitest";

import {
  isNavigationSettingsPath,
  isLocalConfigEditorPath,
  isLogPageAvailable,
  navigationItemForPath,
  navigationItems,
  visibleNavigationItems,
} from "./navigation";

describe("primary navigation", () => {
  it("keeps the six product areas in the required order", () => {
    expect(navigationItems.map((item) => item.id)).toEqual([
      "home",
      "subscriptions",
      "config",
      "connections",
      "logs",
      "tools",
    ]);
  });

  it("provides one dedicated settings route for every primary page", () => {
    const settingsPaths = navigationItems.map((item) => item.settingsPath);

    expect(new Set(settingsPaths).size).toBe(navigationItems.length);
    for (const item of navigationItems) {
      expect(item.settingsPath).toBe(`${item.path}/settings`);
      expect(isNavigationSettingsPath(item.settingsPath)).toBe(true);
      expect(isNavigationSettingsPath(`${item.settingsPath}/`)).toBe(true);
      expect(navigationItemForPath(item.settingsPath).id).toBe(item.id);
    }
  });

  it("hides the log entry while mihomo logging is silent", () => {
    expect(visibleNavigationItems("silent").map((item) => item.id)).toEqual([
      "home",
      "subscriptions",
      "config",
      "connections",
      "tools",
    ]);
    expect(visibleNavigationItems(null).some((item) => item.id === "logs")).toBe(
      false,
    );
    for (const level of ["error", "warning", "info", "debug"] as const) {
      expect(isLogPageAvailable(level)).toBe(true);
      expect(visibleNavigationItems(level).map((item) => item.id)).toEqual(
        navigationItems.map((item) => item.id),
      );
    }
    expect(isLogPageAvailable("silent")).toBe(false);
    expect(isLogPageAvailable(null)).toBe(false);
  });

  it("recognises local configuration and script editor routes", () => {
    expect(isLocalConfigEditorPath("/config/new")).toBe(true);
    expect(
      isLocalConfigEditorPath(
        "/config/0123456789abcdef0123456789abcdef/edit",
      ),
    ).toBe(true);
    expect(isLocalConfigEditorPath("/config/scripts/new")).toBe(true);
    expect(
      isLocalConfigEditorPath(
        "/config/scripts/0123456789abcdef0123456789abcdef/edit",
      ),
    ).toBe(true);
    expect(isLocalConfigEditorPath("/config/settings")).toBe(false);
    expect(isLocalConfigEditorPath("/config/not-an-id/edit")).toBe(false);
  });
});
