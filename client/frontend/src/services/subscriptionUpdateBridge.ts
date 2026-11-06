export function subscribeSubscriptionUpdates(listener: () => void): () => void {
  const unsubscribe = window.runtime?.EventsOn?.(
    "jeemi:subscription-updated",
    listener,
  );
  return typeof unsubscribe === "function" ? unsubscribe : () => undefined;
}
