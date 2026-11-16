import { useEffect, useRef, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { visibleConfigurationMatches } from "./textSearch";

interface ConfigurationTextProps {
  contents: string;
  matches: Uint32Array;
  activeMatch: number;
}

export function ConfigurationText({ contents, matches, activeMatch }: ConfigurationTextProps) {
  const { t } = useTranslation();
  const container = useRef<HTMLPreElement>(null);
  const active = useRef<HTMLElement>(null);

  useEffect(() => {
    const frame = requestAnimationFrame(() => {
      const viewport = container.current;
      const hit = active.current;
      if (!viewport || !hit) return;
      const box = viewport.getBoundingClientRect();
      const target = hit.getBoundingClientRect();
      const top = box.top + viewport.clientTop;
      const left = box.left + viewport.clientLeft;
      // Scroll only the text viewport, never the dialog, page or application.
      if (target.top < top || target.bottom > top + viewport.clientHeight) {
        viewport.scrollTop += target.top - top - (viewport.clientHeight - target.height) / 2;
      }
      if (target.left < left || target.right > left + viewport.clientWidth) {
        viewport.scrollLeft += target.left - left - (viewport.clientWidth - target.width) / 2;
      }
    });
    return () => cancelAnimationFrame(frame);
  }, [contents, matches, activeMatch]);

  const parts: ReactNode[] = [];
  let offset = 0;
  for (const index of visibleConfigurationMatches(matches, activeMatch)) {
    const start = matches[index * 2];
    const end = matches[index * 2 + 1];
    parts.push(contents.slice(offset, start));
    parts.push(
      <mark
        className={index === activeMatch ? "is-current" : undefined}
        key={start}
        ref={index === activeMatch ? active : undefined}
      >{contents.slice(start, end)}</mark>,
    );
    offset = end;
  }
  parts.push(contents.slice(offset));

  return <pre
    aria-label={t("configurationViewer.content")}
    className="configuration-viewer-text"
    ref={container}
    tabIndex={0}
  >{parts}</pre>;
}
