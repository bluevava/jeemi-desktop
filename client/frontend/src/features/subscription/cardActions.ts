import {
  setSubscriptionLocalConfig,
  setSubscriptionLocalScript,
} from "../../services/appBridge";
import { getChainProxyState, setSubscriptionChainProxyGroups } from "../../services/chainProxyBridge";
import type { SubscriptionSummary } from "../../types/subscription";
import type { SubscriptionAssociationKind } from "./components/SubscriptionAssociations";

export async function unbindSubscriptionAssociation(subscription: SubscriptionSummary, kind: SubscriptionAssociationKind) {
  if (kind === "script") return setSubscriptionLocalScript(subscription.id, "");
  if (kind === "config") return setSubscriptionLocalConfig(subscription.id, "");
  const library = await getChainProxyState();
  return setSubscriptionChainProxyGroups(subscription.id, [], subscription.chainProxyRevision, library.revision);
}
