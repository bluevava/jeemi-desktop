import { createElement, type ComponentProps } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { SubscriptionProjection, SubscriptionSelector } from "../../../types/subscription";
import { sortSelectorMembers } from "../selectorSort";
import { SelectorWorkspace } from "./SelectorWorkspace";

const session = vi.hoisted(() => ({
  activeTab: "", expanded: [] as string[], paths: {} as Record<string, string[]>,
  dismissed: {} as Record<string, true>,
  nameQuery: "",
}));
vi.mock("../SubscriptionPageStateContext", async (importOriginal) => ({
  ...await importOriginal<typeof import("../SubscriptionPageStateContext")>(),
  useSubscriptionPageStateField: (field: string) => [
    field === "activeTabByWorkspace"
      ? { "example:rule": session.activeTab }
      : field === "expandedSelectorsByWorkspace" ? { "example:rule": session.expanded }
      : field === "nestedSelectorPaths" ? session.paths
      : field === "selectorNameQuery" ? session.nameQuery : session.dismissed,
    () => undefined,
  ],
}));
vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key, i18n: { language: "en-US" } }),
}));
vi.mock("../../../components/help/FeatureHelp", () => ({ FeatureHelp: () => null }));
vi.mock("./FallbackControl", () => ({ FallbackControl: () => null }));
vi.mock("./NormalizationNotice", () => ({ NormalizationNotice: () => null }));
vi.mock("../selectorSort", async (importOriginal) => {
  const original = await importOriginal<typeof import("../selectorSort")>();
  return { ...original, sortSelectorMembers: vi.fn(original.sortSelectorMembers) };
});

const selectors: SubscriptionSelector[] = Array.from({ length: 40 }, (_, index) => ({
  name: `Group ${index}`, icon: "", hidden: false, type: "select",
  defaultSelection: `Node ${index}-0`, providerNames: [], unresolvedProviderNames: [],
  referencedByRules: true,
  members: Array.from({ length: 8 }, (_, node) => ({
    name: `Node ${index}-${node}`, type: "ss", source: "proxy", providerName: "",
  })),
}));

function render(viewMode: "tabs" | "panel", items = selectors, warnings: string[] = [], runtimeReady = false,
  overrides: Partial<ComponentProps<typeof SelectorWorkspace>> = {}) {
  const noop = async () => undefined;
  const props: ComponentProps<typeof SelectorWorkspace> = {
    fallbackBusy: false, onFallbackChange: async () => true, activeSelections: {},
    busyDelayNodes: new Set(), busySelection: "", delayQueueActive: false, delayResults: {},
    density: "medium", displayPreferencesBusy: false, selectors: items.filter((item) => !item.hidden),
    outboundMode: "rule", outboundModeBusy: false, onOutboundModeChange: noop,
    onDensityChange: noop, onSelect: noop, onSortModeChange: noop, onTestDelay: noop,
    onViewModeChange: noop, query: "", runtimeReady, sortMode: "name", viewMode,
    projection: {
      subscriptionId: "example", status: "ready", selectors: items, warnings, summary: {},
    } as unknown as SubscriptionProjection,
    ...overrides,
  };
  return renderToStaticMarkup(createElement(SelectorWorkspace, props));
}

function renderedNodeCount(html: string) {
  return (html.match(/class="selector-node(?: |")/g) ?? []).length;
}

