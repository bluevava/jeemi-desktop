import { describe, expect, it } from "vitest";

import {
  homeSettingsSectionIds,
  homeSettingsSectionPath,
  resolveHomeSettingsSection,
} from "./homeSettingsNavigation";

describe("home settings section navigation", () => {
  it("builds a direct link with both action parameters and a stable anchor", () => {
    expect(homeSettingsSectionPath("geodata", { check: true })).toBe(
      "/home/settings?focus=geodata&check=1#home-settings-geodata",
    );
    expect(homeSettingsSectionPath("zashboard")).toBe("/home/settings?focus=zashboard#home-settings-zashboard");
    expect(resolveHomeSettingsSection("?focus=zashboard", "")).toBe("zashboard");
    expect(resolveHomeSettingsSection("", "#home-settings-zashboard")).toBe("zashboard");
  });

  it("prefers the anchor while retaining compatibility with old focus links", () => {
    expect(
      resolveHomeSettingsSection(
        "?focus=mihomo&check=1",
        `#${homeSettingsSectionIds.geodata}`,
      ),
    ).toBe("geodata");
    expect(
      resolveHomeSettingsSection("?focus=mihomo&check=1", ""),
    ).toBe("mihomo");
    expect(resolveHomeSettingsSection("?focus=geodata", "#%E0%A4%A")).toBe(
      "geodata",
    );
    expect(resolveHomeSettingsSection("?focus=unknown", "#unknown")).toBeNull();
  });
});
