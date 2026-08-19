package controllers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/storage"

	traceway "go.tracewayapp.com"
)

type statusPageController struct{}

func organizationIdFromPath(ctx *gin.Context) (int, bool) {
	id, err := strconv.Atoi(ctx.Param("organizationId"))
	if err != nil || id < 1 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization id"})
		return 0, false
	}
	return id, true
}

var statusPageSlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,58}[a-z0-9]$`)

const (
	statusPageUptimeDays      = 90
	statusPageCacheTTL        = 30 * time.Second
	statusPageMaxChecks       = 50
	statusPageIncidentCap     = 20
	statusPagePastIncidentCap = 30
)

type statusPageRequest struct {
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	IsPublic     bool   `json:"isPublic"`
	CheckIds     []int  `json:"checkIds"`
	Description  string `json:"description"`
	CustomDomain string `json:"customDomain"`
}

var customDomainPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$`)

func validateStatusPageRequest(tx *sql.Tx, organizationId int, req *statusPageRequest) (string, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Slug = strings.ToLower(strings.TrimSpace(req.Slug))
	if req.Name == "" {
		return "Name is required.", nil
	}
	if len(req.Name) > 200 {
		return "Name must be 200 characters or fewer.", nil
	}
	if !statusPageSlugPattern.MatchString(req.Slug) {
		return "Slug must be 3-60 characters of lowercase letters, digits, and hyphens.", nil
	}
	if len(req.CheckIds) == 0 {
		return "Select at least one check.", nil
	}
	if len(req.CheckIds) > statusPageMaxChecks {
		return "A status page can show at most 50 checks.", nil
	}
	req.Description = strings.TrimSpace(req.Description)
	if len(req.Description) > 500 {
		return "Description must be 500 characters or fewer.", nil
	}
	req.CustomDomain = strings.ToLower(strings.TrimSpace(req.CustomDomain))
	if req.CustomDomain != "" && !customDomainPattern.MatchString(req.CustomDomain) {
		return "Custom domain must be a bare hostname like status.example.com (no scheme or path).", nil
	}
	for _, checkId := range req.CheckIds {
		check, err := transactional.SyntheticCheckRepository.FindByIdInOrganization(tx, checkId, organizationId)
		if err != nil {
			return "", err
		}
		if check == nil {
			return "One of the selected checks does not belong to this organization.", nil
		}
	}
	return "", nil
}

func (ctrl *statusPageController) List(ctx *gin.Context) {
	organizationId, ok := organizationIdFromPath(ctx)
	if !ok {
		return
	}
	tx := db.GetTx(ctx)
	pages, err := transactional.StatusPageRepository.ListByOrganization(tx, organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list status pages: %w", err))
		return
	}
	if pages == nil {
		pages = []*models.StatusPage{}
	}
	ctx.JSON(http.StatusOK, gin.H{"statusPages": pages})
}

func (ctrl *statusPageController) Create(ctx *gin.Context) {
	organizationId, ok := organizationIdFromPath(ctx)
	if !ok {
		return
	}
	var req statusPageRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	tx := db.GetTx(ctx)
	if message, err := validateStatusPageRequest(tx, organizationId, &req); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to validate status page: %w", err))
		return
	} else if message != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
		return
	}
	if existing, err := transactional.StatusPageRepository.FindBySlug(tx, req.Slug); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to check slug uniqueness: %w", err))
		return
	} else if existing != nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "This slug is already taken."})
		return
	}
	if req.CustomDomain != "" {
		if existing, err := transactional.StatusPageRepository.FindByCustomDomain(tx, req.CustomDomain); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to check domain uniqueness: %w", err))
			return
		} else if existing != nil {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "This custom domain is already used by another status page."})
			return
		}
	}

	checkIdsJSON, _ := json.Marshal(req.CheckIds)
	now := time.Now().UTC()
	page := &models.StatusPage{
		OrganizationId: organizationId,
		Slug:           req.Slug,
		Name:           req.Name,
		IsPublic:       req.IsPublic,
		CheckIds:       models.JSONText(checkIdsJSON),
		Description:    req.Description,
		CustomDomain:   req.CustomDomain,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	id, err := transactional.StatusPageRepository.Create(tx, page)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to create status page: %w", err))
		return
	}
	page.Id = id
	ctx.JSON(http.StatusCreated, page)
}

