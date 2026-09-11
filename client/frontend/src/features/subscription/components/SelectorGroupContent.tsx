import { useMemo, type ComponentProps } from "react";
import { useTranslation } from "react-i18next";

import { FeatureHelp } from "../../../components/help/FeatureHelp";
import type { SubscriptionSelector } from "../../../types/subscription";
import { useSubscriptionPageStateField } from "../SubscriptionPageStateContext";
import {
  resolveSelectorPath,
  selectorMemberCounts,
  selectorSelection,
  selectorTypeKey,
  type SelectorIndex,
} from "../selectorNavigation";
import { SelectorNodeGrid } from "./SelectorNodeGrid";
import { SubscriptionWarning } from "./SubscriptionWarning";

type SelectorGroupContentProps = Omit<ComponentProps<typeof SelectorNodeGrid>,
  "currentSelection" | "groupName" | "members" | "onOpenGroup" | "selectionEnabled"
> & {
  root: SubscriptionSelector;
  activeSelections: Readonly<Record<string, string>>;
  rootGroupName: string;
  workspaceKey: string;
  selectorsByName: SelectorIndex;
};

export function SelectorGroupContent({
  root, rootGroupName, workspaceKey, selectorsByName, activeSelections, ...gridProps
}: SelectorGroupContentProps) {
  const { t } = useTranslation();
  const [paths, setPaths] = useSubscriptionPageStateField("nestedSelectorPaths");
  const pathKey = JSON.stringify([workspaceKey, rootGroupName]);
  const requestedPath = paths[pathKey];
  const path = useMemo(
    () => resolveSelectorPath(root, requestedPath ?? [], selectorsByName),
    [root, requestedPath, selectorsByName],
  );
  const current = path[path.length - 1];
  const groupName = path.length === 1 ? rootGroupName : current.name;
  const changePath = (names: string[]) => setPaths((value) => ({ ...value, [pathKey]: names }));
  return (
    <div className="selector-group-content">
      {path.length > 1 ? (
        <div className="selector-nested-heading">
          <nav aria-label={t("subscription.selector.nested.path")} className="selector-breadcrumb">
            {path.map((group, index) => (
              <span key={group.name}>
                {index > 0 ? <span aria-hidden="true" className="selector-breadcrumb-separator">›</span> : null}
                <button
                  type="button"
                  aria-current={index === path.length - 1 ? "location" : undefined}
                  disabled={index === path.length - 1}
                  onClick={() => changePath(path.slice(1, index + 1).map((item) => item.name))}
                  title={group.name}
                >{group.name}</button>
              </span>
            ))}
          </nav>
          <span className="selector-nested-meta" title={t("subscription.selector.memberBreakdown", selectorMemberCounts(current))}>
            {t(selectorTypeKey(current.type))} · {t("subscription.selector.memberCount", { count: current.members.length })}
            <FeatureHelp compact translationBase="subscription.selector.nested.help" />
          </span>
        </div>
      ) : null}
      {current.unresolvedProviderNames.length > 0 ? (
        <SubscriptionWarning
          noticeKey={JSON.stringify(["providers-pending", [...current.unresolvedProviderNames].sort()])}
          description={t("subscription.selector.providersPending", { names: current.unresolvedProviderNames.join(", ") })}
        />
      ) : null}
      <SelectorNodeGrid
        {...gridProps}
        selectorsByName={selectorsByName}
        groupName={groupName}
        currentSelection={selectorSelection(current, activeSelections, gridProps.runtimeReady, groupName)}
        members={current.members}
        selectionEnabled={current.type === "select"}
        onOpenGroup={(name) => {
          const existing = path.findIndex((item) => item.name === name);
          changePath(existing >= 0
            ? path.slice(1, existing + 1).map((item) => item.name)
            : [...path.slice(1).map((item) => item.name), name]);
        }}
      />
    </div>
  );
}
