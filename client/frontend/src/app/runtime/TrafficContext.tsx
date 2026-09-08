import {
  createContext,
  type PropsWithChildren,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";

import {
  subscribeTraffic,
  type MihomoStreamState,
  type MihomoTrafficSample,
} from "../../lib/mihomo/client";
import { useRuntimeStatus } from "./RuntimeStatusContext";
import { useWindowActivity } from "./WindowActivityContext";

interface TrafficValue {
  samples: MihomoTrafficSample[];
  streamState: MihomoStreamState;
}

const TrafficContext = createContext<TrafficValue | null>(null);

export function TrafficProvider({ children }: PropsWithChildren) {
  const { liveDataReady, runtime } = useRuntimeStatus();
  const { isWindowVisible } = useWindowActivity();
  const [samples, setSamples] = useState<MihomoTrafficSample[]>([]);
  const [streamState, setStreamState] =
    useState<MihomoStreamState>("error");
  const controllerSession = runtime?.mihomo.controllerSession ?? null;
  const trafficReady = Boolean(
    isWindowVisible &&
      liveDataReady &&
      runtime?.mihomo.state === "running" &&
      runtime.mihomo.controllerReady &&
      controllerSession,
  );

  useEffect(() => {
    setSamples([]);
    if (!trafficReady || !controllerSession) {
      setStreamState("error");
      return () => undefined;
    }
    return subscribeTraffic(controllerSession, {
      onMessage: (sample) => {
        setSamples((current) => [...current.slice(-59), sample]);
      },
      onStateChange: setStreamState,
    });
  }, [controllerSession?.id, trafficReady]);

  const value = useMemo(
    () => ({ samples, streamState }),
    [samples, streamState],
  );

  return (
    <TrafficContext.Provider value={value}>{children}</TrafficContext.Provider>
  );
}

export function useTraffic(): TrafficValue {
  const value = useContext(TrafficContext);
  if (!value) {
    throw new Error("useTraffic must be used within TrafficProvider");
  }
  return value;
}
