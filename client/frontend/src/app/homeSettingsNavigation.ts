export type HomeSettingsSection = "mihomo" | "geodata" | "zashboard";

export const homeSettingsSectionIds: Record<HomeSettingsSection, string> = {
  mihomo: "home-settings-mihomo",
  geodata: "home-settings-geodata",
  zashboard: "home-settings-zashboard",
};

export function homeSettingsSectionPath(
  section: HomeSettingsSection,
  options: { check?: boolean } = {},
): string {
  const params = new URLSearchParams({ focus: section });
  if (options.check) params.set("check", "1");
  return `/home/settings?${params.toString()}#${homeSettingsSectionIds[section]}`;
}

export function resolveHomeSettingsSection(
  search: string,
  hash: string,
): HomeSettingsSection | null {
  const rawHashValue = hash.replace(/^#/, "");
  let hashValue = rawHashValue;
  try {
    hashValue = decodeURIComponent(rawHashValue);
  } catch {
    // Ignore malformed external fragments and retain query-link compatibility.
  }
  const hashSection = (Object.entries(homeSettingsSectionIds) as Array<
    [HomeSettingsSection, string]
  >).find(([, id]) => id === hashValue)?.[0];
  if (hashSection) return hashSection;

  const focus = new URLSearchParams(search).get("focus");
  return focus === "mihomo" || focus === "geodata" || focus === "zashboard" ? focus : null;
}
