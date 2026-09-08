import { useEffect, useMemo, useRef, useState } from "react";

import { useRuntimeStatus } from "../../app/runtime/RuntimeStatusContext";
import type { RuntimePreferences } from "../../types/runtime";

const autoSaveDelayMilliseconds = 450;

export interface RuntimePreferencesEditorState {
  clearError: () => void;
  draft: RuntimePreferences | null;
  errorKey: string;
  loading: boolean;
  updateDraft: (update: Partial<RuntimePreferences>) => void;
}

export function useRuntimePreferencesDraft(): RuntimePreferencesEditorState {
  const {
    runtimePreferences,
    preferencesError,
    preferencesLoading,
    savePreferences,
  } = useRuntimeStatus();
  const [persisted, setPersisted] = useState<RuntimePreferences | null>(null);
  const [draft, setDraft] = useState<RuntimePreferences | null>(null);
  const [errorKey, setErrorKey] = useState("");
  const saveRevision = useRef(0);
  const previousSharedSnapshot = useRef("");
  const dirty = useMemo(
    () =>
      persisted !== null &&
      draft !== null &&
      JSON.stringify(persisted) !== JSON.stringify(draft),
    [draft, persisted],
  );

  useEffect(() => {
    if (!runtimePreferences) return;
    const nextSnapshot = JSON.stringify(runtimePreferences);
    setDraft((current) =>
      current === null ||
      JSON.stringify(current) === previousSharedSnapshot.current
        ? runtimePreferences
        : { ...current, outboundMode: runtimePreferences.outboundMode },
    );
    setPersisted(runtimePreferences);
    previousSharedSnapshot.current = nextSnapshot;
  }, [runtimePreferences]);

  useEffect(() => {
    if (preferencesError && !runtimePreferences) {
      setErrorKey("home.runtimePreferences.errors.load");
    }
  }, [preferencesError, runtimePreferences]);

  useEffect(() => {
    if (!draft || !persisted || !dirty) return;
    const revision = ++saveRevision.current;
    const timer = window.setTimeout(() => {
      setErrorKey("");
      void savePreferences(draft)
        .then((saved) => {
          if (saveRevision.current !== revision) return;
          setPersisted(saved);
          setDraft(saved);
        })
        .catch(() => {
          if (saveRevision.current === revision) {
            setErrorKey("home.runtimePreferences.errors.save");
          }
        });
    }, autoSaveDelayMilliseconds);
    return () => window.clearTimeout(timer);
  }, [dirty, draft, persisted, savePreferences]);

  return {
    clearError: () => setErrorKey(""),
    draft,
    errorKey,
    loading: preferencesLoading,
    updateDraft: (update) => {
      setDraft((current) => (current ? { ...current, ...update } : current));
      setErrorKey("");
    },
  };
}
