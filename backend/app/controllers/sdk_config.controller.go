package controllers

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/cache"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/hooks"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/sdkconfig"
	"github.com/tracewayapp/traceway/backend/app/traceway"
)

const (
	maxSdkConfigRules      = 50
	maxActiveSdkCaptures   = 20
	maxSdkConfigRuleName   = 100
	maxCaptureMinutes      = 24 * 60
	defaultCaptureLookback = 600
	captureListLimit       = 50
)

type sdkConfigController struct{}

var SdkConfigController = sdkConfigController{}

type sdkConfigRuleRequest struct {
	ExpectedUpdatedAt string          `json:"expectedUpdatedAt"`
	Name              string          `json:"name"`
	Enabled           *bool           `json:"enabled"`
	Conditions        json.RawMessage `json:"conditions"`
	Settings          json.RawMessage `json:"settings"`
}

// canonicalRule validates a rule and returns its documents re-encoded, so stored rows always parse and a client
// gets the same bytes, and hash, for the same rule.
func canonicalRule(req sdkConfigRuleRequest) (conditions, settings []byte, err error) {
	if strings.TrimSpace(req.Name) == "" || len(req.Name) > maxSdkConfigRuleName {
		return nil, nil, &sdkconfig.ValidationError{Message: "name is required and must be at most 100 characters"}
	}
	parsedConditions, err := sdkconfig.ParseConditions(req.Conditions)
	if err != nil {
		return nil, nil, err
	}
	parsedSettings, err := sdkconfig.ParseSettings(req.Settings)
	if err != nil {
		return nil, nil, err
	}
	if conditions, err = json.Marshal(parsedConditions); err != nil {
		return nil, nil, err
	}
	settings, err = json.Marshal(parsedSettings)
	return conditions, settings, err
}

func answerSdkConfigError(c *gin.Context, err error, reason string) {
	var invalid *sdkconfig.ValidationError
	if errors.As(err, &invalid) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": invalid.Message})
		return
	}
	c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("%s: %w", reason, err))
}

func sdkConfigProjectId(c *gin.Context) (uuid.UUID, bool) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return uuid.Nil, false
	}
	project, err := transactional.ProjectRepository.FindById(db.GetTx(c), projectId)
	if err != nil {
		answerSdkConfigError(c, err, "load project for SDK config")
		return uuid.Nil, false
	}
	if project == nil || !sdkconfig.SupportsFramework(project.Framework) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Remote SDK configuration is only available for browser frontend projects"})
		return uuid.Nil, false
	}
	return projectId, true
}

func invalidateSdkConfigOnCommit(c *gin.Context, projectId uuid.UUID) {
	middleware.OnCommit(c, func() { cache.SdkConfigCache.Invalidate(projectId) })
}

func (sdkConfigController) ListRules(c *gin.Context) {
	projectId, ok := sdkConfigProjectId(c)
	if !ok {
		return
	}
	rules, err := transactional.SdkConfigRepository.ListRules(db.GetTx(c), projectId)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list SDK config rules: %w", err))
		return
	}
	if rules == nil {
		rules = []*models.SdkConfigRule{}
	}
	c.JSON(http.StatusOK, gin.H{"data": rules, "heartbeatIntervalSeconds": config.OpampHeartbeatSeconds()})
}

func (sdkConfigController) CreateRule(c *gin.Context) {
	projectId, ok := sdkConfigProjectId(c)
	if !ok {
		return
	}
	var req sdkConfigRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RejectBindError(c, err, "Invalid request body")
		return
	}
	conditions, settings, err := canonicalRule(req)
	if err != nil {
		answerSdkConfigError(c, err, "failed to encode SDK config rule")
		return
	}
	tx := db.GetTx(c)
	if err := transactional.SdkConfigRepository.LockProject(tx, projectId); err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to lock SDK configuration: %w", err))
		return
	}
	existing, err := transactional.SdkConfigRepository.ListRules(tx, projectId)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list SDK config rules: %w", err))
		return
	}
	if len(existing) >= maxSdkConfigRules {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "a project can have at most 50 rules"})
		return
	}
	position := 0
	if n := len(existing); n > 0 {
		position = existing[n-1].Position + 1
	}
	userId := middleware.GetUserId(c)
	now := ruleTimestamp()
	rule := &models.SdkConfigRule{
		ProjectId:  projectId,
		Name:       strings.TrimSpace(req.Name),
		Position:   position,
		Enabled:    req.Enabled == nil || *req.Enabled,
		Conditions: conditions,
		Settings:   settings,
		CreatedBy:  &userId,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	id, err := transactional.SdkConfigRepository.CreateRule(tx, rule)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to create SDK config rule: %w", err))
		return
	}
	rule.Id = id
	invalidateSdkConfigOnCommit(c, projectId)
	c.JSON(http.StatusCreated, rule)
}

