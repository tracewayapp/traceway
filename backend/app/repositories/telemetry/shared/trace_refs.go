package shared

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/models"
)

type traceKey struct {
	projectId uuid.UUID
	traceId   uuid.UUID
}

// TraceRefIds returns the distinct project ids and trace ids of refs in
// first-seen order, for the two IN lists of a bulk span lookup.
func TraceRefIds(refs []models.TraceRef) (projectIds, traceIds []uuid.UUID) {
	seenProjects := make(map[uuid.UUID]struct{}, len(refs))
	seenTraces := make(map[uuid.UUID]struct{}, len(refs))
	for _, ref := range refs {
		if _, ok := seenProjects[ref.ProjectId]; !ok {
			seenProjects[ref.ProjectId] = struct{}{}
			projectIds = append(projectIds, ref.ProjectId)
		}
		if _, ok := seenTraces[ref.TraceId]; !ok {
			seenTraces[ref.TraceId] = struct{}{}
			traceIds = append(traceIds, ref.TraceId)
		}
	}
	return projectIds, traceIds
}

// TraceRefsWindowBounds widens the single-trace lookup window so it covers
// every reference: from the earliest RecordedAt to the latest, each padded by
// the usual trace window.
func TraceRefsWindowBounds(refs []models.TraceRef) (time.Time, time.Time) {
	if len(refs) == 0 {
		return time.Time{}, time.Time{}
	}
	earliest, latest := refs[0].RecordedAt, refs[0].RecordedAt
	for _, ref := range refs[1:] {
		if ref.RecordedAt.Before(earliest) {
			earliest = ref.RecordedAt
		}
		if ref.RecordedAt.After(latest) {
			latest = ref.RecordedAt
		}
	}
	from, _ := TraceWindowBounds(earliest)
	_, to := TraceWindowBounds(latest)
	return from, to
}

// FilterSpansByTraceRefs keeps the spans owned by a referenced (project,
// trace) pair. The bulk query's two IN lists match their cross product, so a
// trace id is only trusted inside the project that referenced it.
func FilterSpansByTraceRefs(spans []models.Span, refs []models.TraceRef) []models.Span {
	wanted := make(map[traceKey]struct{}, len(refs))
	for _, ref := range refs {
		wanted[traceKey{ref.ProjectId, ref.TraceId}] = struct{}{}
	}
	kept := make([]models.Span, 0, len(spans))
	for _, s := range spans {
		if _, ok := wanted[traceKey{s.ProjectId, s.TraceId}]; ok {
			kept = append(kept, s)
		}
	}
	return kept
}

// NamedIdList registers ids in params under prefix_N keys and returns the
// matching ":prefix_0, :prefix_1, ..." placeholder list for an IN clause.
func NamedIdList(prefix string, ids []uuid.UUID, params lit.P) string {
	placeholders := make([]string, len(ids))
	for i, id := range ids {
		key := fmt.Sprintf("%s_%d", prefix, i)
		placeholders[i] = ":" + key
		params[key] = id
	}
	return strings.Join(placeholders, ", ")
}
