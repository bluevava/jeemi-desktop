import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import type { LocalConfigResourceState } from "../../types/localConfig";
import {
  ruleSetEntryErrorCodes,
  type RuleMatchType,
  type RuleSetEntryInput,
  type RuleSetEntryPreview,
} from "../../types/ruleSetEntry";
import {
  getLocalConfigResources,
  previewRuleSetEntry,
  saveRuleSetEntry,
} from "../../services/appBridge";
import { errorText } from "../../lib/errorText";
import { unnamedRuleSetName, type ConnectionRuleSeed } from "./ruleEntryModel";

export function useRuleSetEntryDraft(
  seed: ConnectionRuleSeed,
  onSaved: () => void,
) {
  const { t } = useTranslation();
  const [resources, setResources] = useState<LocalConfigResourceState | null>(
    null,
  );
  const [loading, setLoading] = useState(true);
  const [refresh, setRefresh] = useState(0);
  const [loadError, setLoadError] = useState(false);
  const [ruleSetId, setRuleSetId] = useState("");
  const [name, setName] = useState("");
  const [matchType, setMatchType] = useState<RuleMatchType>(seed.matchType);
  const [values, setValues] = useState(seed.values);
  const [caseInsensitive, setCaseInsensitive] = useState(seed.caseInsensitive);
  const [busy, setBusy] = useState(false);
  const [saveError, setSaveError] = useState("");
  const initialized = useRef(false);
  const [preview, setPreview] = useState<{
    key: string;
    value: RuleSetEntryPreview | null;
  } | null>(null);
  const unnamed = t("ruleSetEntry.unnamed");

  useEffect(() => {
    let active = true;
    setLoading(true);
    setLoadError(false);
    void getLocalConfigResources()
      .then((next) => {
        if (!active) return;
        setResources(next);
        if (!initialized.current) {
          setName(unnamedRuleSetName(next.ruleSets, unnamed));
          initialized.current = true;
        }
      })
      .catch(() => {
        if (active) setLoadError(true);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [refresh, unnamed]);

  const input = useMemo<RuleSetEntryInput>(
    () => ({
      ruleSetId,
      name,
      matchType,
      value: values[matchType],
      caseInsensitive,
      expectedRevision: resources?.revision ?? 0,
    }),
    [ruleSetId, name, matchType, values, caseInsensitive, resources?.revision],
  );
  const key = JSON.stringify(input);
  useEffect(() => {
    if (!resources || loading || loadError) return;
    let active = true;
    const timer = window.setTimeout(() => {
      void previewRuleSetEntry(input)
        .then((value) => {
          if (active) setPreview({ key, value });
        })
        .catch(() => {
          if (active) setPreview({ key, value: null });
        });
    }, 250);
    return () => {
      active = false;
      window.clearTimeout(timer);
    };
  }, [input, key, resources, loading, loadError]);

  // Match the exact draft even before effect cleanup; stale validation must
  // never enable Save for a newer input.
  const currentPreview = preview?.key === key ? preview : null;
  const checking = Boolean(
    resources && !loading && !loadError && !currentPreview,
  );
  const errorCode = currentPreview?.value?.errorCode;
  const previewError =
    currentPreview && !currentPreview.value
      ? t("ruleSetEntry.previewError")
      : errorCode
        ? entryErrorMessage(errorCode)
        : "";
  function entryErrorMessage(code: string) {
    return ruleSetEntryErrorCodes.some((known) => known === code)
      ? t(`ruleSetEntry.errors.${code}`)
      : t("ruleSetEntry.previewError");
  }
  const canSave = Boolean(
    !loading && !loadError && !busy && currentPreview?.value?.valid,
  );
  const save = async () => {
    if (!canSave) return;
    setBusy(true);
    setSaveError("");
    try {
      await saveRuleSetEntry(input);
      onSaved();
    } catch (cause) {
      const message = errorText(cause, t("common.unknownError"));
      const code = /^rule entry: ([a-zA-Z]+)$/.exec(message)?.[1];
      setSaveError(code ? entryErrorMessage(code) : message);
    } finally {
      setBusy(false);
    }
  };

  return {
    resources,
    loading,
    loadError,
    refresh: () => {
      setPreview(null);
      setSaveError("");
      setRefresh((value) => value + 1);
    },
    ruleSetId,
    setRuleSetId: (id: string) => {
      setRuleSetId(id);
      const selected = resources?.ruleSets.find((item) => item.id === id);
      if (selected?.behavior === "domain") setMatchType("domain");
      if (selected?.behavior === "ipcidr") setMatchType("ip");
    },
    name,
    setName,
    matchType,
    setMatchType,
    value: values[matchType],
    setValue: (value: string) =>
      setValues((current) => ({ ...current, [matchType]: value })),
    caseInsensitive,
    setCaseInsensitive,
    busy,
    saveError,
    checking,
    previewError,
    preview: currentPreview?.value,
    canSave,
    save,
  };
}
