package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/netguard"
	"github.com/tracewayapp/traceway/backend/app/oncall"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/storage"
	"github.com/tracewayapp/traceway/backend/app/synthetics"

	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

type syntheticCheckController struct{}

var validCheckTypes = map[string]bool{
	models.CheckTypeHttp: true, models.CheckTypeTcp: true, models.CheckTypeBrowser: true,
}

var validHttpMethods = map[string]bool{
	"GET": true, "HEAD": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true, "OPTIONS": true,
}

const maxBrowserScriptBytes = 100 << 10

type checkRequest struct {
	Name             string          `json:"name"`
	CheckType        string          `json:"checkType"`
	Enabled          *bool           `json:"enabled"`
	IntervalSeconds  int             `json:"intervalSeconds"`
	TimeoutSeconds   int             `json:"timeoutSeconds"`
	FailureThreshold int             `json:"failureThreshold"`
	Config           json.RawMessage `json:"config"`
}

// validateCheckRequest returns a 422 message, or an empty string when valid.
func validateCheckRequest(req *checkRequest) string {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return "Name is required."
	}
	if len(req.Name) > 200 {
		return "Name must be 200 characters or fewer."
	}
	if !validCheckTypes[req.CheckType] {
		return "Check type must be one of: http, tcp, browser."
	}
	if req.IntervalSeconds < synthetics.MinIntervalSeconds || req.IntervalSeconds > 86400 {
		return "Interval must be between 30 seconds and 24 hours."
	}
	if req.TimeoutSeconds < 1 || req.TimeoutSeconds > synthetics.MaxTimeoutSeconds {
		return "Timeout must be between 1 and 120 seconds."
	}
	if req.FailureThreshold < 1 || req.FailureThreshold > 10 {
		return "Failure threshold must be between 1 and 10."
	}
	if len(req.Config) == 0 {
		return "Check configuration is required."
	}

	switch req.CheckType {
	case models.CheckTypeHttp:
		var cfg models.HttpCheckConfig
		if err := json.Unmarshal(req.Config, &cfg); err != nil {
			return "The check configuration is not valid JSON."
		}
		return validateHttpCheckConfig(&cfg)
	case models.CheckTypeTcp:
		var cfg models.TcpCheckConfig
		if err := json.Unmarshal(req.Config, &cfg); err != nil {
			return "The check configuration is not valid JSON."
		}
		if strings.TrimSpace(cfg.Host) == "" {
			return "Host is required."
		}
		if cfg.Port < 1 || cfg.Port > 65535 {
			return "Port must be between 1 and 65535."
		}
	case models.CheckTypeBrowser:
		if config.Config.CloudMode == "true" {
			return "Browser checks are not available in cloud mode."
		}
		if synthetics.BrowserMode() == synthetics.BrowserModeOff {
			return "Browser checks require SYNTHETICS_BROWSER_MODE=embedded (the :browser image) or a connected remote runner (SYNTHETICS_BROWSER_MODE=remote)."
		}
		var cfg models.BrowserCheckConfig
		if err := json.Unmarshal(req.Config, &cfg); err != nil {
			return "The check configuration is not valid JSON."
		}
		if strings.TrimSpace(cfg.Script) == "" {
			return "A Playwright test script is required."
		}
		if len(cfg.Script) > maxBrowserScriptBytes {
			return "The test script must be 100KB or smaller."
		}
	}
	return ""
}

func validateHttpCheckConfig(cfg *models.HttpCheckConfig) string {
	cfg.URL = strings.TrimSpace(cfg.URL)
	if cfg.URL == "" {
		return "URL is required."
	}
	parsed, err := url.Parse(cfg.URL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "The URL must be a valid http:// or https:// address."
	}
	if cfg.Method != "" && !validHttpMethods[strings.ToUpper(cfg.Method)] {
		return "Method must be one of: GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS."
	}
	if cfg.Assertions.BodyRegex != "" {
		if _, err := regexp.Compile(cfg.Assertions.BodyRegex); err != nil {
			return "The body regex is not a valid regular expression."
		}
	}
	for _, code := range cfg.Assertions.StatusCodes {
		if code < 100 || code > 599 {
			return "Expected status codes must be between 100 and 599."
		}
	}
	// Friendly save-time rejection; the runner re-validates at dial time.
	if !synthetics.AllowPrivateTargets() {
		if err := netguard.ValidateURL(cfg.URL, false); err != nil {
			return err.Error()
		}
	}
	return ""
}

