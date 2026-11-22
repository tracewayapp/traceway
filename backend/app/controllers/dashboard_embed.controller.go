package controllers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/middleware"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/v2/app/services"
	dashboardsvc "github.com/tracewayapp/traceway/backend/v2/app/services/dashboards"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
)

// One open of a shared dashboard reads the dashboard once and every widget once
// (a dashboard holds at most 100), and viewers behind one address share a budget.
const (
	embedOpensPerMinute       = 60
	embedWidgetReadsPerMinute = 600
)

var embedTimeframePattern = regexp.MustCompile(`^([0-9]+)([mhd])$`)

func validateEmbedView(view *services.DashboardEmbedView, timeframes []string) string {
	window := 42 * time.Hour
	if len(timeframes) > 0 {
		if len(timeframes) != 1 || !view.From.IsZero() || !view.To.IsZero() {
			return "Send one timeframe or an explicit from and to range"
		}
		parts := embedTimeframePattern.FindStringSubmatch(strings.TrimPrefix(timeframes[0], "last"))
		if parts == nil {
			return "timeframe must be a positive duration such as 30m, 42h or 7d"
		}
		unit := map[string]time.Duration{"m": time.Minute, "h": time.Hour, "d": 24 * time.Hour}[parts[2]]
		count, err := strconv.ParseUint(parts[1], 10, 64)
		if err != nil || count == 0 || count > uint64(metricQueryMaxRange/unit) {
			return fmt.Sprintf("timeframe must be positive and at most %d days", metricQueryMaxRange/(24*time.Hour))
		}
		window = time.Duration(count) * unit
	}
	if view.From.IsZero() != view.To.IsZero() {
		return "Send both from and to, or neither for a relative timeframe"
	}
	if view.From.IsZero() {
		view.To = time.Now().UTC()
		view.From = view.To.Add(-window)
	}
	if msg := validateMetricRange(view.From, view.To); msg != "" {
		return msg
	}
	if len(view.TagFilters) > metricQueryMaxTagFilters {
		return fmt.Sprintf("a maximum of %d tag filters can be applied", metricQueryMaxTagFilters)
	}
	// An empty value is an account the host failed to resolve, and ClickHouse would
	// match it against every row that lacks the tag.
	for key, value := range view.TagFilters {
		if key == "" || value == "" {
			return "tag filters need a non-empty tag and value"
		}
	}
	return ""
}

type embedOptions struct {
	services.DashboardEmbedView
	ExpiresAt     *time.Time `json:"expiresAt"`
	ExpiresInDays *int64     `json:"expiresInDays"`
}

// A link cannot be revoked on its own, so its lifetime is bounded like a write token's.
const embedMaxDays = 90

func (o *embedOptions) expiry(now time.Time) (time.Time, string) {
	latest := now.AddDate(0, 0, embedMaxDays)
	switch {
	case o.ExpiresAt != nil && o.ExpiresInDays != nil:
		return time.Time{}, "Send expiresAt or expiresInDays, not both"
	case o.ExpiresAt != nil:
		end := o.ExpiresAt.UTC().Truncate(time.Second)
		if !end.After(now) || end.After(latest) {
			return time.Time{}, fmt.Sprintf("expiresAt must be a future RFC 3339 timestamp at most %d days away", embedMaxDays)
		}
		return end, ""
	case o.ExpiresInDays != nil:
		days := *o.ExpiresInDays
		if days < 1 || days > embedMaxDays {
			return time.Time{}, fmt.Sprintf("expiresInDays must be between 1 and %d", embedMaxDays)
		}
		return now.AddDate(0, 0, int(days)), ""
	default:
		return now.Add(5 * time.Minute), ""
	}
}

