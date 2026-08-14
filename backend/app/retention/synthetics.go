package retention

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/tracewayapp/traceway/backend/app/config"
	traceway "go.tracewayapp.com"
)

const syntheticsSubdir = "synthetics"

// startSyntheticsScreenshotCleanup ages out browser-check failure screenshots
// written to local disk under <STORAGE_PATH>/synthetics/. The check_results
// rows referencing them are pruned by the telemetry retention (SQLite worker
// or ClickHouse TTL); the two are deliberately not coupled, mirroring the
// session-recording split.
func startSyntheticsScreenshotCleanup(ctx context.Context, days int) {
	if days == 0 {
		return
	}
	cfg := config.Config
	storageType := cfg.StorageType
	if storageType == "" {
		storageType = "local"
	}
	if storageType != "local" {
		return
	}

	basePath := cfg.StoragePath
	if basePath == "" {
		basePath = "./storage"
	}
	abs, err := filepath.Abs(basePath)
	if err != nil {
		traceway.CaptureException(fmt.Errorf("retention: cannot resolve storage path %q: %w", basePath, err))
		return
	}
	screenshotsDir := filepath.Clean(filepath.Join(abs, syntheticsSubdir))

	if !isSafeStorageSubdir(screenshotsDir) {
		traceway.CaptureException(fmt.Errorf("retention: refusing to clean shallow synthetics dir %q — set STORAGE_PATH to a dedicated directory", screenshotsDir))
		return
	}

	config.Logf("Starting synthetics screenshot disk cleanup worker (TTL: %d days, dir: %s)", days, screenshotsDir)

	go func() {
		defer traceway.Recover()

		runDirAgeCleanup(screenshotsDir, days)

		ticker := time.NewTicker(tickInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runDirAgeCleanup(screenshotsDir, days)
			}
		}
	}()
}
