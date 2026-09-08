import type {
  ConfigCatalogField,
  LocalConfig,
  LocalConfigField,
  SaveLocalConfigInput,
} from "../../types/localConfig";
import { defaultLocalConfigResourcePlan } from "../../types/localConfig";

export type LocalConfigDraft = SaveLocalConfigInput;

export function createLocalConfigDraft(config?: LocalConfig): LocalConfigDraft {
  if (!config) {
    return {
      id: "",
      name: "",
      description: "",
      fields: [],
      resourcePlan: defaultLocalConfigResourcePlan(),
    };
  }
  return {
    id: config.id,
    name: config.name,
    description: config.description,
    fields: config.fields.map((field) => ({ ...field })),
    resourcePlan: {
      ...config.resourcePlan,
      match: { ...config.resourcePlan.match },
      strategyGroupIds: [...config.resourcePlan.strategyGroupIds],
      disabledStrategyGroupIds: [
        ...config.resourcePlan.disabledStrategyGroupIds,
      ],
    },
  };
}

export function draftSignature(draft: LocalConfigDraft): string {
  return JSON.stringify({
    ...draft,
    fields: [...draft.fields].sort((left, right) =>
      left.path.localeCompare(right.path),
    ),
  });
}

export function findDraftField(
  draft: LocalConfigDraft,
  path: string,
): LocalConfigField | undefined {
  return draft.fields.find((field) => field.path === path);
}

export function shouldShowCatalogField(
  definition: ConfigCatalogField,
  enabledPaths: ReadonlySet<string>,
  onlyEnabled: boolean,
  showLocked: boolean,
): boolean {
  if (definition.hidden && !enabledPaths.has(definition.path)) {
    return false;
  }
  if (definition.locked) {
    return showLocked;
  }
  return !onlyEnabled || enabledPaths.has(definition.path);
}

export function configFieldDisplayName(path: string): string {
  return configFieldTreeSegments("", path)
    .filter((segment) => segment !== "*")
    .join(".");
}

export function configFieldTreeSegments(
  categoryID: string,
  path: string,
): string[] {
  const segments = path
    .split("/")
    .slice(1)
    .filter(Boolean)
    .map((segment) => segment.replaceAll("~1", "/").replaceAll("~0", "~"));
  if (segments[0]?.toLocaleLowerCase() === categoryID.toLocaleLowerCase()) {
    return segments.slice(1);
  }
  return segments;
}

export function setFieldEnabled(
  draft: LocalConfigDraft,
  definition: ConfigCatalogField,
  enabled: boolean,
): LocalConfigDraft {
  const existing = findDraftField(draft, definition.path);
  if (!enabled) {
    if (!existing) {
      return draft;
    }
    return {
      ...draft,
      fields: draft.fields.filter((field) => field.path !== definition.path),
    };
  }
  if (existing || definition.locked) {
    return draft;
  }
  return {
    ...draft,
    fields: [
      ...draft.fields,
      {
        path: definition.path,
        valueYaml: definition.exampleYaml,
        strategy: definition.defaultStrategy || "replace",
        conflictPolicy: definition.defaultConflictPolicy,
      },
    ],
  };
}

export function updateDraftField(
  draft: LocalConfigDraft,
  path: string,
  update: Partial<Omit<LocalConfigField, "path">>,
): LocalConfigDraft {
  return {
    ...draft,
    fields: draft.fields.map((field) =>
      field.path === path ? { ...field, ...update } : field,
    ),
  };
}
