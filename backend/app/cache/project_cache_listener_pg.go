//go:build transactional_pg

package cache

import (
	"context"
	"fmt"

	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
)

func (c *projectCache) startListener(ctx context.Context) error {
	listener := db.NewPostgresListener()
	if err := listener.Listen(db.ProjectCacheNotificationChannel); err != nil {
		listener.Close()
		return err
	}

	go func() {
		defer listener.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-listener.Notify:
				if !ok {
					return
				}
				if err := c.Refresh(ctx); err != nil {
					traceway.CaptureException(fmt.Errorf("project cache refresh after notification failed: %w", err))
				}
			}
		}
	}()

	return nil
}
