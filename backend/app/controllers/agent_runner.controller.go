package controllers

import (
	"database/sql"
	"errors"
	"io"
	"net/http"
	"slices"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/agent"
	"github.com/tracewayapp/traceway/backend/app/agentrunner"
	"github.com/tracewayapp/traceway/backend/app/agentrunner/agents"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	traceway "go.tracewayapp.com"
)

// agentRunnerController is the server side of the runner protocol: what a
// traceway-agent-runner talks to. Every route authenticates with the shared
// AGENT_RUNNER_SECRET and acts as the executor "runner:<name>", through the
// same agentrunner.Local the embedded executor uses, so the two paths
// cannot drift. None of it runs under Transactional: the poll holds the
// request for up to 25s and the handlers open their own short transactions.
type agentRunnerController struct{}

const (
	// agentRunnerRequestsPerMinute bounds one runner host: sixteen workers
	// flushing events every two seconds stay well under it.
	agentRunnerRequestsPerMinute = 1200
	agentRunnerPollRecheck       = 2 * time.Second
	agentRunnerMaxClaims         = 8
	agentRunnerEventsLimit       = 4 << 20
	agentRunnerBlobLimit         = 32 << 20
	agentRunnerReportLimit       = 1 << 20
)

// AgentRunnerPollWindow is how long a poll waits for a claimable attempt
// before answering empty; tests shorten it.
var AgentRunnerPollWindow = 25 * time.Second

var agentBlobNames = []string{agent.BlobTranscript, agent.BlobDiff, agent.BlobReport}

// RegisterAgentRunnerRoutes mounts the runner protocol under /agent-runners.
func RegisterAgentRunnerRoutes(router *gin.RouterGroup) {
	group := router.Group("/agent-runners", middleware.RateLimitPerIP(agentRunnerRequestsPerMinute, time.Minute), func(c *gin.Context) { middleware.UseAgentRunnerAuth(c) })
	group.POST("/poll", AgentRunnerController.Poll)
	group.GET("/attempts/:id/context", AgentRunnerController.Context)
	group.POST("/attempts/:id/git-credential", AgentRunnerController.GitCredential)
	group.POST("/attempts/:id/run-token", AgentRunnerController.RunToken)
	group.POST("/attempts/:id/transition", AgentRunnerController.Transition)
	group.POST("/attempts/:id/events", AgentRunnerController.Events)
	group.POST("/attempts/:id/findings", AgentRunnerController.Finding)
	group.PUT("/attempts/:id/blobs/:name", AgentRunnerController.Blob)
	group.POST("/attempts/:id/result", AgentRunnerController.Result)
}

func runnerControl(ctx *gin.Context) (agentrunner.Local, bool) {
	runner, ok := middleware.GetAgentRunner(ctx)
	if !ok {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("UseAgentRunnerAuth middleware must be applied: %w", errMissingRunner))
		return agentrunner.Local{}, false
	}
	return agentrunner.Local{Executor: agentrunner.RunnerExecutorPrefix + runner.Name, ClaimID: ctx.GetHeader("X-Traceway-Claim"), InstanceURL: config.Config.PublicBaseURLOrDev()}, true
}

type agentRunnerPollRequest struct {
	MaxAttempts int `json:"maxAttempts"`
}

// Poll long-polls for claimable attempts. Only remote mode hands attempts
// to the fleet: in embedded mode the in-process workers own the queue.
func (ctrl *agentRunnerController) Poll(ctx *gin.Context) {
	control, ok := runnerControl(ctx)
	if !ok {
		return
	}
	mode, err := agentrunner.Mode()
	if err != nil || mode != agentrunner.ModeRemote {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "This instance is not in remote agent mode (AGENT_MODE=remote)."})
		return
	}
	var req agentRunnerPollRequest
	if err := ctx.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	limit := min(max(req.MaxAttempts, 1), agentRunnerMaxClaims)

	deadline := time.NewTimer(AgentRunnerPollWindow)
	defer deadline.Stop()
	for {
		claimed, err := control.Claim(ctx.Request.Context(), limit)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to claim attempts for %s: %w", control.Executor, err))
			return
		}
		if len(claimed) > 0 {
			ctx.JSON(http.StatusOK, gin.H{"attempts": claimed})
			return
		}
		select {
		case <-ctx.Request.Context().Done():
			return
		case <-deadline.C:
			ctx.JSON(http.StatusOK, gin.H{"attempts": []*models.AgentAttempt{}})
			return
		case <-agent.WakeChannel():
		case <-time.After(agentRunnerPollRecheck):
		}
	}
}

