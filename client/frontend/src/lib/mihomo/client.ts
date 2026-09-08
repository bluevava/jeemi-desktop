import type { LogLevel, MihomoControllerSession } from "../../types/runtime";

const requestTimeoutMs = 6000;
export const defaultDelayTestURL = "https://www.gstatic.com/generate_204";
export const defaultDelayTestTimeoutMs = 8000;

interface ProxiesResponse {
  proxies?: Record<
    string,
    {
      all?: unknown;
      history?: unknown;
      now?: unknown;
      type?: unknown;
    }
  >;
}

interface DelayResponse {
  delay?: unknown;
}

export interface MihomoConnectionMetadata {
  network: string;
  type: string;
  host: string;
  sourceIP: string;
  sourcePort: string;
  destinationIP: string;
  destinationPort: string;
  process: string;
  processPath: string;
}

export interface MihomoConnection {
  id: string;
  metadata: MihomoConnectionMetadata;
  upload: number;
  download: number;
  start: string;
  rule: string;
  rulePayload: string;
  chains: string[];
  providerChains: string[];
}

export interface MihomoConnectionsSnapshot {
  uploadTotal: number;
  downloadTotal: number;
  memory: number;
  connections: MihomoConnection[];
}

export interface MihomoTrafficSample {
  up: number;
  down: number;
  upTotal: number;
  downTotal: number;
}

export type MihomoLogLevel = Exclude<LogLevel, "silent">;

export interface MihomoLogEntry {
  time: string;
  level: MihomoLogLevel;
  message: string;
  fields: unknown[];
  receivedAt: number;
}

export interface MihomoProxyDelayObservation {
  name: string;
  status: "success" | "error";
  delay: number;
}

export interface MihomoProxyRuntimeState {
  selections: Record<string, string>;
  selectorNames: string[];
  allProxies: MihomoProxyMember[];
  delays: MihomoProxyDelayObservation[];
}

export interface MihomoProxyMember {
  name: string;
  type: string;
}

export interface MihomoRuleProviderRuntimeState {
  name: string;
  ruleCount: number;
  updatedAt: string;
  vehicleType: string;
}

export type MihomoStreamState =
  | "connecting"
  | "live"
  | "reconnecting"
  | "error";

interface StreamHandlers<T> {
  onMessage: (message: T) => void;
  onStateChange?: (state: MihomoStreamState) => void;
}

interface RequestOptions {
  signal?: AbortSignal;
  timeoutMs?: number;
}

export async function getProxySelections(
  session: MihomoControllerSession,
): Promise<Record<string, string>> {
  return (await getProxyRuntimeState(session)).selections;
}

export async function getProxyRuntimeState(
  session: MihomoControllerSession,
): Promise<MihomoProxyRuntimeState> {
  const payload = await request<ProxiesResponse>(session, "/proxies", {
    method: "GET",
  });
  const selections: Record<string, string> = {};
  const selectorNames: string[] = [];
  const delays: MihomoProxyDelayObservation[] = [];
  const proxies = payload.proxies ?? {};
  const groupNames = new Set(
    Object.entries(proxies)
      .filter(([, proxy]) =>
        Array.isArray(proxy?.all) || typeof proxy?.now === "string",
      )
      .map(([name]) => name),
  );
  for (const [name, proxy] of Object.entries(proxies)) {
    if (proxy && typeof proxy.now === "string" && proxy.now.trim()) {
      selections[name] = proxy.now;
    }
    const group = Array.isArray(proxy?.all) || typeof proxy?.now === "string";
    if (group) selectorNames.push(name);
    if (group || !Array.isArray(proxy?.history)) continue;
    const latest = [...proxy.history]
      .reverse()
      .map(asRecord)
      .find((entry) => typeof entry.delay === "number" && Number.isFinite(entry.delay));
    if (!latest) continue;
    const delay = finiteNumber(latest.delay);
    delays.push({
      name,
      status: delay > 0 ? "success" : "error",
      delay: delay > 0 ? Math.round(delay) : 0,
    });
  }
  selectorNames.sort((left, right) => left.localeCompare(right));
  const globalOrder = stringArray(proxies.GLOBAL?.all);
  const allProxies: MihomoProxyMember[] = [];
  const seenProxies = new Set<string>();
  for (const name of [...globalOrder, ...Object.keys(proxies)]) {
    const proxy = proxies[name];
    if (
      !proxy ||
      seenProxies.has(name) ||
      groupNames.has(name) ||
      isBuiltinProxy(name, proxy.type)
    ) {
      continue;
    }
    seenProxies.add(name);
    allProxies.push({ name, type: stringValue(proxy.type) });
  }
  return { selections, selectorNames, allProxies, delays };
}

