import i18n from "i18next";
import { initReactI18next } from "react-i18next";

import { resources } from "./resources";
import { initialLanguage } from "./preferenceLanguage";

void i18n.use(initReactI18next).init({
  resources,
  lng: initialLanguage(),
  fallbackLng: "zh-CN",
  supportedLngs: ["zh-CN", "en-US"],
  interpolation: {
    escapeValue: false,
  },
});

export default i18n;
