package controllers

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/storage"
	"github.com/tracewayapp/traceway/backend/app/synthetics"

	traceway "go.tracewayapp.com"
)

type syntheticRunnerController struct{}

const (
	runnerPollWindow      = 25 * time.Second
	runnerPollRecheck     = 2 * time.Second
	runnerMaxJobs         = 5
	runnerResultBodyLimit = 6 << 20 // screenshotBase64 dominates; gin has no default cap
)

type runnerPollRequest struct {
	MaxJobs int `json:"maxJobs"`
}

type runnerJob struct {
	RunId          int               `json:"runId"`
	CheckId        int               `json:"checkId"`
	TimeoutSeconds int               `json:"timeoutSeconds"`
	Script         string            `json:"script"`
	Env            map[string]string `json:"env"`
}

// Poll long-polls for claimable browser runs across the whole instance
// (runners are the operator's fleet, not tenant infrastructure).
// Deliberately NOT under middleware.Transactional: the request may hold the
// connection for 25s, and the SQLite main DB has a single connection — each
// claim attempt runs its own short transaction, and the wait happens on the
// browser-run wake channel, never against the DB.
func (ctrl *syntheticRunnerController) Poll(ctx *gin.Context) {
	runner, ok := middleware.GetRunner(ctx)
	if !ok {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("UseRunnerAuth middleware must be applied: %w", errMissingRunner))
		return
	}
	// Only remote mode hands browser runs to the fleet. In embedded mode the
	// in-process executor owns them; serving a stray runner here would split
	// execution across environments the operator did not choose.
	if synthetics.BrowserMode() != synthetics.BrowserModeRemote {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "This instance is not in remote browser mode (SYNTHETICS_BROWSER_MODE=remote)."})
		return
	}
	var req runnerPollRequest
	if err := ctx.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	maxJobs := req.MaxJobs
	if maxJobs < 1 {
		maxJobs = 1
	}
	if maxJobs > runnerMaxJobs {
		maxJobs = runnerMaxJobs
	}

	deadline := time.NewTimer(runnerPollWindow)
	defer deadline.Stop()
	for {
		jobs, err := claimRunnerJobs(runner, maxJobs)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to claim browser runs: %w", err))
			return
		}
		if len(jobs) > 0 {
			ctx.JSON(http.StatusOK, gin.H{"jobs": jobs})
			return
		}

		recheck := time.NewTimer(runnerPollRecheck)
		select {
		case <-ctx.Request.Context().Done():
			recheck.Stop()
			return
		case <-deadline.C:
			recheck.Stop()
			ctx.JSON(http.StatusOK, gin.H{"jobs": []runnerJob{}})
			return
		case <-synthetics.BrowserWakeChannel():
			recheck.Stop()
		case <-recheck.C:
		}
	}
}

func claimRunnerJobs(runner *models.SyntheticRunner, maxJobs int) ([]runnerJob, error) {
	claimedBy := runnerClaimedBy(runner.Name)
	now := time.Now().UTC()
	return db.ExecuteTransaction(func(tx *sql.Tx) ([]runnerJob, error) {
		due, err := transactional.CheckRunRepository.FindClaimableBrowser(tx, now, maxJobs)
		if err != nil {
			return nil, err
		}
		jobs := make([]runnerJob, 0, len(due))
		for _, run := range due {
			won, err := transactional.CheckRunRepository.Claim(tx, run.Id, claimedBy, now)
			if err != nil {
				return nil, err
			}
			if !won {
				continue
			}
			check, err := transactional.SyntheticCheckRepository.FindById(tx, run.CheckId)
			if err != nil {
				return nil, err
			}
			if check == nil {
				continue
			}
			var cfg models.BrowserCheckConfig
			if err := json.Unmarshal(check.Config, &cfg); err != nil {
				continue
			}
			jobs = append(jobs, runnerJob{
				RunId:          run.Id,
				CheckId:        check.Id,
				TimeoutSeconds: check.TimeoutSeconds,
				Script:         cfg.Script,
				Env:            cfg.Env,
			})
		}
		return jobs, nil
	})
}

