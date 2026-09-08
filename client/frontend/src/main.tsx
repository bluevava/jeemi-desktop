import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import "./i18n";
import { App } from "./app/App";
import { UIErrorBoundary } from "./app/recovery/UIErrorBoundary";
import { installUIErrorReporting, reportUIFailure } from "./app/recovery/diagnostics";
import "./styles/tokens.css";
import "./styles/global.css";
import "./styles/layout.css";
import "./styles/components.css";
import "./styles/runtime.css";
import "./styles/authorization.css";
import "./styles/connections.css";
import "./styles/logs.css";
import "./styles/local-config.css";
import "./styles/subscription.css";
import "./styles/recovery.css";
import "./styles/tools.css";

const uninstallErrorReporting = installUIErrorReporting();
import.meta.hot?.dispose(uninstallErrorReporting);

createRoot(document.getElementById("root")!, {
  // Boundaries persist only sanitized structural data; do not echo raw props
  // or errors (which may contain subscription credentials) to the console.
  onCaughtError: () => undefined,
  onUncaughtError: (error) => { void reportUIFailure(error, "render", "app"); },
}).render(
  <StrictMode>
    <UIErrorBoundary scope="app"><App /></UIErrorBoundary>
  </StrictMode>,
);