// ruleTimestamp is microsecond-precise because Postgres stores no more: a nanosecond updatedAt in a response would
// never equal the stored one, so an agent sending it back as expectedUpdatedAt would get a 409.
func ruleTimestamp() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}

func ruleIdParam(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid rule id"})
		return 0, false
	}
	return id, true
}

func (sdkConfigController) UpdateRule(c *gin.Context) {
	projectId, ok := sdkConfigProjectId(c)
	if !ok {
		return
	}
	id, ok := ruleIdParam(c)
	if !ok {
		return
	}
	var req sdkConfigRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RejectBindError(c, err, "Invalid request body")
		return
	}
	conditions, settings, err := canonicalRule(req)
	if err != nil {
		answerSdkConfigError(c, err, "failed to encode SDK config rule")
		return
	}
	tx := db.GetTx(c)
	if err := transactional.SdkConfigRepository.LockProject(tx, projectId); err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to lock SDK configuration: %w", err))
		return
	}
	rule, err := transactional.SdkConfigRepository.FindRule(tx, projectId, id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load SDK config rule: %w", err))
		return
	}
	if rule == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Rule not found"})
		return
	}
	if identity := middleware.GetBearerIdentity(c); identity != nil && identity.IsAgent() && req.ExpectedUpdatedAt == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "expectedUpdatedAt is required for agent updates; use updatedAt from the rule list"})
		return
	}
	if req.ExpectedUpdatedAt != "" {
		expected, err := time.Parse(time.RFC3339Nano, req.ExpectedUpdatedAt)
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "expectedUpdatedAt must be an RFC3339 timestamp"})
			return
		}
		if !rule.UpdatedAt.Equal(expected) {
			c.JSON(http.StatusConflict, gin.H{"error": "Rule changed since it was read; list rules again and reapply the change"})
			return
		}
	}
	rule.Name = strings.TrimSpace(req.Name)
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}
	rule.Conditions = conditions
	rule.Settings = settings
	rule.UpdatedAt = ruleTimestamp()
	if err := transactional.SdkConfigRepository.UpdateRule(tx, rule); err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update SDK config rule: %w", err))
		return
	}
	invalidateSdkConfigOnCommit(c, projectId)
	c.JSON(http.StatusOK, rule)
}

func (sdkConfigController) DeleteRule(c *gin.Context) {
	projectId, ok := sdkConfigProjectId(c)
	if !ok {
		return
	}
	id, ok := ruleIdParam(c)
	if !ok {
		return
	}
	tx := db.GetTx(c)
	rule, err := transactional.SdkConfigRepository.FindRule(tx, projectId, id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load SDK config rule: %w", err))
		return
	}
	if rule == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Rule not found"})
		return
	}
	if err := transactional.SdkConfigRepository.DeleteRule(tx, projectId, id); err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete SDK config rule: %w", err))
		return
	}
	invalidateSdkConfigOnCommit(c, projectId)
	c.JSON(http.StatusOK, gin.H{})
}

