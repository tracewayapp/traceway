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
	"slices"
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

var codeChallengePattern = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)

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
	default:
		if forbiddenRedirectSchemes[u.Scheme] {
			return fmt.Errorf("redirect_uri %q: scheme %q is not allowed", raw, u.Scheme)
		}
	}
	return nil
}

var forbiddenRedirectSchemes = map[string]bool{
	"javascript": true,
	"data":       true,
	"vbscript":   true,
	"file":       true,
	"blob":       true,
	"about":      true,
}

func isLoopbackHost(hostname string) bool {
	return hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1"
}

func redirectURIMatches(registered []string, presented string) bool {
	if slices.Contains(registered, presented) {
		return true
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
	if !strings.EqualFold(r.Scheme, i.Scheme) || normalizedHostPort(r) != normalizedHostPort(i) {
		return ErrInvalidTarget
	}
	return nil
}

func normalizedHostPort(u *url.URL) string {
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if port == "" || (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") {
		return host
	}
	return host + ":" + port
}

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

func DenyAuthorization(ex lit.Executor, clientId, redirectUri, state string) (string, error) {
	if _, err := validateAuthorizeTarget(ex, clientId, redirectUri); err != nil {
		return "", err
	}
	return appendRedirectParams(redirectUri, url.Values{"error": {"access_denied"}}, state), nil
}

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

func RedeemAuthorizationCode(clientId, code, verifier, redirectUri string) (*models.TokenSetResponse, error) {
	if clientId == "" || code == "" || verifier == "" || redirectUri == "" {
		return nil, ErrInvalidGrant
	}

	type redeemOutcome struct {
		tokens   *models.TokenSetResponse
		grantErr error
	}
	out, err := db.ExecuteTransaction(func(tx *sql.Tx) (redeemOutcome, error) {
		ac, err := repositories.AuthorizationCodeRepository.FindByCode(tx, code)
		if err != nil {
			return redeemOutcome{}, err
		}
		if ac == nil {
			return redeemOutcome{grantErr: ErrInvalidGrant}, nil
		}
		deleted, err := repositories.AuthorizationCodeRepository.Delete(tx, code)
		if err != nil {
			return redeemOutcome{}, err
		}
		if deleted == 0 {
			return redeemOutcome{grantErr: ErrInvalidGrant}, nil
		}
		if time.Now().After(ac.ExpiresAt) || ac.ClientId != clientId || ac.RedirectUri != redirectUri || !verifyPKCE(ac.CodeChallenge, verifier) {
			return redeemOutcome{grantErr: ErrInvalidGrant}, nil
		}
		user, err := repositories.UserRepository.FindById(tx, ac.UserId)
		if err != nil {
			return redeemOutcome{}, err
		}
		if user == nil {
			return redeemOutcome{grantErr: ErrInvalidGrant}, nil
		}
		tokens, err := IssueTokenSet(tx, ac.UserId, user.Email, clientId)
		if err != nil {
			return redeemOutcome{}, err
		}
		return redeemOutcome{tokens: tokens}, nil
	})
	if err != nil {
		return nil, err
	}
	if out.grantErr != nil {
		return nil, out.grantErr
	}
	return out.tokens, nil
}

func verifyPKCE(challenge, verifier string) bool {
	if !codeChallengePattern.MatchString(verifier) {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	computed := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(computed), []byte(challenge)) == 1
}
