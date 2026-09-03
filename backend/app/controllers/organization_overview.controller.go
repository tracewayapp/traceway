package controllers

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
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

type organizationOverviewController struct{}

var OrganizationOverviewController = organizationOverviewController{}

const orgOverviewMaxIssueFetch = 1000

// orgOverviewProjects loads the organization's projects in one short
// transaction: the fan-out handlers run their telemetry queries outside any
// tx so the single-connection SQLite main DB is never held across them.
func orgOverviewProjects(organizationId int) ([]*models.Project, error) {
	projects, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.Project, error) {
		return transactional.ProjectRepository.FindByOrganizationId(tx, organizationId)
	})
	if err != nil {
		return nil, err
	}
	return projects, nil
}

type orgServerRow struct {
	ServerName      string                   `json:"serverName"`
	ProjectId       uuid.UUID                `json:"projectId"`
	ProjectName     string                   `json:"projectName"`
	CpuPct          *float64                 `json:"cpuPct"`
	MemoryPct       *float64                 `json:"memoryPct"`
	DiskPct         *float64                 `json:"diskPct"`
	NetworkRxBps    *float64                 `json:"networkRxBps"`
	NetworkTxBps    *float64                 `json:"networkTxBps"`
	LastReportedAt  time.Time                `json:"lastReportedAt"`
	Trend           []models.TimeSeriesPoint `json:"trend"`
	TelemetrySource string                   `json:"telemetrySource"`
	DashboardId     *int                     `json:"dashboardId"`
	HostName        string                   `json:"hostName"`
	HostId          string                   `json:"hostId"`
	HostArch        string                   `json:"hostArch"`
	OsType          string                   `json:"osType"`
	OsDescription   string                   `json:"osDescription"`
	CloudProvider   string                   `json:"cloudProvider"`
	CloudRegion     string                   `json:"cloudRegion"`
	K8sClusterName  string                   `json:"k8sClusterName"`
	K8sNodeName     string                   `json:"k8sNodeName"`
}

func orgServerDashboardIds(organizationId int) (map[uuid.UUID]int, error) {
	return db.ExecuteTransaction(func(tx *sql.Tx) (map[uuid.UUID]int, error) {
		dashboards, err := transactional.DashboardRepository.FindByOrganization(tx, organizationId)
		if err != nil {
			return nil, err
		}
		assignments, err := transactional.DashboardRepository.FindAssignmentsByOrganization(tx, organizationId)
		if err != nil {
			return nil, err
		}

		serverDashboardIds := make(map[int]bool)
		for _, dashboard := range dashboards {
			if dashboard.TemplateKey != nil && *dashboard.TemplateKey == "traceway-otel-agent" {
				serverDashboardIds[dashboard.Id] = true
			}
		}

		result := make(map[uuid.UUID]int)
		for _, assignment := range assignments {
			if serverDashboardIds[assignment.DashboardId] && result[assignment.ProjectId] == 0 {
				result[assignment.ProjectId] = assignment.DashboardId
			}
		}
		return result, nil
	})
}

func clampPercent(value float64) float64 {
	return math.Max(0, math.Min(100, value))
}

func otelFractionToPercent(value float64) float64 {
	if value <= 1 {
		value *= 100
	}
	return clampPercent(value)
}

func pointerTo(value float64) *float64 {
	return &value
}

func lastSeriesValue(series map[string][]models.TimeSeriesPoint, serverName string) (float64, bool) {
	points := series[serverName]
	if len(points) == 0 {
		return 0, false
	}
	return points[len(points)-1].Value, true
}

func cpuUsageTrend(idle []models.TimeSeriesPoint) []models.TimeSeriesPoint {
	trend := make([]models.TimeSeriesPoint, 0, len(idle))
	for _, point := range idle {
		trend = append(trend, models.TimeSeriesPoint{
			Timestamp: point.Timestamp,
			Value:     clampPercent(100 - otelFractionToPercent(point.Value)),
		})
	}
	return trend
}

func latestCounterRate(points []models.TimeSeriesPoint) (float64, bool) {
	if len(points) < 2 {
		return 0, false
	}
	previous := points[len(points)-2]
	latest := points[len(points)-1]
	seconds := latest.Timestamp.Sub(previous.Timestamp).Seconds()
	if seconds <= 0 {
		return 0, false
	}
	delta := latest.Value - previous.Value
	if delta < 0 {
		return 0, true
	}
	return delta / seconds, true
}

