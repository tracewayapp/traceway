package controllers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/tracewayapp/traceway/backend/v2/app/cache"
	"github.com/tracewayapp/traceway/backend/v2/app/clientlogs"
	"github.com/tracewayapp/traceway/backend/v2/app/clientmetrics"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/middleware"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type metricQueryController struct{}

const (
	metricQueryMaxQueries         = 50
	metricQueryMaxBuckets         = shared.MaxTimeSeriesBuckets
	metricQueryMaxGroups          = 200
	metricQueryMaxRange           = 731 * 24 * time.Hour
	metricQueryMaxTagFilters      = 20
	metricQueryTimeout            = 30 * time.Second
	metricQueryMaxIntervalMinutes = 366 * 24 * 60
)

func minIntervalMinutes(rangeSeconds, maxBuckets int) int {
	if maxBuckets < 2 {
		maxBuckets = 2
	}
	per := 60 * (maxBuckets - 1)
	iv := (rangeSeconds + per - 1) / per
	if iv < 1 {
		iv = 1
	}
	return iv
}

func validateMetricQuery(req *MetricQueryRequest) string {
	if len(req.Queries) > metricQueryMaxQueries {
		return fmt.Sprintf("a maximum of %d queries can be requested at once", metricQueryMaxQueries)
	}
	if msg := validateMetricRange(req.From, req.To); msg != "" {
		return msg
	}
	for _, q := range req.Queries {
		if len(q.TagFilters) > metricQueryMaxTagFilters {
			return fmt.Sprintf("a maximum of %d tag filters can be applied per query", metricQueryMaxTagFilters)
		}
		if q.Aggregation != "" && !pointAggregations[q.Aggregation] && !shared.IsHistogramOnlyAggregation(q.Aggregation) {
			return fmt.Sprintf("unknown aggregation %q; use one of avg, min, max, sum, count, last, rate, p50, p75, p90, p95, p99, good_ratio", q.Aggregation)
		}
	}

	if req.WholeRange {
		req.IntervalMinutes = min(int(req.To.Sub(req.From).Minutes())+1, metricQueryMaxIntervalMinutes)
	}
	if req.IntervalMinutes <= 0 {
		req.IntervalMinutes = calculateIntervalMinutes(req.To.Sub(req.From))
	}
	if req.IntervalMinutes > metricQueryMaxIntervalMinutes {
		return fmt.Sprintf("intervalMinutes must be at most %d", metricQueryMaxIntervalMinutes)
	}
	if minInterval := minIntervalMinutes(int(req.To.Sub(req.From).Seconds()), metricQueryMaxBuckets); req.IntervalMinutes < minInterval {
		req.IntervalMinutes = minInterval
	}
	return ""
}

// pointAggregations are answered from metric points. A histogram metric answers all of
// them but last, and the percentiles and good_ratio besides.
var pointAggregations = map[string]bool{"avg": true, "min": true, "max": true, "sum": true, "count": true, "last": true, "rate": true}

