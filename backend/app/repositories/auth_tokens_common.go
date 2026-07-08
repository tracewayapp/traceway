package repositories

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

const authTimeLayout = "2006-01-02T15:04:05.000000000Z07:00"

func hashAuthToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

func formatAuthTime(t time.Time) string {
	return t.UTC().Format(authTimeLayout)
}

func parseAuthTime(s string) (time.Time, error) {
	layouts := []string{authTimeLayout, time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05"}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, s); err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("auth: cannot parse time %q", s)
}

func parseNullableAuthTime(ns sql.NullString) (*time.Time, error) {
	if !ns.Valid || ns.String == "" {
		return nil, nil
	}
	t, err := parseAuthTime(ns.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
