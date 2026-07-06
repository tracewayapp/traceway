package shared

import (
	"context"
	"sync"
	"time"
)

const discoverLabelsTimeout = 10 * time.Second

// DiscoverLabels fans out over allowKeys and collects the distinct values per
// key; distinct is the backend-specific query for a single key.
func DiscoverLabels(ctx context.Context, allowKeys []string, distinct func(ctx context.Context, key string) ([]string, error)) (map[string][]string, error) {
	ctx, cancel := context.WithTimeout(ctx, discoverLabelsTimeout)
	defer cancel()

	out := map[string][]string{}
	var mu sync.Mutex
	var firstErr error
	var wg sync.WaitGroup

	for _, key := range allowKeys {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			values, err := distinct(ctx, key)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				return
			}
			if len(values) > 0 {
				out[key] = values
			}
		}(key)
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}
	return out, nil
}
