import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  type PropsWithChildren,
} from "react";

interface GuardState {
  dirty: boolean;
  message: string;
}

interface NavigationGuardValue {
  canLeave: () => boolean;
  clear: () => void;
  set: (state: GuardState) => void;
}

const NavigationGuardContext = createContext<NavigationGuardValue | null>(null);

export function NavigationGuardProvider({ children }: PropsWithChildren) {
  const guard = useRef<GuardState>({ dirty: false, message: "" });

  const set = useCallback((state: GuardState) => {
    guard.current = state;
  }, []);
  const clear = useCallback(() => {
    guard.current = { dirty: false, message: "" };
  }, []);
  const canLeave = useCallback(() => {
    if (!guard.current.dirty) {
      return true;
    }
    return window.confirm(guard.current.message);
  }, []);
  const value = useMemo(() => ({ canLeave, clear, set }), [canLeave, clear, set]);

  return (
    <NavigationGuardContext.Provider value={value}>
      {children}
    </NavigationGuardContext.Provider>
  );
}

export function useNavigationGuard() {
  const value = useContext(NavigationGuardContext);
  if (!value) {
    throw new Error("useNavigationGuard must be used inside NavigationGuardProvider");
  }
  return value;
}

export function useUnsavedChangesGuard(dirty: boolean, message: string) {
  const guard = useNavigationGuard();

  useEffect(() => {
    guard.set({ dirty, message });
    const beforeUnload = (event: BeforeUnloadEvent) => {
      if (!dirty) {
        return;
      }
      event.preventDefault();
      event.returnValue = "";
    };
    window.addEventListener("beforeunload", beforeUnload);
    return () => {
      window.removeEventListener("beforeunload", beforeUnload);
      guard.clear();
    };
  }, [dirty, guard, message]);

  return guard;
}
