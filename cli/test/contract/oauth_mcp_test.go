package contract

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// These tests drive the remote MCP story end to end against the booted
// backend: RFC 8414 discovery, RFC 7591 registration, the authorization-code
// + PKCE grant (with the consent step exercised at the API level, exactly as
// the SPA consent page performs it), and finally a live MCP session over the
// streamable HTTP mount using the minted access token.

func doJSON(t *testing.T, method, path, token string, body any) (int, http.Header, map[string]any) {
	t.Helper()
	var payload *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		payload = bytes.NewReader(b)
	} else {
		payload = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, baseURL+path, payload)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	var decoded map[string]any
	_ = json.NewDecoder(res.Body).Decode(&decoded)
	return res.StatusCode, res.Header, decoded
}

func pkcePair(t *testing.T) (verifier, challenge string) {
	t.Helper()
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	verifier = base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:])
}

func registerOAuthClient(t *testing.T, redirectURI string) string {
	t.Helper()
	status, _, body := doJSON(t, http.MethodPost, "/api/oauth/register", "", map[string]any{
		"client_name":   "Contract MCP Client",
		"redirect_uris": []string{redirectURI},
	})
	if status != http.StatusCreated {
		t.Fatalf("register: status %d body %v", status, body)
	}
	clientID, _ := body["client_id"].(string)
	if clientID == "" {
		t.Fatalf("register: missing client_id in %v", body)
	}
	if body["token_endpoint_auth_method"] != "none" {
		t.Fatalf("register: expected a public client, got %v", body)
	}
	return clientID
}

// approveAndGetCode performs the consent step as the SPA does and returns the
// authorization code from the validated redirect.
func approveAndGetCode(t *testing.T, clientID, redirectURI, challenge, state string) string {
	t.Helper()
	status, _, body := doJSON(t, http.MethodPost, "/api/oauth/approve", jwtToken, map[string]any{
		"client_id":             clientID,
		"redirect_uri":          redirectURI,
		"state":                 state,
		"code_challenge":        challenge,
		"code_challenge_method": "S256",
		"resource":              baseURL + "/mcp",
	})
	if status != http.StatusOK {
		t.Fatalf("approve: status %d body %v", status, body)
	}
	redirectTo, _ := body["redirectTo"].(string)
	u, err := url.Parse(redirectTo)
	if err != nil {
		t.Fatalf("approve: bad redirectTo %q: %v", redirectTo, err)
	}
	if got := u.Query().Get("state"); got != state {
		t.Fatalf("approve: state = %q, want %q", got, state)
	}
	code := u.Query().Get("code")
	if !strings.HasPrefix(code, "twa_") {
		t.Fatalf("approve: expected a twa_ code in %q", redirectTo)
	}
	return code
}

func exchangeCode(t *testing.T, clientID, redirectURI, code, verifier string) (int, map[string]any) {
	t.Helper()
	status, _, body := doJSON(t, http.MethodPost, "/api/auth/token", "", map[string]any{
		"grant_type":    "authorization_code",
		"client_id":     clientID,
		"redirect_uri":  redirectURI,
		"code":          code,
		"code_verifier": verifier,
		"resource":      baseURL + "/mcp",
	})
	return status, body
}

func oauthAccessToken(t *testing.T) string {
	t.Helper()
	redirectURI := "http://127.0.0.1:47831/callback"
	clientID := registerOAuthClient(t, redirectURI)
	verifier, challenge := pkcePair(t)
	code := approveAndGetCode(t, clientID, redirectURI, challenge, "mcp-state")
	status, body := exchangeCode(t, clientID, redirectURI, code, verifier)
	if status != http.StatusOK {
		t.Fatalf("exchange: status %d body %v", status, body)
	}
	token, _ := body["access_token"].(string)
	if token == "" {
		t.Fatalf("exchange: missing access_token in %v", body)
	}
	return token
}

