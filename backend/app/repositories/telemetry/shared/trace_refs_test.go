package shared

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/models"
)

func TestTraceRefIdsDedupesInFirstSeenOrder(t *testing.T) {
	p1, p2 := uuid.New(), uuid.New()
	t1, t2, t3 := uuid.New(), uuid.New(), uuid.New()
	projectIds, traceIds := TraceRefIds([]models.TraceRef{
		{ProjectId: p1, TraceId: t1},
		{ProjectId: p2, TraceId: t2},
		{ProjectId: p1, TraceId: t3},
		{ProjectId: p2, TraceId: t2},
	})
	if len(projectIds) != 2 || projectIds[0] != p1 || projectIds[1] != p2 {
		t.Fatalf("projectIds = %v, want [%s %s]", projectIds, p1, p2)
	}
	if len(traceIds) != 3 || traceIds[0] != t1 || traceIds[1] != t2 || traceIds[2] != t3 {
		t.Fatalf("traceIds = %v, want [%s %s %s]", traceIds, t1, t2, t3)
	}
}

func TestTraceRefsWindowBoundsCoversEveryRef(t *testing.T) {
	base := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	from, to := TraceRefsWindowBounds([]models.TraceRef{
		{RecordedAt: base.Add(3 * time.Hour)},
		{RecordedAt: base},
		{RecordedAt: base.Add(time.Hour)},
	})
	if want := base.Add(-traceLookupWindow); !from.Equal(want) {
		t.Errorf("from = %v, want %v", from, want)
	}
	if want := base.Add(3*time.Hour + traceLookupWindow); !to.Equal(want) {
		t.Errorf("to = %v, want %v", to, want)
	}

	from, to = TraceRefsWindowBounds(nil)
	if !from.IsZero() || !to.IsZero() {
		t.Errorf("empty refs gave a window %v..%v", from, to)
	}
}

func TestFilterSpansByTraceRefsKeepsReferencedPairsOnly(t *testing.T) {
	p1, p2 := uuid.New(), uuid.New()
	t1, t2 := uuid.New(), uuid.New()
	refs := []models.TraceRef{{ProjectId: p1, TraceId: t1}, {ProjectId: p2, TraceId: t2}}
	kept := FilterSpansByTraceRefs([]models.Span{
		{Name: "p1/t1", ProjectId: p1, TraceId: t1},
		{Name: "p2/t1", ProjectId: p2, TraceId: t1},
		{Name: "p1/t2", ProjectId: p1, TraceId: t2},
		{Name: "p2/t2", ProjectId: p2, TraceId: t2},
	}, refs)
	if len(kept) != 2 || kept[0].Name != "p1/t1" || kept[1].Name != "p2/t2" {
		t.Fatalf("kept = %+v, want the two referenced pairs", kept)
	}
	if kept = FilterSpansByTraceRefs(nil, refs); kept == nil || len(kept) != 0 {
		t.Fatalf("no spans should give an empty, non-nil slice, got %#v", kept)
	}
}

func TestNamedIdListRegistersEveryId(t *testing.T) {
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	params := lit.P{}
	if got := NamedIdList("pid", ids, params); got != ":pid_0, :pid_1, :pid_2" {
		t.Fatalf("placeholders = %q", got)
	}
	for i, id := range ids {
		key := "pid_" + string(rune('0'+i))
		if params[key] != id {
			t.Errorf("params[%s] = %v, want %s", key, params[key], id)
		}
	}
	if got := NamedIdList("tid", nil, params); got != "" {
		t.Fatalf("empty ids gave %q", got)
	}
}
