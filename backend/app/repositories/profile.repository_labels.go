package repositories

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

const discoverLabelsTimeout = 10 * time.Second

func (r *profileRepository) DiscoverLabels(ctx context.Context, projectId uuid.UUID, service, profileType string, from, to time.Time, allowKeys []string) (map[string][]string, error) {
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
			values, err := r.distinctLabelValues(ctx, projectId, service, profileType, key, from, to)
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
