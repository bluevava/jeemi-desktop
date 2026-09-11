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

export interface SubscriptionPageSessionState {
  fallbackResetNotices: Record<string, number>;
  activeTabByWorkspace: Record<string, string>;
  expandedSelectorsByWorkspace: Record<string, string[]>;
  nestedSelectorPaths: Record<string, string[]>;
  dismissedWarnings: Record<string, true>;
  selectorQuery: string;
  selectorNameQuery: string;
  shelfExpanded: boolean;
  showHiddenSelectors: boolean;
}

const initialState: SubscriptionPageSessionState = {
  fallbackResetNotices: {},
  activeTabByWorkspace: {},
  expandedSelectorsByWorkspace: {},
  nestedSelectorPaths: {},
  dismissedWarnings: {},
  selectorQuery: "",
  selectorNameQuery: "",
  shelfExpanded: false,
  showHiddenSelectors: false,
};

interface SubscriptionPageStateContextValue {
  state: SubscriptionPageSessionState;
  setState: Dispatch<SetStateAction<SubscriptionPageSessionState>>;
}

const SubscriptionPageStateContext =
  createContext<SubscriptionPageStateContextValue | null>(null);

export function SubscriptionPageStateProvider({
  children,
}: PropsWithChildren) {
  const [state, setState] = useState(initialState);
  const value = useMemo(() => ({ state, setState }), [state]);
  return (
    <SubscriptionPageStateContext.Provider value={value}>
      {children}
    </SubscriptionPageStateContext.Provider>
  );
}

export function useSubscriptionPageStateField<
  Key extends keyof SubscriptionPageSessionState,
>(
  key: Key,
): [
  SubscriptionPageSessionState[Key],
  Dispatch<SetStateAction<SubscriptionPageSessionState[Key]>>,
] {
  const context = useContext(SubscriptionPageStateContext);
  if (!context) {
    throw new Error(
      "useSubscriptionPageStateField must be used within SubscriptionPageStateProvider",
    );
  }
  const setValue = useCallback<
    Dispatch<SetStateAction<SubscriptionPageSessionState[Key]>>
  >(
    (value) => {
      context.setState((current) => {
        const nextValue =
          typeof value === "function"
            ? (
                value as (
                  previous: SubscriptionPageSessionState[Key],
                ) => SubscriptionPageSessionState[Key]
              )(current[key])
            : value;
        return Object.is(current[key], nextValue)
          ? current
          : { ...current, [key]: nextValue };
      });
    },
    [context.setState, key],
  );
  return [context.state[key], setValue];
}

export function setSelectorExpanded(
  current: Record<string, string[]>,
  workspace: string,
  selector: string,
  expanded: boolean,
): Record<string, string[]> {
  const previous = current[workspace] ?? [];
  const contains = previous.includes(selector);
  if (contains === expanded) return current;
  const next = expanded
    ? [...previous, selector]
    : previous.filter((name) => name !== selector);
  if (next.length === 0) {
    const result = { ...current };
    delete result[workspace];
    return result;
  }
  return { ...current, [workspace]: next };
}
