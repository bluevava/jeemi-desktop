import {
  CloseOutlined,
  MinusOutlined,
  PlayCircleOutlined,
  ReloadOutlined,
  SettingOutlined,
  SwitcherOutlined,
} from "@ant-design/icons";
import { Button, Divider, Tooltip } from "antd";
import { useTranslation } from "react-i18next";
import { useLocation, useNavigate } from "react-router-dom";

import {
  isNavigationSettingsPath,
  navigationItemForPath,
} from "../../app/navigation";
import { useNavigationGuard } from "../../app/navigationGuard/NavigationGuardContext";
import { useRuntimeStatus } from "../../app/runtime/RuntimeStatusContext";
import { runtimePrimaryAction } from "../../app/runtime/runtimeActionState";
import appIconUrl from "../../assets/appicon.png";
import { hideWindowToTray } from "../../services/appBridge";
import { usesNativeWindowControls, windowBridge } from "../../services/windowBridge";
import { FeatureHelp } from "../help/FeatureHelp";
import { TopBarRuntimeIndicators } from "./TopBarRuntimeIndicators";
import { UIErrorBoundary } from "../../app/recovery/UIErrorBoundary";

export function TopBar() {
  const { t } = useTranslation();
  const location = useLocation();
  const navigate = useNavigate();
  const { runtime, actionBusy, restart, start } = useRuntimeStatus();
  const navigationGuard = useNavigationGuard();
  const activeItem = navigationItemForPath(location.pathname);
  const settingsMode = isNavigationSettingsPath(location.pathname);
  const coreState = runtime?.core.state ?? "not_installed";
  const mihomoState = runtime?.mihomo.state ?? "stopped";
  const primaryAction = runtimePrimaryAction(
    mihomoState,
    Boolean(runtime?.mihomo.desiredRunning),
  );
  const restartAvailable = primaryAction === "restart";
  const coreTransitioning =
    actionBusy ||
    [
      "preparing",
      "validating",
      "starting",
      "reloading",
      "stopping",
      "recovering",
    ].includes(mihomoState);
  const nativeWindowControls = usesNativeWindowControls(runtime?.platform.os);

  return (
    <header className={`top-bar${nativeWindowControls ? " top-bar-native" : ""}`}>
      <div className="top-bar-brand">
        <img
          alt=""
          aria-hidden="true"
          className="brand-mark"
          draggable={false}
          src={appIconUrl}
        />
        <strong className="top-bar-app-name">{t("app.name")}</strong>
        <FeatureHelp
          compact
          topic={
            settingsMode
              ? activeItem.settingsHelpTopic
              : activeItem.helpTopic
          }
        />
        <h1 className="top-bar-page-title">
          {settingsMode
            ? t("settings.pageTitle", { page: t(activeItem.labelKey) })
            : t(activeItem.labelKey)}
        </h1>
        <Tooltip
          title={
            settingsMode
              ? t("settings.currentPage")
              : t("settings.openPage", { page: t(activeItem.labelKey) })
          }
        >
          <span className="top-bar-button-slot">
            <Button
              aria-label={
                settingsMode
                  ? t("settings.currentPage")
                  : t("settings.openPage", {
                      page: t(activeItem.labelKey),
                    })
              }
              aria-pressed={settingsMode}
              className={`page-settings-button${settingsMode ? " active" : ""}`}
              disabled={settingsMode}
              icon={<SettingOutlined />}
              onClick={() => {
                if (navigationGuard.canLeave()) {
                  navigate(activeItem.settingsPath);
                }
              }}
              size="small"
              type="text"
            />
          </span>
        </Tooltip>
      </div>

      <div className="top-bar-actions">
        <UIErrorBoundary scope="status"><TopBarRuntimeIndicators /></UIErrorBoundary>
        <Tooltip title={t(`action.${primaryAction}`)}>
          <span>
            <Button
              aria-label={t(`action.${primaryAction}`)}
              className={`runtime-power-button${
                restartAvailable ? " active" : ""
              }`}
              disabled={
                coreTransitioning ||
                coreState === "not_installed" ||
                coreState === "downloading"
              }
              icon={
                restartAvailable ? <ReloadOutlined /> : <PlayCircleOutlined />
              }
              loading={actionBusy}
              onClick={() => void (restartAvailable ? restart() : start())}
              type="text"
            />
          </span>
        </Tooltip>
        {!nativeWindowControls ? (
          <>
            <Divider className="window-control-divider" orientation="vertical" />
            <Tooltip title={t("action.minimise")}>
              <Button
                aria-label={t("action.minimise")}
                icon={<MinusOutlined />}
                onClick={windowBridge.minimise}
                type="text"
              />
            </Tooltip>
            <Tooltip title={t("action.maximise")}>
              <Button
                aria-label={t("action.maximise")}
                icon={<SwitcherOutlined />}
                onClick={windowBridge.toggleMaximise}
                type="text"
              />
            </Tooltip>
            <Tooltip title={t("action.close")}>
              <Button
                aria-label={t("action.close")}
                className="window-close-button"
                icon={<CloseOutlined />}
                onClick={() => void hideWindowToTray()}
                type="text"
              />
            </Tooltip>
          </>
        ) : null}
      </div>
    </header>
  );
}
