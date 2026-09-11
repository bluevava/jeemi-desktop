package daemon

import (
	"os"
	"path"
	"strconv"

	"jeemi/internal/platform/macnetwork"
	"jeemi/internal/platform/rulecache"
)

// Each authenticated UID owns a distinct protected cache. The core still only
// accesses its private session; no sandbox expansion or user-directory write.
func newRuleCache(root, workspace string, uid uint32, configuration string) (*rulecache.Session, error) {
	cache := rulecache.NewSession(workspace, rulecache.Store{
		Root: root, Path: "cache/rule-providers/" + strconv.FormatUint(uint64(uid), 10),
	})
	if err := prepareRuleCacheFiles(cache, configuration); err != nil {
		return nil, err
	}
	return cache, nil
}

func prepareRuleCacheFiles(cache *rulecache.Session, configuration string) error {
	if cache == nil {
		return nil
	}
	for _, name := range []string{"bootstrap.yaml", "config.yaml"} {
		if err := cache.Prepare(path.Join(path.Dir(configuration), name)); err != nil && !os.IsNotExist(err) {
			return macnetwork.Failure("unsafe_path")
		}
	}
	return nil
}
