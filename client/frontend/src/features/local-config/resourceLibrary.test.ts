import { describe, expect, it } from "vitest";
import type { LocalConfigResourceState } from "../../types/localConfig";
import { emptyRuleSet, emptyStrategyGroup } from "./resourceModel";
import {
  defaultResourceFilters,
  eligibleGroupOptions,
  filterResources,
  resourceDeletionPreview,
} from "./resourceLibrary";
import { resourcePresentation } from "./resourcePresentation";

function fixture(): LocalConfigResourceState {
  return {
    directory: "",
    revision: 4,
    updatedAt: "",
    platform: "darwin",
    strategyGroups: [
      {
        ...emptyStrategyGroup(),
        id: "im",
        name: "IM",
        disabled: true,
        ruleSetReferences: [{ ruleSetId: "telegram" }],
      },
      {
        ...emptyStrategyGroup(),
        id: "video",
        name: "Video",
        ruleSetReferences: [{ ruleSetId: "youtube" }],
      },
      {
        ...emptyStrategyGroup("rule"),
        id: "orphan",
        name: "Orphan",
        ruleSetReferences: [{ ruleSetId: "chrome" }],
      },
    ],
    ruleSets: [
      {
        ...emptyRuleSet(),
        id: "telegram",
        name: "✈️ Telegram",
        disabled: true,
        payload: ["DOMAIN,t.me"],
      },
      {
        ...emptyRuleSet(),
        id: "youtube",
        name: "YouTube",
        payload: ["DOMAIN,youtube.com"],
      },
      {
        ...emptyRuleSet(),
        id: "chrome",
        name: "Browser",
        payload: ["PROCESS-NAME,Google Chrome"],
      },
      { ...emptyRuleSet(), id: "unused", name: "Unused" },
    ],
    configReferences: [
      { id: "general", name: "General", strategyGroupIds: ["im", "video"] },
      { id: "process", name: "Process", strategyGroupIds: ["im"] },
    ],
  };
}
describe("resource library filters and references", () => {
  it("intersects config and type filters without treating disabled as unlinked", () => {
    const state = fixture();
    expect(
      filterResources(state.strategyGroups, state, "groups", {
        ...defaultResourceFilters(),
        config: "general",
        kind: "selector",
      }).map((x) => x.id),
    ).toEqual(["im", "video"]);
    expect(eligibleGroupOptions(state, "unlinked").map((x) => x.id)).toEqual([
      "orphan",
    ]);
  });
  it("distinguishes no config from no owning group", () => {
    const state = fixture();
    expect(
      filterResources(state.ruleSets, state, "rules", {
        ...defaultResourceFilters(),
        config: "unlinked",
      }).map((x) => x.id),
    ).toEqual(["chrome", "unused"]);
    expect(
      filterResources(state.ruleSets, state, "rules", {
        ...defaultResourceFilters(),
        groups: ["unlinked"],
      }).map((x) => x.id),
    ).toEqual(["unused"]);
    const preview = resourceDeletionPreview(state, "rules", [
      "chrome",
      "unused",
      "telegram",
    ]);
    expect(preview.deletable.map((x) => x.id)).toEqual(["unused"]);
    expect(preview.blocked.map((x) => x.item.id)).toEqual([
      "telegram",
      "chrome",
    ]);
  });
  it("unions selected groups then intersects config and name/content matching", () => {
    const state = fixture();
    expect(
      filterResources(state.ruleSets, state, "rules", {
        ...defaultResourceFilters(),
        groups: ["im", "video"],
        config: "process",
      }).map((x) => x.id),
    ).toEqual(["telegram"]);
    expect(
      filterResources(state.ruleSets, state, "rules", {
        ...defaultResourceFilters(),
        query: "chrome | youtube ! telegram",
      }).map((x) => x.id),
    ).toEqual(["youtube", "chrome"]);
    expect(
      filterResources(state.ruleSets, state, "rules", {
        ...defaultResourceFilters(),
        query: "chrome & google ! browser",
      }),
    ).toEqual([]);
  });
  it("extracts complete leading emoji without mutating resource names", () => {
    const item = { name: "👩‍💻 Development" };
    expect(resourcePresentation(item)).toEqual({
      emoji: "👩‍💻",
      displayName: "Development",
    });
    expect(item.name).toBe("👩‍💻 Development");
    expect(resourcePresentation({ name: "🇹🇼 Taiwan", emoji: "🌐" })).toEqual({
      emoji: "🌐",
      displayName: "Taiwan",
    });
    expect(resourcePresentation({ name: "Rules" })).toEqual({
      emoji: "",
      displayName: "Rules",
    });
  });
});