func sumDeviceRates(series map[string][]models.TimeSeriesPoint) (float64, bool) {
	total, found := 0.0, false
	for _, points := range series {
		rate, ok := latestCounterRate(points)
		if !ok {
			continue
		}
		total += rate
		found = true
	}
	return total, found
}

// system.network.io is a cumulative counter per device, so summing samples
// inside a bucket (or across devices) makes the bucket delta meaningless. The
// last sample per device per minute, differenced and then summed, is the rate.
func loadNetworkRates(ctx context.Context, projectId uuid.UUID, direction string, since, to time.Time) (map[string]float64, error) {
	series, err := telemetry.MetricPointRepository.QueryTimeSeriesByTags(ctx, projectId, models.MetricNameSystemNetworkIO, since, to, 1, "last", map[string]string{"direction": direction}, []string{"device", "server_name"}, 0)
	if err != nil {
		return nil, err
	}
	return networkRatesByServer(series), nil
}

func networkRatesByServer(series map[string][]models.TimeSeriesPoint) map[string]float64 {
	byServer := map[string]map[string][]models.TimeSeriesPoint{}
	for key, points := range series {
		parts := shared.SplitGroupKey(key)
		if len(parts) != 2 {
			continue
		}
		device, server := parts[0], parts[1]
		if byServer[server] == nil {
			byServer[server] = map[string][]models.TimeSeriesPoint{}
		}
		byServer[server][device] = points
	}
	rates := make(map[string]float64, len(byServer))
	for server, devices := range byServer {
		if rate, ok := sumDeviceRates(devices); ok {
			rates[server] = rate
		}
	}
	return rates
}

func applyServerMetadata(row *orgServerRow, tags map[string]string) {
	row.HostName = tags["host.name"]
	row.HostId = tags["host.id"]
	row.HostArch = tags["host.arch"]
	row.OsType = tags["os.type"]
	row.OsDescription = tags["os.description"]
	row.CloudProvider = tags["cloud.provider"]
	row.CloudRegion = tags["cloud.region"]
	row.K8sClusterName = tags["k8s.cluster.name"]
	row.K8sNodeName = tags["k8s.node.name"]
}