function isBuiltinProxy(name: string, type: unknown): boolean {
  const normalizedName = name.trim().toLocaleUpperCase();
  const normalizedType = stringValue(type).trim().toLocaleLowerCase();
  return (
    ["DIRECT", "REJECT", "REJECT-DROP", "PASS", "COMPATIBLE"].includes(
      normalizedName,
    ) ||
    ["direct", "reject", "rejectdrop", "pass", "compatible"].includes(
      normalizedType,
    )
  );
}

export async function selectProxy(
  session: MihomoControllerSession,
  group: string,
  proxy: string,
): Promise<void> {
  await request<undefined>(
    session,
    `/proxies/${encodeURIComponent(group)}`,
    {
      method: "PUT",
      body: JSON.stringify({ name: proxy }),
      headers: { "Content-Type": "application/json" },
    },
  );
}

export async function testProxyDelay(
  session: MihomoControllerSession,
  proxy: string,
  signal?: AbortSignal,
): Promise<number> {
  const name = proxy.trim();
  if (!name) throw new Error("proxy name is required");
  const query = new URLSearchParams({
    url: defaultDelayTestURL,
    timeout: String(defaultDelayTestTimeoutMs),
    expected: "200-299",
  });
  const payload = await request<DelayResponse>(
    session,
    `/proxies/${encodeURIComponent(name)}/delay?${query.toString()}`,
    { method: "GET" },
    { signal, timeoutMs: defaultDelayTestTimeoutMs + 2500 },
  );
  const delay = finiteNumber(payload.delay);
  if (delay <= 0) throw new Error("mihomo returned an invalid delay");
  return Math.round(delay);
}

export async function getRuleProviderRuntimeState(
  session: MihomoControllerSession,
): Promise<Record<string, MihomoRuleProviderRuntimeState>> {
  const payload = await request<unknown>(session, "/providers/rules", {
    method: "GET",
  });
  const providers = asRecord(asRecord(payload).providers);
  return Object.fromEntries(
    Object.entries(providers).map(([name, value]) => {
      const provider = asRecord(value);
      return [
        name,
        {
          name,
          ruleCount: finiteNumber(provider.ruleCount),
          updatedAt: timestampValue(provider.updatedAt),
          vehicleType: stringValue(provider.vehicleType),
        },
      ];
    }),
  );
}

export async function getConnections(
  session: MihomoControllerSession,
): Promise<MihomoConnectionsSnapshot> {
  const payload = await request<unknown>(session, "/connections", {
    method: "GET",
  });
  return normaliseConnections(payload);
}

export async function closeConnection(
  session: MihomoControllerSession,
  id: string,
): Promise<void> {
  if (!id.trim()) throw new Error("connection id is required");
  await request<undefined>(
    session,
    `/connections/${encodeURIComponent(id)}`,
    { method: "DELETE" },
  );
}

export async function closeConnections(
  session: MihomoControllerSession,
  inputIDs: string[],
  concurrency = 8,
): Promise<void> {
  const ids = Array.from(new Set(inputIDs.map((id) => id.trim()).filter(Boolean)));
  if (ids.length === 0) return;
  let cursor = 0;
  const failures: string[] = [];
  const worker = async () => {
    while (cursor < ids.length) {
      const index = cursor;
      cursor += 1;
      const id = ids[index];
      try {
        await closeConnection(session, id);
      } catch {
        failures.push(id);
      }
    }
  };
  const workerCount = Math.min(
    ids.length,
    Math.max(1, Math.min(16, Math.floor(concurrency) || 1)),
  );
  await Promise.all(Array.from({ length: workerCount }, () => worker()));
  if (failures.length > 0) {
    throw new Error(`mihomo failed to close ${failures.length} connection(s)`);
  }
}

