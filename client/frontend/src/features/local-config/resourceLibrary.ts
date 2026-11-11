import type {
  LocalConfigResourceState,
  RuleSetResource,
  StrategyGroupResource,
} from "../../types/localConfig";
import { nameMatcher } from "../subscription/selectorSearch";

export type ResourceKind = "groups" | "rules";
export interface ResourceFilters {
  query: string;
  config: string;
  groups: string[];
  kind: string;
}
export const ALL_RESOURCES = "all";
export const UNLINKED_RESOURCES = "unlinked";
export const defaultResourceFilters = (): ResourceFilters => ({
  query: "",
  config: ALL_RESOURCES,
  groups: [],
  kind: ALL_RESOURCES,
});

export function resourceOwners(state: LocalConfigResourceState, id: string) {
  return state.strategyGroups.filter((group) =>
    group.ruleSetReferences.some((ref) => ref.ruleSetId === id),
  );
}
export function resourceConfigurations(
  state: LocalConfigResourceState,
  kind: ResourceKind,
  id: string,
) {
  const groupIds =
    kind === "groups"
      ? [id]
      : resourceOwners(state, id).map((group) => group.id);
  return (state.configReferences ?? []).filter((config) =>
    config.strategyGroupIds.some((group) => groupIds.includes(group)),
  );
}
export function eligibleGroupOptions(
  state: LocalConfigResourceState,
  config: string,
) {
  return state.strategyGroups.filter((group) => {
    const refs = resourceConfigurations(state, "groups", group.id);
    return (
      config === ALL_RESOURCES ||
      (config === UNLINKED_RESOURCES
        ? !refs.length
        : refs.some((item) => item.id === config))
    );
  });
}
export function filterResources<
  T extends StrategyGroupResource | RuleSetResource,
>(
  items: T[],
  state: LocalConfigResourceState,
  kind: ResourceKind,
  filter: ResourceFilters,
): T[] {
  const matches = nameMatcher(filter.query);
  return items.filter((item) => {
    const refs = resourceConfigurations(state, kind, item.id);
    if (
      filter.config === UNLINKED_RESOURCES
        ? refs.length > 0
        : filter.config !== ALL_RESOURCES &&
          !refs.some((ref) => ref.id === filter.config)
    )
      return false;
    if (
      kind === "groups" &&
      filter.kind !== ALL_RESOURCES &&
      (item as StrategyGroupResource).kind !== filter.kind
    )
      return false;
    if (kind === "rules" && filter.groups.length) {
      const owners = resourceOwners(state, item.id);
      if (
        !owners.some((group) => filter.groups.includes(group.id)) &&
        !(filter.groups.includes(UNLINKED_RESOURCES) && !owners.length)
      )
        return false;
    }
    return matches(
      `${item.emoji ?? ""} ${item.name} ${kind === "rules" ? (item as RuleSetResource).payload.join(" ") : ""}`,
    );
  });
}
export function resourceDeletionPreview(
  state: LocalConfigResourceState,
  kind: ResourceKind,
  ids: string[],
) {
  const items = kind === "groups" ? state.strategyGroups : state.ruleSets;
  const selected = items.filter((item) => ids.includes(item.id));
  const blocked = selected.flatMap((item) => {
    const parents =
      kind === "groups"
        ? resourceConfigurations(state, kind, item.id)
        : resourceOwners(state, item.id);
    return parents.length ? [{ item, parents }] : [];
  });
  return {
    deletable: selected.filter(
      (item) => !blocked.some((entry) => entry.item.id === item.id),
    ),
    blocked,
  };
}