func (sdkConfigController) ReorderRules(c *gin.Context) {
	projectId, ok := sdkConfigProjectId(c)
	if !ok {
		return
	}
	var req struct {
		Ids []int `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RejectBindError(c, err, "Invalid request body")
		return
	}
	tx := db.GetTx(c)
	if err := transactional.SdkConfigRepository.LockProject(tx, projectId); err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to lock SDK configuration: %w", err))
		return
	}
	rules, err := transactional.SdkConfigRepository.ListRules(tx, projectId)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list SDK config rules: %w", err))
		return
	}
	known := map[int]bool{}
	for _, rule := range rules {
		known[rule.Id] = true
	}
	seen := map[int]bool{}
	for _, id := range req.Ids {
		if !known[id] || seen[id] {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "ids must list every rule of the project exactly once"})
			return
		}
		seen[id] = true
	}
	if len(seen) != len(known) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "ids must list every rule of the project exactly once"})
		return
	}
	if err := transactional.SdkConfigRepository.SetPositions(tx, projectId, req.Ids); err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to reorder SDK config rules: %w", err))
		return
	}
	invalidateSdkConfigOnCommit(c, projectId)
	c.JSON(http.StatusOK, gin.H{})
}

type sdkConfigPreviewRequest struct {
	Attributes  map[string]string `json:"attributes"`
	InstanceUid string            `json:"instanceUid"`
}

type matchedRule struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

// Preview shows the document a client with the given attributes would receive. It reads the database rather than
// the cache, so it reflects a change saved a moment ago.
func (sdkConfigController) Preview(c *gin.Context) {
	projectId, ok := sdkConfigProjectId(c)
	if !ok {
		return
	}
	var req sdkConfigPreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RejectBindError(c, err, "Invalid request body")
		return
	}
	var instanceUid [16]byte
	if req.InstanceUid != "" {
		parsed, err := uuid.Parse(req.InstanceUid)
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "instanceUid must be a UUID"})
			return
		}
		instanceUid = parsed
	}
	if req.Attributes == nil {
		req.Attributes = map[string]string{}
	}
	if req.InstanceUid != "" && req.Attributes["service.instance.id"] == "" {
		req.Attributes["service.instance.id"] = req.InstanceUid
	}

	tx := db.GetTx(c)
	rules, err := transactional.SdkConfigRepository.ListRules(tx, projectId)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list SDK config rules: %w", err))
		return
	}
	captures, err := transactional.SdkConfigRepository.ListActiveCaptures(tx, projectId, time.Now().UTC())
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list SDK captures: %w", err))
		return
	}
	now := time.Now()
	result := cache.CompileSdkConfig(rules, captures, now).Evaluate(req.Attributes, instanceUid, now)
	if perm, ok := projectIngestPermission(c, projectId); ok {
		sdkconfig.ApplyPermission(&result.Settings, perm.Exceptions, perm.Data, perm.Replay)
	}
	body, hash, err := sdkconfig.Render(result.Settings)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to render SDK config: %w", err))
		return
	}

	names := map[string]matchedRule{}
	for _, rule := range rules {
		names["rule:"+strconv.Itoa(rule.Id)] = matchedRule{Id: rule.Id, Name: rule.Name}
	}
	matched := []matchedRule{}
	for _, id := range result.MatchedRules {
		matched = append(matched, names[id])
	}
	c.JSON(http.StatusOK, gin.H{
		"settings":                 json.RawMessage(body),
		"configHash":               hex.EncodeToString(hash),
		"matchedRules":             matched,
		"matchedCaptures":          append([]string{}, result.MatchedCaptures...),
		"heartbeatIntervalSeconds": config.OpampHeartbeatSeconds(),
	})
}

// projectIngestPermission is the plan's ingest permission for the project's organization, which OpAMP folds into
// every document.
func projectIngestPermission(c *gin.Context, projectId uuid.UUID) (hooks.IngestPermission, bool) {
	project, err := transactional.ProjectRepository.FindById(db.GetTx(c), projectId)
	if err != nil || project == nil || project.OrganizationId == nil {
		return hooks.IngestPermission{}, false
	}
	return hooks.IngestPermissionFor(*project.OrganizationId), true
}

type sdkCaptureRequest struct {
	Target struct {
		Kind  string `json:"kind"`
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"target"`
	DurationMinutes int      `json:"durationMinutes"`
	LookbackSeconds *float64 `json:"lookbackSeconds"`
	Replay          *bool    `json:"replay"`
	Label           string   `json:"label"`
}

var captureTargetKeys = map[string]string{
	"instance": "service.instance.id",
	"user":     "user.id",
	"session":  "session.id",
}

type sdkCaptureView struct {
	*models.SdkCapture
	Active bool   `json:"active"`
	Status string `json:"status"`
}

func (sdkConfigController) ListCaptures(c *gin.Context) {
	projectId, ok := sdkConfigProjectId(c)
	if !ok {
		return
	}
	captures, err := transactional.SdkConfigRepository.ListCaptures(db.GetTx(c), projectId, captureListLimit)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list SDK captures: %w", err))
		return
	}
	now := time.Now()
	views := make([]sdkCaptureView, 0, len(captures))
	for _, capture := range captures {
		active := capture.CancelledAt == nil && capture.ExpiresAt.After(now)
		status := "Waiting for client"
		if capture.DeliveredAt != nil {
			status = "Delivered · awaiting acknowledgement"
		}
		if capture.AppliedAt != nil {
			status = "Applied · awaiting data"
		}
		if capture.ReceivedAt != nil {
			status = "Collecting"
		}
		if !active {
			status = "Expired · no data received"
			if capture.ReceivedAt != nil {
				status = "Completed"
			}
		}
		if capture.CancelledAt != nil {
			status = "Cancelled"
		}
		views = append(views, sdkCaptureView{SdkCapture: capture, Active: active, Status: status})
	}
	c.JSON(http.StatusOK, gin.H{"data": views, "heartbeatIntervalSeconds": config.OpampHeartbeatSeconds()})
}