func (ctrl *statusPageController) Update(ctx *gin.Context) {
	organizationId, ok := organizationIdFromPath(ctx)
	if !ok {
		return
	}
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id < 1 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status page id"})
		return
	}
	var req statusPageRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	tx := db.GetTx(ctx)
	page, err := transactional.StatusPageRepository.FindByIdForOrganization(tx, id, organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load status page: %w", err))
		return
	}
	if page == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Status page not found"})
		return
	}
	if message, err := validateStatusPageRequest(tx, organizationId, &req); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to validate status page: %w", err))
		return
	} else if message != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
		return
	}
	if req.Slug != page.Slug {
		if existing, err := transactional.StatusPageRepository.FindBySlug(tx, req.Slug); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to check slug uniqueness: %w", err))
			return
		} else if existing != nil {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "This slug is already taken."})
			return
		}
	}
	if req.CustomDomain != "" && req.CustomDomain != page.CustomDomain {
		if existing, err := transactional.StatusPageRepository.FindByCustomDomain(tx, req.CustomDomain); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to check domain uniqueness: %w", err))
			return
		} else if existing != nil && existing.Id != page.Id {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "This custom domain is already used by another status page."})
			return
		}
	}

	checkIdsJSON, _ := json.Marshal(req.CheckIds)
	previousSlug := page.Slug
	page.Name = req.Name
	page.Slug = req.Slug
	page.IsPublic = req.IsPublic
	page.CheckIds = models.JSONText(checkIdsJSON)
	page.Description = req.Description
	page.CustomDomain = req.CustomDomain
	page.UpdatedAt = time.Now().UTC()
	if err := transactional.StatusPageRepository.Update(tx, page); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update status page: %w", err))
		return
	}
	// A rename must drop the old slug's cached view too, or the retired URL
	// keeps serving for the rest of its TTL.
	invalidateStatusPageCache(previousSlug)
	invalidateStatusPageCache(page.Slug)
	ctx.JSON(http.StatusOK, page)
}

func (ctrl *statusPageController) Delete(ctx *gin.Context) {
	organizationId, ok := organizationIdFromPath(ctx)
	if !ok {
		return
	}
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id < 1 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status page id"})
		return
	}
	tx := db.GetTx(ctx)
	page, err := transactional.StatusPageRepository.FindByIdForOrganization(tx, id, organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load status page: %w", err))
		return
	}
	if page == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Status page not found"})
		return
	}
	if err := transactional.StatusPageRepository.Delete(tx, id, organizationId); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete status page: %w", err))
		return
	}
	invalidateStatusPageCache(page.Slug)
	ctx.JSON(http.StatusOK, gin.H{"deleted": true})
}

// Public payload shapes.

type statusPageCheckView struct {
	Name      string                     `json:"name"`
	CheckType string                     `json:"checkType"`
	Status    string                     `json:"status"`
	UptimePct *float64                   `json:"uptimePct"`
	Days      []*models.CheckSeriesPoint `json:"days"`
	Incidents []statusPageIncidentView   `json:"incidents"`
}

type statusPageIncidentView struct {
	StartedAt  time.Time  `json:"startedAt"`
	ResolvedAt *time.Time `json:"resolvedAt"`
}

type statusPagePastIncidentView struct {
	Title      string                             `json:"title"`
	StartedAt  time.Time                          `json:"startedAt"`
	ResolvedAt *time.Time                         `json:"resolvedAt"`
	Updates    []statusPagePastIncidentUpdateView `json:"updates"`
}

type statusPagePastIncidentUpdateView struct {
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

type statusPageView struct {
	Name          string                       `json:"name"`
	Description   string                       `json:"description"`
	HasLogo       bool                         `json:"hasLogo"`
	Checks        []statusPageCheckView        `json:"checks"`
	PastIncidents []statusPagePastIncidentView `json:"pastIncidents"`
	GeneratedAt   time.Time                    `json:"generatedAt"`
}

// The public endpoint is anonymous and its 90-day aggregation is far heavier
// than a point lookup, so responses are cached briefly per slug. Only found
// pages are cached: a miss is a single indexed lookup, and negative entries
// would let unknown-slug spam grow the map without bound.
var statusPageCache = struct {
	mu      sync.Mutex
	entries map[string]statusPageCacheEntry
}{entries: map[string]statusPageCacheEntry{}}

type statusPageCacheEntry struct {
	view      *statusPageView
	expiresAt time.Time
}

func invalidateStatusPageCache(slug string) {
	statusPageCache.mu.Lock()
	delete(statusPageCache.entries, slug)
	statusPageCache.mu.Unlock()
}

// Public serves GET /api/status/:slug with no authentication. Unknown and
// private slugs are both 404.
func (ctrl *statusPageController) Public(ctx *gin.Context) {
	slug := strings.ToLower(ctx.Param("slug"))
	if !statusPageSlugPattern.MatchString(slug) {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Status page not found"})
		return
	}

	now := time.Now().UTC()
	statusPageCache.mu.Lock()
	cached, ok := statusPageCache.entries[slug]
	statusPageCache.mu.Unlock()
	if ok && now.Before(cached.expiresAt) {
		ctx.JSON(http.StatusOK, cached.view)
		return
	}

	view, err := buildStatusPageView(ctx, slug, now)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to build status page: %w", err))
		return
	}
	if view == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Status page not found"})
		return
	}
	statusPageCache.mu.Lock()
	for key, entry := range statusPageCache.entries {
		if now.After(entry.expiresAt) {
			delete(statusPageCache.entries, key)
		}
	}
	statusPageCache.entries[slug] = statusPageCacheEntry{view: view, expiresAt: now.Add(statusPageCacheTTL)}
	statusPageCache.mu.Unlock()

	ctx.JSON(http.StatusOK, view)
}

