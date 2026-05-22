package controllers

import (
	"net/http"
	"time"

	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

type endpointDetailController struct{}

type EndpointExceptionInfo struct {
	ExceptionHash string `json:"exceptionHash"`
	StackTrace    string `json:"stackTrace"`
	RecordedAt    string `json:"recordedAt"`
}

type EndpointMessageInfo struct {
	Id            uuid.UUID         `json:"id"`
	ExceptionHash string            `json:"exceptionHash"`
	StackTrace    string            `json:"stackTrace"`
	RecordedAt    string            `json:"recordedAt"`
	Attributes    map[string]string `json:"attributes,omitempty"`
}

type EnrichedSpan struct {
	models.Span
	LinkedAiTraceId   *uuid.UUID `json:"linkedAiTraceId,omitempty"`
	LinkedAiTraceName *string    `json:"linkedAiTraceName,omitempty"`
}

type ParentRefResponse struct {
	Kind    string    `json:"kind"`
	Id      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	TraceId uuid.UUID `json:"traceId"`
}

// ChildEntityResponse mirrors repositories.ChildEntityRef for JSON.
// Duration is nanoseconds to match models.Span.Duration on the wire.
type ChildEntityResponse struct {
	Kind         string    `json:"kind"`
	Id           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	ParentSpanId uuid.UUID `json:"parentSpanId"`
	TraceId      uuid.UUID `json:"traceId"`
	RecordedAt   string    `json:"recordedAt"`
	Duration     int64     `json:"duration"`
}

func toChildEntityResponses(refs []repositories.ChildEntityRef) []ChildEntityResponse {
	out := make([]ChildEntityResponse, 0, len(refs))
	for _, ref := range refs {
		out = append(out, ChildEntityResponse{
			Kind:         ref.Kind,
			Id:           ref.Id,
			Name:         ref.Name,
			ParentSpanId: ref.ParentSpanId,
			TraceId:      ref.TraceId,
			RecordedAt:   ref.RecordedAt.Format(time.RFC3339Nano),
			Duration:     int64(ref.Duration),
		})
	}
	return out
}

type EndpointDetailResponse struct {
	Endpoint      *models.Endpoint       `json:"endpoint"`
	Spans         []EnrichedSpan         `json:"spans"`
	HasSpans      bool                   `json:"hasSpans"`
	Exception     *EndpointExceptionInfo `json:"exception,omitempty"`
	Messages      []EndpointMessageInfo  `json:"messages"`
	ParentRef     *ParentRefResponse     `json:"parentRef,omitempty"`
	ChildEntities []ChildEntityResponse  `json:"childEntities"`
}

func (t endpointDetailController) GetEndpointDetail(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	endpointId, err := uuid.Parse(c.Param("endpointId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid endpointId"})
		return
	}

	// Get endpoint
	span := traceway.StartSpan(c, "loading endpoint")
	endpoint, err := repositories.EndpointRepository.FindById(c, projectId, endpointId)
	span.End()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error loading endpoint: %w", err))
		return
	}
	if endpoint == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Endpoint not found"})
		return
	}

	// Load only spans whose entity_id resolves to this endpoint, so a trace that
	// also contains a queued task (or a sibling endpoint) doesn't leak that
	// subtree into this waterfall. Historical rows (pre-Phase-2) have entity_id
	// NULL — recognise them by the Phase-1 backfill invariant id == trace_id and
	// fall back to the trace-wide query so old detail pages keep working until
	// the operator runs the CH UPDATE to populate entity_id.
	span = traceway.StartSpan(c, "loading spans")
	spans, err := repositories.SpanRepository.FindByEntityId(c, projectId, endpoint.Id)
	span.End()
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading spans: %w", err))
		return
	}
	if len(spans) == 0 && endpoint.Id == endpoint.TraceId {
		span = traceway.StartSpan(c, "loading spans (legacy fallback)")
		spans, err = repositories.SpanRepository.FindByTraceId(c, projectId, endpoint.TraceId)
		span.End()
		if err != nil {
			c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading spans (legacy fallback): %w", err))
			return
		}
	}

	// Enrich spans with AI-trace links so the frontend can render a "View AI trace"
	// icon next to spans that landed in the ai_traces table.
	spanIds := make([]uuid.UUID, 0, len(spans))
	for _, s := range spans {
		spanIds = append(spanIds, s.Id)
	}
	aiRefs, err := repositories.AiTraceRepository.FindBySpanIds(c, projectId, spanIds)
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading linked ai traces: %w", err))
		return
	}
	enrichedSpans := make([]EnrichedSpan, 0, len(spans))
	for _, s := range spans {
		es := EnrichedSpan{Span: s}
		if ref, ok := aiRefs[s.Id]; ok {
			id := ref.Id
			name := ref.TraceName
			es.LinkedAiTraceId = &id
			es.LinkedAiTraceName = &name
		}
		enrichedSpans = append(enrichedSpans, es)
	}

	// Get all linked exceptions and messages
	var exceptionInfo *EndpointExceptionInfo
	var messages []EndpointMessageInfo

	span = traceway.StartSpan(c, "loading exceptions")
	allExceptions, err := repositories.ExceptionStackTraceRepository.FindAllByTraceId(c, projectId, endpoint.TraceId)
	span.End()
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading all exceptions: %w", err))
		return
	}

	for _, exc := range allExceptions {
		if exc.IsMessage {
			// Add to messages list
			messages = append(messages, EndpointMessageInfo{
				Id:            exc.Id,
				ExceptionHash: exc.ExceptionHash,
				StackTrace:    exc.StackTrace,
				RecordedAt:    exc.RecordedAt.Format("2006-01-02T15:04:05Z07:00"),
				Attributes:    exc.Attributes,
			})
		} else if exceptionInfo == nil {
			// Only take the first actual exception
			exceptionInfo = &EndpointExceptionInfo{
				ExceptionHash: exc.ExceptionHash,
				StackTrace:    exc.StackTrace,
				RecordedAt:    exc.RecordedAt.Format("2006-01-02T15:04:05Z07:00"),
			}
		}
	}

	if messages == nil {
		messages = []EndpointMessageInfo{}
	}

	var parentRefResp *ParentRefResponse
	if endpoint.ParentSpanId != nil {
		ref, err := repositories.FindParentRef(c, projectId, *endpoint.ParentSpanId)
		if err != nil {
			c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading parent ref: %w", err))
			return
		}
		if ref != nil {
			parentRefResp = &ParentRefResponse{Kind: ref.Kind, Id: ref.Id, Name: ref.Name, TraceId: ref.TraceId}
		}
	}

	// Find entities nested anywhere in this endpoint's owned-span subtree (the
	// endpoint itself plus every span returned above). Surfaces the
	// dispatched-task / nested-gen_ai cases that Phase 2 stopped including in
	// the waterfall — without re-introducing the cross-subtree leak.
	subtreeIds := make([]uuid.UUID, 0, len(spans)+1)
	subtreeIds = append(subtreeIds, endpoint.Id)
	for _, s := range spans {
		subtreeIds = append(subtreeIds, s.Id)
	}
	span = traceway.StartSpan(c, "loading child entities")
	children, err := repositories.FindChildEntitiesBySpanIds(c, projectId, subtreeIds)
	span.End()
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading child entities: %w", err))
		return
	}

	c.JSON(http.StatusOK, EndpointDetailResponse{
		Endpoint:      endpoint,
		Spans:         enrichedSpans,
		HasSpans:      len(enrichedSpans) > 0,
		Exception:     exceptionInfo,
		Messages:      messages,
		ParentRef:     parentRefResp,
		ChildEntities: toChildEntityResponses(children),
	})
}

var EndpointDetailController = endpointDetailController{}