// queryHistogramSeries reads a distribution metric and reduces every time bucket to
// the one number the aggregation asks for.
func queryHistogramSeries(ctx context.Context, projectId uuid.UUID, q MetricQueryItem, agg string, from, to time.Time, intervalMinutes int, wholeRange bool) (map[string][]models.TimeSeriesPoint, bool, string, error) {
	if agg == "last" {
		return nil, false, fmt.Sprintf("%s is a distribution; last does not apply to it. Use a percentile, avg, min, max, sum, count or rate", q.Name), nil
	}
	threshold := 0.0
	if agg == shared.AggregationGoodRatio {
		switch entry, known := clientmetrics.Lookup(q.Name); {
		case q.Threshold != nil:
			threshold = *q.Threshold
		case known && entry.Good > 0:
			threshold = entry.Good
		default:
			return nil, false, "good_ratio needs a threshold: the share of observations at or under it", nil
		}
	}
	var groupBy []string
	if q.GroupBy != "" {
		groupBy = []string{q.GroupBy}
	}
	histograms, truncated, err := telemetry.MetricHistogramRepository.Query(ctx, projectId, q.Name, from, to, intervalMinutes, q.TagFilters, groupBy, metricQueryMaxGroups)
	if err != nil {
		return nil, false, "", err
	}
	series := make(map[string][]models.TimeSeriesPoint, len(histograms))
	for key, points := range histograms {
		if wholeRange && len(points) > 1 {
			merged := shared.NewHistogram()
			for _, point := range points {
				merged.Merge(point.Hist)
			}
			points = []shared.HistogramSeriesPoint{{Timestamp: from.UTC(), Hist: merged}}
		}
		if !wholeRange {
			points = shared.SpreadHistogramPoints(points, intervalMinutes, from, to)
		}
		out := make([]models.TimeSeriesPoint, 0, len(points))
		for _, point := range points {
			value := point.Hist.Aggregate(agg, threshold)
			if agg == "rate" {
				value = point.Hist.Count / float64(intervalMinutes*60)
			}
			out = append(out, models.TimeSeriesPoint{Timestamp: point.Timestamp, Value: value})
		}
		series[key] = out
	}
	return series, truncated, "", nil
}

func isCatalogDistribution(name string) bool {
	entry, known := clientmetrics.Lookup(name)
	return known && entry.Shape == clientmetrics.Distribution
}

func histogramResultUnit(agg, unit string) string {
	switch agg {
	case shared.AggregationGoodRatio:
		return "1"
	case "count":
		return "count"
	case "rate":
		return "count/s"
	}
	return unit
}

func validateMetricRange(from, to time.Time) string {
	if to.Before(from) {
		return "to must not be before from"
	}
	if to.Sub(from) > metricQueryMaxRange {
		return fmt.Sprintf("the time range must be at most %d days", int(metricQueryMaxRange.Hours()/24))
	}
	return ""
}

func trimGroupSeries(series map[string][]models.TimeSeriesPoint) bool {
	truncated := false
	for k, points := range series {
		if points == nil {
			delete(series, k)
			truncated = true
		}
	}
	return truncated
}

type MetricQueryRequest struct {
	Queries         []MetricQueryItem `json:"queries" binding:"required,min=1"`
	From            time.Time         `json:"from" binding:"required"`
	To              time.Time         `json:"to" binding:"required"`
	IntervalMinutes int               `json:"intervalMinutes"`
	// WholeRange asks for one value per series over the whole range instead of a
	// time series: what a stat, a rating bar or a breakdown table shows. A percentile
	// of the range cannot be put together from the percentiles of its buckets.
	WholeRange bool `json:"wholeRange,omitempty"`
}

type MetricQueryItem struct {
	Name        string            `json:"name" binding:"required"`
	Aggregation string            `json:"aggregation"`
	TagFilters  map[string]string `json:"tagFilters"`
	GroupBy     string            `json:"groupBy"`
	// Threshold is the "good" limit of good_ratio. A rated catalog metric has a default.
	Threshold *float64 `json:"threshold,omitempty"`
}

type MetricQueryResponse struct {
	Results         []MetricQueryResult `json:"results"`
	IntervalMinutes int                 `json:"intervalMinutes"`
}

type MetricQueryResult struct {
	Name            string                              `json:"name"`
	Unit            string                              `json:"unit"`
	Series          map[string][]models.TimeSeriesPoint `json:"series"`
	TruncatedGroups bool                                `json:"truncatedGroups,omitempty"`
}

