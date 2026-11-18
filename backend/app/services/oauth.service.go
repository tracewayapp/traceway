package services

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/tracewayapp/traceway/backend/v2/app/config"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"

	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/github"
	"github.com/markbates/goth/providers/google"
	"github.com/markbates/goth/providers/openidConnect"
)

type oauthService struct {
	googleEnabled   bool
	githubEnabled   bool
	oidcEnabled     bool
	oidcDisplayName string
	oidcAutoCreate  bool
	oidcOrgClaim    string
	oidcRoleClaim   string
	oidcRoleMap     map[string]string
}

var OAuthService *oauthService

func InitOAuth() {
	cfg := config.Config

	hasManualEndpoints := cfg.OIDCAuthURL != "" && cfg.OIDCTokenURL != "" && cfg.OIDCUserInfoURL != ""
	oidcCredentialsSet := cfg.OIDCClientID != "" && cfg.OIDCClientSecret != ""

	svc := &oauthService{
		googleEnabled:   cfg.GoogleClientID != "" && cfg.GoogleClientSecret != "",
		githubEnabled:   cfg.GitHubClientID != "" && cfg.GitHubClientSecret != "",
		oidcEnabled:     oidcCredentialsSet && (cfg.OIDCDiscoveryURL != "" || hasManualEndpoints),
		oidcDisplayName: cfg.OIDCDisplayName,
		oidcAutoCreate:  cfg.OIDCAutoCreateUsers == "true",
		oidcOrgClaim:    cfg.OIDCOrgClaim,
		oidcRoleClaim:   cfg.OIDCRoleClaim,
	}
	OAuthService = svc

	if cfg.OIDCRoleClaim != "" && cfg.OIDCRoleMap != "" {
		if err := json.Unmarshal([]byte(cfg.OIDCRoleMap), &svc.oidcRoleMap); err != nil {
			traceway.CaptureException(fmt.Errorf("OIDC_ROLE_MAP is not valid JSON, role mapping disabled: %w", err))
		}
	}

	if !svc.googleEnabled && !svc.githubEnabled && !svc.oidcEnabled {
		return
	}

	secret := cfg.OAuthSessionSecret
	if secret == "" {
		secret = cfg.JWTSecret
	}

	gothic.Store = newDBSessionStore(secret, &sessions.Options{
		Path:     "/",
		HttpOnly: true,
		MaxAge:   600,
		Secure:   strings.HasPrefix(cfg.AppBaseURL, "https://"),
		SameSite: http.SameSiteLaxMode,
	})

	providers := []goth.Provider{}
	base := strings.TrimRight(cfg.AppBaseURL, "/")
	if svc.googleEnabled {
		providers = append(providers, google.New(
			cfg.GoogleClientID,
			cfg.GoogleClientSecret,
			base+"/api/auth/callback/google",
			"email", "profile",
		))
	}
	if svc.githubEnabled {
		providers = append(providers, github.New(
			cfg.GitHubClientID,
			cfg.GitHubClientSecret,
			base+"/api/auth/callback/github",
			"user:email",
		))
	}
	if svc.oidcEnabled {
		scopes := []string{"openid", "email", "profile"}
		for _, s := range strings.Split(cfg.OIDCExtraScopes, ",") {
			if s = strings.TrimSpace(s); s != "" {
				scopes = append(scopes, s)
			}
		}

		discoveryURL := cfg.OIDCDiscoveryURL
		if discoveryURL == "" && hasManualEndpoints {
			srv, addr := newSyntheticDiscoveryServer(cfg.OIDCAuthURL, cfg.OIDCTokenURL, cfg.OIDCUserInfoURL)
			if srv != nil {
				defer srv.Close()
				discoveryURL = addr
			}
		}

		if discoveryURL == "" {
			svc.oidcEnabled = false
		} else {
			oidcProvider, err := openidConnect.New(
				cfg.OIDCClientID,
				cfg.OIDCClientSecret,
				base+"/api/auth/callback/oidc",
				discoveryURL,
				scopes...,
			)
			if err != nil {
				traceway.CaptureException(fmt.Errorf("OIDC provider init failed (discovery URL may be unreachable): %w", err))
				svc.oidcEnabled = false
			} else {
				providers = append(providers, oidcProvider)
			}
		}
	}
	goth.UseProviders(providers...)
}

func newSyntheticDiscoveryServer(authURL, tokenURL, userInfoURL string) (*http.Server, string) {
	issuer := extractURLOrigin(authURL)
	doc, _ := json.Marshal(map[string]string{
		"issuer":                 issuer,
		"authorization_endpoint": authURL,
		"token_endpoint":         tokenURL,
		"userinfo_endpoint":      userInfoURL,
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		traceway.CaptureException(fmt.Errorf("OIDC manual-endpoint: failed to start discovery server: %w", err))
		return nil, ""
	}

	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write(doc)
		}),
	}
	go srv.Serve(ln)

	return srv, "http://" + ln.Addr().String()
}

func extractURLOrigin(rawURL string) string {
	if i := strings.Index(rawURL, "://"); i != -1 {
		rest := rawURL[i+3:]
		if j := strings.Index(rest, "/"); j != -1 {
			return rawURL[:i+3+j]
		}
	}
	return rawURL
}

func (s *oauthService) IsEnabled() bool {
	return s.googleEnabled || s.githubEnabled || s.oidcEnabled
}

func (s *oauthService) IsProviderEnabled(name string) bool {
	switch name {
	case "google":
		return s.googleEnabled
	case "github":
		return s.githubEnabled
	case "oidc":
		return s.oidcEnabled
	}
	return false
}

func (s *oauthService) EnabledProviders() []string {
	out := []string{}
	if s.googleEnabled {
		out = append(out, "google")
	}
	if s.githubEnabled {
		out = append(out, "github")
	}
	if s.oidcEnabled {
		out = append(out, "oidc")
	}
	return out
}

func (s *oauthService) OIDCAutoCreateEnabled() bool {
	return s.oidcAutoCreate
}

func (s *oauthService) OIDCDisplayName() string {
	return s.oidcDisplayName
}

func (s *oauthService) OIDCOrgClaim() string {
	return s.oidcOrgClaim
}

func (s *oauthService) OIDCRoleClaimEnabled() bool {
	return s.oidcRoleClaim != "" && len(s.oidcRoleMap) > 0
}

func (s *oauthService) ResolveRole(rawData map[string]interface{}) string {
	value := resolveClaimPath(rawData, s.oidcRoleClaim)
	claimValues := normalizeClaimToStrings(value)

	rolePriority := map[string]int{"admin": 3, "user": 2, "readonly": 1}
	bestPriority := 0
	bestRole := ""

	for _, cv := range claimValues {
		if role, ok := s.oidcRoleMap[cv]; ok {
			if p := rolePriority[role]; p > bestPriority {
				bestPriority = p
				bestRole = role
			}
		}
	}
	return bestRole
}

func resolveClaimPath(data map[string]interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	var current interface{} = data
	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current = m[part]
	}
	return current
}

func normalizeClaimToStrings(v interface{}) []string {
	switch val := v.(type) {
	case string:
		return []string{val}
	case []interface{}:
		out := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
