import {
  createContext,
  type PropsWithChildren,
  useContext,
  useEffect,
  useLayoutEffect,
  useMemo,
  useState,
} from "react";
import { App as AntdApp, ConfigProvider, type ThemeConfig } from "antd";
import enUS from "antd/locale/en_US";
import zhCN from "antd/locale/zh_CN";

import i18n from "../../i18n";
import type { AppLanguage } from "../../i18n/resources";
import { initialLanguage, languageStorageKey } from "../../i18n/preferenceLanguage";
import { setTrayLanguage } from "../../services/appBridge";
import { createAntdTheme } from "./antdTheme";
import { initialTheme, writeUIPreference } from "../../lib/uiPreferences";

export type ThemeMode = "light" | "dark";

interface PreferencesValue {
  theme: ThemeMode;
  language: AppLanguage;
  setTheme: (theme: ThemeMode) => void;
  setLanguage: (language: AppLanguage) => void;
}

const PreferencesContext = createContext<PreferencesValue | null>(null);

const themeStorageKey = "jeemi.ui.theme";

export function PreferencesProvider({ children }: PropsWithChildren) {
  const [theme, setThemeState] = useState<ThemeMode>(initialTheme);
  const [language, setLanguageState] = useState<AppLanguage>(initialLanguage);
  // Keep Ant Design's theme provider mounted while the CSS palette is resolved.
  const [componentTheme, setComponentTheme] = useState<ThemeConfig>({});

  useLayoutEffect(() => {
    document.documentElement.dataset.theme = theme;
    document.documentElement.style.colorScheme = theme;
    const style = getComputedStyle(document.documentElement);
    setComponentTheme(createAntdTheme(theme, (name) => style.getPropertyValue(name)));
    writeUIPreference(themeStorageKey, theme);
  }, [theme]);

  useLayoutEffect(() => {
    document.documentElement.lang = language;
    writeUIPreference(languageStorageKey, language);
    void i18n.changeLanguage(language);
  }, [language]);

  useEffect(() => {
    void setTrayLanguage(language).catch(() => undefined);
  }, [language]);

  const value = useMemo<PreferencesValue>(
    () => ({
      theme,
      language,
      setTheme: setThemeState,
      setLanguage: setLanguageState,
    }),
    [language, theme],
  );

  return (
    <PreferencesContext.Provider value={value}>
      <ConfigProvider
        locale={language === "zh-CN" ? zhCN : enUS}
        theme={componentTheme}
      >
        <AntdApp component={false}>{children}</AntdApp>
      </ConfigProvider>
    </PreferencesContext.Provider>
  );
}

export function usePreferences(): PreferencesValue {
  const value = useContext(PreferencesContext);
  if (!value) {
    throw new Error("usePreferences must be used within PreferencesProvider");
  }
  return value;
}
