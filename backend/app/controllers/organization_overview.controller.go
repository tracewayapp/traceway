package controllers

import (
	"database/sql"
	"fmt"
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

const orgOverviewMaxProjects = 25
const orgOverviewMaxIssues = 50
const orgOverviewMaxPages = 50
const orgOverviewIssuesPerProject = 20

// orgOverviewProjects loads the organization's projects in one short
// transaction: the fan-out handlers run their telemetry queries outside any
// tx so the single-connection SQLite main DB is never held across them.
func orgOverviewProjects(organizationId int) ([]*models.Project, bool, error) {
	projects, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.Project, error) {
		return transactional.ProjectRepository.FindByOrganizationId(tx, organizationId)
	})
	if err != nil {
		return nil, false, err
	}
	truncated := false
	if len(projects) > orgOverviewMaxProjects {
		projects = projects[:orgOverviewMaxProjects]
		truncated = true
	}
	return projects, truncated, nil
}

type orgServerRow struct {
	ServerName     string                   `json:"serverName"`
	ProjectId      uuid.UUID                `json:"projectId"`
	ProjectName    string                   `json:"projectName"`
	CpuPct         float64                  `json:"cpuPct"`
	LastReportedAt time.Time                `json:"lastReportedAt"`
	Trend          []models.TimeSeriesPoint `json:"trend"`
}

func (c *organizationOverviewController) Servers(ctx *gin.Context) {
	organizationId := middleware.GetOrganizationId(ctx)
	projects, truncated, err := orgOverviewProjects(organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list organization projects: %w", err))
		return
	}

	now := time.Now()
	since := now.Add(-30 * time.Minute)

	servers := make([]orgServerRow, 0)
	for _, project := range projects {
		span := traceway.StartSpan(ctx, fmt.Sprintf("org overview servers: project %s", project.Id))
		latest, err := telemetry.MetricPointRepository.LatestPerServer(ctx, project.Id, models.MetricNameCpuUsage, since)
		if err != nil {
			span.End()
			traceway.CaptureException(fmt.Errorf("failed to load latest server metrics for project %s: %w", project.Id, err))
			continue
		}
		if len(latest) == 0 {
			span.End()
			continue
		}
		serverNames := make([]string, 0, len(latest))
		for _, point := range latest {
			serverNames = append(serverNames, point.ServerName)
		}
		trends, err := telemetry.MetricPointRepository.GetAverageByIntervalPerServer(ctx, project.Id, models.MetricNameCpuUsage, since, now, 3, serverNames)
		span.End()
		if err != nil {
			traceway.CaptureException(fmt.Errorf("failed to load server metric trends for project %s: %w", project.Id, err))
			trends = map[string][]models.TimeSeriesPoint{}
		}
		for _, point := range latest {
			trend := trends[point.ServerName]
			if trend == nil {
				trend = []models.TimeSeriesPoint{}
			}
			servers = append(servers, orgServerRow{
				ServerName:     point.ServerName,
				ProjectId:      project.Id,
				ProjectName:    project.Name,
				CpuPct:         point.Value,
				LastReportedAt: point.LastReportedAt,
				Trend:          trend,
			})
		}
	}

	sort.Slice(servers, func(i, j int) bool {
		return servers[i].LastReportedAt.After(servers[j].LastReportedAt)
	})

	ctx.JSON(http.StatusOK, gin.H{"servers": servers, "truncated": truncated})
}

type orgIssueRow struct {
	models.ExceptionGroup
	ProjectId   uuid.UUID `json:"projectId"`
	ProjectName string    `json:"projectName"`
}

func (c *organizationOverviewController) Issues(ctx *gin.Context) {
	organizationId := middleware.GetOrganizationId(ctx)
	projects, truncated, err := orgOverviewProjects(organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list organization projects: %w", err))
		return
	}

	now := time.Now()
	from := now.Add(-24 * time.Hour)

	issues := make([]orgIssueRow, 0)
	var totalGroups int64
	for _, project := range projects {
		span := traceway.StartSpan(ctx, fmt.Sprintf("org overview issues: project %s", project.Id))
		groups, total, err := telemetry.ExceptionStackTraceRepository.FindGrouped(ctx, project.Id, from, now, 1, orgOverviewIssuesPerProject, "last_seen", "", "", false)
		span.End()
		if err != nil {
			traceway.CaptureException(fmt.Errorf("failed to load issues for project %s: %w", project.Id, err))
			continue
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

	ctx.JSON(http.StatusOK, gin.H{"issues": issues, "totalGroups": totalGroups, "truncated": truncated})
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
	truncated := false
	if len(projectsWithChecks) > orgOverviewMaxProjects {
		projectsWithChecks = projectsWithChecks[:orgOverviewMaxProjects]
		truncated = true
	}

	now := time.Now()
	from := now.AddDate(0, 0, -30)

	aggregates := make(map[int]*models.CheckAggregates)
	for _, projectId := range projectsWithChecks {
		span := traceway.StartSpan(ctx, fmt.Sprintf("org overview monitors: project %s", projectId))
		projectAggregates, err := telemetry.CheckResultRepository.AggregatesByProject(ctx, projectId, from, now)
		span.End()
		if err != nil {
			traceway.CaptureException(fmt.Errorf("failed to aggregate check results for project %s: %w", projectId, err))
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

	ctx.JSON(http.StatusOK, gin.H{"checks": items, "truncated": truncated})
}
