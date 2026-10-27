package controllers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/notifications"
	"github.com/tracewayapp/traceway/backend/app/oncall"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/slo"

	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

type sloController struct{}

var SLOController = &sloController{}

const (
	sloPreviewWindow   = 24 * time.Hour
	sloMaxSeriesRange  = 91 * 24 * time.Hour
	sloDefaultInterval = 60
	sloMaxSeriesPoints = 2000
)

type sloRequest struct {
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	SliKind       string          `json:"sliKind"`
	SliConfig     json.RawMessage `json:"sliConfig"`
	TargetPercent float64         `json:"targetPercent"`
	WindowDays    int             `json:"windowDays"`
	BucketMinutes int             `json:"bucketMinutes"`
	Enabled       *bool           `json:"enabled"`
}

type sloSeriesRequest struct {
	FromDate        string `json:"fromDate"`
	ToDate          string `json:"toDate"`
	IntervalMinutes int    `json:"intervalMinutes"`
}

type sloPreviewRequest struct {
	SliKind       string          `json:"sliKind"`
	SliConfig     json.RawMessage `json:"sliConfig"`
	BucketMinutes int             `json:"bucketMinutes"`
}

type sloDefinition struct {
	Kind          string
	Config        json.RawMessage
	BucketMinutes int
}

