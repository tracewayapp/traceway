package retention

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/v2/app/config"
)

func TestProfileArchiveDir(t *testing.T) {
	prev := config.Config
	t.Cleanup(func() { config.Config = prev })
	config.Config = &config.Cfg{}

	cases := []struct {
		name        string
		archiveRaw  string
		storageType string
		storagePath string
		wantOK      bool
		wantSuffix  string
	}{
		{"disabled", "", "local", "/var/lib/traceway", false, ""},
		{"enabled but s3", "true", "s3", "/var/lib/traceway", false, ""},
		{"enabled local", "true", "local", "/var/lib/traceway", true, filepath.Join("traceway", "profiles")},
		{"enabled default storage type", "true", "", "/var/lib/traceway", true, filepath.Join("traceway", "profiles")},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			config.Config.ProfileArchiveRaw = c.archiveRaw
			config.Config.StorageType = c.storageType
			config.Config.StoragePath = c.storagePath

			dir, ok := profileArchiveDir(config.Config)
			if ok != c.wantOK {
				t.Fatalf("profileArchiveDir ok = %v, want %v", ok, c.wantOK)
			}
			if ok {
				if filepath.Base(dir) != profilesSubdir {
					t.Errorf("profileArchiveDir dir = %q, want it to end in %q", dir, profilesSubdir)
				}
				if c.wantSuffix != "" && filepath.Base(filepath.Dir(dir))+string(filepath.Separator)+filepath.Base(dir) != c.wantSuffix {
					t.Errorf("profileArchiveDir dir = %q, want suffix %q", dir, c.wantSuffix)
				}
			}
		})
	}
}

func TestRunProfileArchiveDiskCleanup_DeletesOldKeepsFresh(t *testing.T) {
	dir := t.TempDir()

	oldFile := filepath.Join(dir, "project-1", "20260101", "old.pprof")
	freshFile := filepath.Join(dir, "project-1", "20260616", "fresh.pprof")

	mustWriteFile(t, oldFile, "old")
	mustWriteFile(t, freshFile, "fresh")

	mustChtime(t, oldFile, time.Now().Add(-40*24*time.Hour))

	runDirAgeCleanup(context.Background(), dir, 30)

	assertNotExists(t, oldFile)
	assertExists(t, freshFile)
}
