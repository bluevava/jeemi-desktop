package application

import "jeemi/internal/mihomo/delaycache"

// ProxyDelayCache returns only observations matching the exact subscription
// revision and associated local-configuration revision shown by the caller.
// A cache from an older projection is treated as empty, never as live health.
func (s *Service) ProxyDelayCache(scope delaycache.Scope) (delaycache.Snapshot, error) {
	return s.delayCache.Load(scope)
}

// SaveProxyDelayCache merges manual Jeemi measurements or mihomo-observed
// automatic health-check results into the managed per-projection cache.
func (s *Service) SaveProxyDelayCache(update delaycache.Update) (delaycache.Snapshot, error) {
	return s.delayCache.Merge(update)
}