func validateSLIDefinition(tx *sql.Tx, projectId uuid.UUID, kind string, raw json.RawMessage, bucketMinutes int) (sloDefinition, string, error) {
	def := sloDefinition{Kind: kind, BucketMinutes: 1}
	if !models.ValidSLIKind(kind) {
		return def, "SLI kind must be one of: endpoint, span, task, monitor, metric, session.", nil
	}
	if len(raw) == 0 {
		return def, "SLI configuration is required.", nil
	}
	cfg, err := models.ParseSLIConfig(kind, raw)
	if err != nil {
		return def, "The SLI configuration is not valid JSON.", nil
	}
	var canonical any
	switch kind {
	case models.SLIKindEndpoint:
		c := cfg.Endpoint
		c.Endpoint = strings.TrimSpace(c.Endpoint)
		if message := validateGoodCondition(c.Good, c.LatencyThresholdMs); message != "" {
			return def, message, nil
		}
		if c.Good != models.SLOGoodLatency {
			c.LatencyThresholdMs = 0
		}
		canonical = c
	case models.SLIKindSpan:
		c := cfg.Span
		c.ServiceName = strings.TrimSpace(c.ServiceName)
		c.Name = strings.TrimSpace(c.Name)
		if c.Kind != nil && (*c.Kind < 0 || *c.Kind > 5) {
			return def, "Span kind must be between 0 and 5.", nil
		}
		if len(c.Attributes) > models.SLOMaxSpanAttributeFilters {
			return def, fmt.Sprintf("At most %d attribute filters are allowed.", models.SLOMaxSpanAttributeFilters), nil
		}
		attributes := make([]models.SpanSLIAttribute, 0, len(c.Attributes))
		for _, attribute := range c.Attributes {
			key := strings.TrimSpace(attribute.Key)
			if key == "" {
				return def, "Attribute filters need a key.", nil
			}
			if strings.ContainsAny(key, `"\`) || len(key) > 256 {
				return def, "Attribute keys cannot contain quotes or backslashes.", nil
			}
			if attribute.Value == "" {
				return def, "An attribute filter needs a value.", nil
			}
			attributes = append(attributes, models.SpanSLIAttribute{Key: key, Value: attribute.Value})
		}
		c.Attributes = attributes
		if message := validateGoodCondition(c.Good, c.LatencyThresholdMs); message != "" {
			return def, message, nil
		}
		if c.Good != models.SLOGoodLatency {
			c.LatencyThresholdMs = 0
		}
		canonical = c
	case models.SLIKindTask:
		cfg.Task.TaskName = strings.TrimSpace(cfg.Task.TaskName)
		canonical = cfg.Task
	case models.SLIKindMonitor:
		c := cfg.Monitor
		if len(c.CheckIds) == 0 {
			return def, "Select at least one monitor.", nil
		}
		if len(c.CheckIds) > models.SLOMaxMonitors {
			return def, fmt.Sprintf("At most %d monitors are allowed.", models.SLOMaxMonitors), nil
		}
		seen := map[int]bool{}
		ids := make([]int, 0, len(c.CheckIds))
		for _, id := range c.CheckIds {
			if id < 1 || seen[id] {
				continue
			}
			seen[id] = true
			if tx != nil {
				check, err := transactional.SyntheticCheckRepository.FindByIdForProject(tx, id, projectId)
				if err != nil {
					return def, "", err
				}
				if check == nil {
					return def, fmt.Sprintf("Monitor %d was not found in this project.", id), nil
				}
			}
			ids = append(ids, id)
		}
		if len(ids) == 0 {
			return def, "Select at least one monitor.", nil
		}
		c.CheckIds = ids
		canonical = c
	case models.SLIKindMetric:
		c := cfg.Metric
		c.MetricName = strings.TrimSpace(c.MetricName)
		if c.MetricName == "" {
			return def, "Metric name is required.", nil
		}
		if !models.SLOMetricAggregations[c.Aggregation] {
			return def, "Aggregation must be one of: avg, min, max, sum, last.", nil
		}
		if !models.SLOMetricOperators[c.Operator] {
			return def, "Operator must be one of: gt, gte, lt, lte.", nil
		}
		var threshold struct {
			Threshold *float64 `json:"threshold"`
		}
		if json.Unmarshal(raw, &threshold) != nil || threshold.Threshold == nil {
			return def, "Threshold is required.", nil
		}
		if c.Tags == nil {
			c.Tags = map[string]string{}
		}
		if len(c.Tags) > models.SLOMaxMetricTagFilters {
			return def, fmt.Sprintf("At most %d tag filters are allowed.", models.SLOMaxMetricTagFilters), nil
		}
		validSlice := false
		for _, minutes := range models.SLOMetricSliceMinutes {
			if bucketMinutes == minutes {
				validSlice = true
			}
		}
		if !validSlice {
			return def, "Slice length must be 1, 5, 15 or 60 minutes.", nil
		}
		def.BucketMinutes = bucketMinutes
		canonical = c
	case models.SLIKindSession:
		cfg.Session.AppVersion = strings.TrimSpace(cfg.Session.AppVersion)
		canonical = cfg.Session
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return def, "", err
	}
	def.Config = encoded
	return def, "", nil
}

func validateGoodCondition(good string, latencyThresholdMs float64) string {
	switch good {
	case models.SLOGoodAvailability:
		return ""
	case models.SLOGoodLatency:
		if latencyThresholdMs <= 0 {
			return "Latency threshold must be above 0 ms."
		}
		return ""
	}
	return "Good condition must be availability or latency."
}

func validateSLORequest(tx *sql.Tx, projectId uuid.UUID, req *sloRequest) (sloDefinition, string, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	if req.Name == "" {
		return sloDefinition{}, "Name is required.", nil
	}
	if len(req.Name) > 200 {
		return sloDefinition{}, "Name must be 200 characters or fewer.", nil
	}
	if len(req.Description) > 2000 {
		return sloDefinition{}, "Description must be 2000 characters or fewer.", nil
	}
	if req.TargetPercent <= 0 || req.TargetPercent >= 100 {
		return sloDefinition{}, "Target must be above 0 and below 100 percent.", nil
	}
	validWindow := false
	for _, days := range models.SLOWindowDays {
		if req.WindowDays == days {
			validWindow = true
		}
	}
	if !validWindow {
		return sloDefinition{}, "Window must be 7, 30 or 90 days.", nil
	}
	return validateSLIDefinition(tx, projectId, req.SliKind, req.SliConfig, req.BucketMinutes)
}

func sloIdFromPath(ctx *gin.Context) (projectId uuid.UUID, sloId int, ok bool) {
	pid, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return pid, 0, false
	}
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id < 1 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid SLO id"})
		return pid, 0, false
	}
	return pid, id, true
}

func sloRuleTarget(ruleType string, config json.RawMessage) int {
	if !strings.HasPrefix(ruleType, "slo_") {
		return 0
	}
	var cfg struct {
		SloId int `json:"sloId"`
	}
	if err := json.Unmarshal(config, &cfg); err != nil {
		return 0
	}
	return cfg.SloId
}

func (ctrl *sloController) List(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	tx := db.GetTx(ctx)
	slos, err := transactional.SLORepository.FindByProject(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list SLOs: %w", err))
		return
	}
	if slos == nil {
		slos = []*models.SLO{}
	}
	ctx.JSON(http.StatusOK, gin.H{"slos": slos})
}

func (ctrl *sloController) Create(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	var req sloRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		middleware.RejectBindError(ctx, err, "Invalid request body")
		return
	}
	tx := db.GetTx(ctx)
	def, message, err := validateSLORequest(tx, projectId, &req)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to validate SLO: %w", err))
		return
	}
	if message != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
		return
	}
	now := time.Now().UTC()
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	userId := middleware.GetUserId(ctx)
	var createdBy *int
	if userId > 0 {
		createdBy = &userId
	}
	entity := &models.SLO{
		ProjectId:     projectId,
		Name:          req.Name,
		Description:   req.Description,
		SliKind:       def.Kind,
		SliConfig:     models.JSONText(def.Config),
		TargetPercent: req.TargetPercent,
		WindowDays:    req.WindowDays,
		BucketMinutes: def.BucketMinutes,
		Enabled:       enabled,
		Status:        models.SLOStatusNoData,
		SliSince:      &now,
		CreatedBy:     createdBy,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	id, err := transactional.SLORepository.Create(tx, entity)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to create SLO: %w", err))
		return
	}
	entity.Id = id
	middleware.OnCommit(ctx, slo.Wake)
	ctx.JSON(http.StatusCreated, entity)
}

func (ctrl *sloController) Get(ctx *gin.Context) {
	projectId, sloId, ok := sloIdFromPath(ctx)
	if !ok {
		return
	}
	tx := db.GetTx(ctx)
	entity, err := transactional.SLORepository.FindByIdForProject(tx, sloId, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load SLO: %w", err))
		return
	}
	if entity == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "SLO not found"})
		return
	}
	rules, err := transactional.NotificationRuleRepository.FindByProjectWithChannel(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load notification rules for SLO: %w", err))
		return
	}
	referencing := make([]*models.NotificationRuleWithChannel, 0)
	for _, rule := range rules {
		if sloRuleTarget(rule.RuleType, rule.Config) == sloId {
			referencing = append(referencing, rule)
		}
	}
	ctx.JSON(http.StatusOK, gin.H{"slo": entity, "rules": referencing})
}

func (ctrl *sloController) Update(ctx *gin.Context) {
	projectId, sloId, ok := sloIdFromPath(ctx)
	if !ok {
		return
	}
	var req sloRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		middleware.RejectBindError(ctx, err, "Invalid request body")
		return
	}
	tx := db.GetTx(ctx)
	existing, err := transactional.SLORepository.FindByIdForProject(tx, sloId, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load SLO: %w", err))
		return
	}
	if existing == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "SLO not found"})
		return
	}
	def, message, err := validateSLORequest(tx, projectId, &req)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to validate SLO: %w", err))
		return
	}
	if message != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
		return
	}
	definitionChanged := existing.SliKind != def.Kind || existing.BucketMinutes != def.BucketMinutes || !sameSLIConfig(existing, def)

	existing.Name = req.Name
	existing.Description = req.Description
	existing.SliKind = def.Kind
	existing.SliConfig = models.JSONText(def.Config)
	existing.TargetPercent = req.TargetPercent
	existing.WindowDays = req.WindowDays
	existing.BucketMinutes = def.BucketMinutes
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	existing.UpdatedAt = time.Now().UTC()
	if err := transactional.SLORepository.Update(tx, existing); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update SLO: %w", err))
		return
	}
	if definitionChanged {
		if err := transactional.SLORepository.ResetHistory(tx, sloId, existing.UpdatedAt); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to reset SLO history: %w", err))
			return
		}
	}
	middleware.OnCommit(ctx, slo.Wake)
	updated, err := transactional.SLORepository.FindByIdForProject(tx, sloId, projectId)
	if err != nil || updated == nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to reload SLO: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, updated)
}

func sameSLIConfig(existing *models.SLO, def sloDefinition) bool {
	stored, message, _ := validateSLIDefinition(nil, existing.ProjectId, existing.SliKind, json.RawMessage(existing.SliConfig), existing.BucketMinutes)
	if message != "" {
		return false
	}
	return bytes.Equal(stored.Config, def.Config)
}

func (ctrl *sloController) Delete(ctx *gin.Context) {
	projectId, sloId, ok := sloIdFromPath(ctx)
	if !ok {
		return
	}
	tx := db.GetTx(ctx)
	entity, err := transactional.SLORepository.FindByIdForProject(tx, sloId, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load SLO: %w", err))
		return
	}
	if entity == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "SLO not found"})
		return
	}
	rules, err := transactional.NotificationRuleRepository.FindByProjectWithChannel(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load notification rules for SLO cleanup: %w", err))
		return
	}
	now := time.Now().UTC()
	for _, rule := range rules {
		if sloRuleTarget(rule.RuleType, rule.Config) != sloId {
			continue
		}
		if _, err := oncall.AutoResolveByDedupKeyTx(tx, rule.Id, notifications.SLODedupToken(sloId), now); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to resolve pages for deleted SLO: %w", err))
			return
		}
		if err := transactional.NotificationRuleRepository.Delete(tx, rule.Id); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete rule of deleted SLO: %w", err))
			return
		}
		notifications.ClearRuleState(rule.Id)
	}
	if err := transactional.SLORepository.Delete(tx, sloId, projectId); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete SLO: %w", err))
		return
	}
	middleware.OnCommit(ctx, func() {
		if err := telemetry.SLOBucketRepository.DeleteBySLO(context.Background(), projectId, sloId); err != nil {
			traceway.CaptureException(fmt.Errorf("failed to delete buckets of deleted SLO %d: %w", sloId, err))
		}
	})
	ctx.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (ctrl *sloController) Series(ctx *gin.Context) {
	projectId, sloId, ok := sloIdFromPath(ctx)
	if !ok {
		return
	}
	var req sloSeriesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		middleware.RejectBindError(ctx, err, "Invalid request body")
		return
	}
	from, to, ok := parseCheckRange(ctx, req.FromDate, req.ToDate)
	if !ok {
		return
	}
	if !to.After(from) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "toDate must be after fromDate."})
		return
	}
	if to.Sub(from) > sloMaxSeriesRange {
		from = to.Add(-sloMaxSeriesRange)
	}
	interval := req.IntervalMinutes
	if interval < 1 {
		interval = sloDefaultInterval
	}
	if interval > 1440 {
		interval = 1440
	}
	if minimum := int((to.Sub(from).Minutes() + sloMaxSeriesPoints - 1) / sloMaxSeriesPoints); interval < minimum {
		interval = minimum
	}
	points, err := telemetry.SLOBucketRepository.Series(ctx, projectId, sloId, from, to, interval)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load SLO series: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"buckets": points})
}

func (ctrl *sloController) Preview(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	var req sloPreviewRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		middleware.RejectBindError(ctx, err, "Invalid request body")
		return
	}
	def, message, err := validateSLIDefinition(nil, projectId, req.SliKind, req.SliConfig, req.BucketMinutes)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to validate the SLO preview: %w", err))
		return
	}
	if message != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
		return
	}
	draft := &models.SLO{ProjectId: projectId, SliKind: def.Kind, SliConfig: models.JSONText(def.Config), BucketMinutes: def.BucketMinutes}
	to := time.Now().UTC().Truncate(draft.BucketDuration())
	draft.SliSince = &to
	from := to.Add(-sloPreviewWindow)
	points, err := slo.ComputeBuckets(ctx, draft, from, to)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to preview SLO: %w", err))
		return
	}
	var window slo.Window
	for _, point := range points {
		window.Good += point.Good
		window.Total += point.Total
	}
	response := gin.H{"good": window.Good, "total": window.Total, "sli": nil, "from": from, "to": to}
	if sli, ok := window.SLI(); ok {
		response["sli"] = sli * 100
	}
	ctx.JSON(http.StatusOK, response)
}

func (ctrl *sloController) BreachedCount(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	tx := db.GetTx(ctx)
	count, err := transactional.SLORepository.CountBreachedByProject(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to count breached SLOs: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"count": count})
}