func loadProjectServerRows(ctx *gin.Context, project *models.Project, dashboardId int, since, now time.Time) ([]orgServerRow, error) {
	latestOtel, err := telemetry.MetricPointRepository.LatestPerServer(ctx, project.Id, models.MetricNameSystemCPUUtilization, since)
	if err != nil {
		return nil, err
	}
	latestSdk, err := telemetry.MetricPointRepository.LatestPerServer(ctx, project.Id, models.MetricNameCpuUsage, since)
	if err != nil {
		return nil, err
	}
	if len(latestOtel) == 0 && len(latestSdk) == 0 {
		return []orgServerRow{}, nil
	}

	rows := make(map[string]*orgServerRow, len(latestOtel)+len(latestSdk))
	for _, point := range latestOtel {
		row := &orgServerRow{
			ServerName:      point.ServerName,
			ProjectId:       project.Id,
			ProjectName:     project.Name,
			LastReportedAt:  point.LastReportedAt,
			Trend:           []models.TimeSeriesPoint{},
			TelemetrySource: "otel",
		}
		if dashboardId > 0 {
			row.DashboardId = &dashboardId
		}
		applyServerMetadata(row, point.Tags)
		rows[point.ServerName] = row
	}
	for _, point := range latestSdk {
		if row := rows[point.ServerName]; row != nil {
			if point.LastReportedAt.After(row.LastReportedAt) {
				row.LastReportedAt = point.LastReportedAt
			}
			continue
		}
		row := &orgServerRow{
			ServerName:      point.ServerName,
			ProjectId:       project.Id,
			ProjectName:     project.Name,
			LastReportedAt:  point.LastReportedAt,
			Trend:           []models.TimeSeriesPoint{},
			TelemetrySource: "sdk",
		}
		if dashboardId > 0 {
			row.DashboardId = &dashboardId
		}
		applyServerMetadata(row, point.Tags)
		rows[point.ServerName] = row
	}

	if len(latestOtel) > 0 {
		cpuIdle, err := telemetry.MetricPointRepository.QueryTimeSeries(ctx, project.Id, models.MetricNameSystemCPUUtilization, since, now, 2, "avg", map[string]string{"state": "idle"}, "server_name", 0)
		if err != nil {
			return nil, err
		}
		memoryUsed, err := telemetry.MetricPointRepository.QueryTimeSeries(ctx, project.Id, models.MetricNameSystemMemoryUtilization, since, now, 2, "avg", map[string]string{"state": "used"}, "server_name", 0)
		if err != nil {
			return nil, err
		}
		filesystemUsed, err := telemetry.MetricPointRepository.QueryTimeSeries(ctx, project.Id, models.MetricNameSystemFilesystemUtilization, since, now, 2, "max", map[string]string{"state": "used"}, "server_name", 0)
		if err != nil {
			return nil, err
		}
		networkTo := now.Truncate(time.Minute).Add(-time.Millisecond)
		receive, err := loadNetworkRates(ctx, project.Id, "receive", since, networkTo)
		if err != nil {
			return nil, err
		}
		transmit, err := loadNetworkRates(ctx, project.Id, "transmit", since, networkTo)
		if err != nil {
			return nil, err
		}

		for _, point := range latestOtel {
			row := rows[point.ServerName]
			row.Trend = cpuUsageTrend(cpuIdle[point.ServerName])
			if len(row.Trend) > 0 {
				row.CpuPct = pointerTo(row.Trend[len(row.Trend)-1].Value)
			}
			if value, ok := lastSeriesValue(memoryUsed, point.ServerName); ok {
				row.MemoryPct = pointerTo(otelFractionToPercent(value))
			}
			if value, ok := lastSeriesValue(filesystemUsed, point.ServerName); ok {
				row.DiskPct = pointerTo(otelFractionToPercent(value))
			}
			if rate, ok := receive[point.ServerName]; ok {
				row.NetworkRxBps = pointerTo(rate)
			}
			if rate, ok := transmit[point.ServerName]; ok {
				row.NetworkTxBps = pointerTo(rate)
			}
		}
	}

	if len(latestSdk) > 0 {
		serverNames := make([]string, 0, len(latestSdk))
		for _, point := range latestSdk {
			serverNames = append(serverNames, point.ServerName)
		}
		cpu, err := telemetry.MetricPointRepository.GetAverageByIntervalPerServer(ctx, project.Id, models.MetricNameCpuUsage, since, now, 2, serverNames)
		if err != nil {
			return nil, err
		}
		memoryUsed, err := telemetry.MetricPointRepository.GetAverageByIntervalPerServer(ctx, project.Id, models.MetricNameMemoryUsage, since, now, 2, serverNames)
		if err != nil {
			return nil, err
		}
		memoryTotal, err := telemetry.MetricPointRepository.GetAverageByIntervalPerServer(ctx, project.Id, models.MetricNameMemoryTotal, since, now, 2, serverNames)
		if err != nil {
			return nil, err
		}

		for _, point := range latestSdk {
			row := rows[point.ServerName]
			if row.CpuPct == nil {
				row.Trend = cpu[point.ServerName]
				if value, ok := lastSeriesValue(cpu, point.ServerName); ok {
					row.CpuPct = pointerTo(clampPercent(value))
				}
			}
			used, hasUsed := lastSeriesValue(memoryUsed, point.ServerName)
			total, hasTotal := lastSeriesValue(memoryTotal, point.ServerName)
			if row.MemoryPct == nil && hasUsed && hasTotal && total > 0 {
				row.MemoryPct = pointerTo(clampPercent(used / total * 100))
			}
		}
	}

	result := make([]orgServerRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, *row)
	}
	return result, nil
}

