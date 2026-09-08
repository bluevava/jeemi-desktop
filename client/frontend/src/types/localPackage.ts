export type LocalPackageKind = "local-config" | "local-script";

export interface LocalPackagePreview {
  cancelled: boolean;
  token: string;
  kind: LocalPackageKind;
  targetName: string;
  importedName: string;
  overwrittenGroups: string[];
  overwrittenRuleSets: string[];
  addedGroups: string[];
  addedRuleSets: string[];
  affectedConfigs: string[];
  affectedSubscriptions: string[];
}