func (c *metricQueryController) Query(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	limitJSONBody(ctx)

	var req MetricQueryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		middleware.RejectBindError(ctx, err, "Invalid request body")
		return
	}
	if msg := validateMetricQuery(&req); msg != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": msg})
		return
	}

	queryCtx, cancel := context.WithTimeout(ctx.Request.Context(), metricQueryTimeout)
	defer cancel()

	unitMap := make(map[string]string)
	histogramMetrics := make(map[string]bool)
	registered := make(map[string]bool)
	registry, regErr := db.ExecuteTransactionContext(ctx, func(tx *sql.Tx) ([]*models.MetricRegistry, error) {
		return transactional.MetricRegistryRepository.FindByProject(tx, projectId)
	})
	if regErr != nil {
		traceway.CaptureException(fmt.Errorf("failed to load metric registry for query: %w", regErr))
	}
	if regErr == nil {
		for _, r := range registry {
			unitMap[r.Name] = r.Unit
			histogramMetrics[r.Name] = r.MetricType == models.MetricTypeHistogram
			registered[r.Name] = true
		}
	}

	clientProject := clientlogs.IsClient(cache.ProjectCache.GetById(projectId))

	var results []MetricQueryResult
	for _, q := range req.Queries {
		agg := q.Aggregation
		if agg == "" {
			agg = "avg"
		}

		spanName := fmt.Sprintf("query metric: %s agg=%s", q.Name, agg)
		if q.GroupBy != "" {
			spanName += " group=" + q.GroupBy
		}
		if len(q.TagFilters) > 0 {
			spanName += fmt.Sprintf(" filters=%d", len(q.TagFilters))
		}
		// A metric nothing has reported yet is not in the registry. A percentile of it
		// is an empty chart, not a mistake: a dashboard is installed before its data.
		// A client project's catalog distribution is one whatever its registry row says:
		// the release before histograms registered it as a gauge, and that row is only
		// upgraded by the next client that reports the metric.
		isHistogram := histogramMetrics[q.Name] ||
			(clientProject && isCatalogDistribution(q.Name)) ||
			(!registered[q.Name] && (shared.IsHistogramOnlyAggregation(agg) || isCatalogDistribution(q.Name)))
		if !isHistogram && shared.IsHistogramOnlyAggregation(agg) {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("%s needs a distribution metric, and %s is not one", agg, q.Name)})
			return
		}
		spanCtx, span := traceway.StartSpan(queryCtx, spanName)
		var series map[string][]models.TimeSeriesPoint
		var err error
		histogramTruncated := false
		if isHistogram {
			var problem string
			series, histogramTruncated, problem, err = queryHistogramSeries(spanCtx, projectId, q, agg, req.From, req.To, req.IntervalMinutes, req.WholeRange)
			if problem != "" {
				span.End()
				ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": problem})
				return
			}
		} else {
			series, err = telemetry.MetricPointRepository.QueryTimeSeries(
				spanCtx, projectId, q.Name, req.From, req.To,
				req.IntervalMinutes, agg, q.TagFilters, q.GroupBy, metricQueryMaxGroups,
			)
			if err == nil && req.WholeRange {
				var counts map[string][]models.TimeSeriesPoint
				if agg == "avg" {
					counts, err = telemetry.MetricPointRepository.QueryTimeSeries(spanCtx, projectId, q.Name, req.From, req.To, req.IntervalMinutes, "count", q.TagFilters, q.GroupBy, metricQueryMaxGroups)
				}
				if err == nil {
					series = wholeRangePoints(series, counts, agg, req.From, req.To, req.IntervalMinutes)
				}
			}
		}
		span.End()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(queryCtx.Err(), context.DeadlineExceeded) {
				ctx.JSON(http.StatusGatewayTimeout, gin.H{"error": fmt.Sprintf("metric query timed out after %s; narrow the time range or send fewer queries", metricQueryTimeout)})
				return
			}
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to query metric %s: %w", q.Name, err))
			return
		}
		truncated := trimGroupSeries(series) || histogramTruncated

		resultUnit := rateUnit(agg, unitMap[q.Name])
		if agg == "count" {
			resultUnit = "count"
		}
		if isHistogram {
			unit := unitMap[q.Name]
			if entry, known := clientmetrics.Lookup(q.Name); known && unit == "" {
				unit = entry.Unit
			}
			resultUnit = histogramResultUnit(agg, unit)
		}

		results = append(results, MetricQueryResult{
			Name:            q.Name,
			Unit:            resultUnit,
			Series:          series,
			TruncatedGroups: truncated,
		})
	}

	ctx.JSON(http.StatusOK, MetricQueryResponse{Results: results, IntervalMinutes: req.IntervalMinutes})
}