func buildStatusPageView(ctx *gin.Context, slug string, now time.Time) (*statusPageView, error) {
	type pageWithChecks struct {
		page   *models.StatusPage
		checks []*models.SyntheticCheck
	}
	loaded, err := db.ExecuteTransaction(func(tx *sql.Tx) (pageWithChecks, error) {
		page, err := transactional.StatusPageRepository.FindPublicBySlug(tx, slug)
		if err != nil || page == nil {
			return pageWithChecks{}, err
		}
		var checkIds []int
		if err := json.Unmarshal(page.CheckIds, &checkIds); err != nil {
			return pageWithChecks{page: page}, nil
		}
		var checks []*models.SyntheticCheck
		for _, checkId := range checkIds {
			// Tolerates ids pointing at deleted checks.
			check, err := transactional.SyntheticCheckRepository.FindByIdInOrganization(tx, checkId, page.OrganizationId)
			if err != nil {
				return pageWithChecks{}, err
			}
			if check != nil {
				checks = append(checks, check)
			}
		}
		return pageWithChecks{page: page, checks: checks}, nil
	})
	if err != nil || loaded.page == nil {
		return nil, err
	}

	from := now.AddDate(0, 0, -statusPageUptimeDays)
	view := &statusPageView{
		Name:          loaded.page.Name,
		Description:   loaded.page.Description,
		HasLogo:       loaded.page.LogoKey != "",
		Checks:        []statusPageCheckView{},
		PastIncidents: []statusPagePastIncidentView{},
		GeneratedAt:   now,
	}
	type pastIncidentSource struct {
		incident  *models.CheckIncident
		checkName string
	}
	var pastIncidents []pastIncidentSource
	for _, check := range loaded.checks {
		days, err := telemetry.CheckResultRepository.SeriesByCheck(ctx, check.ProjectId, check.Id, from, now, 1440)
		if err != nil {
			return nil, err
		}
		if days == nil {
			days = []*models.CheckSeriesPoint{}
		}
		var up, measured int64
		for _, day := range days {
			up += day.Up
			measured += day.Total - day.Missed
		}
		var uptimePct *float64
		if measured > 0 {
			pct := float64(up) / float64(measured) * 100
			uptimePct = &pct
		}
		// Latency values are internal detail; the public payload only carries
		// availability counts.
		for _, day := range days {
			day.AvgLatencyMs = 0
			day.MaxLatencyMs = 0
		}

		incidents, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.CheckIncident, error) {
			return transactional.CheckIncidentRepository.FindByCheckSince(tx, check.Id, from)
		})
		if err != nil {
			return nil, err
		}
		incidentViews := make([]statusPageIncidentView, 0, len(incidents))
		for i, incident := range incidents {
			if i >= statusPageIncidentCap {
				break
			}
			incidentViews = append(incidentViews, statusPageIncidentView{StartedAt: incident.StartedAt, ResolvedAt: incident.ResolvedAt})
		}
		for _, incident := range incidents {
			pastIncidents = append(pastIncidents, pastIncidentSource{incident: incident, checkName: check.Name})
		}

		status := check.CurrentStatus
		if !check.Enabled {
			status = models.CheckStatusUnknown
		}
		view.Checks = append(view.Checks, statusPageCheckView{
			Name:      check.Name,
			CheckType: check.CheckType,
			Status:    status,
			UptimePct: uptimePct,
			Days:      days,
			Incidents: incidentViews,
		})
	}

	manualIncidents, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.CheckIncident, error) {
		return transactional.CheckIncidentRepository.FindByStatusPageSince(tx, loaded.page.Id, from)
	})
	if err != nil {
		return nil, err
	}
	for _, incident := range manualIncidents {
		pastIncidents = append(pastIncidents, pastIncidentSource{incident: incident})
	}
	sort.SliceStable(pastIncidents, func(a, b int) bool {
		if pastIncidents[a].incident.StartedAt.Equal(pastIncidents[b].incident.StartedAt) {
			return pastIncidents[a].incident.Id > pastIncidents[b].incident.Id
		}
		return pastIncidents[a].incident.StartedAt.After(pastIncidents[b].incident.StartedAt)
	})
	if len(pastIncidents) > statusPagePastIncidentCap {
		pastIncidents = pastIncidents[:statusPagePastIncidentCap]
	}
	incidentIds := make([]int, len(pastIncidents))
	for i, source := range pastIncidents {
		incidentIds[i] = source.incident.Id
	}
	updates, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.IncidentUpdate, error) {
		return transactional.IncidentUpdateRepository.FindByIncidentIds(tx, incidentIds)
	})
	if err != nil {
		return nil, err
	}
	updatesByIncident := map[int][]statusPagePastIncidentUpdateView{}
	for _, update := range updates {
		updatesByIncident[update.IncidentId] = append(updatesByIncident[update.IncidentId], statusPagePastIncidentUpdateView{
			Status:    update.Status,
			Message:   update.Message,
			CreatedAt: update.CreatedAt,
		})
	}
	for _, source := range pastIncidents {
		title := source.incident.Title
		if title == "" {
			title = source.checkName + " is down"
		}
		incidentUpdates := updatesByIncident[source.incident.Id]
		if incidentUpdates == nil {
			incidentUpdates = []statusPagePastIncidentUpdateView{}
		}
		view.PastIncidents = append(view.PastIncidents, statusPagePastIncidentView{
			Title:      title,
			StartedAt:  source.incident.StartedAt,
			ResolvedAt: source.incident.ResolvedAt,
			Updates:    incidentUpdates,
		})
	}
	return view, nil
}

