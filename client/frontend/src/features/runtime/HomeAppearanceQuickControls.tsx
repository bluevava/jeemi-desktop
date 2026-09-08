import { Segmented } from "antd";
import { useTranslation } from "react-i18next";

import {
  usePreferences,
  type ThemeMode,
} from "../../app/preferences/PreferencesContext";
import type { AppLanguage } from "../../i18n/resources";

export function HomeAppearanceQuickControls() {
  const { t } = useTranslation();
  const { language, setLanguage, setTheme, theme } = usePreferences();

  return (
    <div className="overview-appearance-controls">
      <Segmented
        aria-label={t("settings.home.theme")}
        block
        onChange={(value) => setTheme(value as ThemeMode)}
        options={([
          ["light", "settings.home.light"],
          ["dark", "settings.home.dark"],
        ] as const).map(([value, labelKey]) => ({
          label: t(labelKey),
          value,
        }))}
        value={theme}
      />
      <Segmented
        aria-label={t("settings.home.language")}
        block
        onChange={(value) => setLanguage(value as AppLanguage)}
        options={([
          ["zh-CN", "settings.home.chineseShort"],
          ["en-US", "settings.home.englishShort"],
        ] as const).map(([value, labelKey]) => ({
          label: t(labelKey),
          value,
        }))}
        value={language}
      />
    </div>
  );
}
