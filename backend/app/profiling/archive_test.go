package profiling

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/models"
)

func TestArchiveKey(t *testing.T) {
	projectId := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	profileId := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	recordedAt := time.Date(2026, 6, 16, 10, 30, 0, 0, time.UTC)

	got := ArchiveKey(projectId, profileId, recordedAt)
	want := "profiles/11111111-1111-1111-1111-111111111111/20260616/22222222-2222-2222-2222-222222222222.pprof"

	if got != want {
		t.Errorf("ArchiveKey() = %q, want %q", got, want)
	}
}

func TestArchiveKeyUsesUTCDate(t *testing.T) {
	projectId := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	profileId := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	loc := time.FixedZone("UTC+2", 2*60*60)
	recordedAt := time.Date(2026, 6, 17, 0, 30, 0, 0, loc)

	got := ArchiveKey(projectId, profileId, recordedAt)
	want := "profiles/11111111-1111-1111-1111-111111111111/20260616/22222222-2222-2222-2222-222222222222.pprof"

	if got != want {
		t.Errorf("ArchiveKey() = %q, want %q (date must be UTC-normalized)", got, want)
	}
}

func TestRawArchiveEnabled(t *testing.T) {
	prev := config.Config
	t.Cleanup(func() { config.Config = prev })
	config.Config = &config.Cfg{}

	cases := []struct {
		value string
		want  bool
	}{
		{"true", true},
		{"1", true},
		{"TRUE", true},
		{"", false},
		{"false", false},
		{"0", false},
		{"nonsense", false},
	}

	for _, c := range cases {
		config.Config.ProfileArchiveRaw = c.value
		if got := RawArchiveEnabled(); got != c.want {
			t.Errorf("RawArchiveEnabled() with %q = %v, want %v", c.value, got, c.want)
		}
	}
}

func TestStampArchive(t *testing.T) {
	prev := config.Config
	t.Cleanup(func() { config.Config = prev })
	config.Config = &config.Cfg{}

	projectId := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	recordedAt := time.Date(2026, 6, 16, 10, 30, 0, 0, time.UTC)

	t.Run("enabled stamps all rows with one key", func(t *testing.T) {
		config.Config.ProfileArchiveRaw = "true"
		profiles := []models.Profile{
			{ProfileType: "alloc_space"},
			{ProfileType: "inuse_space"},
		}

		key, ok := StampArchive(projectId, recordedAt, profiles)
		if !ok {
			t.Fatal("StampArchive ok = false, want true when enabled")
		}
		wantPrefix := "profiles/11111111-1111-1111-1111-111111111111/20260616/"
		if len(key) < len(wantPrefix) || key[:len(wantPrefix)] != wantPrefix {
			t.Errorf("key = %q, want prefix %q", key, wantPrefix)
		}
		if key[len(key)-6:] != ".pprof" {
			t.Errorf("key = %q, want .pprof suffix", key)
		}
		for i, p := range profiles {
			if p.StorageKey != key {
				t.Errorf("profiles[%d].StorageKey = %q, want %q (all rows share one blob)", i, p.StorageKey, key)
			}
		}
	})

	t.Run("disabled is a no-op", func(t *testing.T) {
		config.Config.ProfileArchiveRaw = "false"
		profiles := []models.Profile{{ProfileType: "cpu"}}

		key, ok := StampArchive(projectId, recordedAt, profiles)
		if ok || key != "" {
			t.Errorf("StampArchive = (%q, %v), want (\"\", false) when disabled", key, ok)
		}
		if profiles[0].StorageKey != "" {
			t.Errorf("StorageKey = %q, want empty when disabled", profiles[0].StorageKey)
		}
	})

	t.Run("no profiles is a no-op", func(t *testing.T) {
		config.Config.ProfileArchiveRaw = "true"
		key, ok := StampArchive(projectId, recordedAt, nil)
		if ok || key != "" {
			t.Errorf("StampArchive = (%q, %v), want (\"\", false) for empty profiles", key, ok)
		}
	})
}