func (c *dashboardsController) CreateEmbedToken(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid dashboard id"})
		return
	}
	theme := ctx.Query("theme")
	if theme != "" && theme != "light" && theme != "dark" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "theme must be light or dark"})
		return
	}
	opts := new(embedOptions)
	// Keep the signed view small enough to travel in a bearer header and an iframe URL.
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 2048)
	decoder := json.NewDecoder(ctx.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&opts); err != nil && !errors.Is(err, io.EOF) {
		middleware.RejectBindError(ctx, err, "Invalid embed options")
		return
	}
	if err := decoder.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		middleware.RejectBindError(ctx, err, "Expected a single embed options object")
		return
	}
	if opts == nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Expected an embed options object"})
		return
	}
	expiresAt, msg := opts.expiry(time.Now().UTC())
	if msg != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": msg})
		return
	}
	rolling := opts.From.IsZero() && (opts.ExpiresAt != nil || opts.ExpiresInDays != nil)
	if msg := validateEmbedView(&opts.DashboardEmbedView, ctx.Request.URL.Query()["timeframe"]); msg != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": msg})
		return
	}
	dashboard, err := transactional.DashboardRepository.FindAppliedDashboard(db.GetTx(ctx), projectId, id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load dashboard %d for embedding: %w", id, err))
		return
	}
	if dashboard == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Dashboard is not applied to this project"})
		return
	}
	claims, token, err := services.GenerateDashboardEmbed(id, projectId, opts.DashboardEmbedView, expiresAt, rolling)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to sign the embed token of dashboard %d: %w", id, err))
		return
	}
	query := url.Values{"dashboard": {claims.Subject}}
	if theme != "" {
		query.Set("theme", theme)
	}
	ctx.JSON(http.StatusCreated, gin.H{
		"dashboardUuid":    claims.Subject,
		"token":            token,
		"expiresAt":        claims.ExpiresAt.Time.UTC(),
		"expiresInSeconds": embedSecondsLeft(claims),
		"embedPath":        "/embed/dashboard?" + query.Encode() + "#token=" + token,
	})
}

// The viewer's clock decides nothing: a browser that runs minutes ahead would
// otherwise treat a fresh link as expired.
func embedSecondsLeft(claims *services.DashboardEmbedClaims) int64 {
	return max(0, claims.ExpiresAt.Unix()-time.Now().Unix())
}

func loadEmbeddedDashboard(ctx *gin.Context) (*services.DashboardEmbedClaims, *models.Dashboard, *models.DashboardDefinition) {
	ctx.Header("Cache-Control", "no-store")
	raw, bearer := strings.CutPrefix(ctx.GetHeader("Authorization"), "Bearer ")
	claims, err := services.ValidateDashboardEmbed(raw, ctx.Param("uuid"))
	if !bearer || err != nil {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired dashboard link"})
		return nil, nil, nil
	}
	dashboard, err := db.ExecuteTransactionContext(ctx, func(tx *sql.Tx) (*models.Dashboard, error) {
		return transactional.DashboardRepository.FindAppliedDashboard(tx, claims.ProjectId, claims.DashboardId)
	})
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load embedded dashboard %d: %w", claims.DashboardId, err))
		return nil, nil, nil
	}
	if dashboard == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Dashboard is no longer available"})
		return nil, nil, nil
	}
	if claims.Rolling {
		window := claims.To.Sub(claims.From)
		claims.To = time.Now().UTC()
		claims.From = claims.To.Add(-window)
	}
	return claims, dashboard, dashboardDefinitionOrError(ctx, dashboard)
}

func (c *dashboardsController) GetEmbed(ctx *gin.Context) {
	claims, dashboard, def := loadEmbeddedDashboard(ctx)
	if def == nil {
		return
	}
	widgets, err := embedWidgets(def.Widgets)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to prepare the widgets of embedded dashboard %d: %w", claims.DashboardId, err))
		return
	}
	response := gin.H{
		"name":             dashboard.Name,
		"widgets":          widgets,
		"from":             claims.From,
		"to":               claims.To,
		"expiresAt":        claims.ExpiresAt.Time.UTC(),
		"expiresInSeconds": embedSecondsLeft(claims),
	}
	if claims.Rolling {
		// Pin every widget in this open page to the exact range shown in its header.
		_, token, err := services.GenerateDashboardEmbed(claims.DashboardId, claims.ProjectId, claims.DashboardEmbedView, claims.ExpiresAt.Time, false)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to sign the widget token of embedded dashboard %d: %w", claims.DashboardId, err))
			return
		}
		response["widgetToken"] = token
	}
	ctx.JSON(http.StatusOK, response)
}