var StatusPageController = statusPageController{}

const statusPageLogoMaxBytes = 1 << 20

// UploadLogo stores a PNG/JPEG logo for the status page. SVG is deliberately
// rejected: it can carry scripts and this file is served to anonymous
// visitors.
func (ctrl *statusPageController) UploadLogo(ctx *gin.Context) {
	organizationId, ok := organizationIdFromPath(ctx)
	if !ok {
		return
	}
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id < 1 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status page id"})
		return
	}
	tx := db.GetTx(ctx)
	page, err := transactional.StatusPageRepository.FindByIdForOrganization(tx, id, organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load status page: %w", err))
		return
	}
	if page == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Status page not found"})
		return
	}

	data, err := io.ReadAll(http.MaxBytesReader(ctx.Writer, ctx.Request.Body, statusPageLogoMaxBytes))
	if err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "The logo must be 1MB or smaller."})
		return
	}
	contentType := http.DetectContentType(data)
	if contentType != "image/png" && contentType != "image/jpeg" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "The logo must be a PNG or JPEG image."})
		return
	}
	key := fmt.Sprintf("statuspages/%d/logo", page.Id)
	if err := storage.Store.Write(ctx, key, data); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to store status page logo: %w", err))
		return
	}
	page.LogoKey = key
	page.UpdatedAt = time.Now().UTC()
	if err := transactional.StatusPageRepository.Update(tx, page); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to save status page logo key: %w", err))
		return
	}
	invalidateStatusPageCache(page.Slug)
	ctx.JSON(http.StatusOK, page)
}

// PublicLogo streams the logo of a public status page.
func (ctrl *statusPageController) PublicLogo(ctx *gin.Context) {
	slug := strings.ToLower(ctx.Param("slug"))
	page, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.StatusPage, error) {
		return transactional.StatusPageRepository.FindPublicBySlug(tx, slug)
	})
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load status page: %w", err))
		return
	}
	if page == nil || page.LogoKey == "" {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}
	data, err := storage.Store.Read(ctx, page.LogoKey)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}
	ctx.Header("Cache-Control", "public, max-age=300")
	ctx.Data(http.StatusOK, http.DetectContentType(data), data)
}

// ResolveHost maps the request's Host header to a public status page slug,
// so a CNAMEd vanity domain (status.example.com -> this instance) can land
// on its status page. TLS for the vanity host terminates at the operator's
// proxy; this endpoint only does the host-to-slug mapping.
func (ctrl *statusPageController) ResolveHost(ctx *gin.Context) {
	host := strings.ToLower(ctx.Request.Host)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if host == "" || !customDomainPattern.MatchString(host) {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}
	page, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.StatusPage, error) {
		return transactional.StatusPageRepository.FindPublicByCustomDomain(tx, host)
	})
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to resolve status host: %w", err))
		return
	}
	if page == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"slug": page.Slug})
}