func (c *organizationOverviewController) Servers(ctx *gin.Context) {
	organizationId := middleware.GetOrganizationId(ctx)
	projects, err := orgOverviewProjects(organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list organization projects: %w", err))
		return
	}
	dashboardIds, err := orgServerDashboardIds(organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to resolve organization server dashboards: %w", err))
		return
	}

	now := time.Now().UTC()
	since := now.Add(-30 * time.Minute)
	servers := make([]orgServerRow, 0)
	partial := false
	for _, project := range projects {
		span := traceway.StartSpan(ctx, fmt.Sprintf("org overview servers: project %s", project.Id))
		projectServers, err := loadProjectServerRows(ctx, project, dashboardIds[project.Id], since, now)
		span.End()
		if err != nil {
			partial = true
			traceway.CaptureException(traceway.NewStackTraceErrorf("failed to load server metrics for project %s: %w", project.Id, err))
			continue
		}
		servers = append(servers, projectServers...)
	}

	sort.Slice(servers, func(i, j int) bool {
		return servers[i].LastReportedAt.After(servers[j].LastReportedAt)
	})

	ctx.JSON(http.StatusOK, gin.H{"servers": servers, "refreshedAt": now, "partial": partial})
}

type orgIssueRow struct {
	models.ExceptionGroup
	ProjectId   uuid.UUID `json:"projectId"`
	ProjectName string    `json:"projectName"`
}

type orgIssuesRequest struct {
	FromDate   time.Time        `json:"fromDate"`
	ToDate     time.Time        `json:"toDate"`
	OrderBy    string           `json:"orderBy"`
	Pagination PaginationParams `json:"pagination" binding:"required"`
	Search     string           `json:"search"`
	SearchType string           `json:"searchType"`
}

func normalizeIssueOrderBy(orderBy string) string {
	switch strings.TrimSuffix(orderBy, "_asc") {
	case "last_seen", "first_seen", "count":
		return orderBy
	}
	return "last_seen"
}

func orgIssueLess(orderBy string) func(a, b orgIssueRow) bool {
	ascending := strings.HasSuffix(orderBy, "_asc")
	field := strings.TrimSuffix(orderBy, "_asc")
	return func(a, b orgIssueRow) bool {
		var less, equal bool
		switch field {
		case "count":
			less, equal = a.Count < b.Count, a.Count == b.Count
		case "first_seen":
			less, equal = a.FirstSeen.Before(b.FirstSeen), a.FirstSeen.Equal(b.FirstSeen)
		default:
			less, equal = a.LastSeen.Before(b.LastSeen), a.LastSeen.Equal(b.LastSeen)
		}
		if equal {
			if a.ExceptionHash != b.ExceptionHash {
				return a.ExceptionHash < b.ExceptionHash
			}
			return a.ProjectId.String() < b.ProjectId.String()
		}
		if ascending {
			return less
		}
		return !less
	}
}

func attachOrgIssueTrends(ctx *gin.Context, rows []orgIssueRow) (partial bool) {
	hashesByProject := map[uuid.UUID][]string{}
	for _, row := range rows {
		hashesByProject[row.ProjectId] = append(hashesByProject[row.ProjectId], row.ExceptionHash)
	}
	now := time.Now()
	start24h := now.Add(-24 * time.Hour)
	trends := map[uuid.UUID]map[string][]models.ExceptionTrendPoint{}
	for projectId, hashes := range hashesByProject {
		span := traceway.StartSpan(ctx, fmt.Sprintf("org overview issue trends: project %s", projectId))
		projectTrends, err := telemetry.ExceptionStackTraceRepository.GetHourlyTrendForHashes(ctx, projectId, hashes, start24h, now)
		span.End()
		if err != nil {
			partial = true
			traceway.CaptureException(traceway.NewStackTraceErrorf("failed to load issue trends for project %s: %w", projectId, err))
			continue
		}
		trends[projectId] = projectTrends
	}
	for i := range rows {
		if trend, ok := trends[rows[i].ProjectId][rows[i].ExceptionHash]; ok {
			rows[i].HourlyTrend = trend
		} else {
			rows[i].HourlyTrend = []models.ExceptionTrendPoint{}
		}
	}
	return partial
}

