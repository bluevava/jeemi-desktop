import { AimOutlined } from "@ant-design/icons";
import type { SubscriptionSelector } from "../../../types/subscription";
import { selectorPresentation } from "../selectorPresentation";

export function SelectorIcon({ selector }: { selector: SubscriptionSelector }) {
  const presentation = selectorPresentation(selector.name, selector.icon);
  return (
    <span aria-hidden="true" className="selector-group-icon">
      {presentation.emoji || <AimOutlined />}
      {presentation.iconUrl ? (
        <img
          alt=""
          loading="lazy"
          onError={(event) => { event.currentTarget.hidden = true; }}
          referrerPolicy="no-referrer"
          src={presentation.iconUrl}
        />
      ) : null}
    </span>
  );
}