describe("selector rendering cost", () => {
  beforeEach(() => {
    session.activeTab = "";
    session.expanded = [];
    session.paths = {};
    session.dismissed = {};
    session.nameQuery = "";
    vi.clearAllMocks();
  });

  it("does not render or sort the nodes of collapsed groups", () => {
    const html = render("panel");
    expect(html.match(/<details /g)).toHaveLength(40);
    expect(renderedNodeCount(html)).toBe(0);
    expect(sortSelectorMembers).not.toHaveBeenCalled();
  });

  it("only renders and sorts expanded groups", () => {
    session.expanded = ["Group 3", "Group 28"];
    expect(renderedNodeCount(render("panel"))).toBe(16);
    expect(sortSelectorMembers).toHaveBeenCalledTimes(2);
  });

  it("only renders and sorts the active tab", () => {
    session.activeTab = "Group 28";
    const html = render("tabs");
    expect(renderedNodeCount(html)).toBe(8);
    expect(html).toContain('title="Node 28-1"');
    expect(html).not.toContain('title="Node 3-1"');
    expect(sortSelectorMembers).toHaveBeenCalledTimes(1);
  });

  it("shows the first group immediately if the remembered tab was filtered out", () => {
    session.activeTab = "Removed group";
    const html = render("tabs");
    expect(renderedNodeCount(html)).toBe(8);
    expect(html).toContain('title="Node 0-1"');
    expect(sortSelectorMembers).toHaveBeenCalledTimes(1);
  });

  it("renders a group card with a separate inspect action and real group type", () => {
    const root = { ...selectors[0], members: [{ name: "Automatic", type: "group", source: "group" as const, providerName: "" }] };
    const child = { ...selectors[1], name: "Automatic", type: "url-test", hidden: true };
    const html = render("tabs", [root, child]);
    expect(renderedNodeCount(html)).toBe(1);
    expect(html).toContain("subscription.selector.groupTypes.urlTest");
    expect(html).toContain('class="selector-node-open"');
    expect(html).not.toContain('title="Node 1-1"');
  });

  it("only mounts the inspected hidden group and disables automatic selection", () => {
    const root = { ...selectors[0], members: [{ name: "Automatic", type: "group", source: "group" as const, providerName: "" }] };
    const child = { ...selectors[1], name: "Automatic", type: "url-test", hidden: true };
    session.paths[JSON.stringify(["example:rule", root.name])] = [child.name];
    const html = render("tabs", [root, child], [], true);
    expect(renderedNodeCount(html)).toBe(8);
    expect(html).toContain('title="Node 1-1"');
    expect(html).toContain('class="selector-node-select" disabled=""');
    expect(html).toContain('class="selector-breadcrumb"');
    expect(sortSelectorMembers).toHaveBeenCalledTimes(1);
  });

  it("keeps dismissed warnings hidden across remounts in this session", () => {
    const warnings = ["selector_dynamic_filter_not_evaluated"];
    expect(render("tabs", selectors, warnings)).toContain("subscription.selector.dynamicFiltersPending");
    session.dismissed["selector-dynamic-filters"] = true;
    expect(render("panel", selectors, warnings)).not.toContain("subscription.selector.dynamicFiltersPending");
    session.dismissed = {};
    expect(render("tabs", selectors, warnings)).toContain("subscription.selector.dynamicFiltersPending");
  });

  it.each(["tabs", "panel"] as const)("shows complete egress paths in %s without current labels", (view) => {
    const root = { ...selectors[0], name: "Google", defaultSelection: "Manual",
      members: [{ name: "Manual", type: "group", source: "group" as const, providerName: "" }] };
    const manual = { ...root, name: "Manual", defaultSelection: "Automatic", hidden: true,
      members: [{ name: "Automatic", type: "group", source: "group" as const, providerName: "" }] };
    const automatic = { ...selectors[1], name: "Automatic", type: "url-test", hidden: true };
    session.expanded = ["Google"];
    const html = render(view, [root, manual, automatic], [], true, {
      activeSelections: { Google: "Manual", Manual: "Automatic", Automatic: "Node 1-5" },
    });
    expect(html).toContain("Manual · Automatic · Node 1-5");
    expect(html).toContain("Automatic · Node 1-5</span>");
    expect(html).not.toContain("subscription.selector.currentSelection");
    expect(html).not.toContain("subscription.selector.nested.current");
    expect(html).not.toContain("subscription.selector.nested.default");
  });

  it("filters top-level group names without filtering the nested group cards", () => {
    const root = { ...selectors[0], name: "Google", defaultSelection: "Manual",
      members: [{ name: "Manual", type: "group", source: "group" as const, providerName: "" }] };
    const child = { ...selectors[1], name: "Manual" };
    session.nameQuery = "Google !backup";
    session.expanded = ["Google"];
    const html = render("panel", [root, child]);
    expect(html.match(/<details /g)).toHaveLength(1);
    expect(html).toContain('title="Manual"');
    expect(html).toContain("Manual · Node 1-0");
    expect(html).toContain("subscription.selector.nameSearch");
  });

  it("shows a distinct empty result for selector search", () => {
    session.nameQuery = "No such selector";
    expect(render("tabs")).toContain("subscription.selector.noSelectorSearchResults");
  });
});
