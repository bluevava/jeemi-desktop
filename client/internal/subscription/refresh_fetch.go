package subscription

import (
	"context"
	"errors"
	"fmt"
)

// fetchRefresh retries only acquisition, before composition or persistence.
// Resolve the alternate route after failure so a stopped core is never reused.
func (m *Manager) fetchRefresh(ctx context.Context, sourceURL string) (FetchedContent, error) {
	if err := ctx.Err(); err != nil {
		return FetchedContent{}, err
	}
	fetched, err := m.fetcher.Fetch(ctx, sourceURL)
	if ctx.Err() != nil {
		return FetchedContent{}, ctx.Err()
	}
	if err == nil || m.refreshFallback == nil {
		return fetched, err
	}
	if errors.Is(err, context.Canceled) {
		return FetchedContent{}, err
	}
	fallback := m.refreshFallback()
	if fallback == nil {
		return FetchedContent{}, err
	}
	fetched, retryErr := fallback.Fetch(ctx, sourceURL)
	if retryErr != nil {
		return FetchedContent{}, fmt.Errorf("%v; direct retry failed: %w", err, retryErr)
	}
	if err := ctx.Err(); err != nil {
		return FetchedContent{}, err
	}
	return fetched, nil
}
