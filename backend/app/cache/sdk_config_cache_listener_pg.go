//go:build transactional_pg

package cache

import (
	"context"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
)

func (c *sdkConfigCache) startListener(ctx context.Context) error {
	listener := db.NewPostgresListener()
	if err := listener.Listen(db.SdkConfigNotificationChannel); err != nil {
		listener.Close()
		return err
	}

	go func() {
		defer listener.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case n, ok := <-listener.Notify:
				if !ok {
					return
				}
				// A nil notification follows a reconnect, after which changes may have been missed.
				if n == nil {
					c.InvalidateAll()
					continue
				}
				if id, err := uuid.Parse(n.Extra); err == nil {
					c.Invalidate(id)
				} else {
					c.InvalidateAll()
				}
			}
		}
	}()
	return nil
}
