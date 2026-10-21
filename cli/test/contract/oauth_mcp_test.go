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
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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

func approveAndGetCode(t *testing.T, clientID, redirectURI, challenge, state string) string {
	t.Helper()
	return approveScopedCode(t, clientID, redirectURI, challenge, state, "")
}

func approveScopedCode(t *testing.T, clientID, redirectURI, challenge, state, scope string) string {
	t.Helper()
	status, _, body := doJSON(t, http.MethodPost, "/api/oauth/approve", jwtToken, map[string]any{
		"client_id":             clientID,
		"redirect_uri":          redirectURI,
		"state":                 state,
		"code_challenge":        challenge,
		"code_challenge_method": "S256",
		"resource":              baseURL + "/mcp",
		"scope":                 scope,
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
	return oauthScopedAccessToken(t, "")
}

func oauthScopedAccessToken(t *testing.T, scope string) string {
	t.Helper()
	redirectURI := "http://127.0.0.1:47831/callback"
	clientID := registerOAuthClient(t, redirectURI)
	verifier, challenge := pkcePair(t)
	code := approveScopedCode(t, clientID, redirectURI, challenge, "mcp-state", scope)
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

func TestContract_protectedResourceMetadata(t *testing.T) {
	status, _, body := doJSON(t, http.MethodGet, "/.well-known/oauth-protected-resource", "", nil)
	if status != http.StatusOK {
		t.Fatalf("metadata: status %d", status)
	}
	if body["resource"] != baseURL {
		t.Errorf("resource = %v, want %v", body["resource"], baseURL)
	}

	status, _, body = doJSON(t, http.MethodGet, "/.well-known/oauth-protected-resource/mcp", "", nil)
	if status != http.StatusOK {
		t.Fatalf("path-inserted metadata: status %d", status)
	}
	if body["resource"] != baseURL+"/mcp" {
		t.Errorf("path-inserted resource = %v, want %v", body["resource"], baseURL+"/mcp")
	}
	servers, _ := body["authorization_servers"].([]any)
	if !slices.Contains(servers, any(baseURL)) {
		t.Errorf("authorization_servers missing issuer: %v", servers)
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

func TestContract_mcpCors(t *testing.T) {
	for _, path := range []string{
		"/mcp",
		"/mcp/",
		"/.well-known/oauth-protected-resource",
		"/.well-known/oauth-protected-resource/mcp",
		"/.well-known/oauth-authorization-server",
		"/api/oauth/register",
		"/api/auth/token",
		"/api/auth/device/authorize",
		"/api/auth/device/token",
		"/api/auth/logout",
	} {
		req, err := http.NewRequest(http.MethodOptions, baseURL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", "https://inspector.example.com")
		req.Header.Set("Access-Control-Request-Method", http.MethodPost)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("OPTIONS %s: %v", path, err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusNoContent {
			t.Errorf("OPTIONS %s: status %d, want 204", path, res.StatusCode)
		}
		if got := res.Header.Get("Access-Control-Allow-Origin"); got != "*" {
			t.Errorf("OPTIONS %s: Access-Control-Allow-Origin = %q, want *", path, got)
		}
	}

	req, err := http.NewRequest(http.MethodGet, baseURL+"/mcp", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "https://inspector.example.com")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /mcp: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Errorf("GET /mcp without token: status %d, want 401", res.StatusCode)
	}
	if got := res.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("GET /mcp: Access-Control-Allow-Origin = %q, want *", got)
	}
	expose := res.Header.Get("Access-Control-Expose-Headers")
	if !strings.Contains(expose, "Mcp-Session-Id") || !strings.Contains(expose, "WWW-Authenticate") {
		t.Errorf("GET /mcp: Access-Control-Expose-Headers = %q, want Mcp-Session-Id and WWW-Authenticate", expose)
	}

	res, err = http.Get(baseURL + "/.well-known/oauth-authorization-server")
	if err != nil {
		t.Fatalf("GET authorization-server metadata: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Errorf("GET authorization-server metadata: status %d, want 200", res.StatusCode)
	}
	if got := res.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("GET authorization-server metadata: Access-Control-Allow-Origin = %q, want *", got)
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
	if challenge := res.Header.Get("WWW-Authenticate"); !strings.Contains(challenge, "/.well-known/oauth-protected-resource/mcp") {
		t.Fatalf("401 must point at the /mcp resource metadata (RFC 9728), got %q", challenge)
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
	if len(tools.Tools) != 22 {
		t.Errorf("got %d tools, want 22", len(tools.Tools))
	}
	for _, tool := range tools.Tools {
		if !tool.Annotations.ReadOnlyHint {
			t.Errorf("/mcp must only expose read tools, found %s", tool.Name)
		}
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

func connectRemote(t *testing.T, path, token string) (*mcp.ClientSession, error) {
	t.Helper()
	transport := &mcp.StreamableClientTransport{
		Endpoint:   baseURL + path,
		HTTPClient: &http.Client{Transport: bearerTransport{token}, Timeout: 60 * time.Second},
	}
	return mcp.NewClient(&mcp.Implementation{Name: "contract-remote", Version: "0"}, nil).Connect(t.Context(), transport, nil)
}

func organizationOfProject(t *testing.T) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, baseURL+"/api/projects", nil)
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var projects []struct {
		Id             string `json:"id"`
		OrganizationId int    `json:"organizationId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		t.Fatal(err)
	}
	for _, p := range projects {
		if p.Id == projectID {
			return p.OrganizationId
		}
	}
	t.Fatalf("seeded project %s not in /api/projects", projectID)
	return 0
}

func setAgentWrites(t *testing.T, enabled bool) {
	t.Helper()
	path := "/api/organizations/" + strconv.Itoa(organizationOfProject(t)) + "/agent-access"
	if status, _, body := doJSON(t, http.MethodPut, path, jwtToken, map[string]any{"agentWriteEnabled": enabled}); status != http.StatusOK {
		t.Fatalf("set agent writes: %d %v", status, body)
	}
}

// TestContract_mcpWriteMount walks the three opt-in levels over the real
// /mcp/write mount: a read token is refused at the door, a write token is
// refused by the organization until an admin opts in, and then it writes.
func TestContract_mcpWriteMount(t *testing.T) {
	status, _, prm := doJSON(t, http.MethodGet, "/.well-known/oauth-protected-resource/mcp/write", "", nil)
	if status != http.StatusOK || prm["resource"] != baseURL+"/mcp/write" {
		t.Fatalf("write resource metadata: %d %v", status, prm)
	}
	if scopes, _ := prm["scopes_supported"].([]any); !slices.Contains(scopes, any("traceway:write")) {
		t.Errorf("write resource must advertise traceway:write, got %v", prm["scopes_supported"])
	}

	// A dashboard session is refused too: sessions skip the agent gate, so
	// one behind /mcp/write would write past the opt-in and the audit log.
	for name, token := range map[string]string{"read token": oauthAccessToken(t), "dashboard session": jwtToken} {
		req, _ := http.NewRequest(http.MethodPost, baseURL+"/mcp/write", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden || !strings.Contains(resp.Header.Get("WWW-Authenticate"), `scope="traceway:write"`) {
			t.Fatalf("%s on /mcp/write = %d %q, want 403 with a traceway:write scope challenge", name, resp.StatusCode, resp.Header.Get("WWW-Authenticate"))
		}
	}

	writeToken := oauthScopedAccessToken(t, "traceway:write")
	cs, err := connectRemote(t, "/mcp/write", writeToken)
	if err != nil {
		t.Fatalf("connect /mcp/write: %v", err)
	}
	defer func() { _ = cs.Close() }()
	tools, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 32 {
		t.Errorf("/mcp/write tools = %d, want 32", len(tools.Tools))
	}

	archive := func() (*mcp.CallToolResult, string) {
		res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: "archive_exceptions", Arguments: map[string]any{"project_id": projectID, "hashes": []string{seedHash}}})
		if err != nil {
			t.Fatalf("archive: %v", err)
		}
		var sb strings.Builder
		for _, c := range res.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				sb.WriteString(tc.Text)
			}
		}
		return res, sb.String()
	}

	setAgentWrites(t, false)
	if res, text := archive(); !res.IsError || !strings.Contains(text, "turned off") {
		t.Fatalf("write before opt-in should be refused by the organization, got %v %s", res.IsError, text)
	}

	setAgentWrites(t, true)
	t.Cleanup(func() { setAgentWrites(t, false) })
	if res, text := archive(); res.IsError {
		t.Fatalf("write after opt-in: %s", text)
	}
	unarchive, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: "unarchive_exceptions", Arguments: map[string]any{"project_id": projectID, "hashes": []string{seedHash}}})
	if err != nil || unarchive.IsError {
		t.Fatalf("unarchive after opt-in: %v", err)
	}

	readSession, err := connectRemote(t, "/mcp", writeToken)
	if err != nil {
		t.Fatalf("connect /mcp with a write token: %v", err)
	}
	defer func() { _ = readSession.Close() }()
	readTools, err := readSession.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range readTools.Tools {
		if tool.Name == "archive_exceptions" {
			t.Error("/mcp must not expose write tools even to a write token")
		}
	}

	path := "/api/organizations/" + strconv.Itoa(organizationOfProject(t)) + "/agent-activity"
	status, _, activity := doJSON(t, http.MethodGet, path, jwtToken, nil)
	if status != http.StatusOK {
		t.Fatalf("agent activity: %d %v", status, activity)
	}
	pagination, _ := activity["pagination"].(map[string]any)
	if total, _ := pagination["total"].(float64); total < 3 {
		t.Errorf("agent activity should hold the refused and the two accepted writes, total = %v", pagination["total"])
	}
}
