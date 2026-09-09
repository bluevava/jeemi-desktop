import { getConnections, subscribeConnections } from "../../lib/mihomo/client";
import type { MihomoControllerSession } from "../../types/runtime";

type ConnectionHandlers = Parameters<typeof subscribeConnections>[1];

/** Owned by the visible Connections page; cleanup cancels both sources of data. */
export function observeConnections(
  session: MihomoControllerSession,
  handlers: ConnectionHandlers,
): () => void {
  const controller = new AbortController();
  let active = true;
  let receivedStreamSnapshot = false;

  void getConnections(session, controller.signal)
    .then((snapshot) => {
      if (active && !receivedStreamSnapshot) handlers.onMessage(snapshot);
    })
    .catch(() => {
      // The initial REST snapshot only accelerates first paint. Stream state
      // remains authoritative, even if this optional request fails or is cancelled.
    });

  const stopStream = subscribeConnections(session, {
    onMessage: (snapshot) => {
      if (!active) return;
      receivedStreamSnapshot = true;
      controller.abort();
      handlers.onMessage(snapshot);
    },
    onStateChange: (state) => {
      if (active) handlers.onStateChange?.(state);
    },
  });

  return () => {
    if (!active) return;
    active = false;
    controller.abort();
    stopStream();
  };
}
