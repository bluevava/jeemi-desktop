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
vi.mock("../../../components/help/FeatureHelp", () => ({
  FeatureHelp: ({ translationBase }: { translationBase?: string }) =>
    translationBase ? createElement("span", { "data-help": translationBase }) : null,
}));
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

function nestedCard(html: string) {
  return html.match(/<div class="selector-node selector-group-member[^\"]*">[\s\S]*?<\/div>/)?.[0] ?? "";
}

function delayResult(delay: number) {
  return { status: "success" as const, source: "manual" as const, testedAt: "2026-09-11T00:00:00Z", delay };
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
    expect(html).toContain('aria-label="subscription.selector.nested.open" class="selector-node-delay">—</button>');
    expect(html).not.toContain('class="selector-node-open"');
    expect(html).not.toContain('class="selector-nested-heading"');
    expect(html).not.toContain('data-help="subscription.selector.nested.help"');
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
    expect(html).toContain('data-help="subscription.selector.nested.help"');
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

  it.each(["tabs", "panel"] as const)("shows only the final exit in %s and uses its delay on nested cards", (view) => {
    const root = { ...selectors[0], name: "Google", defaultSelection: "Manual",
      members: [{ name: "Manual", type: "group", source: "group" as const, providerName: "" }] };
    const manual = { ...root, name: "Manual", defaultSelection: "Automatic", hidden: true,
      members: [{ name: "Automatic", type: "group", source: "group" as const, providerName: "" }] };
    const automatic = { ...selectors[1], name: "Automatic", type: "url-test", hidden: true };
    session.expanded = ["Google"];
    const html = render(view, [root, manual, automatic], [], true, {
      activeSelections: { Google: "Manual", Manual: "Automatic", Automatic: "Node 1-5" },
      delayResults: { Manual: delayResult(999), Automatic: delayResult(888),
        "Node 1-0": delayResult(32), "Node 1-5": delayResult(96) },
    });
    expect(html).toContain(">Node 1-5</");
    expect(html).not.toContain("Manual · Automatic");
    expect(html).not.toContain("Automatic · Node 1-5");
    const card = nestedCard(html);
    expect(card).toContain('class="selector-node-delay fast">96 ms</button>');
    expect(card).not.toContain("888 ms");
    expect(card).not.toContain("999 ms");
    expect(card).not.toContain("32 ms");
    expect(card).not.toContain("Node 1-5");
    expect(html).not.toContain('class="selector-nested-heading"');
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
    expect(html).toContain(">Node 1-0</strong>");
    expect(html).not.toContain("Manual · Node 1-0");
    expect(html).toContain("subscription.selector.nameSearch");
  });

  it("shows a distinct empty result for selector search", () => {
    session.nameQuery = "No such selector";
    expect(render("tabs")).toContain("subscription.selector.noSelectorSearchResults");
  });

  it.each(["small", "medium", "large"] as const)("keeps two-line nested cards in %s density with and without proxy siblings", (density) => {
    const child = { ...selectors[1], name: "Automatic", type: "url-test", hidden: true };
    const group = { name: child.name, type: "group", source: "group" as const, providerName: "" };
    for (const members of [[group], [group, selectors[0].members[0]]]) {
      const root = { ...selectors[0], members };
      const card = nestedCard(render("tabs", [root, child], [], false, { density }));
      expect(card).toContain('class="selector-group-member-name">Automatic</span>');
      expect(card).toContain("subscription.selector.groupTypes.urlTest");
      expect(card).not.toContain("selector-group-member-current");
      expect(card).not.toContain("Node 1-0");
      expect(card).toContain('class="selector-node-delay">—</button>');
    }
  });

  it.each(["load-balance", "relay"])("keeps %s browsable while testing without inventing an exit delay", (type) => {
    const root = { ...selectors[0], defaultSelection: "Child",
      members: [{ name: "Child", type: "group", source: "group" as const, providerName: "" }] };
    const child = { ...selectors[1], name: "Child", type, hidden: true };
    const card = nestedCard(render("tabs", [root, child], [], true, {
      activeSelections: { [root.name]: child.name, Child: "Node 1-0" },
      delayResults: { Child: delayResult(21), "Node 1-0": delayResult(50) },
      delayQueueActive: true,
    }));
    expect(card).toContain('class="selector-node-delay">—</button>');
    expect(card).not.toContain(" ms");
  });

  it("does not borrow stale delay from a builtin exit or fall back to an offline default when live selection is missing", () => {
    const root = { ...selectors[0], defaultSelection: "Child",
      members: [{ name: "Child", type: "group", source: "group" as const, providerName: "" }] };
    const child = { ...selectors[1], name: "Child", hidden: true, defaultSelection: "DIRECT",
      members: [{ name: "DIRECT", type: "direct", source: "builtin" as const, providerName: "" }] };
    for (const activeSelections of [{ [root.name]: child.name, Child: "DIRECT" }, { [root.name]: child.name }]) {
      const card = nestedCard(render("tabs", [root, child], [], true, {
        activeSelections, delayResults: { Child: delayResult(21), DIRECT: delayResult(50) },
      }));
      expect(card).toContain('class="selector-node-delay">—</button>');
      expect(card).not.toContain(" ms");
    }
  });

  it("sorts a nested group using the final node delay displayed on its badge", () => {
    const root = { ...selectors[0], members: [selectors[0].members[0],
      { name: "Automatic", type: "group", source: "group" as const, providerName: "" }] };
    const child = { ...selectors[1], name: "Automatic", type: "url-test", hidden: true };
    const html = render("tabs", [root, child], [], true, {
      sortMode: "delay", activeSelections: { Automatic: "Node 1-4" },
      delayResults: { Automatic: delayResult(999), "Node 1-4": delayResult(50), "Node 0-0": delayResult(90) },
    });
    const grid = html.slice(html.indexOf('class="selector-node-grid"'));
    expect(grid.indexOf('title="Automatic"')).toBeLessThan(grid.indexOf('title="Node 0-0"'));
  });
});
