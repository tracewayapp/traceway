package authserver

import (
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories"
)

var (
	ErrInvalidGrantRequest   = errors.New("invalid_request")
	ErrInvalidRedirect       = errors.New("invalid_redirect_uri")
	ErrInvalidChallenge      = errors.New("invalid_code_challenge")
	ErrInvalidTarget         = errors.New("invalid_target")
	ErrInvalidClientMetadata = errors.New("invalid_client_metadata")
)

// codeChallengePattern is the RFC 7636 charset for both the S256 challenge
// (43 chars of base64url) and the verifier (43-128 chars).
var codeChallengePattern = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)

// RegisterClient stores a dynamically registered public client (RFC 7591).
// Registration is open (the endpoint is rate limited per IP): MCP clients
// register themselves before starting the authorization-code flow. Every
// client is public (no secret) and PKCE is mandatory, so a registration by
// itself grants nothing; the user still approves each authorization on the
// consent page, which shows the self-asserted client name as untrusted.
func RegisterClient(tx *sql.Tx, name string, redirectURIs []string) (*models.OauthClient, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 100 {
		return nil, fmt.Errorf("%w: client_name is required and must be at most 100 characters", ErrInvalidClientMetadata)
	}
	if len(redirectURIs) == 0 || len(redirectURIs) > 10 {
		return nil, fmt.Errorf("%w: redirect_uris must contain between 1 and 10 entries", ErrInvalidClientMetadata)
	}
	for _, raw := range redirectURIs {
		if err := validateRedirectURI(raw); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidClientMetadata, err)
		}
	}

	client := &models.OauthClient{
		Id:           uuid.New().String(),
		Name:         name,
		RedirectUris: redirectURIs,
		CreatedAt:    time.Now().UTC(),
	}
	if err := repositories.OauthClientRepository.Create(tx, client); err != nil {
		return nil, err
	}
	return client, nil
}

// validateRedirectURI enforces RFC 8252 rules for public clients: https and
// private-use (custom) schemes are allowed anywhere; plain http only on
// loopback hosts. Fragments are forbidden by RFC 6749 §3.1.2.
func validateRedirectURI(raw string) error {
	if raw == "" || len(raw) > 2048 {
		return fmt.Errorf("redirect_uri must be a non-empty URL of at most 2048 characters")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("redirect_uri %q is not a valid URL", raw)
	}
	if u.Scheme == "" {
		return fmt.Errorf("redirect_uri %q must be absolute", raw)
	}
	if u.Fragment != "" {
		return fmt.Errorf("redirect_uri %q must not contain a fragment", raw)
	}
	switch u.Scheme {
	case "https":
		if u.Host == "" {
			return fmt.Errorf("redirect_uri %q must have a host", raw)
		}
	case "http":
		if !isLoopbackHost(u.Hostname()) {
			return fmt.Errorf("redirect_uri %q: http is only allowed for loopback addresses", raw)
		}
	}
	return nil
}

func isLoopbackHost(hostname string) bool {
	return hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1"
}

// redirectURIMatches reports whether a presented redirect_uri is allowed for
// the client: exact match against a registered URI, except that loopback http
// URIs match with any port (RFC 8252 §7.3: native apps bind an ephemeral
// port per flow).
func redirectURIMatches(registered []string, presented string) bool {
	for _, reg := range registered {
		if reg == presented {
			return true
		}
	}
	p, err := url.Parse(presented)
	if err != nil || p.Scheme != "http" || !isLoopbackHost(p.Hostname()) {
		return false
	}
	for _, reg := range registered {
		r, err := url.Parse(reg)
		if err != nil || r.Scheme != "http" || !isLoopbackHost(r.Hostname()) {
			continue
		}
		if r.Hostname() == p.Hostname() && r.Path == p.Path {
			return true
		}
	}
	return false
}

// ValidateResource enforces RFC 8707 for the MCP flow: when the client names
// the resource it wants the token for, it must be this deployment. An empty
// resource is fine (the token is origin-wide either way).
func ValidateResource(resource, issuer string) error {
	if resource == "" {
		return nil
	}
	if len(resource) > 2048 {
		return ErrInvalidTarget
	}
	r, err := url.Parse(resource)
	if err != nil {
		return ErrInvalidTarget
	}
	i, err := url.Parse(issuer)
	if err != nil {
		return ErrInvalidTarget
	}
	if !strings.EqualFold(r.Scheme, i.Scheme) || !strings.EqualFold(r.Host, i.Host) {
		return ErrInvalidTarget
	}
	return nil
}

