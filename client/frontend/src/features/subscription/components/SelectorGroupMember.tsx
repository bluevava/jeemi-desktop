import { RightOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";

import type { SubscriptionSelector } from "../../../types/subscription";
import { selectorMemberCounts, selectorTypeKey } from "../selectorNavigation";
import { selectorPresentation } from "../selectorPresentation";
import { SelectorIcon } from "./SelectorIcon";

interface SelectorGroupMemberProps {
  name: string;
  selector?: SubscriptionSelector;
  egress: string;
  selected: boolean;
  selectionDisabled: boolean;
  runtimeReady: boolean;
  onSelect: () => void;
  onOpen: () => void;
}

export function SelectorGroupMember({
  name, selector, egress, selected, selectionDisabled, runtimeReady, onSelect, onOpen,
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
        <span className="selector-group-member-heading">
          {selector ? <SelectorIcon selector={selector} /> : null}
          <strong title={name}>{selector ? selectorPresentation(name, selector.icon).displayName : name}</strong>
        </span>
        <span className="selector-group-member-type" title={t(selectorTypeKey(selector?.type ?? ""))}>
          {t(selectorTypeKey(selector?.type ?? ""))}
        </span>
        <span className="selector-group-member-current" title={t(
          runtimeReady ? "subscription.selector.egress.live" : "subscription.selector.egress.offline",
          { path: egress },
        )}>{egress}</span>
      </button>
      <button
        type="button"
        className="selector-node-open"
        aria-label={t("subscription.selector.nested.open", { name })}
        title={selector ? t("subscription.selector.memberBreakdown", selectorMemberCounts(selector)) : t("subscription.selector.nested.unavailable")}
        disabled={!selector}
        onClick={onOpen}
      ><RightOutlined /></button>
    </div>
  );
}
