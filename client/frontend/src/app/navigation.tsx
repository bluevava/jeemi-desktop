import type { ReactNode } from "react";
import {
  CodeOutlined,
  CloudOutlined,
  FileTextOutlined,
  HomeOutlined,
  LinkOutlined,
  ToolOutlined,
} from "@ant-design/icons";

import type { HelpTopic } from "../components/help/FeatureHelp";
import type { LogLevel } from "../types/runtime";

export type NavigationPage =
  | "home"
  | "subscriptions"
  | "config"
  | "connections"
  | "logs"
  | "tools";

export interface NavigationItem {
  id: NavigationPage;
  path: string;
  settingsPath: string;
  labelKey: string;
  icon: ReactNode;
  helpTopic: HelpTopic;
  settingsHelpTopic: HelpTopic;
}

export const navigationItems: NavigationItem[] = [
  {
    id: "home",
    path: "/home",
    settingsPath: "/home/settings",
    labelKey: "nav.home",
    icon: <HomeOutlined />,
    helpTopic: "home",
    settingsHelpTopic: "homeSettings",
  },
  {
    id: "subscriptions",
    path: "/subscriptions",
    settingsPath: "/subscriptions/settings",
    labelKey: "nav.subscriptions",
    icon: <CloudOutlined />,
    helpTopic: "subscriptions",
    settingsHelpTopic: "subscriptionSettings",
  },
  {
    id: "config",
    path: "/config",
    settingsPath: "/config/settings",
    labelKey: "nav.config",
    icon: <CodeOutlined />,
    helpTopic: "config",
    settingsHelpTopic: "emptySettings",
  },
  {
    id: "connections",
    path: "/connections",
    settingsPath: "/connections/settings",
    labelKey: "nav.connections",
    icon: <LinkOutlined />,
    helpTopic: "connections",
    settingsHelpTopic: "emptySettings",
  },
  {
    id: "logs",
    path: "/logs",
    settingsPath: "/logs/settings",
    labelKey: "nav.logs",
    icon: <FileTextOutlined />,
    helpTopic: "logs",
    settingsHelpTopic: "emptySettings",
  },
  {
    id: "tools",
    path: "/tools",
    settingsPath: "/tools/settings",
    labelKey: "nav.tools",
    icon: <ToolOutlined />,
    helpTopic: "tools",
    settingsHelpTopic: "emptySettings",
  },
];

export function visibleNavigationItems(
  logLevel: LogLevel | null | undefined,
): NavigationItem[] {
  if (isLogPageAvailable(logLevel)) return navigationItems;
  return navigationItems.filter((item) => item.id !== "logs");
}

export function isLogPageAvailable(
  logLevel: LogLevel | null | undefined,
): boolean {
  return Boolean(logLevel && logLevel !== "silent");
}

export function navigationItemForPath(pathname: string): NavigationItem {
  const normalizedPath = pathname.replace(/\/+$/, "") || "/";
  return (
    navigationItems.find(
      (item) =>
        normalizedPath === item.path ||
        normalizedPath.startsWith(`${item.path}/`),
    ) ?? navigationItems[0]
  );
}

export function isNavigationSettingsPath(pathname: string): boolean {
  const normalizedPath = pathname.replace(/\/+$/, "") || "/";
  return navigationItems.some((item) => normalizedPath === item.settingsPath);
}

export function isLocalConfigEditorPath(pathname: string): boolean {
  const normalizedPath = pathname.replace(/\/+$/, "") || "/";
  return (
    normalizedPath === "/config/new" ||
    normalizedPath === "/config/scripts/new" ||
    normalizedPath === "/config/rule-sets/new" ||
    /^\/config\/groups\/new\/(rule|selector)$/.test(normalizedPath) ||
    /^\/config\/(groups|rule-sets)\/[a-f0-9]{32}\/edit$/.test(normalizedPath) ||
    /^\/config\/scripts\/[a-f0-9]{32}\/edit$/.test(normalizedPath) ||
    /^\/config\/[a-f0-9]{32}\/edit$/.test(normalizedPath)
  );
}