type DiscoverResponse struct {
	Metrics []models.DiscoveredMetric `json:"metrics"`
}

type DiscoverInstancesResponse struct {
	Instances []string `json:"instances"`
}

func parseMetricDiscoveryRange(ctx *gin.Context) (time.Time, time.Time, string) {
	from := time.Now().AddDate(0, 0, -7)
	to := time.Now()

	if fromStr := ctx.Query("from"); fromStr != "" {
		if parsed, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = parsed
		}
	}
	if toStr := ctx.Query("to"); toStr != "" {
		if parsed, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = parsed
		}
	}

	return from, to, validateMetricRange(from, to)
}

func (c *metricQueryController) Discover(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	from, to, rangeMsg := parseMetricDiscoveryRange(ctx)
	if rangeMsg != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": rangeMsg})
		return
	}

	spanCtx, span := traceway.StartSpan(ctx, "discover metrics")
	discovered, err := telemetry.MetricPointRepository.DiscoverMetrics(spanCtx, projectId, from, to)
	span.End()
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to discover metrics: %w", err))
		return
	}
	histograms, err := telemetry.MetricHistogramRepository.Discover(ctx.Request.Context(), projectId, from, to)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to discover distribution metrics: %w", err))
		return
	}
	discovered = mergeDiscovered(discovered, histograms)

	registry, regErr := db.ExecuteTransactionContext(ctx, func(tx *sql.Tx) ([]*models.MetricRegistry, error) {
		return transactional.MetricRegistryRepository.FindByProject(tx, projectId)
	})

	if regErr != nil {
		traceway.CaptureException(fmt.Errorf("failed to load metric registry for discover: %w", regErr))
	}
	if regErr == nil {
		regMap := make(map[string]*models.MetricRegistry, len(registry))
		for _, r := range registry {
			regMap[r.Name] = r
		}
		for i := range discovered {
			if reg, ok := regMap[discovered[i].Name]; ok {
				discovered[i].MetricType = reg.MetricType
				discovered[i].Unit = reg.Unit
			}
		}
	}

	ctx.JSON(http.StatusOK, DiscoverResponse{Metrics: discovered})
}

func (c *metricQueryController) DiscoverInstances(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	from, to, rangeMsg := parseMetricDiscoveryRange(ctx)
	if rangeMsg != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": rangeMsg})
		return
	}
	spanCtx, span := traceway.StartSpan(ctx, "discover instances")
	instances, err := telemetry.MetricPointRepository.GetDistinctServers(spanCtx, projectId, from, to)
	span.End()
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to discover instances: %w", err))
		return
	}
	if instances == nil {
		instances = []string{}
	}

	ctx.JSON(http.StatusOK, DiscoverInstancesResponse{Instances: instances})
}

// mergeDiscovered adds the distribution metrics to the point metrics. A name present
// in both (points of an earlier release) is listed once, as the distribution.
func mergeDiscovered(points, histograms []models.DiscoveredMetric) []models.DiscoveredMetric {
	if len(histograms) == 0 {
		return points
	}
	byName := make(map[string]int, len(points))
	for i, m := range points {
		byName[m.Name] = i
	}
	for _, h := range histograms {
		if i, ok := byName[h.Name]; ok {
			points[i] = h
			continue
		}
		points = append(points, h)
	}
	sort.SliceStable(points, func(i, j int) bool { return points[i].Name < points[j].Name })
	return points
}

const discoverOrgMaxProjects = 25

type OrgDiscoveredMetric struct {
	Name       string      `json:"name"`
	TagKeys    []string    `json:"tagKeys"`
	MetricType string      `json:"metricType,omitempty"`
	Unit       string      `json:"unit,omitempty"`
	ProjectIds []uuid.UUID `json:"projectIds"`
}

