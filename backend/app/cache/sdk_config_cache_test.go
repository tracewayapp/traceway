package cache

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/sdkconfig"
)

func TestSdkConfigInvalidationCannotBeOverwrittenByAnOlderLoad(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	old := &sdkconfig.Compiled{Rules: []sdkconfig.Rule{{Id: "old"}}}
	fresh := &sdkconfig.Compiled{Rules: []sdkconfig.Rule{{Id: "fresh"}}}
	c := &sdkConfigCache{entries: map[uuid.UUID]*SdkConfigEntry{}, now: time.Now}
	c.load = func(uuid.UUID) (*sdkconfig.Compiled, error) {
		if calls.Add(1) == 1 {
			close(started)
			<-release
			return old, nil
		}
		return fresh, nil
	}
	id := uuid.New()
	done := make(chan struct{})
	go func() { defer close(done); _, _ = c.Get(id) }()
	<-started
	c.Invalidate(id)
	entry, err := c.Get(id)
	if err != nil || entry.Compiled != fresh {
		t.Fatalf("fresh load = %v, %v", entry, err)
	}
	close(release)
	<-done
	entry, err = c.Get(id)
	if err != nil || entry.Compiled != fresh {
		t.Fatalf("old load replaced invalidation: %v, %v", entry, err)
	}
}
