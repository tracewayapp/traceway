package services

import (
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

// dbSessionStore is a gorilla/sessions.Store backed by the main transactional
// DB. The cookie holds only a signed session ID; the session data (which can
// exceed the 4096-byte cookie limit for large OIDC ID tokens) lives in the
// oauth_sessions table.
//
// Contract: every route that hands a request to this store (directly or via
// gothic) must be wrapped in middleware.Transactional so the request context
// carries a *sql.Tx. The store reuses that tx rather than opening its own —
// opening a second tx from the same handler would deadlock against the
// single-connection SQLite main DB.
type dbSessionStore struct {
	codecs  []securecookie.Codec
	options *sessions.Options
}

var errNoTransaction = errors.New("oauth session store: no transaction in request context (route must use middleware.Transactional)")

func newDBSessionStore(secret string, options *sessions.Options) *dbSessionStore {
	hashKey := []byte(secret)
	blockKey := sha256.Sum256([]byte(secret + ":oauth-session-encryption"))
	codecs := securecookie.CodecsFromPairs(hashKey, blockKey[:])
	for _, c := range codecs {
		if sc, ok := c.(*securecookie.SecureCookie); ok {
			sc.MaxLength(0)
		}
	}
	return &dbSessionStore{codecs: codecs, options: options}
}

func (s *dbSessionStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	return sessions.GetRegistry(r).Get(s, name)
}

func (s *dbSessionStore) New(r *http.Request, name string) (*sessions.Session, error) {
	session := sessions.NewSession(s, name)
	opts := *s.options
	session.Options = &opts
	session.IsNew = true

	cookie, err := r.Cookie(name)
	if err != nil {
		return session, nil
	}

	var id string
	if err := securecookie.DecodeMulti(name, cookie.Value, &id, s.codecs...); err != nil {
		return session, nil
	}

	tx := db.GetTx(r.Context())
	if tx == nil {
		return session, errNoTransaction
	}
	data, err := transactional.OAuthSessionRepository.Get(tx, id)
	if err != nil || data == nil {
		return session, nil
	}

	if err := securecookie.DecodeMulti(name, string(data), &session.Values, s.codecs...); err != nil {
		return session, nil
	}
	session.ID = id
	session.IsNew = false
	return session, nil
}

func (s *dbSessionStore) Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	tx := db.GetTx(r.Context())
	if tx == nil {
		return errNoTransaction
	}

	if session.Options != nil && session.Options.MaxAge < 0 {
		if session.ID != "" {
			if err := transactional.OAuthSessionRepository.Delete(tx, session.ID); err != nil {
				return err
			}
		}
		http.SetCookie(w, sessions.NewCookie(session.Name(), "", session.Options))
		return nil
	}

	if session.ID == "" {
		key := securecookie.GenerateRandomKey(32)
		if key == nil {
			return errors.New("oauth session: failed to generate session ID (crypto/rand unavailable)")
		}
		session.ID = strings.TrimRight(base32.StdEncoding.EncodeToString(key), "=")
	}

	encoded, err := securecookie.EncodeMulti(session.Name(), session.Values, s.codecs...)
	if err != nil {
		return err
	}

	expires := time.Now().UTC().Add(time.Duration(session.Options.MaxAge) * time.Second)
	if err := transactional.OAuthSessionRepository.Save(tx, session.ID, []byte(encoded), expires); err != nil {
		return err
	}

	cookieValue, err := securecookie.EncodeMulti(session.Name(), session.ID, s.codecs...)
	if err != nil {
		return err
	}
	http.SetCookie(w, sessions.NewCookie(session.Name(), cookieValue, session.Options))
	return nil
}
