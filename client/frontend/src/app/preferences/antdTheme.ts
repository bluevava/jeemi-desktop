import { theme, type ThemeConfig } from "antd";

/** CSS semantic tokens are the shared palette for native markup and Ant Design. */
export function createAntdTheme(
  mode: "light" | "dark",
  read: (name: string) => string,
): ThemeConfig {
  const color = (name: string) => read(`--color-${name}`).trim();
  const accent = color("accent");
  const text = color("text");
  const secondary = color("text-secondary");
  const surface = color("surface");
  const muted = color("surface-muted");
  const hover = color("surface-hover");
  const status = (name: "positive" | "warning" | "danger") => ({
    text: color(name),
    background: color(`${name}-soft`),
  });
  const success = status("positive");
  const warning = status("warning");
  const error = status("danger");

  return {
    algorithm: (seed, map) => ({
      ...(mode === "dark" ? theme.darkAlgorithm(seed, map) : theme.defaultAlgorithm(seed)),
      // The dark algorithm also transforms seed colors. Keep the shared palette
      // authoritative after that step, including primary button backgrounds.
      colorPrimary: accent,
      colorSuccess: success.text,
      colorWarning: warning.text,
      colorError: error.text,
      colorInfo: accent,
    }),
    token: {
      colorPrimary: accent,
      colorPrimaryHover: color("accent-hover"),
      colorPrimaryActive: color("accent-strong"),
      colorPrimaryBg: color("accent-soft"),
      colorPrimaryBgHover: color("accent-soft"),
      colorPrimaryText: accent,
      colorPrimaryTextHover: color("accent-hover"),
      colorPrimaryTextActive: color("accent-strong"),
      colorLink: accent,
      colorLinkHover: color("accent-hover"),
      colorLinkActive: color("accent-strong"),
      colorText: text,
      colorTextSecondary: secondary,
      colorTextTertiary: secondary,
      colorTextQuaternary: color("text-placeholder"),
      colorTextDescription: secondary,
      colorTextPlaceholder: color("text-placeholder"),
      colorTextDisabled: color("text-disabled"),
      colorTextLightSolid: color("on-accent"),
      colorIcon: secondary,
      colorIconHover: text,
      colorBgBase: surface,
      colorBgLayout: color("bg"),
      colorBgContainer: color("control"),
      colorBgElevated: surface,
      colorBgContainerDisabled: muted,
      colorBgMask: color("mask"),
      colorBorder: color("control-border"),
      colorBorderSecondary: color("border"),
      colorSplit: color("border"),
      colorFill: hover,
      colorFillSecondary: hover,
      colorFillTertiary: muted,
      colorFillQuaternary: muted,
      controlItemBgHover: hover,
      controlItemBgActive: color("accent-soft"),
      controlItemBgActiveHover: color("accent-soft"),
      colorSuccess: success.text,
      colorSuccessText: success.text,
      colorSuccessBg: success.background,
      colorSuccessBorder: success.text,
      colorWarning: warning.text,
      colorWarningText: warning.text,
      colorWarningBg: warning.background,
      colorWarningBorder: warning.text,
      colorError: error.text,
      colorErrorText: error.text,
      colorErrorBg: error.background,
      colorErrorBorder: error.text,
      colorInfo: accent,
      colorInfoText: accent,
      colorInfoBg: color("accent-soft"),
      colorInfoBorder: accent,
      boxShadow: read("--shadow-popup").trim(),
      boxShadowSecondary: read("--shadow-popup").trim(),
      borderRadius: 12,
      controlHeight: 36,
      controlHeightLG: 42,
      controlHeightSM: 30,
      fontFamily: 'Inter, "SF Pro Display", "Segoe UI", "PingFang SC", sans-serif',
      fontSize: 14,
      fontSizeLG: 16,
      fontSizeSM: 13,
    },
    components: {
      Button: {
        primaryColor: color("on-accent"),
        dangerColor: color("on-danger"),
        defaultShadow: "none",
        primaryShadow: "none",
        dangerShadow: "none",
      },
      Card: { colorBgContainer: surface },
      Segmented: {
        trackBg: muted,
        itemColor: secondary,
        itemHoverColor: text,
        itemHoverBg: hover,
        itemActiveBg: color("accent-soft"),
        itemSelectedBg: color("accent-soft"),
        itemSelectedColor: accent,
      },
      Select: { optionSelectedBg: color("accent-soft"), optionSelectedColor: accent },
      Tree: { nodeSelectedBg: color("accent-soft"), nodeSelectedColor: accent },
      Tooltip: { colorBgSpotlight: color("tooltip"), colorTextLightSolid: color("on-tooltip") },
      Switch: { handleBg: surface, colorTextLightSolid: color("on-accent") },
    },
  };
}
