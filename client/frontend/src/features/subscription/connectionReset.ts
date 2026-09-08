import type { MihomoConnection } from "../../lib/mihomo/client";
import type { ConnectionResetMode } from "../../types/subscription";

export function connectionIDsForNodeSwitch(
  connections: MihomoConnection[],
  mode: ConnectionResetMode,
  selector: string,
): string[] {
  if (mode === "off") return [];
  return connections
    .filter(
      (connection) =>
        mode === "all" || connection.chains.some((name) => name === selector),
    )
    .map((connection) => connection.id)
    .filter(Boolean);
}
