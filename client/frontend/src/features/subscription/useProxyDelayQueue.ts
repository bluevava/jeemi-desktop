import { useCallback, useEffect, useRef, useState } from "react";

import { testProxyDelay } from "../../lib/mihomo/client";
import type { MihomoControllerSession } from "../../types/runtime";
import type {
  ProxyDelayCacheEntry,
  ProxyDelayCacheUpdateEntry,
  ProxyDelaySource,
} from "../../types/subscription";

export type ProxyDelayResult =
  | {
      status: "success";
      delay: number;
      source: ProxyDelaySource;
      testedAt: string;
    }
  | {
      status: "error";
      delay: 0;
      source: ProxyDelaySource;
      testedAt: string;
    };

interface ProxyDelayQueue {
  busyNodes: ReadonlySet<string>;
  queueActive: boolean;
  results: Readonly<Record<string, ProxyDelayResult>>;
  hydrate: (entries: ProxyDelayCacheEntry[]) => void;
  observe: (entries: ProxyDelayCacheUpdateEntry[]) => void;
  testAll: (names: string[]) => Promise<void>;
  testNode: (name: string) => Promise<void>;
}

export function useProxyDelayQueue(
  session: MihomoControllerSession | null,
  concurrency: number,
  scopeKey: string,
  persistResults: (entries: ProxyDelayCacheUpdateEntry[]) => Promise<void>,
): ProxyDelayQueue {
  const [results, setResults] = useState<Record<string, ProxyDelayResult>>({});
  const [busyNodes, setBusyNodes] = useState<Set<string>>(new Set());
  const [queueActive, setQueueActive] = useState(false);
  const activeRef = useRef(false);
  const operationRef = useRef(0);
  const abortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    operationRef.current += 1;
    activeRef.current = false;
    abortRef.current?.abort();
    abortRef.current = null;
    setQueueActive(false);
    setBusyNodes(new Set());
    return () => {
      operationRef.current += 1;
      activeRef.current = false;
      abortRef.current?.abort();
      abortRef.current = null;
    };
  }, [scopeKey, session?.id]);

  useEffect(() => {
    setResults({});
  }, [scopeKey]);

  const run = useCallback(
    async (inputNames: string[], requestedConcurrency: number) => {
      const names = Array.from(
        new Set(inputNames.map((name) => name.trim()).filter(Boolean)),
      );
      if (!session || activeRef.current || names.length === 0) return;

      activeRef.current = true;
      const operation = operationRef.current + 1;
      operationRef.current = operation;
      const controller = new AbortController();
      abortRef.current = controller;
      setQueueActive(true);
      setBusyNodes(new Set(names));
      const completed: ProxyDelayCacheUpdateEntry[] = [];

      let cursor = 0;
      const worker = async () => {
        while (!controller.signal.aborted) {
          const index = cursor;
          cursor += 1;
          if (index >= names.length) return;
          const name = names[index];
          try {
            const delay = await testProxyDelay(session, name, controller.signal);
            const testedAt = new Date().toISOString();
            const result: ProxyDelayResult = {
              status: "success",
              delay,
              source: "manual",
              testedAt,
            };
            completed.push({
              name,
              status: result.status,
              delay: result.delay,
              source: result.source,
            });
            if (operation === operationRef.current) {
              setResults((current) => ({
                ...current,
                [name]: result,
              }));
            }
          } catch {
            if (operation === operationRef.current && !controller.signal.aborted) {
              const result: ProxyDelayResult = {
                status: "error",
                delay: 0,
                source: "manual",
                testedAt: new Date().toISOString(),
              };
              completed.push({
                name,
                status: result.status,
                delay: result.delay,
                source: result.source,
              });
              setResults((current) => ({
                ...current,
                [name]: result,
              }));
            }
          } finally {
            if (operation === operationRef.current) {
              setBusyNodes((current) => {
                const next = new Set(current);
                next.delete(name);
                return next;
              });
            }
          }
        }
      };

      const workerCount = Math.min(
        names.length,
        Math.max(1, Math.min(50, Math.floor(requestedConcurrency) || 1)),
      );
      await Promise.all(Array.from({ length: workerCount }, () => worker()));
      if (operation === operationRef.current) {
        if (completed.length > 0) {
          try {
            await persistResults(completed);
          } catch {
            // The measured result remains useful for this session. The page
            // reports cache persistence errors without turning them into a
            // fake delay-test failure.
          }
        }
        activeRef.current = false;
        abortRef.current = null;
        setQueueActive(false);
        setBusyNodes(new Set());
      }
    },
    [persistResults, session],
  );

  const hydrate = useCallback((entries: ProxyDelayCacheEntry[]) => {
    const hydrated: Record<string, ProxyDelayResult> = {};
    for (const entry of entries) {
      hydrated[entry.name] =
        entry.status === "success"
          ? {
              status: "success",
              delay: entry.delay,
              source: entry.source,
              testedAt: entry.testedAt,
            }
          : {
              status: "error",
              delay: 0,
              source: entry.source,
              testedAt: entry.testedAt,
            };
    }
    setResults((current) => ({ ...hydrated, ...current }));
  }, []);

  const observe = useCallback((entries: ProxyDelayCacheUpdateEntry[]) => {
    if (entries.length === 0) return;
    const testedAt = new Date().toISOString();
    setResults((current) => {
      const next = { ...current };
      for (const entry of entries) {
        next[entry.name] =
          entry.status === "success"
            ? {
                status: "success",
                delay: entry.delay,
                source: entry.source,
                testedAt,
              }
            : {
                status: "error",
                delay: 0,
                source: entry.source,
                testedAt,
              };
      }
      return next;
    });
  }, []);

  const testAll = useCallback(
    (names: string[]) => run(names, concurrency),
    [concurrency, run],
  );
  const testNode = useCallback((name: string) => run([name], 1), [run]);

  return {
    busyNodes,
    queueActive,
    results,
    hydrate,
    observe,
    testAll,
    testNode,
  };
}
