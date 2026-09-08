import {
  createContext,
  type PropsWithChildren,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";

import { dnsErrorCode, getDNSQueryPreferences } from "../../services/dnsQueryBridge";

interface DNSQueryPreferencesValue {
  customDNS: string;
  setCustomDNS: (value: string) => void;
  loading: boolean;
  error: string;
}

const DNSQueryPreferencesContext = createContext<DNSQueryPreferencesValue | null>(null);

// Load once when the client opens. Editing stays in memory; only QueryDNS
// persists the submitted custom address through the Go settings store.
export function DNSQueryPreferencesProvider({ children }: PropsWithChildren) {
  const [customDNS, setCustomDNS] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;
    void getDNSQueryPreferences().then((preferences) => {
      if (active) setCustomDNS(preferences.customDNS);
    }).catch((caught) => {
      if (active) setError(dnsErrorCode(caught));
    }).finally(() => {
      if (active) setLoading(false);
    });
    return () => { active = false; };
  }, []);

  const value = useMemo(() => ({ customDNS, setCustomDNS, loading, error }), [customDNS, loading, error]);
  return <DNSQueryPreferencesContext.Provider value={value}>{children}</DNSQueryPreferencesContext.Provider>;
}

export function useDNSQueryPreferences() {
  const value = useContext(DNSQueryPreferencesContext);
  if (!value) throw new Error("DNSQueryPreferencesProvider is missing");
  return value;
}