// runnerClaimedBy keys claims by runner name: names are unique, stable
// across restarts, and independent of whether the liveness row insert
// succeeded.
func runnerClaimedBy(runnerName string) string {
	return "runner:" + runnerName
}

type runnerResultRequest struct {
	Status           string  `json:"status"`
	LatencyMs        float64 `json:"latencyMs"`
	ErrorMessage     string  `json:"errorMessage"`
	ScreenshotBase64 string  `json:"screenshotBase64"`
	// OutputTail is the Playwright stdout/stderr tail of a failed run,
	// stored next to the screenshot for debugging.
	OutputTail string `json:"outputTail"`
}

const runnerOutputTailLimit = 16 << 10

// Result finalizes a run a remote runner executed. Idempotent: a missing or
// re-owned run row means the run was already finalized (or reclaimed), and
// the outcome is discarded with a 200. Not under middleware.Transactional:
// ProcessOutcome manages its own transactions.
func (ctrl *syntheticRunnerController) Result(ctx *gin.Context) {
	runner, ok := middleware.GetRunner(ctx)
	if !ok {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("UseRunnerAuth middleware must be applied: %w", errMissingRunner))
		return
	}
	runId, err := strconv.Atoi(ctx.Param("runId"))
	if err != nil || runId < 1 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid run id"})
		return
	}
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, runnerResultBodyLimit)
	var req runnerResultRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if req.Status != models.CheckResultUp && req.Status != models.CheckResultDown {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Status must be up or down."})
		return
	}

	run, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.CheckRun, error) {
		return transactional.CheckRunRepository.FindById(tx, runId)
	})
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load run: %w", err))
		return
	}
	claimedBy := runnerClaimedBy(runner.Name)
	if run == nil || run.ClaimedBy != claimedBy {
		ctx.JSON(http.StatusOK, gin.H{"accepted": false})
		return
	}

	now := time.Now().UTC()
	outcome := synthetics.Outcome{
		Status:           req.Status,
		LatencyMs:        req.LatencyMs,
		ErrorMsg:         req.ErrorMessage,
		TlsDaysRemaining: -1,
		ExecutedBy:       runner.Name,
		ExecutedAt:       now,
	}
	if req.ScreenshotBase64 != "" && req.Status == models.CheckResultDown {
		if data, decodeErr := base64.StdEncoding.DecodeString(req.ScreenshotBase64); decodeErr == nil && len(data) > 0 {
			key := "synthetics/" + run.ProjectId.String() + "/" + strconv.Itoa(run.CheckId) + "/" + strconv.FormatInt(now.UnixNano(), 10) + ".png"
			if writeErr := storage.Store.Write(ctx, key, data); writeErr == nil {
				outcome.ScreenshotKey = key
			} else {
				traceway.CaptureException(traceway.NewStackTraceErrorf("failed to store runner screenshot (run=%d): %w", runId, writeErr))
			}
		}
	}
	if output := strings.TrimSpace(req.OutputTail); output != "" && req.Status == models.CheckResultDown {
		if len(output) > runnerOutputTailLimit {
			output = output[len(output)-runnerOutputTailLimit:]
		}
		key := "synthetics/" + run.ProjectId.String() + "/" + strconv.Itoa(run.CheckId) + "/" + strconv.FormatInt(now.UnixNano(), 10) + ".log"
		if writeErr := storage.Store.Write(ctx, key, []byte(output)); writeErr == nil {
			outcome.OutputKey = key
		} else {
			traceway.CaptureException(traceway.NewStackTraceErrorf("failed to store runner output (run=%d): %w", runId, writeErr))
		}
	}

	synthetics.ProcessOutcome(ctx, run, claimedBy, outcome)
	ctx.JSON(http.StatusOK, gin.H{"accepted": true})
}

var errMissingRunner = &missingRunnerError{}

type missingRunnerError struct{}

func (*missingRunnerError) Error() string { return "no runner in context" }

var SyntheticRunnerController = syntheticRunnerController{}
