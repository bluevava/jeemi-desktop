import type { SelectProps } from "antd";
import { renderFlagText } from "./FlagText";

/** Render SVG flags without changing option labels used for searching. */
export const flagSelectRenderers = {
  optionRender: (option) => renderFlagText(option.label),
  labelRender: ({ label, value }) => renderFlagText(label ?? value),
} satisfies Pick<SelectProps, "optionRender" | "labelRender">;
