package synthetics

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/storage"
	"github.com/tracewayapp/traceway/backend/app/synthetics/browserexec"
	traceway "go.tracewayapp.com"
)

const defaultPlaywrightDir = "/opt/traceway-playwright"

// PlaywrightDir is the harness directory holding node_modules with
// @playwright/test (baked into the :browser image, or operator-provided).
func PlaywrightDir() string {
	if dir := config.Config.SyntheticsPlaywrightDir; dir != "" {
		return dir
	}
	return defaultPlaywrightDir
}

// probeBrowser executes a @playwright/test spec via the Node harness. Only
// reached in embedded mode (the executor claims browser runs solely then).
func probeBrowser(ctx context.Context, check *models.SyntheticCheck) Outcome {
	outcome := Outcome{Status: models.CheckResultDown, TlsDaysRemaining: -1}

	var cfg models.BrowserCheckConfig
	if err := json.Unmarshal(check.Config, &cfg); err != nil {
		outcome.ErrorMsg = "invalid check config: " + err.Error()
		return outcome
	}

	started := time.Now()
	result, err := browserexec.Run(ctx, browserexec.Options{
		HarnessDir: PlaywrightDir(),
		Script:     cfg.Script,
		Env:        cfg.Env,
		Timeout:    EffectiveTimeout(check),
	})
	if err != nil {
		outcome.LatencyMs = float64(time.Since(started).Microseconds()) / 1000
		outcome.ErrorMsg = "browser run failed to start: " + err.Error()
		return outcome
	}

	outcome.LatencyMs = result.DurationMs
	if outcome.LatencyMs <= 0 {
		outcome.LatencyMs = float64(time.Since(started).Microseconds()) / 1000
	}
	if result.Passed {
		outcome.Status = models.CheckResultUp
		return outcome
	}

	outcome.ErrorMsg = result.ErrorMsg
	artifactStamp := time.Now().UTC().UnixNano()
	if len(result.Screenshot) > 0 {
		key := fmt.Sprintf("synthetics/%s/%d/%d.png", check.ProjectId, check.Id, artifactStamp)
		if err := storage.Store.Write(ctx, key, result.Screenshot); err != nil {
			traceway.CaptureException(fmt.Errorf("failed to store check failure screenshot (check=%d): %w", check.Id, err))
		} else {
			outcome.ScreenshotKey = key
		}
	}
	if output := strings.TrimSpace(result.Output); output != "" {
		key := fmt.Sprintf("synthetics/%s/%d/%d.log", check.ProjectId, check.Id, artifactStamp)
		if err := storage.Store.Write(ctx, key, []byte(output)); err != nil {
			traceway.CaptureException(fmt.Errorf("failed to store check failure output (check=%d): %w", check.Id, err))
		} else {
			outcome.OutputKey = key
		}
	}
	return outcome
}
