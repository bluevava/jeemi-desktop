import { InputNumber, Select, Switch } from "antd";
import { useTranslation } from "react-i18next";

import type {
  ListenerType,
  LogLevel,
  RuntimePreferences,
} from "../../types/runtime";
import {
  RuntimePreferenceItem,
  RuntimePreferenceSection,
} from "./RuntimePreferenceFields";
import { RuntimeYamlResourceEditor } from "./RuntimeYamlResourceEditor";
import { formatYamlStringSequence } from "./runtimeYaml";

export function RuntimeBasicPreferences({
  draft,
  updateDraft,
}: {
  draft: RuntimePreferences;
  updateDraft: (update: Partial<RuntimePreferences>) => void;
}) {
  const { t } = useTranslation();

  return (
    <RuntimePreferenceSection
      helpTopic="runtimeBaseline"
      title={t("home.runtimePreferences.sections.baseline")}
    >
      <div className="runtime-preferences-grid">
        <RuntimePreferenceItem helpTopic="runtimeLogLevel" title={t("home.runtimePreferences.logLevel")}>
          <Select
            aria-label={t("home.runtimePreferences.logLevel")}
            onChange={(logLevel: LogLevel) => updateDraft({ logLevel })}
            options={
              (["silent", "error", "warning", "info", "debug"] as const).map(
                (value) => ({
                  label: t(`home.runtimePreferences.logLevels.${value}`),
                  value,
                }),
              )
            }
            value={draft.logLevel}
          />
        </RuntimePreferenceItem>

        <RuntimePreferenceItem title={t("home.runtimePreferences.ipv6")}>
          <Switch
            aria-label={t("home.runtimePreferences.ipv6")}
            checked={draft.ipv6}
            checkedChildren={t("runtime.on")}
            onChange={(ipv6) => updateDraft({ ipv6 })}
            unCheckedChildren={t("runtime.off")}
          />
        </RuntimePreferenceItem>

        <RuntimePreferenceItem
          helpTopic="runtimeListener"
          title={t("home.runtimePreferences.listener")}
        >
          <div className="runtime-listener-controls">
            <Select
              aria-label={t("home.runtimePreferences.listener")}
              onChange={(listenerType: ListenerType) =>
                updateDraft({ listenerType })
              }
              options={(["http", "socks", "mixed"] as const).map(
                (value) => ({
                  label: t(`home.runtimePreferences.listenerTypes.${value}`),
                  value,
                }),
              )}
              value={draft.listenerType}
            />
            <InputNumber
              aria-label={t("home.runtimePreferences.listenPort")}
              max={65535}
              min={1}
              onChange={(listenPort) => {
                if (typeof listenPort === "number") {
                  updateDraft({ listenPort });
                }
              }}
              precision={0}
              step={1}
              value={draft.listenPort}
            />
          </div>
        </RuntimePreferenceItem>

        <RuntimePreferenceItem
          helpTopic="runtimeLanAccess"
          title={t("home.runtimePreferences.allowLan")}
        >
          <Switch
            aria-label={t("home.runtimePreferences.allowLan")}
            checked={draft.allowLan}
            checkedChildren={t("runtime.on")}
            onChange={(allowLan) => updateDraft({ allowLan })}
            unCheckedChildren={t("runtime.off")}
          />
        </RuntimePreferenceItem>

        <RuntimePreferenceItem title={t("home.runtimePreferences.tunAutoRoute")}>
          <Switch
            aria-label={t("home.runtimePreferences.tunAutoRoute")}
            checked={draft.tunAutoRoute}
            checkedChildren={t("runtime.on")}
            onChange={(tunAutoRoute) => updateDraft({ tunAutoRoute })}
            unCheckedChildren={t("runtime.off")}
          />
        </RuntimePreferenceItem>

        <RuntimePreferenceItem
          title={t("home.runtimePreferences.tunAutoDetectInterface")}
        >
          <Switch
            aria-label={t("home.runtimePreferences.tunAutoDetectInterface")}
            checked={draft.tunAutoDetectInterface}
            checkedChildren={t("runtime.on")}
            onChange={(tunAutoDetectInterface) =>
              updateDraft({ tunAutoDetectInterface })
            }
            unCheckedChildren={t("runtime.off")}
          />
        </RuntimePreferenceItem>
      </div>

      <RuntimeYamlResourceEditor
        enabled={draft.tunRouteExcludeAddressEnabled}
        field="tun.route-exclude-address"
        helpTopic="runtimeTunRouteExclude"
        label={t("home.runtimePreferences.tunRouteExcludeAddress")}
        mergeMode={draft.tunRouteExcludeAddressMerge}
        onCommit={(result) =>
          updateDraft({ tunRouteExcludeAddress: result.values })
        }
        onEnabledChange={(tunRouteExcludeAddressEnabled) =>
          updateDraft({ tunRouteExcludeAddressEnabled })
        }
        onMergeModeChange={(tunRouteExcludeAddressMerge) =>
          updateDraft({ tunRouteExcludeAddressMerge })
        }
        placeholder={t(
          "home.runtimePreferences.tunRouteExcludeAddressPlaceholder",
        )}
        sourceText={formatYamlStringSequence(draft.tunRouteExcludeAddress)}
      />
    </RuntimePreferenceSection>
  );
}
