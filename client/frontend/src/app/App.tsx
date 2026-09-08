import { HashRouter } from "react-router-dom";

import { AppProviders } from "./AppProviders";
import { AppShell } from "./AppShell";
import { NavigationGuardProvider } from "./navigationGuard/NavigationGuardContext";

export function App() {
  return (
    <AppProviders>
      <HashRouter>
        <NavigationGuardProvider>
          <AppShell />
        </NavigationGuardProvider>
      </HashRouter>
    </AppProviders>
  );
}
