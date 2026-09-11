import { AimOutlined } from "@ant-design/icons";
import { useState } from "react";
import type { SubscriptionSelector } from "../../../types/subscription";
import { selectorPresentation } from "../selectorPresentation";
import { useSelectorIcon } from "../useSelectorIcon";

export function SelectorIcon({ selector, configuredOnly = false }: {
  selector: SubscriptionSelector;
  configuredOnly?: boolean;
}) {
  const presentation = selectorPresentation(selector.name, selector.icon);
  const source = useSelectorIcon(presentation.iconUrl);
  const [failedSource, setFailedSource] = useState("");
  if (configuredOnly && (!presentation.iconUrl || !source || failedSource === source)) return null;
  return (
    <span aria-hidden="true" className="selector-group-icon">
      {configuredOnly ? null : presentation.emoji || <AimOutlined />}
      {source ? (
        <img
          alt=""
          key={presentation.iconUrl}
          loading="lazy"
          hidden={failedSource === source}
          onError={() => setFailedSource(source)}
          onLoad={() => setFailedSource("")}
          referrerPolicy="no-referrer"
          src={source}
        />
      ) : null}
    </span>
  );
}