func (sdkConfigController) CreateCapture(c *gin.Context) {
	projectId, ok := sdkConfigProjectId(c)
	if !ok {
		return
	}
	var req sdkCaptureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RejectBindError(c, err, "Invalid request body")
		return
	}
	key := captureTargetKeys[req.Target.Kind]
	if req.Target.Kind == "attribute" {
		key = req.Target.Key
	}
	value := strings.TrimSpace(req.Target.Value)
	if key == "" || value == "" || len(key) > 128 || len(value) > 256 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "target needs a kind (instance, user, session or attribute with a key) and a value"})
		return
	}
	if req.DurationMinutes < 1 || req.DurationMinutes > maxCaptureMinutes {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "durationMinutes must be between 1 and 1440"})
		return
	}
	lookback := float64(defaultCaptureLookback)
	if req.LookbackSeconds != nil {
		if *req.LookbackSeconds < 0 || *req.LookbackSeconds > sdkconfig.MaxDurationSeconds {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "lookbackSeconds must be between 0 and 86400"})
			return
		}
		lookback = *req.LookbackSeconds
	}
	replay := req.Replay == nil || *req.Replay

	tx := db.GetTx(c)
	if err := transactional.SdkConfigRepository.LockProject(tx, projectId); err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to lock SDK configuration: %w", err))
		return
	}
	existing, err := transactional.SdkConfigRepository.ListActiveCaptures(tx, projectId, time.Now().UTC())
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list SDK captures: %w", err))
		return
	}
	now := time.Now().UTC()
	active := 0
	for _, capture := range existing {
		if capture.ExpiresAt.After(now) {
			active++
		}
	}
	if active >= maxActiveSdkCaptures {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "a project can have at most 20 active captures"})
		return
	}

	conditions, err := json.Marshal(sdkconfig.Conditions{All: []sdkconfig.Condition{{Key: key, Op: "eq", Value: value}}})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to encode capture conditions: %w", err))
		return
	}
	actions, err := json.Marshal(sdkconfig.CaptureActions{LookbackSeconds: &lookback, Signals: []string{"*"}, Replay: &replay})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to encode capture actions: %w", err))
		return
	}
	label := strings.TrimSpace(req.Label)
	if label == "" {
		label = key + " = " + value
	}
	userId := middleware.GetUserId(c)
	capture := &models.SdkCapture{
		Id:         uuid.Must(uuid.NewV7()),
		ProjectId:  projectId,
		Label:      label,
		Conditions: conditions,
		Actions:    actions,
		ExpiresAt:  now.Add(time.Duration(req.DurationMinutes) * time.Minute),
		CreatedBy:  &userId,
		CreatedAt:  now,
	}
	if err := transactional.SdkConfigRepository.CreateCapture(tx, capture); err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to create SDK capture: %w", err))
		return
	}
	invalidateSdkConfigOnCommit(c, projectId)
	c.JSON(http.StatusCreated, sdkCaptureView{SdkCapture: capture, Active: true, Status: "Waiting for client"})
}

func (sdkConfigController) CancelCapture(c *gin.Context) {
	projectId, ok := sdkConfigProjectId(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid capture id"})
		return
	}
	if err := transactional.SdkConfigRepository.CancelCapture(db.GetTx(c), projectId, id, time.Now().UTC()); err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to cancel SDK capture: %w", err))
		return
	}
	invalidateSdkConfigOnCommit(c, projectId)
	c.JSON(http.StatusOK, gin.H{})
}
