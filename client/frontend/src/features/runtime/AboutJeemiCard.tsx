import { useTranslation } from "react-i18next";
import type { ReactNode } from "react";
import { useNavigate } from "react-router-dom";
import { App, Button, Typography } from "antd";

import appIcon from "../../assets/appicon.png";
import { useRuntimeStatus } from "../../app/runtime/RuntimeStatusContext";
import { FeatureCard } from "../../components/layout/FeatureCard";
import {
  jeemiCommunityURLs,
  openJeemiCommunity,
  openJeemiRepository,
} from "../../services/appBridge";
import { homeSettingsSectionPath } from "../../app/homeSettingsNavigation";
import { JeemiUpdateButton } from "../update/JeemiUpdateButton";
import { useAuthorizationHelperStatus } from "./useAuthorizationHelperStatus";

export function AboutJeemiCard() {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const navigate = useNavigate();
  const {
    bootstrap,
    runtime,
    removeAuthorization,
    actionBusy,
    removingAuthorization,
  } = useRuntimeStatus();
  const helper = useAuthorizationHelperStatus(actionBusy);

  const openRepository = async () => {
    try {
      await openJeemiRepository();
    } catch {
      void message.error(t("home.about.repositoryError"));
    }
  };

  const openCommunity = async (destination: keyof typeof jeemiCommunityURLs) => {
    try {
      await openJeemiCommunity(destination);
    } catch {
      void message.error(t("home.about.community.error"));
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
        <p>
          {t("home.about.description")}{" "}
          <Typography.Link
            href={jeemiCommunityURLs.group}
            target="_blank"
            rel="noopener noreferrer"
            aria-label={t("home.about.community.groupLabel")}
            onClick={(event) => {
              event.preventDefault();
              void openCommunity("group");
            }}
          >
            {t("home.about.community.group")}
          </Typography.Link>{" "}
          <Typography.Link
            href={jeemiCommunityURLs.channel}
            target="_blank"
            rel="noopener noreferrer"
            aria-label={t("home.about.community.channelLabel")}
            onClick={(event) => {
              event.preventDefault();
              void openCommunity("channel");
            }}
          >
            {t("home.about.community.channel")}
          </Typography.Link>
        </p>
        <dl className="home-about-facts">
          <AboutFact
            label={t("home.version")}
            action={<JeemiUpdateButton className="home-about-action" />}
            value={bootstrap?.app.version ?? "0.1.0-dev"}
          />
          <AboutFact
            label={t("home.coreVersion")}
            action={
              <Button
                className="home-about-action"
                type="text"
                size="small"
                onClick={() => navigate(homeSettingsSectionPath("mihomo"))}
              >
                {t("home.about.manageCore")}
              </Button>
            }
            value={runtime?.core.version || t("home.notAvailable")}
          />
          <AboutFact
            label={t("home.about.helper")}
            action={helper.present ? (
              <Button
                className="home-about-action"
                type="text"
                size="small"
                loading={removingAuthorization}
                disabled={actionBusy}
                onClick={() => void removeAuthorization()}
              >
                {t("home.about.removeHelper")}
              </Button>
            ) : null}
            value={helper.health ? t(`home.about.helperStates.${helper.health}`) : "—"}
            health={helper.health}
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
      </div>
    </FeatureCard>
  );
}

function AboutFact({ label, action, value, health }: {
  label: string;
  action?: ReactNode;
  value: string;
  health?: string | null;
}) {
  return (
    <div>
      <dt>{label}</dt>
      <dd className="home-about-fact-action">{action}</dd>
      <dd className="home-about-fact-value" data-health={health} title={value}>{value}</dd>
    </div>
  );
}
