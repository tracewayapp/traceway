package cache

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
)

func TestProjectCache_ConcurrentAllowlistReadWrite(t *testing.T) {
	id := uuid.New()
	ProjectCache.AddProject(&models.Project{
		Id:                    id,
		Token:                 "tok-" + id.String(),
		ProfileLabelAllowlist: models.StringSlice{"tenant"},
		HealthcheckPaths:      models.StringSlice{"/health"},
	})
	t.Cleanup(func() { ProjectCache.RemoveProject(id) })

	stop := time.Now().Add(200 * time.Millisecond)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		i := 0
		for time.Now().Before(stop) {
			i++
			ProjectCache.UpdateProject(&models.Project{
				Id:                    id,
				ProfileLabelAllowlist: models.StringSlice{"tenant", "region", fmt.Sprintf("k%d", i)},
				HealthcheckPaths:      models.StringSlice{"/health", fmt.Sprintf("/h%d", i)},
			})
		}
	}()

	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(stop) {
				p := ProjectCache.GetById(id)
				if p == nil {
					continue
				}
				sink := 0
				for range p.ProfileLabelAllowlist {
					sink++
				}
				for range p.HealthcheckPaths {
					sink++
				}
				_ = sink
			}
		}()
	}

	wg.Wait()
}
