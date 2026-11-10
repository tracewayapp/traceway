package retention

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/traceway"
)

const profilesSubdir = "profiles"

func profileArchiveDir(cfg *config.Cfg) (string, bool) {
	if cfg == nil {
		return "", false
	}

	enabled, err := strconv.ParseBool(strings.TrimSpace(cfg.ProfileArchiveRaw))
	if err != nil || !enabled {
		return "", false
	}

	storageType := cfg.StorageType
	if storageType == "" {
		storageType = "local"
	}
	if storageType != "local" {
		return "", false
	}

	basePath := cfg.StoragePath
	if basePath == "" {
		basePath = "./storage"
	}
	abs, err := filepath.Abs(basePath)
	if err != nil {
		return "", false
	}

	dir := filepath.Clean(filepath.Join(abs, profilesSubdir))
	if !isSafeStorageSubdir(dir) {
		return "", false
	}

	return dir, true
}

func startProfileArchiveDiskCleanup(ctx context.Context, days int) {
	if days == 0 {
		config.Logln("Profile raw archive disk cleanup disabled (PROFILE_RETENTION_DAYS=0)")
		return
	}

	dir, ok := profileArchiveDir(config.Config)
	if !ok {
		return
	}

	config.Logf("Starting profile raw archive disk cleanup worker (TTL: %d days, dir: %s)", days, dir)

	go func() {
		defer traceway.Recover()

		runDirCleanupTask(ctx, "profile_archives", dir, days)

		ticker := time.NewTicker(tickInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runDirCleanupTask(ctx, "profile_archives", dir, days)
			}
		}
	}()
}
