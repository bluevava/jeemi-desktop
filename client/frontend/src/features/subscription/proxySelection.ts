import {
  closeConnections,
  getConnections,
  getProxySelections,
  selectProxy,
} from "../../lib/mihomo/client";
import { rememberRuntimeProxySelection } from "../../services/appBridge";
import type { MihomoControllerSession } from "../../types/runtime";
import type { ConnectionResetMode } from "../../types/subscription";
import { connectionIDsForNodeSwitch } from "./connectionReset";

type ProxySelectionResult =
  | { changed: false }
  | { changed: true; remembered: boolean; connectionsReset: boolean };

// A persistence failure must not report an already completed REST switch as
// unsuccessful, undo it, or prevent the existing connection-reset policy.
export async function switchProxySelection(
  session: MihomoControllerSession,
  subscriptionId: string,
  group: string,
  proxy: string,
  resetMode: ConnectionResetMode,
  onConfirmed: () => void,
): Promise<ProxySelectionResult> {
  // The displayed selection can be stale or a static configuration default.
  // Compare the core's direct group member, not a nested group's final exit.
  const selections = await getProxySelections(session);
  if (selections[group] === undefined) {
    throw new Error("mihomo selector selection is unavailable");
  }
  if (selections[group] === proxy) {
    onConfirmed();
    return { changed: false };
  }

  await selectProxy(session, group, proxy);
  onConfirmed();
  let remembered = true;
  try {
    await rememberRuntimeProxySelection(
      session.id,
      subscriptionId,
      group,
      proxy,
    );
  } catch {
    remembered = false;
  }

  let connectionsReset = true;
  if (resetMode !== "off") {
    try {
      const snapshot = await getConnections(session);
      const targetIDs = connectionIDsForNodeSwitch(
        snapshot.connections,
        resetMode,
        group,
      );
      await closeConnections(session, targetIDs);
    } catch {
      connectionsReset = false;
    }
  }
  return { changed: true, remembered, connectionsReset };
}
