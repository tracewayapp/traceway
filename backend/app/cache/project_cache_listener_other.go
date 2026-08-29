//go:build !transactional_pg

package cache

import "context"

func (c *projectCache) startListener(context.Context) error {
	return nil
}
