package retention

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/traceway"
)

const recordingsSubdir = "recordings"

func startRecordingDiskCleanup(ctx context.Context, days int) {
	if days == 0 {
		config.Logln("Session recording disk cleanup disabled (SESSION_RECORDING_RETENTION_DAYS=0)")
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
	recordingsDir := filepath.Clean(filepath.Join(abs, recordingsSubdir))

	if !isSafeStorageSubdir(recordingsDir) {
		traceway.CaptureException(fmt.Errorf("retention: refusing to clean shallow recordings dir %q — set STORAGE_PATH to a dedicated directory", recordingsDir))
		return
	}

	config.Logf("Starting session recording disk cleanup worker (TTL: %d days, dir: %s)", days, recordingsDir)

	go func() {
		defer traceway.Recover()

		runDirCleanupTask(ctx, "recordings", recordingsDir, days)

		ticker := time.NewTicker(tickInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runDirCleanupTask(ctx, "recordings", recordingsDir, days)
			}
		}
	}()
}

func isSafeStorageSubdir(path string) bool {
	cleaned := filepath.Clean(path)
	if cleaned == "/" || cleaned == "." {
		return false
	}
	rel := strings.TrimPrefix(cleaned, string(filepath.Separator))
	parts := strings.Split(rel, string(filepath.Separator))
	return len(parts) >= 2
}

func runDirCleanupTask(ctx context.Context, name, dir string, days int) {
	traceway.RunTask(ctx, "retention."+name, func(context.Context) {
		runDirAgeCleanup(dir, days)
	})
}

func runDirAgeCleanup(dir string, days int) {
	info, err := os.Stat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err != nil {
		traceway.CaptureException(fmt.Errorf("retention: stat %s failed: %w", dir, err))
		return
	}
	if !info.IsDir() {
		traceway.CaptureException(fmt.Errorf("retention: cleanup path %s is not a directory", dir))
		return
	}

	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)

	var subDirs []string
	var removed int
	walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			traceway.CaptureException(fmt.Errorf("retention: walk error at %s: %w", path, err))
			return nil
		}
		if d.IsDir() {
			if path != dir {
				subDirs = append(subDirs, path)
			}
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return nil
		}
		if fi.ModTime().Before(cutoff) {
			if rmErr := os.Remove(path); rmErr != nil {
				traceway.CaptureException(fmt.Errorf("retention: failed to remove %s: %w", path, rmErr))
				return nil
			}
			removed++
		}
		return nil
	})
	if walkErr != nil {
		traceway.CaptureException(fmt.Errorf("retention: walk %s failed: %w", dir, walkErr))
	}

	for i := len(subDirs) - 1; i >= 0; i-- {
		_ = os.Remove(subDirs[i])
	}

	if removed > 0 {
		config.Logf("Disk cleanup removed %d file(s) older than %d day(s) under %s", removed, days, dir)
	}
}
