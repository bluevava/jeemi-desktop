import type { PropsWithChildren } from "react";

import { PreferencesProvider } from "./preferences/PreferencesContext";
import { ProxyOverviewProvider } from "./runtime/ProxyOverviewContext";
import { RuntimeStatusProvider } from "./runtime/RuntimeStatusContext";
import { TrafficProvider } from "./runtime/TrafficContext";
import { WindowActivityProvider } from "./runtime/WindowActivityContext";
import { SubscriptionPageStateProvider } from "../features/subscription/SubscriptionPageStateContext";
import { DNSQueryPreferencesProvider } from "../features/tools/DNSQueryPreferencesContext";
import { ToolsPageStateProvider } from "../features/tools/ToolsPageStateContext";
import { JeemiUpdateProvider } from "../features/update/JeemiUpdateProvider";

export function AppProviders({ children }: PropsWithChildren) {
  return (
    <PreferencesProvider>
      <SubscriptionPageStateProvider>
        <WindowActivityProvider>
          <RuntimeStatusProvider>
            <TrafficProvider>
              <ProxyOverviewProvider>
                <DNSQueryPreferencesProvider>
                  <ToolsPageStateProvider><JeemiUpdateProvider>{children}</JeemiUpdateProvider></ToolsPageStateProvider>
                </DNSQueryPreferencesProvider>
              </ProxyOverviewProvider>
            </TrafficProvider>
          </RuntimeStatusProvider>
        </WindowActivityProvider>
      </SubscriptionPageStateProvider>
    </PreferencesProvider>
  );
}
