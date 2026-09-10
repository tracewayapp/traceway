package controllers

import (
	"context"
	"database/sql"
	"errors"
	"github.com/tracewayapp/traceway/backend/app/agentrunner"
	"github.com/tracewayapp/traceway/backend/app/agentrunner/agents"
	"github.com/tracewayapp/traceway/backend/app/config"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/agent"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/integrations/web"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/storage"
	traceway "go.tracewayapp.com"
)

type agentAttemptController struct{}

var exceptionHashPattern = regexp.MustCompile(`^[0-9a-f]{16}$`)

const (
	attemptEventPageLimit   = 500
	attemptMessagePageLimit = 200
	maxAgentMessageBytes    = 20000
)

// preflightCheck is one row of the Fix it dialog: what is wired, what is
// missing, and where to fix it.
type preflightCheck struct {
	Key   string `json:"key"`
	Ok    bool   `json:"ok"`
	Label string `json:"label"`
	Hint  string `json:"hint,omitempty"`
	Href  string `json:"href,omitempty"`
}

type preflightResponse struct {
	Checks         []preflightCheck       `json:"checks"`
	NextNumber     int                    `json:"nextNumber"`
	ActiveAttempt  *models.AgentAttempt   `json:"activeAttempt,omitempty"`
	Profiles       []*models.AgentProfile `json:"profiles"`
	DefaultProfile *int                   `json:"defaultProfileId,omitempty"`
	CanStart       bool                   `json:"canStart"`
}

func (ctrl *agentAttemptController) Capabilities(ctx *gin.Context) {
	mode, err := agentrunner.Mode()
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"enabled": mode != agentrunner.ModeOff})
}

func (ctrl *agentAttemptController) Preflight(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	hash := strings.TrimSpace(ctx.Query("hash"))
	if !exceptionHashPattern.MatchString(hash) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "hash must be 16 lowercase hex characters"})
		return
	}
	var profileId *int
	if value := ctx.Query("profileId"); value != "" {
		id, err := strconv.Atoi(value)
		if err != nil || id <= 0 {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "profileId must be a positive integer"})
			return
		}
		profileId = &id
	}
	response, err := buildAgentPreflight(ctx.Request.Context(), db.GetTx(ctx), projectId, hash, profileId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	ctx.JSON(http.StatusOK, response)
}

func describeModel(model string) string {
	if model == "" {
		return ""
	}
	return " on " + model
}

func hasKind(kinds []string, kind string) bool {
	for _, k := range kinds {
		if k == kind {
			return true
		}
	}
	return false
}

type startAttemptRequest struct {
	Hash      string `json:"hash"`
	ProfileId *int   `json:"profileId"`
}

func (ctrl *agentAttemptController) Start(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	var req startAttemptRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		middleware.RejectBindError(ctx, err, "Invalid request body")
		return
	}
	req.Hash = strings.TrimSpace(req.Hash)
	if !exceptionHashPattern.MatchString(req.Hash) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "The exception hash must be 16 lowercase hex characters."})
		return
	}
	tx := db.GetTx(ctx)
	project, err := transactional.ProjectRepository.FindById(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load project: %w", err))
		return
	}
	if project == nil || project.OrganizationId == nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "The project belongs to no organization."})
		return
	}

	readiness, err := buildAgentPreflight(ctx.Request.Context(), tx, projectId, req.Hash, req.ProfileId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	if readiness.ActiveAttempt != nil {
		ctx.JSON(http.StatusOK, gin.H{"attempt": readiness.ActiveAttempt, "existing": true})
		return
	}
	if !readiness.CanStart {
		for _, check := range readiness.Checks {
			if !check.Ok {
				ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": check.Hint})
				return
			}
		}
	}

	userId := middleware.GetUserId(ctx)
	origin := agent.Link{Provider: web.Provider, Kind: models.LinkKindOrigin, ExternalRef: "issue:" + req.Hash, URL: "/issues/" + req.Hash}
	result, err := agent.StartAttempt(tx, project, agent.Subject{Kind: models.SubjectKindTracewayException, Ref: req.Hash, ProjectId: projectId}, agent.StartOptions{ProfileId: req.ProfileId, Origin: &origin, RequestedBy: &userId})
	if err != nil {
		var limitErr *LimitExceededError
		if errors.As(err, &limitErr) {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": limitErr.Message})
			return
		}
		if strings.Contains(err.Error(), "not found in organization") {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "That agent profile does not belong to this organization."})
			return
		}
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to start attempt: %w", err))
		return
	}
	if result.Existing {
		ctx.JSON(http.StatusOK, gin.H{"attempt": result.Attempt, "existing": true})
		return
	}
	thread, err := web.Surface{}.Open(ctx.Request.Context(), nil, result.Attempt, &origin)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to open the web thread: %w", err))
		return
	}
	if _, err := agent.RecordLink(tx, result.Attempt.Id, thread, time.Now().UTC()); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to record the web thread: %w", err))
		return
	}
	middleware.OnCommit(ctx, agent.Wake)
	ctx.JSON(http.StatusCreated, gin.H{"attempt": result.Attempt, "existing": false})
}

