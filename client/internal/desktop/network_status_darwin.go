//go:build darwin && jeemi_local_test

package desktop

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"jeemi/internal/platform/macnetwork"
)

// Read-only development probe. It creates no application service, user data,
// core or network settings and is absent from release builds.
func runLocalNetworkStatus() bool {
	if len(os.Args) != 2 || os.Args[1] != "--local-network-status" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = json.NewEncoder(os.Stdout).Encode(macnetwork.Inspect(ctx))
	return true
}
