/// <reference types="node" />

import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { theme } from "antd";

import { createAntdTheme } from "./antdTheme";

const css = readFileSync(new URL("../../styles/tokens.css", import.meta.url), "utf8");
const palettes = [...css.matchAll(/:root(?:\[data-theme="dark"\])?\s*\{([^}]+)\}/g)]
  .map((block) => Object.fromEntries([...block[1].matchAll(/(--[\w-]+):\s*([^;]+);/g)]
    .map((match) => [match[1], match[2].trim()])));

describe.each(["light", "dark"] as const)("%s theme", (mode) => {
  const tokens = { ...palettes[0], ...(mode === "dark" ? palettes[1] : {}) };
  const color = (name: string) => tokens[`--color-${name}`];

  it("keeps enabled text readable across neutral and selected surfaces", () => {
    for (const foreground of ["text", "text-secondary", "accent", "positive", "warning", "danger"]) {
      for (const background of ["bg", "surface", "surface-muted", "control"]) {
        expect(contrast(color(foreground), color(background)), `${foreground} on ${background}`).toBeGreaterThanOrEqual(4.5);
      }
    }
    for (const state of ["accent", "positive", "warning", "danger"]) {
      expect(contrast(color(state), color(`${state}-soft`)), `${state} status`).toBeGreaterThanOrEqual(4.5);
    }
    for (const accent of ["accent", "accent-hover", "accent-strong"]) {
      expect(contrast(color("on-accent"), color(accent)), `primary button ${accent}`).toBeGreaterThanOrEqual(4.5);
    }
    expect(contrast(color("on-danger"), color("danger"))).toBeGreaterThanOrEqual(4.5);
    expect(contrast(color("text-placeholder"), color("control"))).toBeGreaterThanOrEqual(4.5);
    expect(contrast(color("on-tooltip"), color("tooltip"))).toBeGreaterThanOrEqual(4.5);
  });

  it("keeps input outlines visible on adjacent surfaces", () => {
    for (const surface of ["surface", "surface-muted", "control"]) {
      expect(contrast(color("control-border"), color(surface)), surface).toBeGreaterThanOrEqual(3);
    }
  });

  it("uses the CSS palette after the Ant Design algorithm resolves", () => {
    const config = createAntdTheme(mode, (name) => tokens[name]);
    const resolved = theme.getDesignToken(config);
    expect(resolved.colorText).toBe(color("text"));
    expect(resolved.colorBgElevated).toBe(color("surface"));
    expect(resolved.colorPrimary).toBe(color("accent"));
    expect(resolved.colorTextLightSolid).toBe(color("on-accent"));
    expect(contrast(resolved.colorText, resolved.colorBgContainer)).toBeGreaterThanOrEqual(4.5);
    expect(contrast(resolved.colorTextLightSolid, resolved.colorPrimary)).toBeGreaterThanOrEqual(4.5);
  });
});

function contrast(foreground: string, background: string): number {
  const luminance = (hex: string) => {
    const channels = hex.slice(1).match(/../g)!.map((channel) => {
      const value = parseInt(channel, 16) / 255;
      return value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
    });
    return channels[0] * 0.2126 + channels[1] * 0.7152 + channels[2] * 0.0722;
  };
  const values = [luminance(foreground), luminance(background)].sort((a, b) => b - a);
  return (values[0] + 0.05) / (values[1] + 0.05);
}