type listAttemptsRequest struct {
	Status     string           `json:"status"`
	Pagination PaginationParams `json:"pagination"`
}

func (ctrl *agentAttemptController) List(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	var req listAttemptsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		middleware.RejectBindError(ctx, err, "Invalid request body")
		return
	}
	tx := db.GetTx(ctx)
	total, err := transactional.AgentAttemptRepository.CountByProject(tx, projectId, req.Status)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to count attempts: %w", err))
		return
	}
	attempts, err := transactional.AgentAttemptRepository.FindByProject(tx, projectId, req.Status, req.Pagination.PageSize, (req.Pagination.Page-1)*req.Pagination.PageSize)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list attempts: %w", err))
		return
	}
	if attempts == nil {
		attempts = []*models.AgentAttempt{}
	}
	ctx.JSON(http.StatusOK, PaginatedResponse[*models.AgentAttempt]{Data: attempts, Pagination: buildPagination(req.Pagination.Page, req.Pagination.PageSize, total)})
}

// attemptWithLinks is an attempt as the issue page's card and the run page
// show it: the row plus every external artifact.
type attemptWithLinks struct {
	*models.AgentAttempt
	Links []*models.AgentLink `json:"links"`
}

func (ctrl *agentAttemptController) BySubject(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	hash := strings.TrimSpace(ctx.Query("hash"))
	if !exceptionHashPattern.MatchString(hash) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "hash must be 16 lowercase hex characters"})
		return
	}
	tx := db.GetTx(ctx)
	attempts, err := transactional.AgentAttemptRepository.FindBySubject(tx, projectId, models.SubjectKindTracewayException, hash)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list attempts: %w", err))
		return
	}
	out := make([]attemptWithLinks, 0, len(attempts))
	for _, attempt := range attempts {
		links, err := transactional.AgentLinkRepository.FindByAttempt(tx, attempt.Id)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load links: %w", err))
			return
		}
		if links == nil {
			links = []*models.AgentLink{}
		}
		out = append(out, attemptWithLinks{AgentAttempt: attempt, Links: links})
	}
	ctx.JSON(http.StatusOK, gin.H{"attempts": out})
}

func (ctrl *agentAttemptController) Badge(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	tx := db.GetTx(ctx)
	needsInput, err := transactional.AgentAttemptRepository.CountByProject(tx, projectId, models.AttemptNeedsInput)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to count attempts: %w", err))
		return
	}
	pending, err := transactional.AgentAttemptRepository.CountByProject(tx, projectId, models.AttemptPendingApproval)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to count attempts: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"needsInput": needsInput, "pendingApproval": pending})
}

// loadProjectAttempt resolves the :id attempt and checks it belongs to the
// request's project; it writes the response on failure.
func loadProjectAttempt(ctx *gin.Context) (*models.AgentAttempt, bool) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return nil, false
	}
	id, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid attempt id"})
		return nil, false
	}
	attempt, err := transactional.AgentAttemptRepository.FindById(db.GetTx(ctx), id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load attempt: %w", err))
		return nil, false
	}
	if attempt == nil || attempt.ProjectId != projectId {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "attempt not found"})
		return nil, false
	}
	return attempt, true
}

func (ctrl *agentAttemptController) Get(ctx *gin.Context) {
	attempt, ok := loadProjectAttempt(ctx)
	if !ok {
		return
	}
	tx := db.GetTx(ctx)
	links, err := transactional.AgentLinkRepository.FindByAttempt(tx, attempt.Id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load links: %w", err))
		return
	}
	if links == nil {
		links = []*models.AgentLink{}
	}
	latestSeq, err := transactional.AgentAttemptEventRepository.LatestSeq(tx, attempt.Id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to read event sequence: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"attempt": attempt, "links": links, "latestSeq": latestSeq})
}

func (ctrl *agentAttemptController) Events(ctx *gin.Context) {
	attempt, ok := loadProjectAttempt(ctx)
	if !ok {
		return
	}
	after, _ := strconv.Atoi(ctx.DefaultQuery("after", "0"))
	events, err := transactional.AgentAttemptEventRepository.ListAfter(db.GetTx(ctx), attempt.Id, after, attemptEventPageLimit)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list events: %w", err))
		return
	}
	if events == nil {
		events = []*models.AgentAttemptEvent{}
	}
	ctx.JSON(http.StatusOK, gin.H{"events": events, "status": attempt.Status})
}

