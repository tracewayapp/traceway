package profiling

import (
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tracewayapp/traceway/backend/v2/app/config"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
)

func ArchiveKey(projectId, profileId uuid.UUID, recordedAt time.Time) string {
	day := recordedAt.UTC().Format("20060102")
	return "profiles/" + projectId.String() + "/" + day + "/" + profileId.String() + ".pprof"
}

func RawArchiveEnabled() bool {
	if config.Config == nil {
		return false
	}
	enabled, err := strconv.ParseBool(strings.TrimSpace(config.Config.ProfileArchiveRaw))
	if err != nil {
		return false
	}
	return enabled
}

func StampArchive(projectId uuid.UUID, recordedAt time.Time, profiles []models.Profile) (string, bool) {
	if !RawArchiveEnabled() || len(profiles) == 0 {
		return "", false
	}
	key := ArchiveKey(projectId, uuid.New(), recordedAt)
	for i := range profiles {
		profiles[i].StorageKey = key
	}
	return key, true
}