func (c *metricQueryController) DiscoverOrg(ctx *gin.Context) {
	organizationId, err := strconv.Atoi(ctx.Query("organizationId"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "organizationId query param is required"})
		return
	}

	userId := middleware.GetUserId(ctx)
	projects, err := db.ExecuteTransactionContext(ctx, func(tx *sql.Tx) ([]*models.Project, error) {
		role, err := transactional.OrganizationRepository.GetUserRole(tx, organizationId, userId)
		if err != nil {
			return nil, err
		}
		if role == "" {
			return nil, nil
		}
		return transactional.ProjectRepository.FindByOrganizationId(tx, organizationId)
	})
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list organization projects: %w", err))
		return
	}
	if projects == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}
	truncated := false
	if len(projects) > discoverOrgMaxProjects {
		projects = projects[:discoverOrgMaxProjects]
		truncated = true
	}

	from := time.Now().AddDate(0, 0, -7)
	to := time.Now()

	merged := map[string]*OrgDiscoveredMetric{}
	order := []string{}
	for _, project := range projects {
		spanCtx, span := traceway.StartSpan(ctx, fmt.Sprintf("discover metrics: project %s", project.Id))
		discovered, err := telemetry.MetricPointRepository.DiscoverMetrics(spanCtx, project.Id, from, to)
		span.End()
		if err != nil {
			traceway.CaptureException(fmt.Errorf("failed to discover metrics for project %s: %w", project.Id, err))
			continue
		}
		if histograms, err := telemetry.MetricHistogramRepository.Discover(ctx.Request.Context(), project.Id, from, to); err == nil {
			discovered = mergeDiscovered(discovered, histograms)
		} else {
			traceway.CaptureException(fmt.Errorf("failed to discover distribution metrics for project %s: %w", project.Id, err))
		}
		for _, m := range discovered {
			entry := merged[m.Name]
			if entry == nil {
				entry = &OrgDiscoveredMetric{Name: m.Name, TagKeys: m.TagKeys, MetricType: m.MetricType, Unit: m.Unit, ProjectIds: []uuid.UUID{}}
				merged[m.Name] = entry
				order = append(order, m.Name)
			}
			entry.ProjectIds = append(entry.ProjectIds, project.Id)
			if entry.Unit == "" {
				entry.Unit = m.Unit
			}
			if entry.MetricType == "" {
				entry.MetricType = m.MetricType
			}
		}
	}

	metrics := make([]OrgDiscoveredMetric, 0, len(order))
	for _, name := range order {
		metrics = append(metrics, *merged[name])
	}

	ctx.JSON(http.StatusOK, gin.H{"metrics": metrics, "truncated": truncated})
}

func (c *metricQueryController) DiscoverTags(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	name := ctx.Query("name")
	key := ctx.Query("key")
	if name == "" || key == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "name and key query params are required"})
		return
	}

	from := time.Now().AddDate(0, 0, -7)
	to := time.Now()

	spanCtx, span := traceway.StartSpan(ctx, fmt.Sprintf("discover tag values: %s.%s", name, key))
	values, err := telemetry.MetricPointRepository.DiscoverTagValues(spanCtx, projectId, name, key, from, to)
	span.End()
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to discover tag values: %w", err))
		return
	}
	dimensionValues, err := telemetry.MetricHistogramRepository.DiscoverTagValues(ctx.Request.Context(), projectId, name, key, from, to)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to discover dimension values: %w", err))
		return
	}
	if len(dimensionValues) > 0 {
		seen := make(map[string]struct{}, len(values))
		for _, v := range values {
			seen[v] = struct{}{}
		}
		for _, v := range dimensionValues {
			if _, ok := seen[v]; !ok {
				values = append(values, v)
			}
		}
		sort.Strings(values)
	}

	ctx.JSON(http.StatusOK, gin.H{"values": values})
}

