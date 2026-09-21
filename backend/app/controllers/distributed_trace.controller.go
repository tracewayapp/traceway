package controllers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

type distributedTraceController struct{}

type distributedTraceRequest struct {
	RecordedAt *time.Time `json:"recordedAt"`
}

type DistributedTraceNode struct {
	ProjectId       uuid.UUID               `json:"projectId"`
	ProjectName     string                  `json:"projectName"`
	TraceType       string                  `json:"traceType"`
	TraceId         string                  `json:"traceId"`
	SpanId          string                  `json:"spanId"`
	Endpoint        *models.Endpoint        `json:"endpoint,omitempty"`
	Task            *models.Task            `json:"task,omitempty"`
	AiTrace         *models.AiTrace         `json:"aiTrace,omitempty"`
	SpanGraphStatus *models.SpanGraphStatus `json:"spanGraphStatus,omitempty"`
	Spans           []models.Span           `json:"spans"`
	Exception       *EndpointExceptionInfo  `json:"exception,omitempty"`
	// ParentEntitySpanId is the span of the nearest endpoint, task or AI trace above this one, found by walking the
	// whole trace across projects. The hops in between need not belong to any of them, or to a project that has one.
	ParentEntitySpanId string `json:"parentEntitySpanId,omitempty"`

	recordedAt time.Time
}

type DistributedTraceResponse struct {
	TraceId string                 `json:"traceId"`
	Nodes   []DistributedTraceNode `json:"nodes"`
}

func (d distributedTraceController) GetDistributedTrace(c *gin.Context) {
	traceId := shared.NormalizeTraceId(c.Param("traceId"))
	if !validTraceHex(traceId) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid traceId"})
		return
	}

	var request distributedTraceRequest
	if err := c.ShouldBindJSON(&request); err != nil && !errors.Is(err, io.EOF) {
		middleware.RejectBindError(c, err, "Invalid distributed trace request")
		return
	}

	userId := middleware.GetUserId(c)

	projects, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.Project, error) {
		return transactional.ProjectRepository.FindByUserId(tx, userId)
	})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load user projects: %w", err))
		return
	}

	if len(projects) == 0 {
		c.JSON(http.StatusOK, DistributedTraceResponse{TraceId: traceId, Nodes: []DistributedTraceNode{}})
		return
	}

	projectIds := make([]uuid.UUID, len(projects))
	projectNameMap := make(map[uuid.UUID]string, len(projects))
	for i, p := range projects {
		projectIds[i] = p.Id
		projectNameMap[p.Id] = p.Name
	}

	ctx := c.Request.Context()

	found, err := findTraceEntities(ctx, traceId, projectIds, request.RecordedAt)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to query the distributed trace: %w", err))
		return
	}

	nodes := make([]DistributedTraceNode, 0, len(found.endpoints)+len(found.tasks)+len(found.aiTraces))
	for _, ep := range found.endpoints {
		nodes = append(nodes, DistributedTraceNode{
			ProjectId: ep.ProjectId, ProjectName: projectNameMap[ep.ProjectId], TraceType: "endpoint",
			TraceId: ep.TraceId, SpanId: ep.SpanId, Endpoint: &ep, Spans: []models.Span{}, recordedAt: ep.RecordedAt,
		})
	}
	for _, t := range found.tasks {
		nodes = append(nodes, DistributedTraceNode{
			ProjectId: t.ProjectId, ProjectName: projectNameMap[t.ProjectId], TraceType: "task",
			TraceId: t.TraceId, SpanId: t.SpanId, Task: &t, Spans: []models.Span{}, recordedAt: t.RecordedAt,
		})
	}
	for _, a := range found.aiTraces {
		nodes = append(nodes, DistributedTraceNode{
			ProjectId: a.ProjectId, ProjectName: projectNameMap[a.ProjectId], TraceType: "ai_trace",
			TraceId: a.TraceId, SpanId: a.SpanId, AiTrace: &a, Spans: []models.Span{}, recordedAt: a.RecordedAt,
		})
	}

	parents := findTraceParents(ctx, nodes, found.exceptions, projectIds)
	linkDistributedTraceNodes(nodes, parents)

	for _, exc := range found.exceptions {
		info := &EndpointExceptionInfo{
			ExceptionHash: exc.ExceptionHash,
			StackTrace:    exc.StackTrace,
			RecordedAt:    exc.RecordedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if owner := exceptionOwner(nodes, parents, exc); owner >= 0 {
			if nodes[owner].Exception == nil {
				nodes[owner].Exception = info
			}
			continue
		}
		nodes = append(nodes, DistributedTraceNode{
			ProjectId: exc.ProjectId, ProjectName: projectNameMap[exc.ProjectId], TraceType: "exception",
			TraceId: exc.TraceId, SpanId: exc.SpanId, Spans: []models.Span{}, Exception: info, recordedAt: exc.RecordedAt,
		})
	}

	if err := loadDistributedTraceSpans(ctx, nodes); err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to query distributed trace spans: %w", err))
		return
	}

	c.JSON(http.StatusOK, DistributedTraceResponse{TraceId: traceId, Nodes: nodes})
}

