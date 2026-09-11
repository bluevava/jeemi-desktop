import type { LocalScript, SaveLocalScriptInput } from "../../types/localScript";

export function isScriptURL(contents: string): boolean {
  return /^https?:\/\/\S+$/i.test(contents.trim());
}

export function scriptEditorInput(script: LocalScript): SaveLocalScriptInput {
  return {
    id: script.id,
    name: script.name,
    description: script.description,
    contents: script.sourceUrl || script.contents,
    expectedRevision: script.revision,
  };
}