func (ctrl *agentAttemptController) Messages(ctx *gin.Context) {
	attempt, ok := loadProjectAttempt(ctx)
	if !ok {
		return
	}
	after, _ := strconv.Atoi(ctx.DefaultQuery("after", "0"))
	messages, err := transactional.AgentMessageRepository.ListAfter(db.GetTx(ctx), attempt.Id, after, "", attemptMessagePageLimit)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list messages: %w", err))
		return
	}
	if messages == nil {
		messages = []*models.AgentMessage{}
	}
	ctx.JSON(http.StatusOK, gin.H{"messages": messages})
}

type postMessageRequest struct {
	Body string `json:"body"`
}

func (ctrl *agentAttemptController) PostMessage(ctx *gin.Context) {
	attempt, ok := loadProjectAttempt(ctx)
	if !ok {
		return
	}
	var req postMessageRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		middleware.RejectBindError(ctx, err, "Invalid request body")
		return
	}
	req.Body = strings.TrimSpace(req.Body)
	if req.Body == "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Message is required."})
		return
	}
	if len(req.Body) > maxAgentMessageBytes {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Message must be 20000 characters or fewer."})
		return
	}
	if !models.AttemptIsActive(attempt.Status) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "This attempt has finished; start a new one to continue."})
		return
	}
	userId := middleware.GetUserId(ctx)
	author := agent.Identity{UserId: userId, Provider: web.Provider, Display: middleware.GetUserEmail(ctx)}
	message, err := agent.Post(db.GetTx(ctx), attempt, agent.Message{Direction: models.MessageInbound, Provider: web.Provider, Kind: models.MessageKindAnswer, Body: req.Body, Author: &author}, nil, "", time.Now().UTC())
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to post message: %w", err))
		return
	}
	middleware.OnCommit(ctx, agent.Wake)
	ctx.JSON(http.StatusCreated, gin.H{"message": message})
}

func (ctrl *agentAttemptController) Cancel(ctx *gin.Context) {
	attempt, ok := loadProjectAttempt(ctx)
	if !ok {
		return
	}
	cancelled, err := agent.Cancel(db.GetTx(ctx), attempt.Id, middleware.GetUserId(ctx), time.Now().UTC())
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to cancel attempt: %w", err))
		return
	}
	if !cancelled {
		ctx.JSON(http.StatusConflict, gin.H{"error": "This attempt has already finished."})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"cancelled": true})
}

func (ctrl *agentAttemptController) Approve(ctx *gin.Context) {
	attempt, ok := loadProjectAttempt(ctx)
	if !ok {
		return
	}
	approved, err := agent.Approve(db.GetTx(ctx), attempt.Id, middleware.GetUserId(ctx), time.Now().UTC())
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to approve attempt: %w", err))
		return
	}
	if !approved {
		ctx.JSON(http.StatusConflict, gin.H{"error": "This attempt is not waiting for approval."})
		return
	}
	middleware.OnCommit(ctx, agent.Wake)
	ctx.JSON(http.StatusOK, gin.H{"approved": true})
}

func (ctrl *agentAttemptController) Diff(ctx *gin.Context) {
	ctrl.serveBlob(ctx, agent.BlobDiff, "text/x-diff; charset=utf-8")
}

func (ctrl *agentAttemptController) Report(ctx *gin.Context) {
	ctrl.serveBlob(ctx, agent.BlobReport, "text/markdown; charset=utf-8")
}

func (ctrl *agentAttemptController) Transcript(ctx *gin.Context) {
	ctrl.serveBlob(ctx, agent.BlobTranscript, "application/x-ndjson; charset=utf-8")
}

func (ctrl *agentAttemptController) serveBlob(ctx *gin.Context, name string, contentType string) {
	attempt, ok := loadProjectAttempt(ctx)
	if !ok {
		return
	}
	if attempt.ReportKey == "" || storage.Store == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "not available"})
		return
	}
	content, err := storage.Store.Read(context.Background(), agent.BlobKey(attempt.Id, name))
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "not available"})
		return
	}
	ctx.Data(http.StatusOK, contentType, content)
}

func (ctrl *agentAttemptController) RenewRunToken(ctx *gin.Context) {
	// A sandbox-held bearer must not extend its own lifetime after claim loss.
	ctx.JSON(http.StatusForbidden, gin.H{"error": "Run tokens are renewed only by the current executor through the runner protocol."})
}

var AgentAttemptController = agentAttemptController{}

