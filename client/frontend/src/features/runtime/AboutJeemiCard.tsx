import { useTranslation } from "react-i18next";
import { App, Button } from "antd";
import { FeatureHelp } from "../../components/help/FeatureHelp";

import appIcon from "../../assets/appicon.png";
import { useRuntimeStatus } from "../../app/runtime/RuntimeStatusContext";
import { FeatureCard } from "../../components/layout/FeatureCard";
import { openJeemiRepository } from "../../services/appBridge";

export function AboutJeemiCard() {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const {
    bootstrap,
    runtime,
    removeAuthorization,
    actionBusy,
    removingAuthorization,
  } = useRuntimeStatus();

  const openRepository = async () => {
    try {
      await openJeemiRepository();
    } catch {
      void message.error(t("home.about.repositoryError"));
    }
  };

  return (
    <FeatureCard
      className="home-about-card"
      title={t("home.about.title")}
      headerAction={
        <Button type="link" size="small" onClick={() => void openRepository()}>
          {t("home.about.repository")}
        </Button>
      }
    >
      <img
        alt=""
        aria-hidden="true"
        className="home-about-watermark"
        draggable={false}
        src={appIcon}
      />
      <div className="home-about-content">
        <p>{t("home.about.description")}</p>
        <dl className="home-about-facts">
          <AboutFact
            label={t("home.version")}
            value={bootstrap?.app.version ?? "0.1.0-dev"}
          />
          <AboutFact
            label={t("home.coreVersion")}
            value={runtime?.core.version || t("home.notAvailable")}
          />
          <AboutFact
            label={t("home.platform")}
            value={
              runtime
                ? `${runtime.platform.os} / ${runtime.platform.architecture}`
                : t("home.notAvailable")
            }
          />
        </dl>
        <div className="home-about-cleanup">
          <Button
            size="small"
            loading={removingAuthorization}
            disabled={actionBusy}
            onClick={() => void removeAuthorization()}
          >
            {t("authorization.remove")}
          </Button>
          <FeatureHelp compact topic="authorizationCleanup" />
        </div>
      </div>
    </FeatureCard>
  );
}

function AboutFact({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt>{label}</dt>
      <dd>{value}</dd>
    </div>
  );
}
