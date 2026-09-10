import {
  createContext,
  useContext,
  useMemo,
  useState,
  type Dispatch,
  type PropsWithChildren,
  type SetStateAction,
} from "react";

export const configSections = ["scripts", "configs", "groups", "ruleSets", "chains"] as const;
export type ConfigSection = (typeof configSections)[number];

export function isConfigSection(value: string): value is ConfigSection {
  return configSections.some(section => section === value);
}

interface ConfigPageState {
  section: ConfigSection;
  setSection: Dispatch<SetStateAction<ConfigSection>>;
  expandedChainGroups: string[];
  setExpandedChainGroups: Dispatch<SetStateAction<string[]>>;
}

const ConfigPageStateContext = createContext<ConfigPageState | null>(null);

export function ConfigPageStateProvider({ children }: PropsWithChildren) {
  const [section, setSection] = useState<ConfigSection>("scripts");
  const [expandedChainGroups, setExpandedChainGroups] = useState<string[]>([]);
  const value = useMemo(() => ({
    section, setSection, expandedChainGroups, setExpandedChainGroups,
  }), [section, expandedChainGroups]);
  return <ConfigPageStateContext.Provider value={value}>{children}</ConfigPageStateContext.Provider>;
}

export function useConfigPageState() {
  const state = useContext(ConfigPageStateContext);
  if (!state) throw new Error("useConfigPageState must be used within ConfigPageStateProvider");
  return state;
}
