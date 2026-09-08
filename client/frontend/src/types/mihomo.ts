export interface AvailableMihomoVersion {
  version: string;
  publishedAt: string;
  assetName: string;
  archiveSize: number;
  sha256: string;
  installed: boolean;
  selected: boolean;
}

export interface InstalledMihomoVersion {
  version: string;
  target: string;
  executablePath: string;
  binarySize: number;
  binarySha256: string;
  installedAt: string;
  source: "official" | "manual";
  selected: boolean;
}

export interface MihomoVersionManagerState {
  dataDirectory: string;
  coreDirectory: string;
  target: string;
  selectedVersion: string;
  latestVersion: string;
  lastCheckedAt: string;
  downloadingVersion: string;
  installedVersions: InstalledMihomoVersion[];
  availableVersions: AvailableMihomoVersion[];
}

export interface MihomoCoreDialogLabels {
  title: string;
  filterName: string;
}

export interface MihomoCoreDialogImportResult {
  cancelled: boolean;
  importedVersion: string;
  state: MihomoVersionManagerState;
}