// The fields of a source the embed page draws with. What a source queries (its
// metric, saved filters and threshold) stays on the server, which runs the query:
// a saved filter can name an account other than the viewer's.
type embedWidgetSource struct {
	Aggregation string `json:"aggregation,omitempty"`
	GroupBy     string `json:"groupBy,omitempty"`
	Label       string `json:"label,omitempty"`
	Complement  bool   `json:"complement,omitempty"`
}

func embedWidgets(widgets []models.DashboardWidget) ([]models.DashboardWidget, error) {
	shown := make([]models.DashboardWidget, len(widgets))
	for i, widget := range widgets {
		shown[i] = widget
		if len(widget.Config) == 0 {
			continue
		}
		var config map[string]json.RawMessage
		if err := json.Unmarshal(widget.Config, &config); err != nil {
			return nil, fmt.Errorf("widget %s: %w", widget.Id, err)
		}
		if _, ok := config["sources"]; !ok {
			continue
		}
		var sources []embedWidgetSource
		if err := json.Unmarshal(config["sources"], &sources); err != nil {
			return nil, fmt.Errorf("widget %s sources: %w", widget.Id, err)
		}
		var err error
		if config["sources"], err = json.Marshal(sources); err != nil {
			return nil, err
		}
		if shown[i].Config, err = json.Marshal(config); err != nil {
			return nil, err
		}
	}
	return shown, nil
}

func (c *dashboardsController) GetEmbedWidget(ctx *gin.Context) {
	claims, _, def := loadEmbeddedDashboard(ctx)
	if def == nil {
		return
	}
	i := dashboardsvc.FindWidget(def, ctx.Param("wid"))
	if i < 0 {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Widget not found"})
		return
	}
	req, err := embedWidgetQuery(def.Widgets[i], claims.DashboardEmbedView)
	if err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Invalid widget sources"})
		return
	}
	if len(req.Queries) == 0 {
		ctx.JSON(http.StatusOK, MetricQueryResponse{Results: []MetricQueryResult{}})
		return
	}
	executeMetricQuery(ctx, claims.ProjectId, req)
}

func embedWidgetQuery(widget models.DashboardWidget, view services.DashboardEmbedView) (MetricQueryRequest, error) {
	var config struct {
		Sources []MetricQueryItem `json:"sources"`
	}
	if err := json.Unmarshal(widget.Config, &config); err != nil {
		return MetricQueryRequest{}, err
	}
	req := MetricQueryRequest{From: view.From, To: view.To, Queries: config.Sources}
	if len(req.Queries) == 0 {
		return req, nil
	}
	for i := range req.Queries {
		q := &req.Queries[i]
		if q.TagFilters == nil && len(view.TagFilters) > 0 {
			q.TagFilters = make(map[string]string)
		}
		maps.Copy(q.TagFilters, view.TagFilters)
	}
	first := req.Queries[0]
	switch widget.WidgetType {
	case "rating_bar":
		req.WholeRange = true
	case "single_value", "gauge":
		req.WholeRange = shared.IsHistogramOnlyAggregation(first.Aggregation)
	case "breakdown_table":
		req.WholeRange = true
		req.Queries = req.Queries[:1]
		if first.Aggregation != "count" && first.Aggregation != "sum" {
			first.Aggregation = "count"
			req.Queries = append(req.Queries, first)
		}
	}
	return req, nil
}