func (c *metricQueryController) UpdateRegistry(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var req struct {
		Name        string `json:"name" binding:"required"`
		MetricType  string `json:"metricType"`
		Unit        string `json:"unit"`
		Description string `json:"description"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		middleware.RejectBindError(ctx, err, "Invalid request body")
		return
	}

	if req.MetricType != "" && req.MetricType != "gauge" && req.MetricType != "counter" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "metricType must be 'gauge' or 'counter'"})
		return
	}

	updated, err := db.ExecuteTransactionContext(ctx, func(tx *sql.Tx) (*models.MetricRegistry, error) {
		entry, err := transactional.MetricRegistryRepository.FindByProjectAndName(tx, projectId, req.Name)
		if err != nil {
			return nil, err
		}
		if entry == nil {
			return nil, nil
		}
		// A distribution is stored differently from points; its type follows the data.
		if req.MetricType != "" && entry.MetricType != models.MetricTypeHistogram {
			entry.MetricType = req.MetricType
		}
		if req.Unit != "" {
			entry.Unit = req.Unit
		}
		entry.Description = req.Description
		if err := transactional.MetricRegistryRepository.Update(tx, entry); err != nil {
			return nil, err
		}
		return entry, nil
	})

	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update metric registry: %w", err))
		return
	}
	if updated == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "metric not found in registry"})
		return
	}

	ctx.JSON(http.StatusOK, updated)
}

type MetricExemplarsRequest struct {
	Name       string            `json:"name" binding:"required"`
	From       time.Time         `json:"from" binding:"required"`
	To         time.Time         `json:"to" binding:"required"`
	TagFilters map[string]string `json:"tagFilters"`
}

// Exemplars lists the sessions behind the worst values of a distribution in a range,
// so a bad percentile on a chart leads to a session that had it.
func (c *metricQueryController) Exemplars(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	limitJSONBody(ctx)
	var req MetricExemplarsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		middleware.RejectBindError(ctx, err, "Invalid request body")
		return
	}
	if msg := validateMetricRange(req.From, req.To); msg != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": msg})
		return
	}
	if len(req.TagFilters) > metricQueryMaxTagFilters {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("a maximum of %d tag filters can be applied per query", metricQueryMaxTagFilters)})
		return
	}
	exemplars, err := telemetry.MetricHistogramRepository.FindExemplars(ctx.Request.Context(), projectId, req.Name, req.From, req.To, req.TagFilters, 20)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load exemplars of %s: %w", req.Name, err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"exemplars": exemplars})
}

var MetricQueryController = metricQueryController{}

func rateUnit(aggregation string, unit string) string {
	if aggregation != "rate" {
		return unit
	}
	switch unit {
	case "s":
		return "1"
	case "":
		return "count/s"
	}
	return unit + "/s"
}

// Epoch-aligned buckets can split even a window shorter than one interval. Merge
// point aggregates explicitly; averages need their original point counts.
func wholeRangePoints(series, counts map[string][]models.TimeSeriesPoint, aggregation string, from, to time.Time, intervalMinutes int) map[string][]models.TimeSeriesPoint {
	for key, points := range series {
		if len(points) == 0 {
			continue
		}
		weights := map[int64]float64{}
		for _, p := range counts[key] {
			weights[p.Timestamp.Unix()] = p.Value
		}
		value, weight := 0.0, 0.0
		last := points[0]
		for i, p := range points {
			switch aggregation {
			case "avg":
				n := weights[p.Timestamp.Unix()]
				value += p.Value * n
				weight += n
			case "min":
				if i == 0 || p.Value < value {
					value = p.Value
				}
			case "max":
				if i == 0 || p.Value > value {
					value = p.Value
				}
			case "last":
				if !p.Timestamp.Before(last.Timestamp) {
					last = p
				}
				value = last.Value
			default:
				value += p.Value
			}
		}
		if aggregation == "avg" && weight > 0 {
			value /= weight
		}
		if aggregation == "rate" {
			value *= float64(intervalMinutes*60) / max(1, to.Sub(from).Seconds())
		}
		series[key] = []models.TimeSeriesPoint{{Timestamp: from, Value: value}}
	}
	return series
}
