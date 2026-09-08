import type { PropsWithChildren } from "react";

import { PreferencesProvider } from "./preferences/PreferencesContext";
import { ProxyOverviewProvider } from "./runtime/ProxyOverviewContext";
import { RuntimeStatusProvider } from "./runtime/RuntimeStatusContext";
import { TrafficProvider } from "./runtime/TrafficContext";
import { WindowActivityProvider } from "./runtime/WindowActivityContext";
import { SubscriptionPageStateProvider } from "../features/subscription/SubscriptionPageStateContext";

export function AppProviders({ children }: PropsWithChildren) {
  return (
    <PreferencesProvider>
      <SubscriptionPageStateProvider>
        <WindowActivityProvider>
          <RuntimeStatusProvider>
            <TrafficProvider>
              <ProxyOverviewProvider>{children}</ProxyOverviewProvider>
            </TrafficProvider>
          </RuntimeStatusProvider>
        </WindowActivityProvider>
      </SubscriptionPageStateProvider>
    </PreferencesProvider>
  );
}