// ApproveAuthorization validates an authorization request and mints the
// single-use code, bound to the approving user. It runs under the
// Transactional middleware (commits on the 200 that carries the redirect).
// The state parameter is never stored: the consent page passes it through
// and it rides back on the redirect only.
func ApproveAuthorization(tx *sql.Tx, userId int, req models.AuthorizeApproveRequest, issuer string) (string, error) {
	client, err := validateAuthorizeTarget(tx, req.ClientId, req.RedirectUri)
	if err != nil {
		return "", err
	}
	if req.CodeChallengeMethod != "S256" {
		return "", fmt.Errorf("%w: code_challenge_method must be S256", ErrInvalidChallenge)
	}
	if !codeChallengePattern.MatchString(req.CodeChallenge) {
		return "", fmt.Errorf("%w: code_challenge must be 43-128 characters of [A-Za-z0-9._~-]", ErrInvalidChallenge)
	}
	if len(req.State) > 2048 {
		return "", fmt.Errorf("%w: state must be at most 2048 characters", ErrInvalidGrantRequest)
	}
	if err := ValidateResource(req.Resource, issuer); err != nil {
		return "", err
	}

	if _, err := repositories.AuthorizationCodeRepository.PruneExpired(tx, time.Now()); err != nil {
		return "", err
	}
	code := newOpaqueToken("twa_")
	if err := repositories.AuthorizationCodeRepository.Create(tx, code, client.Id, userId, req.RedirectUri, req.CodeChallenge, time.Now().Add(authCodeTTL)); err != nil {
		return "", err
	}
	return appendRedirectParams(req.RedirectUri, url.Values{"code": {code}}, req.State), nil
}

// DenyAuthorization validates the client and redirect target, then returns
// the error redirect. Validating first keeps the deny path from becoming an
// open redirector.
func DenyAuthorization(ex lit.Executor, clientId, redirectUri, state string) (string, error) {
	if _, err := validateAuthorizeTarget(ex, clientId, redirectUri); err != nil {
		return "", err
	}
	return appendRedirectParams(redirectUri, url.Values{"error": {"access_denied"}}, state), nil
}

// LookupClient resolves a registered client for the consent page. A nil
// result means the client id is unknown.
func LookupClient(ex lit.Executor, clientId string) (*models.OauthClient, error) {
	if clientId == "" {
		return nil, nil
	}
	return repositories.OauthClientRepository.FindById(ex, clientId)
}

func validateAuthorizeTarget(ex lit.Executor, clientId, redirectUri string) (*models.OauthClient, error) {
	if clientId == "" {
		return nil, ErrUnknownClient
	}
	client, err := repositories.OauthClientRepository.FindById(ex, clientId)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, ErrUnknownClient
	}
	if !redirectURIMatches(client.RedirectUris, redirectUri) {
		return nil, ErrInvalidRedirect
	}
	return client, nil
}

func appendRedirectParams(redirectUri string, params url.Values, state string) string {
	if state != "" {
		params.Set("state", state)
	}
	sep := "?"
	if strings.Contains(redirectUri, "?") {
		sep = "&"
	}
	return redirectUri + sep + params.Encode()
}

// RedeemAuthorizationCode exchanges a code plus PKCE verifier for a token
// set. It owns its own transactions for the same reason PollDeviceToken does
// (the token endpoint returns 400 for flow control), and it consumes the code
// in a transaction of its own so the single-use delete commits even when the
// exchange then fails: a wrong verifier must not leave the code retryable.
// Every failure is invalid_grant: distinguishing wrong-verifier from expired
// from replayed would only help an attacker.
func RedeemAuthorizationCode(clientId, code, verifier, redirectUri string) (*models.TokenSetResponse, error) {
	if clientId == "" || code == "" || verifier == "" || redirectUri == "" {
		return nil, ErrInvalidGrant
	}

	ac, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.AuthorizationCode, error) {
		ac, err := repositories.AuthorizationCodeRepository.FindByCode(tx, code)
		if err != nil {
			return nil, err
		}
		if ac == nil {
			return nil, nil
		}
		// The rows-affected guard means a concurrent duplicate exchange
		// cannot mint a second token set for the same code.
		deleted, err := repositories.AuthorizationCodeRepository.Delete(tx, code)
		if err != nil {
			return nil, err
		}
		if deleted == 0 {
			return nil, nil
		}
		return ac, nil
	})
	if err != nil {
		return nil, err
	}
	if ac == nil || time.Now().After(ac.ExpiresAt) {
		return nil, ErrInvalidGrant
	}
	if ac.ClientId != clientId || ac.RedirectUri != redirectUri {
		return nil, ErrInvalidGrant
	}
	if !verifyPKCE(ac.CodeChallenge, verifier) {
		return nil, ErrInvalidGrant
	}

	return db.ExecuteTransaction(func(tx *sql.Tx) (*models.TokenSetResponse, error) {
		user, err := repositories.UserRepository.FindById(tx, ac.UserId)
		if err != nil {
			return nil, err
		}
		if user == nil {
			return nil, ErrInvalidGrant
		}
		return IssueTokenSet(tx, ac.UserId, user.Email, clientId)
	})
}

func verifyPKCE(challenge, verifier string) bool {
	if !codeChallengePattern.MatchString(verifier) {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	computed := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(computed), []byte(challenge)) == 1
}