func (c *organizationOverviewController) Issues(ctx *gin.Context) {
	organizationId := middleware.GetOrganizationId(ctx)
	var request orgIssuesRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		middleware.RejectBindError(ctx, err, "Invalid request body")
		return
	}
	if request.ToDate.Before(request.FromDate) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "The end of the range must not be before its start"})
		return
	}
	orderBy := normalizeIssueOrderBy(request.OrderBy)
	page, pageSize := request.Pagination.Page, request.Pagination.PageSize
	maxPages := orgOverviewMaxIssueFetch / pageSize
	if page > maxPages {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("Only the first %d issues can be paged; narrow the time range or search", orgOverviewMaxIssueFetch)})
		return
	}

	projects, err := orgOverviewProjects(organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list organization projects: %w", err))
		return
	}

	fetch := page * pageSize
	candidates := make([]orgIssueRow, 0)
	var total int64
	partial := false
	for _, project := range projects {
		span := traceway.StartSpan(ctx, fmt.Sprintf("org overview issues: project %s", project.Id))
		groups, projectTotal, err := telemetry.ExceptionStackTraceRepository.FindGrouped(ctx, project.Id, request.FromDate, request.ToDate, 1, fetch, orderBy, request.Search, request.SearchType, false)
		span.End()
		if err != nil {
			partial = true
			traceway.CaptureException(traceway.NewStackTraceErrorf("failed to load issues for project %s: %w", project.Id, err))
			continue
		}
		total += projectTotal
		for _, group := range groups {
			candidates = append(candidates, orgIssueRow{ExceptionGroup: group, ProjectId: project.Id, ProjectName: project.Name})
		}
	}

	less := orgIssueLess(orderBy)
	sort.Slice(candidates, func(i, j int) bool { return less(candidates[i], candidates[j]) })
	offset := (page - 1) * pageSize
	rows := []orgIssueRow{}
	if offset < len(candidates) {
		rows = candidates[offset:min(offset+pageSize, len(candidates))]
	}
	if attachOrgIssueTrends(ctx, rows) {
		partial = true
	}
	pagination := buildPagination(page, pageSize, int(total))
	if pagination.TotalPages > int64(maxPages) {
		pagination.TotalPages = int64(maxPages)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data":       rows,
		"pagination": pagination,
		"partial":    partial,
	})
}

type orgPageRow struct {
	pageResponse
	ProjectName string `json:"projectName"`
}

type orgPagesRequest struct {
	Status     string           `json:"status"`
	Search     string           `json:"search"`
	FromDate   *time.Time       `json:"fromDate"`
	ToDate     *time.Time       `json:"toDate"`
	Pagination PaginationParams `json:"pagination" binding:"required"`
}

func (c *organizationOverviewController) Pages(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	organizationId := middleware.GetOrganizationId(ctx)
	var request orgPagesRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		middleware.RejectBindError(ctx, err, "Invalid request body")
		return
	}
	if request.Status == "" {
		request.Status = "active"
	}
	if !validPageStatusFilters[request.Status] {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status filter"})
		return
	}
	if request.FromDate != nil && request.ToDate != nil && request.ToDate.Before(*request.FromDate) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "The end of the range must not be before its start"})
		return
	}
	page, pageSize := request.Pagination.Page, request.Pagination.PageSize

	total, err := transactional.PageRepository.CountByOrganizationFiltered(tx, organizationId, request.Status, request.Search, request.FromDate, request.ToDate)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to count organization pages: %w", err))
		return
	}
	pages, err := transactional.PageRepository.FindByOrganizationFiltered(tx, organizationId, request.Status, request.Search, request.FromDate, request.ToDate, pageSize, (page-1)*pageSize)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list organization pages: %w", err))
		return
	}
	openCount, err := transactional.PageRepository.CountByOrganization(tx, organizationId, models.PageStatusOpen)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to count open pages: %w", err))
		return
	}
	projects, err := transactional.ProjectRepository.FindByOrganizationId(tx, organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list organization projects: %w", err))
		return
	}
	projectNames := make(map[uuid.UUID]string, len(projects))
	for _, project := range projects {
		projectNames[project.Id] = project.Name
	}

	names := map[int]string{}
	for _, page := range pages {
		if page.AcknowledgedBy != nil || page.ResolvedBy != nil {
			names, err = memberNames(tx, organizationId)
			if err != nil {
				ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load members: %w", err))
				return
			}
			break
		}
	}

	items := make([]orgPageRow, 0, len(pages))
	for _, page := range pages {
		items = append(items, orgPageRow{pageResponse: toPageResponse(page, names), ProjectName: projectNames[page.ProjectId]})
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data":           items,
		"pagination":     buildPagination(page, pageSize, total),
		"openPagesCount": openCount,
	})
}