// ExecutorCI is the executor recorded on attempts a CI run reports into.
const ExecutorCI = "ci"

var pullRequestURLRe = regexp.MustCompile(`^https?://[^/]+/([^/]+)/([^/]+)/pull/(\d+)/?$`)

type reportAttemptRequest struct {
	Hash           string `json:"hash"`
	Status         string `json:"status"`
	Branch         string `json:"branch"`
	PullRequestURL string `json:"pullRequestUrl"`
	Report         string `json:"report"`
}

// ReportExternal lets a CI executor (the auto-fix GitHub Actions contract)
// report a finished run into an attempt: the attempt is created or reused
// for the issue, walked through the run states as executor "ci", and ended
// with the outcome. It manages its own transactions because the outcome
// path opens several and the SQLite main DB has one connection.
func (ctrl *agentAttemptController) ReportExternal(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	userId := middleware.GetUserId(ctx)
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 1<<20)
	var req reportAttemptRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		middleware.RejectBindError(ctx, err, "Invalid request body")
		return
	}
	req.Hash = strings.TrimSpace(req.Hash)
	if !exceptionHashPattern.MatchString(req.Hash) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "The exception hash must be 16 lowercase hex characters."})
		return
	}
	if req.Status != agents.StatusFixed && req.Status != agents.StatusAnalysis {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Status must be fixed or analysis."})
		return
	}
	var pr *agent.Link
	if req.Status == agents.StatusFixed {
		match := pullRequestURLRe.FindStringSubmatch(strings.TrimSpace(req.PullRequestURL))
		if match == nil {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "A fixed report needs the pull request URL."})
			return
		}
		pr = &agent.Link{Provider: "github", Kind: models.LinkKindPR, ExternalRef: match[1] + "/" + match[2] + "#" + match[3], URL: strings.TrimSpace(req.PullRequestURL)}
	}
	if strings.TrimSpace(req.Report) == "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "The report is required."})
		return
	}

	attempt, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.AgentAttempt, error) {
		project, err := transactional.ProjectRepository.FindById(tx, projectId)
		if err != nil {
			return nil, err
		}
		if project == nil {
			return nil, errors.New("project not found")
		}
		origin := agent.Link{Provider: ExecutorCI, Kind: models.LinkKindOrigin, ExternalRef: "ci:" + req.Hash}
		started, err := agent.StartAttempt(tx, project, agent.Subject{Kind: models.SubjectKindTracewayException, Ref: req.Hash, ProjectId: project.Id}, agent.StartOptions{Origin: &origin, RequestedBy: &userId})
		if err != nil {
			return nil, err
		}
		attempt := started.Attempt
		now := time.Now().UTC()
		if !started.Existing || attempt.Status == models.AttemptPendingApproval || attempt.Status == models.AttemptQueued {
			if attempt.Status == models.AttemptPendingApproval {
				if _, err := transactional.AgentAttemptRepository.Approve(tx, attempt.Id, userId, now); err != nil {
					return nil, err
				}
			}
			if _, err := transactional.AgentAttemptRepository.Claim(tx, attempt.Id, ExecutorCI, now.Add(agent.LeaseDuration), now); err != nil {
				return nil, err
			}
			steps := []string{models.AttemptPreparing, models.AttemptRunning}
			if req.Status == agents.StatusFixed {
				steps = append(steps, models.AttemptVerifying, models.AttemptPublishing)
			}
			for _, to := range steps {
				if err := agent.Transition(tx, attempt.Id, to, now); err != nil {
					return nil, err
				}
			}
			return transactional.AgentAttemptRepository.FindById(tx, attempt.Id)
		}
		return nil, errors.New("an attempt is already running for this issue")
	})
	if errors.Is(err, agent.ErrProjectHasNoOrganization) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "The project belongs to no organization."})
		return
	}
	if err != nil && err.Error() == "an attempt is already running for this issue" {
		ctx.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to record the CI attempt: %w", err))
		return
	}
	control := agentrunner.Local{Executor: ExecutorCI, ClaimID: attempt.ClaimedBy, InstanceURL: config.Config.PublicBaseURLOrDev()}
	if err := control.FinishWithPullRequest(ctx.Request.Context(), attempt.Id, agentrunner.Outcome{SchemaVersion: agentrunner.ProtocolVersion, Status: req.Status, Branch: strings.TrimSpace(req.Branch), Report: req.Report}, pr); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to finish the CI attempt: %w", err))
		return
	}
	finished, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.AgentAttempt, error) {
		return transactional.AgentAttemptRepository.FindById(tx, attempt.Id)
	})
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to reload the attempt: %w", err))
		return
	}
	ctx.JSON(http.StatusCreated, gin.H{"attempt": finished})
}