func TestContract_oauthServerMetadata(t *testing.T) {
	status, _, body := doJSON(t, http.MethodGet, "/.well-known/oauth-authorization-server", "", nil)
	if status != http.StatusOK {
		t.Fatalf("metadata: status %d", status)
	}
	if body["authorization_endpoint"] != baseURL+"/oauth/authorize" {
		t.Errorf("authorization_endpoint = %v", body["authorization_endpoint"])
	}
	if body["registration_endpoint"] != baseURL+"/api/oauth/register" {
		t.Errorf("registration_endpoint = %v", body["registration_endpoint"])
	}
	grants, _ := body["grant_types_supported"].([]any)
	if !slices.Contains(grants, any("authorization_code")) {
		t.Errorf("grant_types_supported missing authorization_code: %v", grants)
	}
	methods, _ := body["code_challenge_methods_supported"].([]any)
	if !slices.Contains(methods, any("S256")) {
		t.Errorf("code_challenge_methods_supported missing S256: %v", methods)
	}
	responses, _ := body["response_types_supported"].([]any)
	if !slices.Contains(responses, any("code")) {
		t.Errorf("response_types_supported missing code: %v", responses)
	}
}

func TestContract_dcrAndAuthorizationCodeGrant(t *testing.T) {
	redirectURI := "http://127.0.0.1:39745/callback"
	clientID := registerOAuthClient(t, redirectURI)

	verifier, challenge := pkcePair(t)
	code := approveAndGetCode(t, clientID, redirectURI, challenge, "st-123")

	status, tokens := exchangeCode(t, clientID, redirectURI, code, verifier)
	if status != http.StatusOK {
		t.Fatalf("exchange: status %d body %v", status, tokens)
	}
	access, _ := tokens["access_token"].(string)
	refresh, _ := tokens["refresh_token"].(string)
	if access == "" || !strings.HasPrefix(refresh, "twr_") || tokens["token_type"] != "Bearer" {
		t.Fatalf("unexpected token set: %v", tokens)
	}

	if st, _, body := doJSON(t, http.MethodGet, "/api/projects", access, nil); st != http.StatusOK {
		t.Fatalf("access token rejected by the API: %d %v", st, body)
	}

	if st, replay := exchangeCode(t, clientID, redirectURI, code, verifier); st != http.StatusBadRequest || replay["error"] != "invalid_grant" {
		t.Errorf("code replay should be invalid_grant, got %d %v", st, replay)
	}

	st, _, rotated := doJSON(t, http.MethodPost, "/api/auth/token", "", map[string]any{
		"grant_type": "refresh_token", "refresh_token": refresh,
	})
	if st != http.StatusOK || rotated["access_token"] == "" || rotated["refresh_token"] == refresh {
		t.Errorf("refresh rotation failed: %d %v", st, rotated)
	}

	badVerifier, _ := pkcePair(t)
	code2 := approveAndGetCode(t, clientID, redirectURI, challenge, "st-2")
	if st, body := exchangeCode(t, clientID, redirectURI, code2, badVerifier); st != http.StatusBadRequest || body["error"] != "invalid_grant" {
		t.Errorf("wrong verifier should be invalid_grant, got %d %v", st, body)
	}
	if st, body := exchangeCode(t, clientID, redirectURI, code2, verifier); st != http.StatusBadRequest || body["error"] != "invalid_grant" {
		t.Errorf("a failed exchange must consume the code, got %d %v", st, body)
	}
}

func TestContract_authorizeValidationAndDeny(t *testing.T) {
	redirectURI := "http://127.0.0.1:39746/callback"
	clientID := registerOAuthClient(t, redirectURI)
	_, challenge := pkcePair(t)

	status, _, body := doJSON(t, http.MethodPost, "/api/oauth/register", "", map[string]any{
		"client_name":   "Evil",
		"redirect_uris": []string{"http://evil.example.com/cb"},
	})
	if status != http.StatusBadRequest || body["error"] != "invalid_client_metadata" {
		t.Errorf("non-loopback http redirect should be rejected at registration, got %d %v", status, body)
	}

	status, _, body = doJSON(t, http.MethodPost, "/api/oauth/approve", jwtToken, map[string]any{
		"client_id": clientID, "redirect_uri": "https://evil.example.com/cb",
		"code_challenge": challenge, "code_challenge_method": "S256",
	})
	if status != http.StatusBadRequest || body["error"] != "invalid_redirect_uri" {
		t.Errorf("unregistered redirect should be rejected, got %d %v", status, body)
	}

	status, _, body = doJSON(t, http.MethodPost, "/api/oauth/approve", jwtToken, map[string]any{
		"client_id": clientID, "redirect_uri": redirectURI,
		"code_challenge": challenge, "code_challenge_method": "S256",
		"resource": "https://other.example.com/mcp",
	})
	if status != http.StatusBadRequest || body["error"] != "invalid_target" {
		t.Errorf("foreign resource should be rejected, got %d %v", status, body)
	}

	status, _, body = doJSON(t, http.MethodPost, "/api/oauth/approve", "", map[string]any{
		"client_id": clientID, "redirect_uri": redirectURI,
		"code_challenge": challenge, "code_challenge_method": "S256",
	})
	if status != http.StatusUnauthorized {
		t.Errorf("approve must require app auth, got %d %v", status, body)
	}

	status, _, body = doJSON(t, http.MethodPost, "/api/oauth/deny", jwtToken, map[string]any{
		"client_id": clientID, "redirect_uri": redirectURI, "state": "st-deny",
	})
	if status != http.StatusOK {
		t.Fatalf("deny: %d %v", status, body)
	}
	redirectTo, _ := body["redirectTo"].(string)
	u, _ := url.Parse(redirectTo)
	if u == nil || u.Query().Get("error") != "access_denied" || u.Query().Get("state") != "st-deny" {
		t.Errorf("deny redirect malformed: %q", redirectTo)
	}
}

