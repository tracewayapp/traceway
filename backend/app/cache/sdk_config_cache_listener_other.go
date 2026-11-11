//go:build !transactional_pg

package cache

import "context"

// The embedded builds run a single instance, so the controller's own invalidation is enough.
func (c *sdkConfigCache) startListener(context.Context) error {
	return nil
}
