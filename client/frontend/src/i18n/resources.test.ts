/// <reference types="node" />

import { describe, expect, it } from "vitest";
import { readdirSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { join } from "node:path";
import ts from "typescript";

import { flattenTranslationKeys, resources } from "./resources";
import { resolveLanguage } from "./preferenceLanguage";
import { isHelpContent } from "../components/help/helpContent";

describe("translation resources", () => {
  it("keeps Simplified Chinese and English keys in parity", () => {
    const chineseKeys = flattenTranslationKeys(
      resources["zh-CN"].translation,
    ).sort();
    const englishKeys = flattenTranslationKeys(
      resources["en-US"].translation,
    ).sort();

    expect(englishKeys).toEqual(chineseKeys);
  });

  it("keeps every feature-help topic complete", () => {
    for (const locale of Object.values(resources)) {
      const topics = helpTopics(locale.translation);
      // Include nested and imported help, not only page topics and field descriptions.
      expect(topics.map(([key]) => key)).toEqual(expect.arrayContaining([
        "help.topics.subscriptionNodeSearch",
        "help.topics.subscriptionSelectorSearch",
        "localConfig.fieldHelp.generic",
        "localConfig.resources.groups.help.namePatterns",
        "localConfig.resources.ruleSets.help.payload",
        "localConfig.editorResources.help.activation",
        "ruleSetEntry.help.processPath",
        "subscription.normalization.help",
      ]));
      for (const [key, topic] of topics) {
        expect(isHelpContent(topic), key).toBe(true);
        expect(Object.keys(topic).filter((field) =>
          !["title", "purpose", "scenarios", "cautions", "example"].includes(field),
        ), key).toEqual([]);
      }
    }
  });

  it("resolves saved and system languages to the two supported locales", () => {
    expect(resolveLanguage("en-US", "zh-CN")).toBe("en-US");
    expect(resolveLanguage("zh-CN", "en-US")).toBe("zh-CN");
    expect(resolveLanguage(null, "zh-TW")).toBe("zh-CN");
    expect(resolveLanguage("fr-FR", "en-GB")).toBe("en-US");
  });

  it("covers every live traffic state including the stopped-core tooltip", () => {
    for (const locale of Object.values(resources)) {
      expect(Object.keys(locale.translation.home.traffic.states).sort()).toEqual(
        ["connecting", "error", "live", "offline", "reconnecting"],
      );
    }
  });

  it("keeps translations non-empty and interpolation variables in parity", () => {
    for (const key of flattenTranslationKeys(resources["zh-CN"].translation)) {
      const chinese = translationAt(resources["zh-CN"].translation, key);
      const english = translationAt(resources["en-US"].translation, key);
      expect(typeof chinese, key).toBe("string");
      expect(typeof english, key).toBe("string");
      expect(String(chinese).trim(), key).not.toBe("");
      expect(String(english).trim(), key).not.toBe("");
      const variables = (value: unknown) =>
        [...String(value).matchAll(/\{\{\s*([^},]+)(?:,[^}]+)?\s*\}\}/g)]
          .map((match) => match[1].trim()).sort();
      expect(variables(english), key).toEqual(variables(chinese));
    }
  });

  it("resolves literal resource references used by application source", () => {
    const root = fileURLToPath(new URL("../", import.meta.url));
    const namespaces = Object.keys(resources["zh-CN"].translation);
    const failures: string[] = [];
    for (const path of sourceFiles(root)) {
      if (path.includes(`${join("i18n", "locales")}`) || path.endsWith(".test.ts")) continue;
      const source = ts.createSourceFile(path, readFileSync(path, "utf8"), ts.ScriptTarget.Latest, true);
      const visit = (node: ts.Node) => {
        if (ts.isStringLiteralLike(node) && namespaces.some((namespace) => node.text.startsWith(`${namespace}.`))) {
          for (const [locale, resource] of Object.entries(resources)) {
            if (translationAt(resource.translation, node.text) === undefined) {
              const line = source.getLineAndCharacterOfPosition(node.getStart()).line + 1;
              failures.push(`${locale}: ${node.text} (${path}:${line})`);
            }
          }
        }
        if (ts.isTemplateExpression(node) && namespaces.some((namespace) => node.head.text.startsWith(`${namespace}.`))) {
          const prefix = node.head.text.replace(/\.$/, "");
          for (const [locale, resource] of Object.entries(resources)) {
            if (translationAt(resource.translation, prefix) === undefined) {
              failures.push(`${locale}: dynamic namespace ${prefix} (${path})`);
            }
          }
        }
        ts.forEachChild(node, visit);
      };
      visit(source);
    }
    expect(failures).toEqual([]);
  });

  it("keeps every native tray label non-empty", () => {
    for (const locale of Object.values(resources)) {
      for (const label of Object.values(locale.translation.tray)) {
        expect(label.trim()).not.toBe("");
      }
    }
  });

});

function helpTopics(root: unknown, prefix = ""): [string, Record<string, unknown>][] {
  if (!root || typeof root !== "object" || prefix === "help.section") return [];
  const value = root as Record<string, unknown>;
  if (["purpose", "scenarios", "cautions", "example"].some((key) => key in value)) {
    return [[prefix, value]];
  }
  return Object.entries(value).flatMap(([key, child]) =>
    helpTopics(child, prefix ? `${prefix}.${key}` : key),
  );
}

function translationAt(root: unknown, path: string): unknown {
  return path.split(".").reduce<unknown>((value, key) =>
    value && typeof value === "object" ? (value as Record<string, unknown>)[key] : undefined,
  root);
}

function sourceFiles(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = join(directory, entry.name);
    return entry.isDirectory() ? sourceFiles(path) : /\.tsx?$/.test(path) ? [path] : [];
  });
}
