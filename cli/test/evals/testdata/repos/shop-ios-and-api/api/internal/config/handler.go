// Package config serves the app's launch config and device registration.
package config

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
)

// Payload is what the apps read at launch.
type Payload struct {
	Flags          map[string]bool `json:"flags"`
	MinimumVersion string          `json:"minimumVersion"`
	LatestVersion  string          `json:"latestVersion"`
}

// Store loads the config.
type Store interface {
	Flags(ctx context.Context, platform string) (map[string]bool, error)
	Versions(ctx context.Context, platform string) (min, latest string, err error)
}

// SQLStore implements Store on database/sql.
type SQLStore struct{ db *sql.DB }

// NewStore returns the store.
func NewStore(db *sql.DB) *SQLStore { return &SQLStore{db: db} }

// Flags returns the enabled flags of a platform.
func (s *SQLStore) Flags(ctx context.Context, platform string) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, value, rollout_pct FROM feature_flags WHERE platform = $1 AND enabled`, platform)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var key string
		var value bool
		var rollout int
		if err := rows.Scan(&key, &value, &rollout); err != nil {
			return nil, err
		}
		out[key] = value && rollout > 0
	}
	return out, rows.Err()
}

// Versions returns the minimum supported and latest app version.
func (s *SQLStore) Versions(ctx context.Context, platform string) (string, string, error) {
	var min, latest string
	err := s.db.QueryRowContext(ctx, `SELECT min_supported, latest FROM app_versions WHERE platform = $1`, platform).Scan(&min, &latest)
	return min, latest, err
}

// Handler serves GET /api/config?platform=ios.
func Handler(s Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		platform := r.URL.Query().Get("platform")
		if platform == "" {
			platform = "ios"
		}
		flags, err := s.Flags(r.Context(), platform)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		min, latest, err := s.Versions(r.Context(), platform)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Payload{Flags: flags, MinimumVersion: min, LatestVersion: latest})
	})
}

// RegisterDevice serves POST /api/devices: stores the push token.
func RegisterDevice(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			UserID   int64  `json:"userId"`
			Platform string `json:"platform"`
			Token    string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		_, err := db.ExecContext(r.Context(), `INSERT INTO push_tokens (user_id, platform, token, updated_at) VALUES ($1, $2, $3, now()) ON CONFLICT (token) DO UPDATE SET user_id = EXCLUDED.user_id, updated_at = now()`,
			in.UserID, in.Platform, in.Token)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
