import {
  isMap,
  isScalar,
  isSeq,
  parseDocument,
  Scalar,
  type Document,
} from "yaml";

// Work on the YAML nodes so comments and quoted process paths survive edits.
export function parseRulePayload(source: string) {
  if (new TextEncoder().encode(source).length > 2 * 1024 * 1024)
    throw new Error("size");
  const document: Document = parseDocument(source || "payload: []\n", {
    strict: true,
  });
  const root = document.contents;
  if (
    document.errors.length ||
    document.warnings.length ||
    !isMap(root) ||
    root.anchor ||
    root.items.length !== 1
  )
    throw new Error("payload");
  const key = root.items[0].key;
  const payload = root.items[0].value;
  if (
    !isScalar(key) ||
    key.value !== "payload" ||
    key.anchor ||
    !isSeq(payload) ||
    payload.anchor ||
    payload.items.length > 20000
  )
    throw new Error("payload");
  const rules = payload.items.map((item) => {
    if (
      !isScalar(item) ||
      item.anchor ||
      typeof item.value !== "string" ||
      !item.value.trim() ||
      /[\r\n]/.test(item.value)
    )
      throw new Error("rule");
    return item.value;
  });
  return { document, payload, rules };
}

export type RulePayloadEdit =
  | { type: "add"; value: string }
  | { type: "edit"; index: number; value: string }
  | { type: "delete"; index: number }
  | { type: "move"; index: number; to: number };

export function editRulePayload(source: string, edit: RulePayloadEdit): string {
  const { document, payload } = parseRulePayload(source);
  if (
    "index" in edit &&
    (!Number.isInteger(edit.index) ||
      edit.index < 0 ||
      edit.index >= payload.items.length)
  )
    throw new Error("index");
  if ("value" in edit && (!edit.value.trim() || /[\r\n]/.test(edit.value)))
    throw new Error("value");
  if (edit.type === "add") {
    if (payload.items.length >= 20000) throw new Error("size");
    payload.add(new Scalar(edit.value.trim()));
  } else if (edit.type === "edit") {
    (payload.items[edit.index] as Scalar<string>).value = edit.value.trim();
  } else if (edit.type === "delete") {
    payload.items.splice(edit.index, 1);
  } else {
    if (
      !Number.isInteger(edit.to) ||
      edit.to < 0 ||
      edit.to >= payload.items.length
    )
      throw new Error("index");
    const [item] = payload.items.splice(edit.index, 1);
    payload.items.splice(edit.to, 0, item);
  }
  return document.toString({ lineWidth: 0 });
}

export function splitClassicalRule(rule: string): {
  type: string;
  value: string;
} {
  const comma = rule.indexOf(",");
  return comma < 0
    ? { type: "DOMAIN-SUFFIX", value: rule }
    : { type: rule.slice(0, comma), value: rule.slice(comma + 1) };
}
