import { useEffect, useRef, useState } from "react";
import { Navigate, Route, Routes, useLocation } from "react-router-dom";

import {
  isLocalConfigEditorPath,
  isNavigationSettingsPath,
  navigationItems,
} from "./navigation";
import { BottomNavigation } from "../components/layout/BottomNavigation";
import { TopBar } from "../components/layout/TopBar";
import { FeatureWorkspacePage } from "../pages/FeatureWorkspacePage";
import { HomePage } from "../pages/HomePage";
import { SettingsPage } from "../pages/SettingsPage";
import { LocalResourceEditorPage } from "../features/local-config/pages/LocalResourceEditorPage";
import { LocalConfigEditorPage } from "../features/local-config/pages/LocalConfigEditorPage";
import { LocalConfigListPage } from "../features/local-config/pages/LocalConfigListPage";
import { SubscriptionListPage } from "../features/subscription/pages/SubscriptionListPage";
import { ConnectionsPage } from "../features/connections/ConnectionsPage";
import { LocalScriptEditorPage } from "../features/local-script/pages/LocalScriptEditorPage";
import { LogsPage, LogsRouteGuard } from "../features/logs/LogsPage";
import { UIErrorBoundary } from "./recovery/UIErrorBoundary";

export function AppShell() {
  const location = useLocation();
  const settingsMode = isNavigationSettingsPath(location.pathname);
  const localConfigEditorMode = isLocalConfigEditorPath(location.pathname);
  const contentRef = useRef<HTMLElement | null>(null);
  const [overlayHost, setOverlayHost] = useState<HTMLDivElement | null>(null);

  useEffect(() => {
    if (contentRef.current) {
      contentRef.current.scrollTop = 0;
    }
  }, [location.pathname]);

  return (
    <div className="app-shell">
      <TopBar />
      <div className="app-overlay-layer" ref={setOverlayHost} />
      <main className="app-content" ref={contentRef}>
        <UIErrorBoundary key={location.pathname} scope="page">
          <Routes>
            <Route element={<Navigate replace to="/home" />} path="/" />
            <Route element={<HomePage />} path="/home" />
            <Route
              element={<SubscriptionListPage overlayHost={overlayHost} />}
              path="/subscriptions"
            />
            <Route element={<LocalConfigListPage />} path="/config" />
            <Route element={<LocalConfigEditorPage />} path="/config/new" />
            <Route
              element={<LocalResourceEditorPage resource="group" kind="rule" />}
              path="/config/groups/new/rule"
            />
            <Route
              element={
                <LocalResourceEditorPage resource="group" kind="selector" />
              }
              path="/config/groups/new/selector"
            />
            <Route
              element={<LocalResourceEditorPage resource="group" />}
              path="/config/groups/:id/edit"
            />
            <Route
              element={<LocalResourceEditorPage resource="rule-set" />}
              path="/config/rule-sets/new"
            />
            <Route
              element={<LocalResourceEditorPage resource="rule-set" />}
              path="/config/rule-sets/:id/edit"
            />
            <Route
              element={<LocalScriptEditorPage />}
              path="/config/scripts/new"
            />
            <Route
              element={<LocalScriptEditorPage />}
              path="/config/scripts/:id/edit"
            />
            <Route
              element={<LocalConfigEditorPage />}
              path="/config/:id/edit"
            />
            <Route element={<ConnectionsPage />} path="/connections" />
            <Route
              element={
                <LogsRouteGuard>
                  <LogsPage />
                </LogsRouteGuard>
              }
              path="/logs"
            />
            <Route
              element={<FeatureWorkspacePage page="tools" />}
              path="/tools"
            />
            {navigationItems.map((item) => (
              <Route
                element={
                  item.id === "logs" ? (
                    <LogsRouteGuard>
                      <SettingsPage page={item.id} />
                    </LogsRouteGuard>
                  ) : (
                    <SettingsPage page={item.id} />
                  )
                }
                key={item.settingsPath}
                path={item.settingsPath}
              />
            ))}
            <Route element={<Navigate replace to="/home" />} path="*" />
          </Routes>
        </UIErrorBoundary>
      </main>
      {settingsMode || localConfigEditorMode ? null : <BottomNavigation />}
    </div>
  );
}
