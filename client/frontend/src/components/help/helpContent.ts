import type { TFunction } from "i18next";

export type HelpContent = {
  title: string;
  purpose: string;
  scenarios: string;
} & ({ cautions: string; example?: string } | { cautions?: never; example: string });

export interface HelpSection {
  key: "purpose" | "scenarios" | "cautions" | "example" | "cautionsAndExample";
  paragraphs: string[];
  caution?: boolean;
}

export function isHelpContent(value: unknown): value is HelpContent {
  if (!value || typeof value !== "object") return false;
  const content = value as Record<string, unknown>;
  const nonEmpty = (text: unknown) => typeof text === "string" && !!text.trim();
  return (
    ["title", "purpose", "scenarios"].every((key) => nonEmpty(content[key])) &&
    ["cautions", "example"].every((key) => content[key] === undefined || nonEmpty(content[key])) &&
    (nonEmpty(content.cautions) || nonEmpty(content.example))
  );
}

export function resolveHelpContent(
  t: TFunction,
  base: string,
  fallbackBase = "help.topics.localConfigField",
  values?: Record<string, string | number>,
): HelpContent {
  // Fall back as a whole topic: an example-only topic must not inherit another topic's warning.
  for (const candidate of new Set([base, fallbackBase, "help.topics.localConfigField"])) {
    const content: unknown = t(candidate, { ...values, returnObjects: true });
    if (isHelpContent(content)) return content;
  }
  throw new Error("Missing default feature-help content");
}

export function helpSections(content: HelpContent): HelpSection[] {
  const { cautions, example } = content;
  return [
    { key: "purpose", paragraphs: [content.purpose] },
    { key: "scenarios", paragraphs: [content.scenarios] },
    {
      key: cautions ? (example ? "cautionsAndExample" : "cautions") : "example",
      paragraphs: [cautions, example].filter((text): text is string => !!text),
      caution: !!cautions,
    },
  ];
}
