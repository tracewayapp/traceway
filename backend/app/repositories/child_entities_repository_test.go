package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
)

// TestFindChildEntitiesBySpanIds_DispatchJobTopology reproduces the
// dispatch-job tree (HTTP endpoint root → producer span → consumer task → DB
// query under handle()) plus a direct gen_ai child of the endpoint and an
// unrelated sibling endpoint in the same project. It verifies the subtree
// filter:
//
//   - the consumer task surfaces as a transitive child of the endpoint (its
//     parent_span_id = producer, which is inside the endpoint's owned subtree)
//   - the gen_ai trace surfaces as a direct child of the endpoint
//   - the unrelated sibling endpoint does NOT surface (its parent is outside
//     our subtree)
//
// This closes the Phase 2 regression where these rows were silently dropped
// from the parent's waterfall.
func TestFindChildEntitiesBySpanIds_DispatchJobTopology(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	traceId := uuid.New()
	now := truncateMs(time.Now().UTC())

	endpointSpanId := uuid.New()
	producerSpanId := uuid.New()
	consumerSpanId := uuid.New()
	aiTraceSpanId := uuid.New()
	siblingEndpointSpanId := uuid.New()
	unrelatedParent := uuid.New()

	endpoint := models.Endpoint{
		Id:         endpointSpanId,
		ProjectId:  projectId,
		Endpoint:   "GET /test/traceway/dispatch-job",
		Duration:   10 * time.Millisecond,
		RecordedAt: now,
		StatusCode: 200,
		AppVersion: "1.0.0",
		ServerName: "zentigo",
		TraceId:    traceId,
		SpanId:     &endpointSpanId,
	}
	if err := EndpointRepository.InsertAsync(ctx, []models.Endpoint{endpoint}); err != nil {
		t.Fatalf("insert endpoint: %v", err)
	}

	producer := models.Span{
		Id:           producerSpanId,
		TraceId:      traceId,
		ProjectId:    projectId,
		Name:         "queue.dispatch TestTracewayJob",
		StartTime:    now.Add(1 * time.Millisecond),
		Duration:     1 * time.Millisecond,
		RecordedAt:   now.Add(1 * time.Millisecond),
		ParentSpanId: &endpointSpanId,
		EntityId:     &endpointSpanId,
	}
	if err := SpanRepository.InsertAsync(ctx, []models.Span{producer}); err != nil {
		t.Fatalf("insert producer span: %v", err)
	}

	consumer := models.Task{
		Id:           consumerSpanId,
		ProjectId:    projectId,
		TaskName:     "TestTracewayJob",
		Duration:     3 * time.Millisecond,
		RecordedAt:   now.Add(5 * time.Millisecond),
		AppVersion:   "1.0.0",
		ServerName:   "zentigo",
		TraceId:      traceId,
		SpanId:       &consumerSpanId,
		ParentSpanId: &producerSpanId,
	}
	if err := TaskRepository.InsertAsync(ctx, []models.Task{consumer}); err != nil {
		t.Fatalf("insert consumer task: %v", err)
	}

	aiTrace := models.AiTrace{
		Id:           aiTraceSpanId,
		ProjectId:    projectId,
		RecordedAt:   now.Add(2 * time.Millisecond),
		Duration:     1500 * time.Microsecond,
		TraceName:    "chat.completion",
		Model:        "gpt-4o",
		Provider:     "openai",
		TraceId:      traceId,
		SpanId:       &aiTraceSpanId,
		ParentSpanId: &endpointSpanId,
	}
	if err := AiTraceRepository.InsertAsync(ctx, []models.AiTrace{aiTrace}); err != nil {
		t.Fatalf("insert ai trace: %v", err)
	}

	// Sibling endpoint outside our subtree — same project, same trace_id even,
	// but parent points at a span we don't own. Must NOT be returned.
	sibling := models.Endpoint{
		Id:           siblingEndpointSpanId,
		ProjectId:    projectId,
		Endpoint:     "GET /other",
		Duration:     5 * time.Millisecond,
		RecordedAt:   now,
		StatusCode:   200,
		AppVersion:   "1.0.0",
		ServerName:   "other",
		TraceId:      traceId,
		SpanId:       &siblingEndpointSpanId,
		ParentSpanId: &unrelatedParent,
	}
	if err := EndpointRepository.InsertAsync(ctx, []models.Endpoint{sibling}); err != nil {
		t.Fatalf("insert sibling endpoint: %v", err)
	}

	// Mimic the endpoint detail controller: subtreeIds = endpoint.Id + all spans
	// owned by the endpoint. (Here that's just the producer span; in production
	// it would come from SpanRepository.FindByEntityId.)
	subtreeIds := []uuid.UUID{endpointSpanId, producerSpanId}

	refs, err := FindChildEntitiesBySpanIds(ctx, projectId, subtreeIds)
	if err != nil {
		t.Fatalf("FindChildEntitiesBySpanIds: %v", err)
	}

	gotKinds := map[uuid.UUID]string{}
	gotParents := map[uuid.UUID]uuid.UUID{}
	for _, ref := range refs {
		gotKinds[ref.Id] = ref.Kind
		gotParents[ref.Id] = ref.ParentSpanId
	}

	if kind := gotKinds[consumerSpanId]; kind != "task" {
		t.Errorf("consumer task missing or wrong kind — got %q for id %s; refs=%+v", kind, consumerSpanId, refs)
	}
	if parent := gotParents[consumerSpanId]; parent != producerSpanId {
		t.Errorf("consumer parent_span_id = %s, want producer %s", parent, producerSpanId)
	}

	if kind := gotKinds[aiTraceSpanId]; kind != "ai_trace" {
		t.Errorf("ai_trace missing or wrong kind — got %q for id %s; refs=%+v", kind, aiTraceSpanId, refs)
	}
	if parent := gotParents[aiTraceSpanId]; parent != endpointSpanId {
		t.Errorf("ai_trace parent_span_id = %s, want endpoint %s", parent, endpointSpanId)
	}

	if _, present := gotKinds[siblingEndpointSpanId]; present {
		t.Errorf("sibling endpoint should NOT be returned — its parent_span_id is outside the subtree")
	}

	// Three rows expected: consumer, ai_trace. (Sibling excluded; endpoint
	// itself is the parent, not a child.)
	if len(refs) != 2 {
		t.Errorf("expected 2 child entities (task + ai_trace), got %d: %+v", len(refs), refs)
	}

	// Empty input → empty output, no query — guards against accidental
	// `IN ()` SQL errors.
	emptyRefs, err := FindChildEntitiesBySpanIds(ctx, projectId, nil)
	if err != nil {
		t.Fatalf("FindChildEntitiesBySpanIds with nil spanIds: %v", err)
	}
	if len(emptyRefs) != 0 {
		t.Errorf("expected 0 child entities for empty input, got %d", len(emptyRefs))
	}
}