type bearerTransport struct {
	token string
}

func (b bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(req)
}

func TestContract_mcpOverStreamableHTTP(t *testing.T) {
	res, err := http.Post(baseURL+"/mcp", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated /mcp should 401, got %d", res.StatusCode)
	}
	if challenge := res.Header.Get("WWW-Authenticate"); !strings.Contains(challenge, "/.well-known/oauth-protected-resource") {
		t.Fatalf("401 must carry the resource metadata challenge, got %q", challenge)
	}

	token := oauthAccessToken(t)
	transport := &mcp.StreamableClientTransport{
		Endpoint:   baseURL + "/mcp",
		HTTPClient: &http.Client{Transport: bearerTransport{token}, Timeout: 60 * time.Second},
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "contract-remote", Version: "0"}, nil).Connect(t.Context(), transport, nil)
	if err != nil {
		t.Fatalf("mcp connect over streamable http: %v", err)
	}
	defer func() { _ = cs.Close() }()

	if init := cs.InitializeResult(); init == nil || !strings.Contains(init.Instructions, baseURL) {
		t.Error("instructions should name the instance origin")
	}

	tools, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 16 {
		t.Errorf("got %d tools, want 16", len(tools.Tools))
	}
	for _, tool := range tools.Tools {
		if tool.OutputSchema == nil {
			t.Errorf("tool %s has no output schema over the mount", tool.Name)
		}
	}

	call := func(name string, args map[string]any) *mcp.CallToolResult {
		t.Helper()
		res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("%s: protocol error: %v", name, err)
		}
		return res
	}
	text := func(res *mcp.CallToolResult) string {
		var sb strings.Builder
		for _, c := range res.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				sb.WriteString(tc.Text)
			}
		}
		return sb.String()
	}

	// The mount is multi-project: no default project, so the tool must say so.
	noProject := call("list_exceptions", nil)
	if !noProject.IsError || !strings.Contains(text(noProject), "no_project") {
		t.Errorf("missing project should be a no_project tool error, got %s", text(noProject))
	}

	from := seedAt.Add(-time.Hour).Format(time.RFC3339)
	to := seedAt.Add(time.Hour).Format(time.RFC3339)
	listRes := call("list_exceptions", map[string]any{"project_id": projectID, "from": from, "to": to})
	if listRes.IsError {
		t.Fatalf("list_exceptions over the mount: %s", text(listRes))
	}
	if !strings.Contains(text(listRes), seedHash) {
		t.Errorf("result should contain the seeded hash: %s", clip([]byte(text(listRes))))
	}
	if listRes.StructuredContent == nil {
		t.Error("mount results should carry structuredContent")
	}

	detail := call("get_exception", map[string]any{"project_id": projectID, "hash": seedHash})
	if detail.IsError || !strings.Contains(text(detail), seedExceptionID.String()) {
		t.Errorf("get_exception over the mount failed: %s", clip([]byte(text(detail))))
	}

	projects := call("list_projects", nil)
	if projects.IsError || !strings.Contains(text(projects), projectID) {
		t.Errorf("list_projects over the mount failed: %s", text(projects))
	}
}
