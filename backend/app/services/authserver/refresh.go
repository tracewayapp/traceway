package authserver

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories"
)

var ErrInvalidGrant = errors.New("invalid_grant")

// rotationCache remembers, for reuseGrace, the token set a rotation produced,
// keyed by the hash of the refresh token that was consumed. It is what makes
// the grace window actually work: a client that lost the rotation response (or
// failed to persist it) re-presents the consumed token and gets the same
// rotated set back instead of an invalid_grant it can never recover from. Only
// hashes are used as keys; the raw presented token is never stored. Being
// in-memory, a restart or another instance falls back to invalid_grant, which
// is the pre-cache behavior.
var (
	rotationCacheMu sync.Mutex
	rotationCache   = map[string]rotationCacheEntry{}
)

type rotationCacheEntry struct {
	ts        *models.TokenSetResponse
	expiresAt time.Time
}

func rotationCacheKey(presented string) string {
	sum := sha256.Sum256([]byte(presented))
	return hex.EncodeToString(sum[:])
}

func storeRotation(presented string, ts *models.TokenSetResponse) {
	now := time.Now()
	rotationCacheMu.Lock()
	defer rotationCacheMu.Unlock()
	for k, e := range rotationCache {
		if now.After(e.expiresAt) {
			delete(rotationCache, k)
		}
	}
	rotationCache[rotationCacheKey(presented)] = rotationCacheEntry{ts: ts, expiresAt: now.Add(reuseGrace)}
}

func cachedRotation(presented string) *models.TokenSetResponse {
	rotationCacheMu.Lock()
	defer rotationCacheMu.Unlock()
	e, ok := rotationCache[rotationCacheKey(presented)]
	if !ok || time.Now().After(e.expiresAt) {
		return nil
	}
	return e.ts
}

// errReplay and errBenignReuse classify a rows-affected==0 outcome from the
// atomic mark-used inside the rotation transaction. They never leave this file:
// errBenignReuse replays the cached rotated token set (falling back to
// ErrInvalidGrant when the cache has no entry), while errReplay maps to
// ErrInvalidGrant and additionally triggers a family revoke, committed in its
// own transaction because the rotation transaction rolls back when the
// closure returns an error.
var (
	errReplay      = errors.New("refresh token replayed")
	errBenignReuse = errors.New("refresh token concurrently reused within grace")
)

// RotateRefresh owns its own transactions. Reuse detection must revoke the
// token family even though the endpoint returns 400, so the family revoke runs
// in its own committed transaction rather than the request-scoped one (which
// the Transactional middleware would roll back on a non-2xx response).
//
// Rotation is atomic: the presented token is flipped to used inside the same
// transaction that mints its replacement, via a conditional UPDATE whose
// rows-affected result is the concurrency guard. If the token was already
// consumed, we distinguish a benign concurrent retry (within reuseGrace, the
// winner's rotated token set is replayed from the rotation cache so the loser
// recovers) from a genuine replay of a long-since-rotated token (family
// revoked).
func RotateRefresh(presented string) (*models.TokenSetResponse, error) {
	rt, err := repositories.RefreshTokenRepository.FindByToken(db.DB, presented)
	if err != nil {
		return nil, err
	}
	if rt == nil {
		return nil, ErrInvalidGrant
	}

	if rt.Revoked {
		revokeFamily(rt.FamilyId)
		return nil, ErrInvalidGrant
	}

	if time.Now().After(rt.ExpiresAt) {
		return nil, ErrInvalidGrant
	}

	ts, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.TokenSetResponse, error) {
		affected, err := repositories.RefreshTokenRepository.MarkUsedIfUnused(tx, presented, time.Now())
		if err != nil {
			return nil, err
		}
		if affected == 0 {
			// Someone else consumed or revoked this token between our read and
			// this write. Re-read inside the transaction to classify it, then
			// signal via a sentinel so the family revoke happens in a committed
			// transaction rather than this one (which rolls back on error).
			current, err := repositories.RefreshTokenRepository.FindByToken(tx, presented)
			if err != nil {
				return nil, err
			}
			if current != nil && !current.Revoked && current.UsedAt != nil && time.Since(*current.UsedAt) <= reuseGrace {
				return nil, errBenignReuse
			}
			return nil, errReplay
		}

		user, err := repositories.UserRepository.FindById(tx, rt.UserId)
		if err != nil {
			return nil, err
		}
		if user == nil {
			return nil, ErrInvalidGrant
		}
		return issueTokenSetWithFamily(tx, rt.UserId, user.Email, rt.ClientId, rt.FamilyId)
	})
	if err != nil {
		if errors.Is(err, errReplay) {
			revokeFamily(rt.FamilyId)
			return nil, ErrInvalidGrant
		}
		if errors.Is(err, errBenignReuse) {
			if cached := cachedRotation(presented); cached != nil {
				return cached, nil
			}
			return nil, ErrInvalidGrant
		}
		return nil, err
	}
	storeRotation(presented, ts)
	return ts, nil
}

// RevokeByToken revokes the entire family the presented refresh token belongs
// to. It is idempotent and never reveals whether the token existed, so it is
// safe to expose on an unauthenticated logout endpoint. Like RotateRefresh it
// self-manages its transaction because the endpoint may return a non-2xx that
// the Transactional middleware would otherwise roll back.
func RevokeByToken(presented string) error {
	rt, err := repositories.RefreshTokenRepository.FindByToken(db.DB, presented)
	if err != nil {
		return err
	}
	if rt == nil {
		return nil
	}
	_, err = db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		return struct{}{}, repositories.RefreshTokenRepository.RevokeFamily(tx, rt.FamilyId)
	})
	return err
}

// revokeFamily revokes a token family in its own committed transaction. It is
// used on the error paths of RotateRefresh, where the rotation transaction has
// rolled back but the revoke must persist.
func revokeFamily(familyId string) {
	_, _ = db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		return struct{}{}, repositories.RefreshTokenRepository.RevokeFamily(tx, familyId)
	})
}
