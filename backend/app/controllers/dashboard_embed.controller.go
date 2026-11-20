package controllers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"net/http"
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
)

func (c *dashboardsController) CreateEmbedToken(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid dashboard id"})
		return
	}
	var view services.DashboardEmbedView
	// Keep the signed view small enough to travel in a bearer header and an iframe URL.
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 2048)
	decoder := json.NewDecoder(ctx.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&view); err != nil && !errors.Is(err, io.EOF) {
		middleware.RejectBindError(ctx, err, "Invalid embed options")
		return
	}
	if err := decoder.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		middleware.RejectBindError(ctx, err, "Expected a single embed options object")
		return
	}
	if view.To.IsZero() && view.From.IsZero() {
		view.To = time.Now().UTC()
		view.From = view.To.Add(-24 * time.Hour)
	}
	if view.From.IsZero() || view.To.IsZero() || validateMetricRange(view.From, view.To) != "" || len(view.TagFilters) > metricQueryMaxTagFilters {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Invalid embed time range or tag filters"})
		return
	}
	dashboard, err := transactional.DashboardRepository.FindAppliedDashboard(db.GetTx(ctx), projectId, id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	if dashboard == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Dashboard is not applied to this project"})
		return
	}
	claims, token, err := services.GenerateDashboardEmbed(id, projectId, view)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	ctx.JSON(http.StatusCreated, gin.H{
		"dashboardUuid": claims.Subject,
		"token":         token,
		"expiresAt":     claims.ExpiresAt.Time,
		"embedPath":     "/embed/dashboard?dashboard=" + claims.Subject + "#token=" + token,
	})
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
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return nil, nil, nil
	}
	if dashboard == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Dashboard is no longer available"})
		return nil, nil, nil
	}
	return claims, dashboard, dashboardDefinitionOrError(ctx, dashboard)
}

func (c *dashboardsController) GetEmbed(ctx *gin.Context) {
	claims, dashboard, def := loadEmbeddedDashboard(ctx)
	if def == nil {
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"name":      dashboard.Name,
		"widgets":   def.Widgets,
		"from":      claims.From,
		"to":        claims.To,
		"expiresAt": claims.ExpiresAt.Time,
	})
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
