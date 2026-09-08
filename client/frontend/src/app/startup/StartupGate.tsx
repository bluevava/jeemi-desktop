import { useEffect, useState, type PropsWithChildren } from "react";
import { useTranslation } from "react-i18next";

import { initializeClient } from "../../services/startupBridge";

export function StartupGate({ children }: PropsWithChildren) {
  const { t, i18n } = useTranslation();
  const [state, setState] = useState<"checking" | "ready" | "failed">("checking");
  useEffect(() => {
    let active = true;
    void initializeClient(i18n.language).then(
      () => { if (active) setState("ready"); },
      () => { if (active) setState("failed"); },
    );
    return () => { active = false; };
  }, [i18n.language]);

  if (state === "ready") return children;
  return (
    <div className="ui-recovery ui-recovery-app">
      <div className="ui-recovery-card">
        <p role="status">{t(`startup.${state}`)}</p>
        <div className="ui-recovery-actions">
          <button onClick={() => { void window.go?.desktop?.App?.HideWindowToTray(); }} type="button">
            {t("startup.quit")}
          </button>
        </div>
      </div>
    </div>
  );
}
