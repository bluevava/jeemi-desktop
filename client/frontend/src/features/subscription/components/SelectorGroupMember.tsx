import { useTranslation } from "react-i18next";

import type { SubscriptionSelector } from "../../../types/subscription";
import { selectorTypeKey } from "../selectorNavigation";
import type { ProxyDelayResult } from "../useProxyDelayQueue";
import { SelectorDelayBadge } from "./SelectorDelayBadge";
import { SelectorIcon } from "./SelectorIcon";

interface SelectorGroupMemberProps {
  name: string;
  selector?: SubscriptionSelector;
  delay?: ProxyDelayResult;
  delayBusy: boolean;
  selected: boolean;
  selectionDisabled: boolean;
  onSelect: () => void;
  onOpen: () => void;
}

export function SelectorGroupMember({
  name, selector, delay, delayBusy, selected, selectionDisabled, onSelect, onOpen,
}: SelectorGroupMemberProps) {
  const { t } = useTranslation();
  return (
    <div className={`selector-node selector-group-member${selected ? " default" : ""}`}>
      <button
        type="button"
        aria-pressed={selected}
        className="selector-node-select"
        disabled={selectionDisabled}
        onClick={onSelect}
      >
        <strong className="selector-group-member-heading" title={name}>
          {selector ? <SelectorIcon selector={selector} configuredOnly /> : null}
          <span className="selector-group-member-name">{name}</span>
        </strong>
        <span className="selector-group-member-type" title={t(selectorTypeKey(selector?.type ?? ""))}>
          {t(selectorTypeKey(selector?.type ?? ""))}
        </span>
      </button>
      <SelectorDelayBadge
        result={delay}
        busy={delayBusy}
        idle="—"
        label={t("subscription.selector.nested.open", { name })}
        title={t(selector ? "subscription.selector.nested.delayNavigation" : "subscription.selector.nested.unavailable")}
        disabled={!selector}
        onClick={onOpen}
      />
    </div>
  );
}
