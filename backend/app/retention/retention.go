package retention

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/config"
)

const (
	defaultRetentionDays = 30
	tickInterval         = time.Hour
)

func Start(ctx context.Context) {
	cfg := config.Config

	startSQLiteRetention(ctx, parseRetentionDays(cfg.SQLiteRetentionDays))
	startRecordingDiskCleanup(ctx, parseRetentionDays(cfg.SessionRecordingRetentionDays))
	startProfileArchiveDiskCleanup(ctx, parseRetentionDays(cfg.ProfileRetentionDays))
	startOAuthSessionsPrune(ctx)
	startAuthTokensPrune(ctx)
	startOutboxPrune(ctx)
}

func parseRetentionDays(value string) int {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return defaultRetentionDays
	}
	days, err := strconv.Atoi(trimmed)
	if err != nil || days < 0 {
		return defaultRetentionDays
	}
	return days
}
