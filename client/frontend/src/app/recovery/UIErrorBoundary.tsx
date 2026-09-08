import { Component, createRef, type PropsWithChildren } from "react";
import { initialLanguage, resolveLanguage } from "../../i18n/preferenceLanguage";
import recoveryLabels from "../../i18n/recovery-labels.json";
import type { UIFailure } from "../../types/uiDiagnostics";
import { diagnosticPage, reportUIFailure } from "./diagnostics";
import { canOpenUIDiagnostics, openUIDiagnosticsDirectory, reloadInterface } from "../../services/uiRecoveryBridge";

interface Props extends PropsWithChildren {
  scope: Extract<UIFailure["scope"], "app" | "page" | "status">;
}

interface State {
  failed: boolean;
  diagnostic: "saving" | "saved" | "unavailable";
  actionFailed: boolean;
}

// Deliberately independent of providers, routing, Ant Design and live data:
// it must still render when one of those dependencies caused the failure.
export class UIErrorBoundary extends Component<Props, State> {
  state: State = { failed: false, diagnostic: "saving", actionFailed: false };
  private heading = createRef<HTMLHeadingElement>();

  static getDerivedStateFromError(): Partial<State> {
    return { failed: true, diagnostic: "saving", actionFailed: false };
  }

  componentDidCatch(error: unknown) {
    this.heading.current?.focus();
    void reportUIFailure(error, "render", this.props.scope).then((saved) => {
      this.setState({ diagnostic: saved ? "saved" : "unavailable" });
    });
  }

  private retry = () => {
    this.setState({ failed: false, diagnostic: "saving", actionFailed: false });
  };

  private perform = async (action: () => Promise<void>) => {
    try {
      await action();
    } catch {
      this.setState({ actionFailed: true });
    }
  };

  render() {
    if (!this.state.failed) return this.props.children;
    const language = typeof document === "undefined" ? initialLanguage()
      : resolveLanguage(document.documentElement.lang, initialLanguage());
    const labels = recoveryLabels[language];
    const scope = this.props.scope;
    if (scope === "status") {
      return <button className="ui-recovery-status" title={labels.retry} onClick={this.retry} type="button">
        {labels.statusTitle}
      </button>;
    }
    return (
      <section className={`ui-recovery ui-recovery-${scope}`} role="alert" aria-labelledby={`ui-recovery-${scope}-title`}>
        <div className="ui-recovery-card">
          <h2 id={`ui-recovery-${scope}-title`} ref={this.heading} tabIndex={-1}>{scope === "app" ? labels.appTitle : labels.pageTitle}</h2>
          <p>{labels.description}</p>
          <p>{labels.draftCaution}</p>
          <div className="ui-recovery-actions">
            <button className="ui-recovery-primary" onClick={this.retry} type="button">{labels.retry}</button>
            <button onClick={() => void this.perform(() => reloadInterface(labels.reloadConfirm))} type="button">{labels.reload}</button>
            {scope === "page" && typeof window !== "undefined" && diagnosticPage(window.location.hash) !== "home"
              ? <a href="#/home">{labels.home}</a> : null}
            {canOpenUIDiagnostics() ? (
              <button onClick={() => void this.perform(openUIDiagnosticsDirectory)} type="button">
                {labels.diagnostics}
              </button>
            ) : null}
          </div>
          <p className="ui-recovery-diagnostic">{labels[this.state.diagnostic]}</p>
          {this.state.actionFailed ? <p>{labels.actionFailed}</p> : null}
        </div>
      </section>
    );
  }
}