func (ctrl *syntheticCheckController) List(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	tx := db.GetTx(ctx)
	checks, err := transactional.SyntheticCheckRepository.FindByProject(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list synthetic checks: %w", err))
		return
	}
	if checks == nil {
		checks = []*models.SyntheticCheck{}
	}
	ctx.JSON(http.StatusOK, gin.H{"checks": checks})
}

type checkOverviewRequest struct {
	FromDate string `json:"fromDate"`
	ToDate   string `json:"toDate"`
}

type checkOverviewItem struct {
	*models.SyntheticCheck
	Aggregates *models.CheckAggregates `json:"aggregates"`
}

func (ctrl *syntheticCheckController) Overview(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	var req checkOverviewRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	from, to, ok := parseCheckRange(ctx, req.FromDate, req.ToDate)
	if !ok {
		return
	}

	tx := db.GetTx(ctx)
	checks, err := transactional.SyntheticCheckRepository.FindByProject(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list synthetic checks: %w", err))
		return
	}
	aggregates, err := telemetry.CheckResultRepository.AggregatesByProject(ctx, projectId, from, to)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to aggregate check results: %w", err))
		return
	}

	items := make([]checkOverviewItem, 0, len(checks))
	for _, check := range checks {
		items = append(items, checkOverviewItem{SyntheticCheck: check, Aggregates: aggregates[check.Id]})
	}
	ctx.JSON(http.StatusOK, gin.H{"checks": items})
}

func (ctrl *syntheticCheckController) Create(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	var req checkRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if message := validateCheckRequest(&req); message != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
		return
	}

	now := time.Now().UTC()
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	check := &models.SyntheticCheck{
		ProjectId:        projectId,
		Name:             req.Name,
		CheckType:        req.CheckType,
		Enabled:          enabled,
		IntervalSeconds:  req.IntervalSeconds,
		TimeoutSeconds:   req.TimeoutSeconds,
		Config:           models.JSONText(req.Config),
		FailureThreshold: req.FailureThreshold,
		CurrentStatus:    models.CheckStatusUnknown,
		NextRunAt:        now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	tx := db.GetTx(ctx)
	id, err := transactional.SyntheticCheckRepository.Create(tx, check)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to create synthetic check: %w", err))
		return
	}
	check.Id = id
	ctx.JSON(http.StatusCreated, check)
}

func (ctrl *syntheticCheckController) Get(ctx *gin.Context) {
	projectId, checkId, ok := checkIdFromPath(ctx)
	if !ok {
		return
	}
	tx := db.GetTx(ctx)
	check, err := transactional.SyntheticCheckRepository.FindByIdForProject(tx, checkId, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load synthetic check: %w", err))
		return
	}
	if check == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Check not found"})
		return
	}
	incidents, err := transactional.CheckIncidentRepository.FindRecentByCheck(tx, checkId, 20)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load check incidents: %w", err))
		return
	}
	if incidents == nil {
		incidents = []*models.CheckIncident{}
	}
	ctx.JSON(http.StatusOK, gin.H{"check": check, "incidents": incidents})
}

