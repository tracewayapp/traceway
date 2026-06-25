//go:build !pgch

package repositories

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/profiling"
)

func TestProfileRepository_DiscoverLabels(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	from := base.Add(-time.Hour)
	to := base.Add(time.Hour)

	hashX := profiling.HashFrames([]string{"a"})
	sample := func(start time.Time, service, profileType string, labels map[string]string) models.ProfileSample {
		return models.ProfileSample{
			ProjectId: projectId, ProfileId: uuid.New(), ServiceName: service,
			Type: profileType, Start: start, End: start, StackHash: hashX, Value: 1, Labels: labels,
		}
	}
	mustInsertSamples(t, ctx,
		sample(base, "checkout", "cpu", map[string]string{"endpoint": "/a"}),
		sample(base, "checkout", "cpu", map[string]string{"endpoint": "/b"}),
		sample(base, "checkout", "cpu", map[string]string{"endpoint": "/a"}),
		sample(base, "checkout", "cpu", map[string]string{"endpoint": "", "region": "us"}),
		sample(base.Add(-2*time.Hour), "checkout", "cpu", map[string]string{"endpoint": "/before"}),
		sample(base, "billing", "cpu", map[string]string{"endpoint": "/other-service"}),
		sample(base, "checkout", "inuse_space", map[string]string{"endpoint": "/other-type"}),
	)

	labels, err := ProfileRepository.DiscoverLabels(ctx, projectId, "checkout", "cpu", from, to, profiling.LabelAllowKeys(nil))
	if err != nil {
		t.Fatalf("DiscoverLabels: %v", err)
	}

	endpoints := labels["endpoint"]
	if len(endpoints) != 2 || endpoints[0] != "/a" || endpoints[1] != "/b" {
		t.Errorf("endpoint values = %v, want [/a /b] (deduped, sorted, time/service/type filtered)", endpoints)
	}
	if _, ok := labels["region"]; ok {
		t.Errorf("non-allowlisted label key region must not be returned, got %v", labels)
	}
}

func TestProfileRepository_DiscoverLabels_NoValuesOmitsKey(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	from := base.Add(-time.Hour)
	to := base.Add(time.Hour)

	hashX := profiling.HashFrames([]string{"a"})
	mustInsertSamples(t, ctx,
		models.ProfileSample{
			ProjectId: projectId, ProfileId: uuid.New(), ServiceName: "checkout",
			Type: "cpu", Start: base, End: base, StackHash: hashX, Value: 1,
			Labels: map[string]string{"endpoint": ""},
		},
	)

	labels, err := ProfileRepository.DiscoverLabels(ctx, projectId, "checkout", "cpu", from, to, profiling.LabelAllowKeys(nil))
	if err != nil {
		t.Fatalf("DiscoverLabels: %v", err)
	}
	if _, ok := labels["endpoint"]; ok {
		t.Errorf("a label key with no non-empty values must be omitted, got %v", labels)
	}
}

func TestProfileRepository_DiscoverLabels_DottedKey(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	from := base.Add(-time.Hour)
	to := base.Add(time.Hour)

	hashX := profiling.HashFrames([]string{"a"})
	sample := func(labels map[string]string) models.ProfileSample {
		return models.ProfileSample{
			ProjectId: projectId, ProfileId: uuid.New(), ServiceName: "checkout",
			Type: "cpu", Start: base, End: base, StackHash: hashX, Value: 1, Labels: labels,
		}
	}
	mustInsertSamples(t, ctx,
		sample(map[string]string{"endpoint": "/x", "http.route": "GET /checkout"}),
		sample(map[string]string{"endpoint": "/y", "http.route": "GET /cart"}),
	)

	labels, err := ProfileRepository.DiscoverLabels(ctx, projectId, "checkout", "cpu", from, to, profiling.LabelAllowKeys([]string{"http.route"}))
	if err != nil {
		t.Fatalf("DiscoverLabels: %v", err)
	}
	routes := labels["http.route"]
	if len(routes) != 2 || routes[0] != "GET /cart" || routes[1] != "GET /checkout" {
		t.Errorf("http.route values = %v, want [GET /cart GET /checkout] (dotted key must round-trip in SQLite mode)", routes)
	}
}

func TestProfileRepository_DiscoverLabels_LimitsValuesPerKey(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	from := base.Add(-time.Hour)
	to := base.Add(time.Hour)

	hashX := profiling.HashFrames([]string{"a"})
	total := profiling.MaxLabelValuesPerKey + 50
	samples := make([]models.ProfileSample, 0, total)
	for i := range total {
		samples = append(samples, models.ProfileSample{
			ProjectId: projectId, ProfileId: uuid.New(), ServiceName: "checkout",
			Type: "cpu", Start: base, End: base, StackHash: hashX, Value: 1,
			Labels: map[string]string{"endpoint": fmt.Sprintf("/e%05d", i)},
		})
	}
	mustInsertSamples(t, ctx, samples...)

	labels, err := ProfileRepository.DiscoverLabels(ctx, projectId, "checkout", "cpu", from, to, profiling.LabelAllowKeys(nil))
	if err != nil {
		t.Fatalf("DiscoverLabels: %v", err)
	}
	if len(labels["endpoint"]) != profiling.MaxLabelValuesPerKey {
		t.Errorf("endpoint values = %d, want capped at %d", len(labels["endpoint"]), profiling.MaxLabelValuesPerKey)
	}
}
