import { afterEach, describe, expect, it, vi } from "vitest";
import { initialTheme, readUIPreference, writeUIPreference } from "./uiPreferences";
import { initialLanguage } from "../i18n/preferenceLanguage";

afterEach(() => vi.unstubAllGlobals());

describe("optional UI preference storage", () => {
  it("survives denied storage and quota errors without losing in-memory UI", () => {
    vi.stubGlobal("localStorage", {
      getItem: () => { throw new Error("denied"); },
      setItem: () => { throw new Error("quota"); },
    });
    vi.stubGlobal("matchMedia", () => ({ matches: true }));
    vi.stubGlobal("navigator", { language: "en-GB" });
    expect(readUIPreference("jeemi.ui.theme")).toBeNull();
    expect(() => writeUIPreference("jeemi.ui.theme", "dark")).not.toThrow();
    expect(initialTheme()).toBe("dark");
    expect(initialLanguage()).toBe("en-US");
  });

  it("supports missing browser APIs and ignores invalid stored themes", () => {
    vi.stubGlobal("localStorage", { getItem: () => "invalid" });
    vi.stubGlobal("matchMedia", undefined);
    expect(initialTheme()).toBe("light");
    vi.stubGlobal("localStorage", undefined);
    expect(() => writeUIPreference("jeemi.ui.language", "zh-CN")).not.toThrow();
  });
});
