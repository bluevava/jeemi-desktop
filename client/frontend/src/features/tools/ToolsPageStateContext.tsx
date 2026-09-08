import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  type Dispatch,
  type PropsWithChildren,
  type SetStateAction,
} from "react";

import type { DNSQueryResponse } from "../../types/dnsQuery";

interface ToolsPageSessionState {
  expandedPanels: string[];
  domain: string;
  servers: { proxy: string; direct: string };
  customProxy: boolean;
  response?: DNSQueryResponse;
  error: string;
}

const initialState: ToolsPageSessionState = {
  expandedPanels: ["dns"],
  domain: "",
  servers: { proxy: "https://cloudflare-dns.com/dns-query", direct: "223.5.5.5" },
  customProxy: true,
  error: "",
};

interface ToolsPageStateValue {
  state: ToolsPageSessionState;
  setState: Dispatch<SetStateAction<ToolsPageSessionState>>;
}

const ToolsPageStateContext = createContext<ToolsPageStateValue | null>(null);

// The provider outlives route changes. Query handles stay in the panel so
// navigation still cancels network work without erasing the last results.
export function ToolsPageStateProvider({ children }: PropsWithChildren) {
  const [state, setState] = useState(initialState);
  const value = useMemo(() => ({ state, setState }), [state]);
  return <ToolsPageStateContext.Provider value={value}>{children}</ToolsPageStateContext.Provider>;
}

export function useToolsPageStateField<Key extends keyof ToolsPageSessionState>(
  key: Key,
): [ToolsPageSessionState[Key], Dispatch<SetStateAction<ToolsPageSessionState[Key]>>] {
  const context = useContext(ToolsPageStateContext);
  if (!context) throw new Error("ToolsPageStateProvider is missing");
  const setValue = useCallback<Dispatch<SetStateAction<ToolsPageSessionState[Key]>>>(
    (value) => {
      context.setState((current) => {
        const next = typeof value === "function"
          ? (value as (previous: ToolsPageSessionState[Key]) => ToolsPageSessionState[Key])(current[key])
          : value;
        return Object.is(current[key], next) ? current : { ...current, [key]: next };
      });
    },
    [context.setState, key],
  );
  return [context.state[key], setValue];
}