func (ctrl *syntheticCheckController) Update(ctx *gin.Context) {
	projectId, checkId, ok := checkIdFromPath(ctx)
	if !ok {
		return
	}
	var req checkRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	tx := db.GetTx(ctx)
	check, err := transactional.SyntheticCheckRepository.FindByIdForProject(tx, checkId, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load synthetic check: %w", err))
		return
	}
	if check == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Check not found"})
		return
	}
	// The type shapes the config, the queue rows, and the telemetry history;
	// changing it would corrupt all three.
	req.CheckType = check.CheckType
	if message := validateCheckRequest(&req); message != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
		return
	}

	now := time.Now().UTC()
	enabled := check.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	wasEnabled := check.Enabled
	check.Name = req.Name
	check.Enabled = enabled
	check.IntervalSeconds = req.IntervalSeconds
	check.TimeoutSeconds = req.TimeoutSeconds
	check.FailureThreshold = req.FailureThreshold
	check.Config = models.JSONText(req.Config)
	check.UpdatedAt = now
	// Apply the new config/interval promptly instead of waiting out the old
	// interval.
	check.NextRunAt = now

	if err := transactional.SyntheticCheckRepository.Update(tx, check); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update synthetic check: %w", err))
		return
	}
	if wasEnabled && !enabled {
		if err := transactional.CheckRunRepository.DeleteQueuedByCheck(tx, check.Id); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to drop queued runs for paused check: %w", err))
			return
		}
	}
	ctx.JSON(http.StatusOK, check)
}

func (ctrl *syntheticCheckController) Delete(ctx *gin.Context) {
	projectId, checkId, ok := checkIdFromPath(ctx)
	if !ok {
		return
	}
	tx := db.GetTx(ctx)
	check, err := transactional.SyntheticCheckRepository.FindByIdForProject(tx, checkId, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load synthetic check: %w", err))
		return
	}
	if check == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Check not found"})
		return
	}
	// Queued runs and incidents cascade with the check, but a page opened by a
	// check_down rule would survive with nothing left to ever resolve it.
	rules, err := transactional.NotificationRuleRepository.FindByProjectWithChannel(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load notification rules for check cleanup: %w", err))
		return
	}
	now := time.Now().UTC()
	for _, rule := range rules {
		if rule.RuleType != "check_down" {
			continue
		}
		if _, err := oncall.AutoResolveByDedupKeyTx(tx, rule.Id, fmt.Sprintf("check:%d", checkId), now); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to resolve pages for deleted check: %w", err))
			return
		}
	}
	if err := transactional.SyntheticCheckRepository.Delete(tx, checkId, projectId); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete synthetic check: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (ctrl *syntheticCheckController) RunNow(ctx *gin.Context) {
	projectId, checkId, ok := checkIdFromPath(ctx)
	if !ok {
		return
	}
	tx := db.GetTx(ctx)
	check, err := transactional.SyntheticCheckRepository.FindByIdForProject(tx, checkId, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load synthetic check: %w", err))
		return
	}
	if check == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Check not found"})
		return
	}
	pending, err := transactional.CheckRunRepository.HasPendingForCheck(tx, checkId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to inspect queued runs: %w", err))
		return
	}
	if pending {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "A run for this check is already queued."})
		return
	}

	now := time.Now().UTC()
	run := &models.CheckRun{
		CheckId:      check.Id,
		ProjectId:    projectId,
		CheckType:    check.CheckType,
		Status:       models.CheckRunQueued,
		ScheduledFor: now,
		ExpiresAt:    synthetics.RunExpiry(now, check.IntervalSeconds),
		CreatedAt:    now,
	}
	if _, err := transactional.CheckRunRepository.Enqueue(tx, run); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to enqueue check run: %w", err))
		return
	}
	middleware.OnCommit(ctx, synthetics.Wake)
	ctx.JSON(http.StatusOK, gin.H{"queued": true})
}

type checkResultsRequest struct {
	FromDate   string           `json:"fromDate"`
	ToDate     string           `json:"toDate"`
	Status     string           `json:"status"`
	Pagination PaginationParams `json:"pagination"`
}