func loadDistributedTraceSpans(ctx context.Context, nodes []DistributedTraceNode) error {
	nodeIndexes := make(map[shared.SpanOwner][]int)
	lookups := make([]shared.SpanLookup, 0, len(nodes))
	for i, node := range nodes {
		if node.TraceId == "" || node.SpanId == "" {
			continue
		}
		nodes[i].SpanGraphStatus = &models.SpanGraphStatus{State: models.SpanGraphComplete}
		lookup := shared.NewSpanLookup(node.ProjectId, node.TraceId, node.SpanId, node.recordedAt)
		nodeIndexes[lookup.Owner()] = append(nodeIndexes[lookup.Owner()], i)
		lookups = append(lookups, lookup)
	}
	graphs, err := telemetry.SpanRepository.FindGraphs(ctx, lookups)
	if err != nil {
		return err
	}
	for owner, graph := range graphs {
		for _, i := range nodeIndexes[owner] {
			nodes[i].SpanGraphStatus = &graph.Status
			nodes[i].Spans = graph.Spans
		}
	}
	return nil
}

const (
	maxEntityLinkDepth = 10000
)

type traceEntities struct {
	endpoints  []models.Endpoint
	tasks      []models.Task
	aiTraces   []models.AiTrace
	exceptions []models.ExceptionStackTrace
}

// findTraceEntities reads one trace across the projects the caller can access.
func findTraceEntities(ctx context.Context, traceId string, projectIds []uuid.UUID, recordedAt *time.Time) (*traceEntities, error) {
	if recordedAt == nil || recordedAt.IsZero() {
		now := time.Now().UTC()
		recordedAt = &now
	}
	found := &traceEntities{}
	var err error
	ids := []string{traceId}
	found.endpoints, err = telemetry.EndpointRepository.FindByTraceIds(ctx, ids, projectIds, recordedAt)
	if err != nil {
		return nil, fmt.Errorf("endpoints: %w", err)
	}
	found.tasks, err = telemetry.TaskRepository.FindByTraceIds(ctx, ids, projectIds, recordedAt)
	if err != nil {
		return nil, fmt.Errorf("tasks: %w", err)
	}
	found.aiTraces, err = telemetry.AiTraceRepository.FindByTraceIds(ctx, ids, projectIds, recordedAt)
	if err != nil {
		return nil, fmt.Errorf("ai traces: %w", err)
	}
	found.exceptions, err = telemetry.ExceptionStackTraceRepository.FindByTraceIds(ctx, ids, projectIds, recordedAt)
	if err != nil {
		return nil, fmt.Errorf("exceptions: %w", err)
	}
	endpoints, tasks, aiTraces, exceptions := found.endpoints, found.tasks, found.aiTraces, found.exceptions
	found = &traceEntities{}
	seen := map[string]bool{}
	fresh := func(kind string, projectId, id uuid.UUID, traceId, spanId string) bool {
		key := kind + ":" + projectId.String() + ":" + id.String() + ":" + traceId + ":" + spanId
		if seen[key] {
			return false
		}
		seen[key] = true
		return true
	}
	for _, endpoint := range endpoints {
		if fresh("endpoint", endpoint.ProjectId, endpoint.Id, endpoint.TraceId, endpoint.SpanId) {
			found.endpoints = append(found.endpoints, endpoint)
		}
	}
	for _, task := range tasks {
		if fresh("task", task.ProjectId, task.Id, task.TraceId, task.SpanId) {
			found.tasks = append(found.tasks, task)
		}
	}
	for _, aiTrace := range aiTraces {
		if fresh("ai_trace", aiTrace.ProjectId, aiTrace.Id, aiTrace.TraceId, aiTrace.SpanId) {
			found.aiTraces = append(found.aiTraces, aiTrace)
		}
	}
	for _, exception := range exceptions {
		if fresh("exception", exception.ProjectId, exception.Id, exception.TraceId, exception.SpanId) {
			found.exceptions = append(found.exceptions, exception)
		}
	}
	return found, nil
}

