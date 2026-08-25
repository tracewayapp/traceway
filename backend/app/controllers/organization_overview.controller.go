package controllers

import (
	"database/sql"
	"fmt"
	"math"
	"net/http"
	"sort"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

type organizationOverviewController struct{}

var OrganizationOverviewController = organizationOverviewController{}

const orgOverviewMaxIssues = 50
const orgOverviewMaxPages = 50

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
	if value <= 1.5 {
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
		networkReceive, err := telemetry.MetricPointRepository.QueryTimeSeries(ctx, project.Id, models.MetricNameSystemNetworkIO, since, now, 1, "sum", map[string]string{"direction": "receive"}, "server_name", 0)
		if err != nil {
			return nil, err
		}
		networkTransmit, err := telemetry.MetricPointRepository.QueryTimeSeries(ctx, project.Id, models.MetricNameSystemNetworkIO, since, now, 1, "sum", map[string]string{"direction": "transmit"}, "server_name", 0)
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
			if value, ok := latestCounterRate(networkReceive[point.ServerName]); ok {
				row.NetworkRxBps = pointerTo(value)
			}
			if value, ok := latestCounterRate(networkTransmit[point.ServerName]); ok {
				row.NetworkTxBps = pointerTo(value)
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

func (c *organizationOverviewController) Issues(ctx *gin.Context) {
	organizationId := middleware.GetOrganizationId(ctx)
	projects, err := orgOverviewProjects(organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list organization projects: %w", err))
		return
	}

	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)

	issues := make([]orgIssueRow, 0)
	var totalGroups int64
	for _, project := range projects {
		span := traceway.StartSpan(ctx, fmt.Sprintf("org overview issues: project %s", project.Id))
		groups, total, err := telemetry.ExceptionStackTraceRepository.FindGrouped(ctx, project.Id, from, now, 1, orgOverviewMaxIssues, "last_seen", "", "", false)
		span.End()
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load issues for project %s: %w", project.Id, err))
			return
		}
		totalGroups += total
		for _, group := range groups {
			issues = append(issues, orgIssueRow{ExceptionGroup: group, ProjectId: project.Id, ProjectName: project.Name})
		}
	}

	sort.Slice(issues, func(i, j int) bool {
		return issues[i].LastSeen.After(issues[j].LastSeen)
	})
	if len(issues) > orgOverviewMaxIssues {
		issues = issues[:orgOverviewMaxIssues]
	}

	ctx.JSON(http.StatusOK, gin.H{"issues": issues, "totalGroups": totalGroups})
}

type orgPageRow struct {
	pageResponse
	ProjectName string `json:"projectName"`
}

func (c *organizationOverviewController) Pages(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	organizationId := middleware.GetOrganizationId(ctx)

	pages, err := transactional.PageRepository.FindByOrganization(tx, organizationId, "active", orgOverviewMaxPages, 0)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list organization pages: %w", err))
		return
	}
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

	ctx.JSON(http.StatusOK, gin.H{"pages": items, "openPagesCount": openCount, "downMonitorsCount": downCount})
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
	for _, projectId := range projectsWithChecks {
		span := traceway.StartSpan(ctx, fmt.Sprintf("org overview monitors: project %s", projectId))
		projectAggregates, err := telemetry.CheckResultRepository.AggregatesByProject(ctx, projectId, from, now)
		span.End()
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to aggregate check results for project %s: %w", projectId, err))
			return
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

	ctx.JSON(http.StatusOK, gin.H{"checks": items})
}
