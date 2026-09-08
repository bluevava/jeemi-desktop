import { selectProxy } from "../../lib/mihomo/client";
import { rememberRuntimeProxySelection } from "../../services/appBridge";
import type { MihomoControllerSession } from "../../types/runtime";

// A persistence failure must not report an already completed REST switch as
// unsuccessful, undo it, or prevent the existing connection-reset policy.
export async function selectAndRememberProxy(
  session: MihomoControllerSession,
  subscriptionId: string,
  group: string,
  proxy: string,
  onSelected: () => void,
): Promise<boolean> {
  await selectProxy(session, group, proxy);
  onSelected();
  try {
    await rememberRuntimeProxySelection(
      session.id,
      subscriptionId,
      group,
      proxy,
    );
    return true;
  } catch {
    return false;
  }
}