// findTraceParents maps span to parent span for every trace on the card, keyed by trace id. It reads no attributes. The
// subtrees loaded per node are not enough to nest the nodes: a hop that was never promoted, such as a gRPC server
// reporting to its own project, belongs to no subtree, and the nodes on either side of it would show side by side.
func findTraceParents(ctx context.Context, nodes []DistributedTraceNode, exceptions []models.ExceptionStackTrace, projectIds []uuid.UUID) map[string]map[string]string {
	earliest := map[string]time.Time{}
	note := func(traceId string, at time.Time) {
		if traceId == "" {
			return
		}
		if known, ok := earliest[traceId]; !ok || at.Before(known) {
			earliest[traceId] = at
		}
	}
	for _, node := range nodes {
		note(node.TraceId, node.recordedAt)
	}
	for _, exception := range exceptions {
		note(exception.TraceId, exception.RecordedAt)
	}
	parents := map[string]map[string]string{}
	for traceId, at := range earliest {
		traceParents, err := telemetry.SpanRepository.FindTraceParents(ctx, projectIds, traceId, at)
		if err != nil {
			traceway.CaptureException(traceway.NewStackTraceErrorf("failed to read the span parents of trace %s: %w", traceId, err))
			continue
		}
		parents[traceId] = traceParents
	}
	return parents
}

// nearestEntity walks up from a span until it meets the span of a node. It prefers a node of the same project, because
// one payload exported to two projects promotes the same span twice.
func nearestEntity(nodes []DistributedTraceNode, parents map[string]map[string]string, projectId uuid.UUID, traceId, spanId string, includeSelf bool) int {
	bySpan := map[string][]int{}
	for i, node := range nodes {
		if node.TraceId == traceId && node.SpanId != "" && node.TraceType != "exception" {
			bySpan[node.SpanId] = append(bySpan[node.SpanId], i)
		}
	}
	pick := func(candidates []int) int {
		for _, i := range candidates {
			if nodes[i].ProjectId == projectId {
				return i
			}
		}
		return candidates[0]
	}
	parentOf := parents[traceId]
	current := spanId
	if !includeSelf {
		current = parentOf[spanId]
	}
	visited := map[string]bool{}
	for depth := 0; current != "" && !visited[current] && depth < maxEntityLinkDepth; depth++ {
		if candidates := bySpan[current]; len(candidates) > 0 {
			return pick(candidates)
		}
		visited[current] = true
		current = parentOf[current]
	}
	return -1
}

func exceptionOwner(nodes []DistributedTraceNode, parents map[string]map[string]string, exc models.ExceptionStackTrace) int {
	if exc.TraceId == "" || exc.SpanId == "" {
		return -1
	}
	return nearestEntity(nodes, parents, exc.ProjectId, exc.TraceId, exc.SpanId, true)
}

func linkDistributedTraceNodes(nodes []DistributedTraceNode, parents map[string]map[string]string) {
	for i, node := range nodes {
		if node.SpanId == "" {
			continue
		}
		if parent := nearestEntity(nodes, parents, node.ProjectId, node.TraceId, node.SpanId, false); parent >= 0 {
			nodes[i].ParentEntitySpanId = nodes[parent].SpanId
		}
	}
}

var DistributedTraceController = distributedTraceController{}