func (ctrl *syntheticCheckController) Results(ctx *gin.Context) {
	projectId, checkId, ok := checkIdFromPath(ctx)
	if !ok {
		return
	}
	var req checkResultsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	from, to, ok := parseCheckRange(ctx, req.FromDate, req.ToDate)
	if !ok {
		return
	}

	if req.Status != "" && req.Status != models.CheckResultUp && req.Status != models.CheckResultDown && req.Status != models.CheckResultMissed {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status filter"})
		return
	}
	results, total, err := telemetry.CheckResultRepository.FindByCheck(ctx, projectId, checkId, &from, &to, req.Status, req.Pagination.Page, req.Pagination.PageSize)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list check results: %w", err))
		return
	}
	if results == nil {
		results = []*models.CheckResultEntry{}
	}
	totalPages := total / int64(req.Pagination.PageSize)
	if total%int64(req.Pagination.PageSize) > 0 {
		totalPages++
	}
	ctx.JSON(http.StatusOK, PaginatedResponse[*models.CheckResultEntry]{
		Data: results,
		Pagination: Pagination{
			Page:       req.Pagination.Page,
			PageSize:   req.Pagination.PageSize,
			Total:      total,
			TotalPages: totalPages,
		},
	})
}

type checkSeriesRequest struct {
	FromDate        string `json:"fromDate"`
	ToDate          string `json:"toDate"`
	IntervalMinutes int    `json:"intervalMinutes"`
}

func (ctrl *syntheticCheckController) Series(ctx *gin.Context) {
	projectId, checkId, ok := checkIdFromPath(ctx)
	if !ok {
		return
	}
	var req checkSeriesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	from, to, ok := parseCheckRange(ctx, req.FromDate, req.ToDate)
	if !ok {
		return
	}
	interval := req.IntervalMinutes
	if interval < 1 {
		interval = 5
	}
	if interval > 1440 {
		interval = 1440
	}

	points, err := telemetry.CheckResultRepository.SeriesByCheck(ctx, projectId, checkId, from, to, interval)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load check series: %w", err))
		return
	}
	if points == nil {
		points = []*models.CheckSeriesPoint{}
	}
	ctx.JSON(http.StatusOK, gin.H{"series": points})
}

// Screenshot streams a stored browser-check failure screenshot. The key is
// prefix-checked against the caller's project so one project can never read
// another's screenshots.
func (ctrl *syntheticCheckController) Screenshot(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	key := ctx.Query("key")
	expectedPrefix := fmt.Sprintf("synthetics/%s/", projectId)
	if !strings.HasPrefix(key, expectedPrefix) || strings.Contains(key, "..") {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid screenshot key"})
		return
	}
	data, err := storage.Store.Read(ctx, key)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Screenshot not found"})
		return
	}
	ctx.Data(http.StatusOK, "image/png", data)
}

// Output streams the stored Playwright output (stdout/stderr tail) of a
// failed browser run, with the same project prefix check as Screenshot.
func (ctrl *syntheticCheckController) Output(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	key := ctx.Query("key")
	expectedPrefix := fmt.Sprintf("synthetics/%s/", projectId)
	if !strings.HasPrefix(key, expectedPrefix) || strings.Contains(key, "..") || !strings.HasSuffix(key, ".log") {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid output key"})
		return
	}
	data, err := storage.Store.Read(ctx, key)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Output not found"})
		return
	}
	ctx.Data(http.StatusOK, "text/plain; charset=utf-8", data)
}

func (ctrl *syntheticCheckController) OpenCount(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	tx := db.GetTx(ctx)
	count, err := transactional.SyntheticCheckRepository.CountDownByProject(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to count down checks: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"count": count})
}

// checkIdFromPath resolves the project id and the :id path param.
func checkIdFromPath(ctx *gin.Context) (projectId uuid.UUID, checkId int, ok bool) {
	pid, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return pid, 0, false
	}
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id < 1 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid check id"})
		return pid, 0, false
	}
	return pid, id, true
}

// parseCheckRange parses the fromDate/toDate body fields, defaulting to the
// last 24 hours.
func parseCheckRange(ctx *gin.Context, fromDate string, toDate string) (time.Time, time.Time, bool) {
	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)
	to := now
	if fromDate != "" {
		parsed, err := time.Parse(time.RFC3339, fromDate)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid fromDate"})
			return from, to, false
		}
		from = parsed.UTC()
	}
	if toDate != "" {
		parsed, err := time.Parse(time.RFC3339, toDate)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid toDate"})
			return from, to, false
		}
		to = parsed.UTC()
	}
	return from, to, true
}

var SyntheticCheckController = syntheticCheckController{}
