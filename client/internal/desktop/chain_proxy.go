package desktop

import (
	"jeemi/internal/chainproxy"
	"jeemi/internal/subscription"
)

func (a *App) GetChainProxyState() (chainproxy.State, error) { return a.service.ChainProxyState() }
func (a *App) SaveChainProxyGroup(input chainproxy.SaveGroupInput) (chainproxy.MutationResult, error) {
	return a.service.SaveChainProxyGroup(input)
}
func (a *App) ImportChainProxyNodes(input chainproxy.ImportInput) (chainproxy.MutationResult, error) {
	return a.service.ImportChainProxyNodes(input)
}
func (a *App) SaveChainProxySource(input chainproxy.SourceInput) (chainproxy.MutationResult, error) {
	return a.service.SaveChainProxySource(input)
}
func (a *App) RefreshChainProxySources(groupID string, revision int) (chainproxy.MutationResult, error) {
	return a.service.RefreshChainProxySources(groupID, revision)
}
func (a *App) CancelChainProxyRefresh() { a.service.CancelChainProxyRefresh() }
func (a *App) DeleteChainProxyItem(groupID, kind, id string, revision int) (chainproxy.MutationResult, error) {
	return a.service.DeleteChainProxyItem(groupID, kind, id, revision)
}
func (a *App) GetChainProxyNodeText(groupID, nodeID string) (string, error) {
	return a.service.ChainProxyNodeText(groupID, nodeID)
}
func (a *App) GetChainProxySource(groupID, sourceID string) (chainproxy.SourceInput, error) {
	return a.service.ChainProxySource(groupID, sourceID)
}
func (a *App) SetSubscriptionChainProxyGroups(id string, groupIDs []string, revision int, libraryRevision int) (subscription.State, error) {
	return a.service.SetSubscriptionChainProxyGroups(id, groupIDs, revision, libraryRevision)
}
