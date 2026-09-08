import {
  createContext,
  type PropsWithChildren,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";

import { subscribeWindowActivity } from "../../services/windowBridge";

interface WindowActivityValue {
  isWindowVisible: boolean;
}

const WindowActivityContext = createContext<WindowActivityValue | null>(null);

export function WindowActivityProvider({ children }: PropsWithChildren) {
  const [nativeVisible, setNativeVisible] = useState(true);
  const [documentVisible, setDocumentVisible] = useState(
    () => document.visibilityState !== "hidden",
  );
  const isWindowVisible = nativeVisible && documentVisible;

  useEffect(() => {
    const syncDocumentVisibility = () => {
      setDocumentVisible(document.visibilityState !== "hidden");
    };
    const unsubscribe = subscribeWindowActivity((activity) => {
      setNativeVisible(activity === "visible");
    });
    document.addEventListener("visibilitychange", syncDocumentVisibility);
    return () => {
      unsubscribe();
      document.removeEventListener("visibilitychange", syncDocumentVisibility);
    };
  }, []);

  useEffect(() => {
    document.documentElement.dataset.windowActivity = isWindowVisible
      ? "visible"
      : "hidden";
    return () => {
      delete document.documentElement.dataset.windowActivity;
    };
  }, [isWindowVisible]);

  const value = useMemo(
    () => ({ isWindowVisible }),
    [isWindowVisible],
  );

  return (
    <WindowActivityContext.Provider value={value}>
      {children}
    </WindowActivityContext.Provider>
  );
}

export function useWindowActivity(): WindowActivityValue {
  const value = useContext(WindowActivityContext);
  if (!value) {
    throw new Error(
      "useWindowActivity must be used within WindowActivityProvider",
    );
  }
  return value;
}
