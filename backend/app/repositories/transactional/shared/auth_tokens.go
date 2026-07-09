package shared

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

// AuthTimeLayout is the canonical storage format for auth-token timestamps.
// It is shared by every transactional backend so hashes and times written by
// one backend stay readable after switching to another.
const AuthTimeLayout = "2006-01-02T15:04:05.000000000Z07:00"

func HashAuthToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

func FormatAuthTime(t time.Time) string {
	return t.UTC().Format(AuthTimeLayout)
}

func ParseAuthTime(s string) (time.Time, error) {
	layouts := []string{AuthTimeLayout, time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05"}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, s); err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("auth: cannot parse time %q", s)
}

func ParseNullableAuthTime(ns sql.NullString) (*time.Time, error) {
	if !ns.Valid || ns.String == "" {
		return nil, nil
	}
	t, err := ParseAuthTime(ns.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
