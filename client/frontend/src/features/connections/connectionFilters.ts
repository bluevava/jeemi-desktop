import type { MihomoConnection } from "../../lib/mihomo/client";
import { connectionEndpoint, connectionMatchedRule } from "./connectionPresentation";

export type ConnectionRouteFilter = "all" | "direct" | "proxy";

interface ConnectionFilterInput {
  query: string;
  route: ConnectionRouteFilter;
}

export function filterConnections(
  connections: MihomoConnection[],
  input: ConnectionFilterInput,
): MihomoConnection[] {
  const query = input.query.trim().toLocaleLowerCase();
  return connections.filter((connection) => {
    const direct = isDirectConnection(connection);
    if (input.route === "direct" && !direct) return false;
    if (input.route === "proxy" && (direct || !connection.chains[0])) return false;
    return !query || connectionSearchText(connection).includes(query);
  });
}

export function isDirectConnection(connection: MihomoConnection): boolean {
  const actualOutbound = connection.chains[0]?.trim() ?? "";
  return actualOutbound.toLocaleUpperCase() === "DIRECT";
}

export function connectionTarget(connection: MihomoConnection): string {
  const host = connection.metadata.host || connection.metadata.destinationIP;
  const port = connection.metadata.destinationPort;
  return connectionEndpoint(host, port);
}

function connectionSearchText(connection: MihomoConnection): string {
  return [
    connectionTarget(connection),
    connection.metadata.sourceIP,
    connection.metadata.process,
    connection.metadata.processPath,
    connection.rule,
    connection.rulePayload,
    connectionMatchedRule(connection),
    ...connection.chains,
  ]
    .join(" ")
    .toLocaleLowerCase();
}