// claimedAttempt resolves the route's attempt and checks it is still this
// runner's to act on; anything else is a lost claim the runner stops on.
func claimedAttempt(ctx *gin.Context, control agentrunner.Local) (*models.AgentAttempt, bool) {
	id, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "attempt not found"})
		return nil, false
	}
	attempt, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.AgentAttempt, error) {
		return transactional.AgentAttemptRepository.FindById(tx, id)
	})
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load attempt: %w", err))
		return nil, false
	}
	if attempt == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "attempt not found"})
		return nil, false
	}
	if attempt.Executor != control.Executor || control.ClaimID == "" || attempt.ClaimedBy != control.ClaimID || !slices.Contains(models.AttemptLeasedStatuses, attempt.Status) || attempt.LeaseExpiresAt == nil || !attempt.LeaseExpiresAt.After(time.Now()) {
		ctx.JSON(http.StatusConflict, gin.H{"error": "claim lost"})
		return nil, false
	}
	return attempt, true
}

func (ctrl *agentRunnerController) Context(ctx *gin.Context) {
	control, ok := runnerControl(ctx)
	if !ok {
		return
	}
	attempt, ok := claimedAttempt(ctx, control)
	if !ok {
		return
	}
	inputs, err := control.Inputs(ctx.Request.Context(), attempt)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to build inputs for %s: %w", attempt.Id, err))
		return
	}
	ctx.JSON(http.StatusOK, inputs)
}

func (ctrl *agentRunnerController) GitCredential(ctx *gin.Context) {
	control, ok := runnerControl(ctx)
	if !ok {
		return
	}
	attempt, ok := claimedAttempt(ctx, control)
	if !ok {
		return
	}
	cred, err := control.GitCredential(ctx.Request.Context(), attempt)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to issue a git credential for %s: %w", attempt.Id, err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"username": cred.Username, "password": cred.Password, "expiresAt": cred.ExpiresAt})
}

type agentRunnerTransitionRequest struct {
	To string `json:"to"`
}

func (ctrl *agentRunnerController) Transition(ctx *gin.Context) {
	control, ok := runnerControl(ctx)
	if !ok {
		return
	}
	attempt, ok := claimedAttempt(ctx, control)
	if !ok {
		return
	}
	var req agentRunnerTransitionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil || req.To == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	err := control.Transition(ctx.Request.Context(), attempt.Id, req.To)
	if errors.Is(err, agentrunner.ErrClaimLost) {
		ctx.JSON(http.StatusConflict, gin.H{"error": "claim lost"})
		return
	}
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to transition %s: %w", attempt.Id, err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": req.To})
}

type agentRunnerEventsRequest struct {
	SchemaVersion int            `json:"schemaVersion"`
	Events        []agents.Event `json:"events"`
}

// Events appends a batch and renews the lease; an empty batch is the
// heartbeat.
func (ctrl *agentRunnerController) Events(ctx *gin.Context) {
	control, ok := runnerControl(ctx)
	if !ok {
		return
	}
	attempt, ok := claimedAttempt(ctx, control)
	if !ok {
		return
	}
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, agentRunnerEventsLimit)
	var req agentRunnerEventsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		middleware.RejectBindError(ctx, err, "Invalid request body")
		return
	}
	err := control.Events(ctx.Request.Context(), attempt.Id, req.Events)
	if errors.Is(err, agentrunner.ErrClaimLost) {
		ctx.JSON(http.StatusConflict, gin.H{"error": "claim lost"})
		return
	}
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to append events for %s: %w", attempt.Id, err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"held": true, "accepted": len(req.Events)})
}