func (c *organizationOverviewController) Counts(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	organizationId := middleware.GetOrganizationId(ctx)
	openCount, err := transactional.PageRepository.CountByOrganization(tx, organizationId, models.PageStatusOpen)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to count open pages: %w", err))
		return
	}
	downCount, err := transactional.SyntheticCheckRepository.CountDownByOrganization(tx, organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to count down monitors: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"openPagesCount": openCount, "downMonitorsCount": downCount})
}

type orgIncidentsRequest struct {
	Search     string           `json:"search"`
	FromDate   *time.Time       `json:"fromDate"`
	ToDate     *time.Time       `json:"toDate"`
	Pagination PaginationParams `json:"pagination" binding:"required"`
}

func (c *organizationOverviewController) Incidents(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	organizationId := middleware.GetOrganizationId(ctx)
	var request orgIncidentsRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		middleware.RejectBindError(ctx, err, "Invalid request body")
		return
	}
	if request.FromDate != nil && request.ToDate != nil && request.ToDate.Before(*request.FromDate) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "The end of the range must not be before its start"})
		return
	}
	page, pageSize := request.Pagination.Page, request.Pagination.PageSize

	total, err := transactional.CheckIncidentRepository.CountByOrganizationFiltered(tx, organizationId, request.Search, request.FromDate, request.ToDate)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to count organization incidents: %w", err))
		return
	}
	incidents, err := transactional.CheckIncidentRepository.FindByOrganizationPaged(tx, organizationId, request.Search, request.FromDate, request.ToDate, pageSize, (page-1)*pageSize)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list organization incidents: %w", err))
		return
	}
	if incidents == nil {
		incidents = []*models.OrgIncident{}
	}
	views, ok := buildIncidentViews(ctx, tx, incidents)
	if !ok {
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": views, "pagination": buildPagination(page, pageSize, total)})
}

type orgMonitorRow struct {
	*models.SyntheticCheck
	ProjectName string                  `json:"projectName"`
	Aggregates  *models.CheckAggregates `json:"aggregates"`
}

func (c *organizationOverviewController) Monitors(ctx *gin.Context) {
	organizationId := middleware.GetOrganizationId(ctx)

	type orgMonitorsData struct {
		projects []*models.Project
		checks   []*models.SyntheticCheck
	}
	data, err := db.ExecuteTransaction(func(tx *sql.Tx) (*orgMonitorsData, error) {
		projects, err := transactional.ProjectRepository.FindByOrganizationId(tx, organizationId)
		if err != nil {
			return nil, err
		}
		checks, err := transactional.SyntheticCheckRepository.FindByOrganization(tx, organizationId)
		if err != nil {
			return nil, err
		}
		return &orgMonitorsData{projects: projects, checks: checks}, nil
	})
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list organization monitors: %w", err))
		return
	}

	projectNames := make(map[uuid.UUID]string, len(data.projects))
	for _, project := range data.projects {
		projectNames[project.Id] = project.Name
	}

	projectsWithChecks := make([]uuid.UUID, 0)
	seen := make(map[uuid.UUID]bool)
	for _, check := range data.checks {
		if !seen[check.ProjectId] {
			seen[check.ProjectId] = true
			projectsWithChecks = append(projectsWithChecks, check.ProjectId)
		}
	}
	now := time.Now().UTC()
	from := now.AddDate(0, 0, -30)

	aggregates := make(map[int]*models.CheckAggregates)
	partial := false
	for _, projectId := range projectsWithChecks {
		span := traceway.StartSpan(ctx, fmt.Sprintf("org overview monitors: project %s", projectId))
		projectAggregates, err := telemetry.CheckResultRepository.AggregatesByProject(ctx, projectId, from, now)
		span.End()
		if err != nil {
			partial = true
			traceway.CaptureException(traceway.NewStackTraceErrorf("failed to aggregate check results for project %s: %w", projectId, err))
			continue
		}
		for checkId, agg := range projectAggregates {
			aggregates[checkId] = agg
		}
	}

	items := make([]orgMonitorRow, 0, len(data.checks))
	for _, check := range data.checks {
		items = append(items, orgMonitorRow{
			SyntheticCheck: check,
			ProjectName:    projectNames[check.ProjectId],
			Aggregates:     aggregates[check.Id],
		})
	}

	ctx.JSON(http.StatusOK, gin.H{"checks": items, "partial": partial})
}