export function subscribeConnections(
  session: MihomoControllerSession,
  handlers: StreamHandlers<MihomoConnectionsSnapshot>,
): () => void {
  return subscribeControllerStream(
    session,
    "/connections",
    { interval: "1000" },
    normaliseConnections,
    handlers,
  );
}

export function subscribeTraffic(
  session: MihomoControllerSession,
  handlers: StreamHandlers<MihomoTrafficSample>,
): () => void {
  return subscribeControllerStream(
    session,
    "/traffic",
    {},
    normaliseTraffic,
    handlers,
  );
}

export function subscribeLogs(
  session: MihomoControllerSession,
  level: MihomoLogLevel,
  handlers: StreamHandlers<MihomoLogEntry>,
): () => void {
  return subscribeControllerStream(
    session,
    "/logs",
    { level, format: "structured" },
    normaliseLog,
    handlers,
  );
}

function subscribeControllerStream<T>(
  session: MihomoControllerSession,
  path: string,
  query: Record<string, string>,
  normalise: (value: unknown) => T,
  handlers: StreamHandlers<T>,
): () => void {
  if (!isValidSession(session) || typeof WebSocket === "undefined") {
    queueMicrotask(() => handlers.onStateChange?.("error"));
    return () => undefined;
  }

  let stopped = false;
  let socket: WebSocket | null = null;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  let failures = 0;
  const reconnectDelays = [1000, 2000, 5000];

  const scheduleReconnect = () => {
    if (stopped || reconnectTimer !== null) return;
    failures += 1;
    handlers.onStateChange?.("reconnecting");
    const delay = reconnectDelays[Math.min(failures - 1, reconnectDelays.length - 1)];
    reconnectTimer = globalThis.setTimeout(() => {
      reconnectTimer = null;
      connect();
    }, delay);
  };

  const connect = () => {
    if (stopped) return;
    handlers.onStateChange?.(failures === 0 ? "connecting" : "reconnecting");
    try {
      socket = new WebSocket(controllerWebSocketURL(session, path, query));
    } catch {
      handlers.onStateChange?.("error");
      scheduleReconnect();
      return;
    }
    socket.onopen = () => {
      if (stopped) return;
      handlers.onStateChange?.("live");
    };
    socket.onmessage = (event) => {
      if (stopped) return;
      try {
        failures = 0;
        handlers.onMessage(normalise(JSON.parse(String(event.data))));
      } catch {
        // Ignore one malformed frame. A later valid frame restores the live
        // data without turning sensitive controller details into an error.
      }
    };
    socket.onerror = () => {
      if (stopped) return;
      handlers.onStateChange?.("error");
      socket?.close();
    };
    socket.onclose = () => {
      socket = null;
      if (!stopped) scheduleReconnect();
    };
  };

  connect();
  return () => {
    stopped = true;
    if (reconnectTimer !== null) globalThis.clearTimeout(reconnectTimer);
    reconnectTimer = null;
    if (socket && socket.readyState < WebSocket.CLOSING) socket.close();
    socket = null;
  };
}

function controllerWebSocketURL(
  session: MihomoControllerSession,
  path: string,
  query: Record<string, string>,
): string {
  const url = new URL(session.baseUrl);
  url.protocol = "ws:";
  url.pathname = path;
  url.search = "";
  for (const [key, value] of Object.entries(query)) url.searchParams.set(key, value);
  url.searchParams.set("token", session.secret);
  return url.toString();
}

async function request<T>(
  session: MihomoControllerSession,
  path: string,
  init: RequestInit,
  options: RequestOptions = {},
): Promise<T> {
  if (!isValidSession(session)) {
    throw new Error("mihomo controller session is unavailable");
  }
  const controller = new AbortController();
  const abort = () => controller.abort();
  options.signal?.addEventListener("abort", abort, { once: true });
  if (options.signal?.aborted) controller.abort();
  const timeout = globalThis.setTimeout(
    abort,
    options.timeoutMs ?? requestTimeoutMs,
  );
  try {
    const response = await fetch(`${session.baseUrl}${path}`, {
      ...init,
      signal: controller.signal,
      headers: {
        ...init.headers,
        Authorization: `Bearer ${session.secret}`,
      },
    });
    if (!response.ok) {
      throw new Error(`mihomo controller returned HTTP ${response.status}`);
    }
    if (response.status === 204) {
      return undefined as T;
    }
    return (await response.json()) as T;
  } finally {
    globalThis.clearTimeout(timeout);
    options.signal?.removeEventListener("abort", abort);
  }
}

