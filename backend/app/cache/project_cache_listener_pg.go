//go:build transactional_pg

package cache

import (
	"context"

	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
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
			case notification, ok := <-listener.Notify:
				if !ok {
					return
				}
				if notification == nil {
					continue
				}
				if err := c.Refresh(ctx); err != nil {
					config.Logf("project cache refresh failed after notification for project %s: %v", notification.Extra, err)
				}
			}
		}
	}()

	return nil
}
