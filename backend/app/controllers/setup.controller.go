package controllers

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/middleware"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
)

type setupController struct{}

var envVarNameRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// A Gradle local.properties or other .properties file also takes dotted keys (traceway.token).
var propertiesKeyRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z0-9_]+)*$`)

// Bundlers put variables with these prefixes into the shipped app, so an upload token never gets one.
var publicEnvPrefixes = []string{"VITE_", "NEXT_PUBLIC_", "PUBLIC_", "REACT_APP_", "EXPO_PUBLIC_", "NUXT_PUBLIC_", "VUE_APP_", "GATSBY_"}

func validEnvVarName(name, file string) bool {
	if strings.HasSuffix(strings.ToLower(file), ".properties") {
		return propertiesKeyRegex.MatchString(name)
	}
	return envVarNameRegex.MatchString(name)
}

func hasPublicEnvPrefix(name string) bool {
	upper := strings.ToUpper(name)
	for _, prefix := range publicEnvPrefixes {
		if strings.HasPrefix(upper, prefix) {
			return true
		}
	}
	return false
}

const (
	setupEnvFormatToken            = "token"
	setupEnvFormatConnectionString = "connectionString"
)

type SetupSessionProject struct {
	Id        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Framework string    `json:"framework"`
}

type SetupSessionResponse struct {
	OrganizationId   int                   `json:"organizationId"`
	OrganizationName string                `json:"organizationName"`
	Email            string                `json:"email"`
	BackendUrl       string                `json:"backendUrl"`
	Projects         []SetupSessionProject `json:"projects"`
}

func (s setupController) GetSession(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	orgId := middleware.GetSetupOrganizationId(ctx)

	org, err := transactional.OrganizationRepository.FindById(tx, orgId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("setup session: load organization: %w", err))
		return
	}
	if org == nil {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	projects, err := transactional.ProjectRepository.FindByOrganizationId(tx, orgId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("setup session: load projects: %w", err))
		return
	}

	items := make([]SetupSessionProject, 0, len(projects))
	for _, p := range projects {
		items = append(items, SetupSessionProject{Id: p.Id, Name: p.Name, Framework: p.Framework})
	}

	ctx.JSON(http.StatusOK, SetupSessionResponse{
		OrganizationId:   orgId,
		OrganizationName: org.Name,
		Email:            middleware.GetUserEmail(ctx),
		BackendUrl:       instanceUrl(ctx),
		Projects:         items,
	})
}

// The CLI and shell fallback also write default keys when a setup plan omits them.
// Validate those destinations before a proposal can be approved.
type setupFileKey struct {
	file, key string
	serverURL bool
}

func setupFileKeys(p *models.SetupPlanProject) []setupFileKey {
	urlKey := p.UrlEnvVar
	if urlKey == "" {
		switch {
		case strings.HasSuffix(p.EnvVar, "TOKEN"):
			urlKey = strings.TrimSuffix(p.EnvVar, "TOKEN") + "URL"
		case strings.HasSuffix(p.EnvVar, "token"):
			urlKey = strings.TrimSuffix(p.EnvVar, "token") + "url"
		case strings.Contains(p.EnvVar, "."):
			urlKey = p.EnvVar + ".url"
		default:
			urlKey = p.EnvVar + "_URL"
		}
	}
	var keys []setupFileKey
	if p.EnvFile != "" && p.EnvVar != "" {
		keys = append(keys, setupFileKey{p.EnvFile, p.EnvVar, false})
		if p.EnvFormat != setupEnvFormatConnectionString {
			keys = append(keys, setupFileKey{p.EnvFile, urlKey, true})
		}
	}
	file, uploadKey := p.UploadTokenEnvFile, p.UploadTokenEnvVar
	if file == "" && uploadKey != "" {
		file = p.EnvFile
	}
	if file != "" {
		if uploadKey == "" {
			switch p.Framework {
			case "react", "svelte", "vuejs", "jquery", "nextjs":
				uploadKey = "TRACEWAY_SOURCEMAP_TOKEN"
			default:
				uploadKey = "TRACEWAY_UPLOAD_TOKEN"
			}
		}
		uploadURL := "TRACEWAY_URL"
		if strings.HasSuffix(strings.ToLower(file), ".properties") && strings.Contains(p.EnvVar, ".") {
			uploadURL = urlKey
		}
		keys = append(keys, setupFileKey{file, uploadKey, false}, setupFileKey{file, uploadURL, true})
	}
	return keys
}

func validateSetupPlanPayload(payload *models.SetupPlanPayload) string {
	if len(payload.Projects) == 0 {
		return "At least one project is required"
	}
	if len(payload.Projects) > maxBatchProjects {
		return "At most 20 projects per plan"
	}

	seen := map[string]bool{}
	files := map[[2]string]bool{}
	for i := range payload.Projects {
		p := &payload.Projects[i]
		p.Name = strings.TrimSpace(p.Name)
		if msg := validateProjectName(p.Name); msg != "" {
			return msg
		}
		if seen[p.Name] {
			return "Duplicate project name in plan: " + p.Name
		}
		seen[p.Name] = true
		if !validFrameworks[p.Framework] {
			return invalidFrameworkMessage
		}
		if utf8.RuneCountInString(p.EnvFile) > 300 || utf8.RuneCountInString(p.UploadTokenEnvFile) > 300 {
			return "envFile must be 300 characters or fewer"
		}
		if p.EnvVar != "" && !validEnvVarName(p.EnvVar, p.EnvFile) {
			return "envVar must be a valid environment variable name"
		}
		if p.UrlEnvVar != "" && !validEnvVarName(p.UrlEnvVar, p.EnvFile) {
			return "urlEnvVar must be a valid environment variable name"
		}
		uploadFile := p.UploadTokenEnvFile
		if uploadFile == "" {
			uploadFile = p.EnvFile
		}
		if p.UploadTokenEnvVar != "" && !validEnvVarName(p.UploadTokenEnvVar, uploadFile) {
			return "uploadTokenEnvVar must be a valid environment variable name"
		}
		if hasPublicEnvPrefix(p.UploadTokenEnvVar) {
			return "uploadTokenEnvVar must not have a public prefix such as VITE_ or NEXT_PUBLIC_: the upload token is a secret"
		}
		if p.EnvFile != "" && p.EnvVar == "" {
			return "envFile requires envVar"
		}
		if p.UploadTokenEnvVar != "" && uploadFile == "" {
			return "uploadTokenEnvVar requires an uploadTokenEnvFile or envFile"
		}
		for _, w := range setupFileKeys(p) {
			slot := [2]string{filepath.Clean(w.file), w.key}
			if isURL, exists := files[slot]; exists && !(isURL && w.serverURL) {
				return "Conflicting environment variable assignments in " + w.file + ": " + w.key
			}
			files[slot] = w.serverURL
		}
		if utf8.RuneCountInString(p.EnvVar) > 100 || utf8.RuneCountInString(p.UrlEnvVar) > 100 || utf8.RuneCountInString(p.UploadTokenEnvVar) > 100 {
			return "envVar must be 100 characters or fewer"
		}
		switch p.EnvFormat {
		case "", setupEnvFormatToken, setupEnvFormatConnectionString:
		default:
			return "envFormat must be \"token\" or \"connectionString\""
		}
		if p.Deployment != nil {
			p.Deployment.Platform = strings.TrimSpace(p.Deployment.Platform)
			p.Deployment.Instructions = strings.TrimSpace(p.Deployment.Instructions)
			if utf8.RuneCountInString(p.Deployment.Platform) > 100 {
				return "deployment platform must be 100 characters or fewer"
			}
			if utf8.RuneCountInString(p.Deployment.Instructions) > 2000 {
				return "deployment instructions must be 2000 characters or fewer"
			}
		}
	}
	return ""
}

func (s setupController) SubmitPlan(ctx *gin.Context) {
	var payload models.SetupPlanPayload
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		middleware.RejectBindError(ctx, err, "Invalid request body")
		return
	}
	if msg := validateSetupPlanPayload(&payload); msg != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": msg})
		return
	}

	canonical, err := json.Marshal(payload)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("marshal setup plan: %w", err))
		return
	}

	tx := db.GetTx(ctx)
	if err := transactional.SetupPlanRepository.Upsert(tx, uuid.New().String(), middleware.GetUserId(ctx), middleware.GetSetupOrganizationId(ctx), string(canonical)); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("store setup plan: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "pending"})
}

type setupPlanResultItem struct {
	Id     uuid.UUID `json:"id"`
	Status string    `json:"status"`
}

func resolvePlanResultProjects(ctx *gin.Context, plan *transactional.SetupPlanRow) ([]BatchProjectResponseItem, bool) {
	tx := db.GetTx(ctx)

	var resultItems []setupPlanResultItem
	if plan.Result != "" {
		if err := json.Unmarshal([]byte(plan.Result), &resultItems); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("parse setup plan result: %w", err))
			return nil, false
		}
	}

	items := make([]BatchProjectResponseItem, 0, len(resultItems))
	for _, r := range resultItems {
		project, err := transactional.ProjectRepository.FindById(tx, r.Id)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("load setup plan project: %w", err))
			return nil, false
		}
		if project == nil {
			continue
		}
		items = append(items, BatchProjectResponseItem{
			Id:             project.Id,
			Name:           project.Name,
			Framework:      project.Framework,
			Token:          project.Token,
			SourceMapToken: project.SourceMapToken,
			BackendUrl:     instanceUrl(ctx),
			Status:         r.Status,
		})
	}
	return items, true
}

func (s setupController) GetPlan(ctx *gin.Context) {
	tx := db.GetTx(ctx)

	plan, err := transactional.SetupPlanRepository.FindLatestByUserAndOrganization(tx, middleware.GetUserId(ctx), middleware.GetSetupOrganizationId(ctx))
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("load setup plan: %w", err))
		return
	}
	if plan == nil {
		ctx.JSON(http.StatusOK, gin.H{"status": "none"})
		return
	}

	switch plan.Status {
	case "approved":
		projects, ok := resolvePlanResultProjects(ctx, plan)
		if !ok {
			return
		}
		ctx.JSON(http.StatusOK, gin.H{"status": "approved", "projects": projects})
	case "rejected":
		ctx.JSON(http.StatusOK, gin.H{"status": "rejected", "reason": plan.RejectReason})
	default:
		ctx.JSON(http.StatusOK, gin.H{"status": "pending"})
	}
}

type SetupDraftResponse struct {
	Id               string                  `json:"id"`
	Status           string                  `json:"status"`
	Payload          models.SetupPlanPayload `json:"payload"`
	CreatedAt        time.Time               `json:"createdAt"`
	RequestedByEmail string                  `json:"requestedByEmail"`
	RejectReason     string                  `json:"rejectReason,omitempty"`
	// BackendUrl lets the review name the server URL variable the plan writes off Traceway Cloud, before any
	// project exists to carry it.
	BackendUrl string                     `json:"backendUrl"`
	Projects   []BatchProjectResponseItem `json:"projects,omitempty"`
}

func (s setupController) draftResponse(ctx *gin.Context, plan *transactional.SetupPlanRow) (*SetupDraftResponse, bool) {
	var payload models.SetupPlanPayload
	if err := json.Unmarshal([]byte(plan.Payload), &payload); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("parse setup plan payload: %w", err))
		return nil, false
	}

	response := &SetupDraftResponse{
		Id:               plan.Id,
		Status:           plan.Status,
		Payload:          payload,
		CreatedAt:        plan.CreatedAt,
		RequestedByEmail: plan.RequestedByEmail,
		RejectReason:     plan.RejectReason,
		BackendUrl:       instanceUrl(ctx),
	}
	if plan.Status == "approved" {
		projects, ok := resolvePlanResultProjects(ctx, plan)
		if !ok {
			return nil, false
		}
		response.Projects = projects
	}
	return response, true
}

func (s setupController) ListDraft(ctx *gin.Context) {
	orgId, err := strconv.Atoi(ctx.Query("organizationId"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "organizationId is required"})
		return
	}

	tx := db.GetTx(ctx)
	if !requireOrgWrite(ctx, tx, orgId) {
		return
	}

	plan, err := transactional.SetupPlanRepository.FindReviewableByOrganization(tx, orgId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("load setup plan: %w", err))
		return
	}
	if plan == nil {
		ctx.JSON(http.StatusOK, gin.H{"draft": nil})
		return
	}

	draft, ok := s.draftResponse(ctx, plan)
	if !ok {
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"draft": draft})
}

func (s setupController) loadDecidablePlan(ctx *gin.Context) *transactional.SetupPlanRow {
	tx := db.GetTx(ctx)

	plan, err := transactional.SetupPlanRepository.FindById(tx, ctx.Param("id"))
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("load setup plan: %w", err))
		return nil
	}
	if plan == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Proposal not found"})
		return nil
	}
	if !requireOrgWrite(ctx, tx, plan.OrganizationId) {
		return nil
	}
	if plan.Status != "pending" {
		ctx.JSON(http.StatusConflict, gin.H{"error": "This proposal was already decided"})
		return nil
	}
	return plan
}

func (s setupController) ApproveDraft(ctx *gin.Context) {
	plan := s.loadDecidablePlan(ctx)
	if plan == nil {
		return
	}

	var payload models.SetupPlanPayload
	if err := json.Unmarshal([]byte(plan.Payload), &payload); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("parse setup plan payload: %w", err))
		return
	}

	inputs := make([]BatchProjectInput, 0, len(payload.Projects))
	for _, p := range payload.Projects {
		inputs = append(inputs, BatchProjectInput{Name: p.Name, Framework: p.Framework})
	}

	tx := db.GetTx(ctx)
	userId := middleware.GetUserId(ctx)

	results, err := batchCreateProjects(tx, plan.OrganizationId, userId, inputs)
	if err != nil {
		respondBatchCreateError(ctx, err)
		return
	}

	resultItems := make([]setupPlanResultItem, 0, len(results))
	for _, r := range results {
		resultItems = append(resultItems, setupPlanResultItem{Id: r.Project.Id, Status: r.Status})
	}
	resultJSON, err := json.Marshal(resultItems)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("marshal setup plan result: %w", err))
		return
	}

	rows, err := transactional.SetupPlanRepository.Decide(tx, plan.Id, "approved", "", string(resultJSON), userId, time.Now().UTC())
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("approve setup plan: %w", err))
		return
	}
	if rows == 0 {
		ctx.JSON(http.StatusConflict, gin.H{"error": "This proposal was already decided"})
		return
	}

	cacheCreatedProjectsOnCommit(ctx, results)
	ctx.JSON(http.StatusOK, gin.H{"projects": toBatchProjectResponse(ctx, results)})
}

type RejectDraftRequest struct {
	Reason string `json:"reason"`
}

func (s setupController) RejectDraft(ctx *gin.Context) {
	plan := s.loadDecidablePlan(ctx)
	if plan == nil {
		return
	}

	var request RejectDraftRequest
	_ = ctx.ShouldBindJSON(&request)
	reason := strings.TrimSpace(request.Reason)
	if utf8.RuneCountInString(reason) > 500 {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Reason must be 500 characters or fewer"})
		return
	}

	tx := db.GetTx(ctx)
	rows, err := transactional.SetupPlanRepository.Decide(tx, plan.Id, "rejected", reason, "", middleware.GetUserId(ctx), time.Now().UTC())
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("reject setup plan: %w", err))
		return
	}
	if rows == 0 {
		ctx.JSON(http.StatusConflict, gin.H{"error": "This proposal was already decided"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "rejected"})
}

var SetupController = setupController{}