type agentRunnerFindingRequest struct {
	Body string `json:"body"`
}

func (ctrl *agentRunnerController) Finding(ctx *gin.Context) {
	control, ok := runnerControl(ctx)
	if !ok {
		return
	}
	attempt, ok := claimedAttempt(ctx, control)
	if !ok {
		return
	}
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, agentRunnerReportLimit)
	var req agentRunnerFindingRequest
	if err := ctx.ShouldBindJSON(&req); err != nil || req.Body == "" {
		middleware.RejectBindError(ctx, err, "Invalid request body")
		return
	}
	if err := control.Finding(ctx.Request.Context(), attempt.Id, req.Body); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to post a finding on %s: %w", attempt.Id, err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"posted": true})
}

// Blob stores the transcript, diff or report of a running attempt; append
// keeps earlier sessions of a resumed attempt.
func (ctrl *agentRunnerController) Blob(ctx *gin.Context) {
	control, ok := runnerControl(ctx)
	if !ok {
		return
	}
	attempt, ok := claimedAttempt(ctx, control)
	if !ok {
		return
	}
	name := ctx.Param("name")
	if !slices.Contains(agentBlobNames, name) {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "unknown blob"})
		return
	}
	content, err := io.ReadAll(http.MaxBytesReader(ctx.Writer, ctx.Request.Body, agentRunnerBlobLimit))
	if err != nil {
		middleware.RejectBindError(ctx, err, "Invalid body")
		return
	}
	if err := control.Blob(ctx.Request.Context(), attempt.Id, name, content, ctx.Query("append") == "1"); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to store %s for %s: %w", name, attempt.Id, err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"stored": len(content)})
}

// Result applies the terminal outcome. A lost claim answers 200 with
// applied=false: the runner is done either way, and a retry after a
// timeout must not fail.
func (ctrl *agentRunnerController) Result(ctx *gin.Context) {
	control, ok := runnerControl(ctx)
	if !ok {
		return
	}
	id, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "attempt not found"})
		return
	}
	attempt, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.AgentAttempt, error) {
		return transactional.AgentAttemptRepository.FindById(tx, id)
	})
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load attempt: %w", err))
		return
	}
	if attempt == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "attempt not found"})
		return
	}
	if attempt.Executor != control.Executor || control.ClaimID == "" || attempt.ClaimedBy != control.ClaimID || !slices.Contains(models.AttemptLeasedStatuses, attempt.Status) || attempt.LeaseExpiresAt == nil || !attempt.LeaseExpiresAt.After(time.Now()) {
		ctx.JSON(http.StatusOK, gin.H{"applied": false})
		return
	}
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, agentRunnerReportLimit)
	var outcome agentrunner.Outcome
	if err := ctx.ShouldBindJSON(&outcome); err != nil {
		middleware.RejectBindError(ctx, err, "Invalid request body")
		return
	}
	if !slices.Contains([]string{agents.StatusFixed, agents.StatusAnalysis, agents.StatusQuestion, agents.StatusError}, outcome.Status) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "unknown outcome status"})
		return
	}
	err = control.Finish(ctx.Request.Context(), attempt.Id, outcome)
	if errors.Is(err, agentrunner.ErrClaimLost) {
		ctx.JSON(http.StatusOK, gin.H{"applied": false})
		return
	}
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to apply the outcome of %s: %w", attempt.Id, err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"applied": true})
}

var AgentRunnerController = agentRunnerController{}

func (ctrl *agentRunnerController) RunToken(ctx *gin.Context) {
	control, ok := runnerControl(ctx)
	if !ok {
		return
	}
	attempt, ok := claimedAttempt(ctx, control)
	if !ok {
		return
	}
	token, err := control.RenewRunToken(ctx.Request.Context(), attempt.Id)
	if errors.Is(err, agentrunner.ErrClaimLost) {
		ctx.JSON(http.StatusConflict, gin.H{"error": "claim lost"})
		return
	}
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	ctx.JSON(http.StatusOK, token)
}