function isValidSession(session: MihomoControllerSession): boolean {
  if (!session.secret) return false;
  try {
    const url = new URL(session.baseUrl);
    return (
      url.protocol === "http:" &&
      url.hostname === "127.0.0.1" &&
      Boolean(url.port) &&
      url.pathname === "/"
    );
  } catch {
    return false;
  }
}

function normaliseTraffic(value: unknown): MihomoTrafficSample {
  const record = asRecord(value);
  return {
    up: finiteNumber(record.up),
    down: finiteNumber(record.down),
    upTotal: finiteNumber(record.upTotal),
    downTotal: finiteNumber(record.downTotal),
  };
}

function normaliseLog(value: unknown): MihomoLogEntry {
  const record = asRecord(value);
  const level = normaliseLogLevel(record.level ?? record.type);
  if (!level) throw new Error("mihomo returned an invalid log level");
  const structured = typeof record.message === "string";
  const fields = Array.isArray(record.fields)
    ? record.fields
    : record.fields === undefined || record.fields === null
      ? []
      : [record.fields];
  return {
    time: structured ? stringValue(record.time) : "",
    level,
    message: structured
      ? stringValue(record.message)
      : stringValue(record.payload),
    fields,
    receivedAt: Date.now(),
  };
}

function normaliseLogLevel(value: unknown): MihomoLogLevel | null {
  switch (stringValue(value).trim().toLocaleLowerCase()) {
    case "debug":
      return "debug";
    case "info":
      return "info";
    case "warn":
    case "warning":
      return "warning";
    case "error":
    case "fatal":
      return "error";
    default:
      return null;
  }
}

function normaliseConnections(value: unknown): MihomoConnectionsSnapshot {
  const record = asRecord(value);
  const connections = Array.isArray(record.connections)
    ? record.connections.map(normaliseConnection).filter((item) => item.id)
    : [];
  return {
    uploadTotal: finiteNumber(record.uploadTotal),
    downloadTotal: finiteNumber(record.downloadTotal),
    memory: finiteNumber(record.memory),
    connections,
  };
}

function normaliseConnection(value: unknown): MihomoConnection {
  const record = asRecord(value);
  const metadata = asRecord(record.metadata);
  return {
    id: stringValue(record.id),
    metadata: {
      network: stringValue(metadata.network),
      type: stringValue(metadata.type),
      host: stringValue(metadata.host),
      sourceIP: stringValue(metadata.sourceIP),
      sourcePort: stringValue(metadata.sourcePort),
      destinationIP: stringValue(metadata.destinationIP),
      destinationPort: stringValue(metadata.destinationPort),
      process: stringValue(metadata.process),
      processPath: stringValue(metadata.processPath),
    },
    upload: finiteNumber(record.upload),
    download: finiteNumber(record.download),
    start: stringValue(record.start),
    rule: stringValue(record.rule),
    rulePayload: stringValue(record.rulePayload),
    chains: stringArray(record.chains),
    providerChains: stringArray(record.providerChains),
  };
}

function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === "object"
    ? (value as Record<string, unknown>)
    : {};
}

function finiteNumber(value: unknown): number {
  return typeof value === "number" && Number.isFinite(value) && value >= 0
    ? value
    : 0;
}

function stringValue(value: unknown): string {
  if (typeof value === "string") return value;
  if (typeof value === "number" && Number.isFinite(value)) return String(value);
  return "";
}

function timestampValue(value: unknown): string {
  const timestamp = stringValue(value);
  const parsed = Date.parse(timestamp);
  if (!Number.isFinite(parsed) || new Date(parsed).getUTCFullYear() <= 1) {
    return "";
  }
  return timestamp;
}

function stringArray(value: unknown): string[] {
  return Array.isArray(value)
    ? value.filter((item): item is string => typeof item === "string")
    : [];
}
