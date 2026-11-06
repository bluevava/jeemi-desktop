import { memo, type ReactNode } from "react";
import { splitFlagText } from "../../lib/flagEmoji";
import "./flag-text.css";

/** Keep the original text for selection, copying and assistive technology. */
export const FlagText = memo(function FlagText({ children }: { children: string }) {
  return splitFlagText(children).map((part, index) => part.code ? (
    <span
      className="flag-emoji"
      key={index}
      style={{ backgroundImage: `url("${import.meta.env.BASE_URL}flags/${part.code}.svg")` }}
    >
      <span className="flag-emoji-source">{part.text}</span>
    </span>
  ) : part.text);
});

/** Ant Design options keep string labels for filtering and original values. */
export function renderFlagText(value: ReactNode): ReactNode {
  return typeof value === "string" ? <FlagText>{value}</FlagText> : value;
}
