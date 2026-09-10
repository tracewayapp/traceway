//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package github

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/tracewayapp/traceway/backend/app/agent"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/dbtest"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/secrets"
	"github.com/tracewayapp/traceway/backend/app/services"
)

const (
	webhookSecret = "whsec-test"
	testHash      = "0123456789abcdef"
)

// fakeGitHub records the API calls the App path makes and mints
// installation tokens with a short expiry so the cache can be watched.
type fakeGitHub struct {
	server   *httptest.Server
	mu       sync.Mutex
	calls    []string
	comments []string
	minted   int
	ttl      time.Duration
	pem      string
}

func newFakeGitHub(t *testing.T) *fakeGitHub {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeGitHub{ttl: time.Hour, pem: string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.calls = append(f.calls, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization"))
		switch {
		case strings.HasPrefix(r.URL.Path, "/app/installations/") && strings.HasSuffix(r.URL.Path, "/access_tokens"):
			token, _ := jwt.Parse(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), func(*jwt.Token) (any, error) { return &key.PublicKey, nil })
			if token == nil || !token.Valid {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"message":"bad app jwt"}`))
				return
			}
			f.minted++
			w.WriteHeader(http.StatusCreated)
			fmt.Fprintf(w, `{"token":"ghs_%d","expires_at":%q}`, f.minted, time.Now().Add(f.ttl).UTC().Format(time.RFC3339))
		case strings.HasPrefix(r.URL.Path, "/app-manifests/") && strings.HasSuffix(r.URL.Path, "/conversions"):
			w.WriteHeader(http.StatusCreated)
			fmt.Fprintf(w, `{"id":4242,"slug":"traceway-test","html_url":"%s/apps/traceway-test","pem":%q,"webhook_secret":%q,"client_id":"Iv1.abc","client_secret":"cs_secret"}`, f.server.URL, f.pem, webhookSecret)
		case r.URL.Path == "/repos/acme/app/pulls" && r.Method == http.MethodGet:
			w.Write([]byte(`[]`))
		case r.URL.Path == "/repos/acme/app/pulls":
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"number":42,"html_url":"https://github.com/acme/app/pull/42"}`))
		case strings.HasSuffix(r.URL.Path, "/comments"):
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			f.comments = append(f.comments, r.URL.Path+": "+body["body"])
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id":1}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"message":"Not Found"}`))
		}
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeGitHub) tokenCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.minted
}

type fixture struct {
	host    *Host
	fake    *fakeGitHub
	in      *models.Integration
	project *models.Project
	ownerId int
}

func setup(t *testing.T) *fixture {
	t.Helper()
	config.LoggingEnabled = false
	t.Cleanup(func() { config.LoggingEnabled = true })
	dbtest.SetupSQLite(t)
	config.Config.JWTSecret = "test-secret-that-is-long-enough-for-jwt-0123"
	if err := services.InitJWT(); err != nil {
		t.Fatal(err)
	}
	key, _, _ := secrets.GenerateKey()
	secrets.Init(key)
	t.Cleanup(func() { secrets.Init(nil) })

	fake := newFakeGitHub(t)
	host := New()
	host.APIBase = fake.server.URL
	host.WebBase = fake.server.URL
	Register(host)

	fx := &fixture{host: host, fake: fake}
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		owner, err := transactional.UserRepository.Create(tx, "dev@example.com", "Dev", "hashed")
		if err != nil {
			return struct{}{}, err
		}
		org, err := transactional.OrganizationRepository.Create(tx, "Org", "UTC")
		if err != nil {
			return struct{}{}, err
		}
		if _, err := transactional.OrganizationRepository.AddUser(tx, org.Id, owner.Id, "owner"); err != nil {
			return struct{}{}, err
		}
		project, err := transactional.ProjectRepository.CreateWithOrganization(tx, "Project", "opentelemetry", org.Id)
		if err != nil {
			return struct{}{}, err
		}
		now := time.Now().UTC()
		raw := fmt.Sprintf(`{"mode":"app","appId":"4242","privateKey":%q,"webhookSecret":%q,"installationId":"77","label":"traceway"}`, fake.pem, webhookSecret)
		encrypted, err := secrets.EncryptFields(json.RawMessage(raw), secretFields)
		if err != nil {
			return struct{}{}, err
		}
		in := &models.Integration{OrganizationId: org.Id, Provider: Provider, Kinds: models.StringSlice(host.Kinds()), Name: "GitHub App", Config: models.JSONText(encrypted), Enabled: true, CreatedAt: now, UpdatedAt: now}
		id, err := transactional.IntegrationRepository.Create(tx, in)
		if err != nil {
			return struct{}{}, err
		}
		in.Id = id
		if _, err := transactional.RepositoryRepository.Create(tx, &models.Repository{ProjectId: project.Id, IntegrationId: &id, Owner: "acme", Name: "app", DefaultBranch: "main", CreatedAt: now, UpdatedAt: now}); err != nil {
			return struct{}{}, err
		}
		if _, err := transactional.IdentityRepository.Create(tx, &models.Identity{UserId: owner.Id, Provider: Provider, ExternalId: "octodev", Display: "Octo Dev", CreatedAt: now}); err != nil {
			return struct{}{}, err
		}
		fx.in, fx.project, fx.ownerId = in, project, owner.Id
		return struct{}{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return fx
}

func (fx *fixture) delivery(event string, payload map[string]any, secret string) *http.Request {
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/integrations/github/inbound/1", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", event)
	req.Header.Set("X-Hub-Signature-256", Sign(secret, body))
	return req
}

func (fx *fixture) inbound(t *testing.T, req *http.Request) agent.InboundResult {
	t.Helper()
	result, err := agent.HandleInbound(context.Background(), Provider, fx.in, req)
	if err != nil {
		t.Fatalf("inbound: %v", err)
	}
	return result
}

func (fx *fixture) attempt(t *testing.T) *models.AgentAttempt {
	t.Helper()
	attempt, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.AgentAttempt, error) {
		return transactional.AgentAttemptRepository.FindActiveBySubject(tx, fx.project.Id, models.SubjectKindTracewayException, testHash)
	})
	if err != nil {
		t.Fatal(err)
	}
	return attempt
}

func (fx *fixture) reload(t *testing.T, attempt *models.AgentAttempt) *models.AgentAttempt {
	t.Helper()
	row, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.AgentAttempt, error) {
		return transactional.AgentAttemptRepository.FindById(tx, attempt.Id)
	})
	if err != nil || row == nil {
		t.Fatal(err)
	}
	return row
}

func repoPayload() map[string]any {
	return map[string]any{"name": "app", "owner": map[string]any{"login": "acme"}}
}

func issuePayload(action string, label string, body string, sender string) map[string]any {
	return map[string]any{
		"action":     action,
		"label":      map[string]any{"name": label},
		"issue":      map[string]any{"number": 9, "title": "RuntimeError", "body": body, "labels": []map[string]any{{"name": label}}},
		"repository": repoPayload(),
		"sender":     map[string]any{"login": sender},
	}
}

func (fx *fixture) issueBody() string {
	return "**RuntimeError: boom**\n\nHash: " + testHash + "\nProject: " + fx.project.Id.String() + "\n"
}

func TestSignatureAndPing(t *testing.T) {
	fx := setup(t)
	if _, err := agent.HandleInbound(context.Background(), Provider, fx.in, fx.delivery("ping", map[string]any{"zen": "x"}, "wrong")); err == nil {
		t.Fatal("wrong signature accepted")
	}
	unsigned := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
	if _, err := agent.HandleInbound(context.Background(), Provider, fx.in, unsigned); err == nil {
		t.Fatal("unsigned delivery accepted")
	}
	result := fx.inbound(t, fx.delivery("ping", map[string]any{"zen": "x"}, webhookSecret))
	if string(result.Response) != `{"ok":true}` {
		t.Fatalf("ping = %s", result.Response)
	}
}

func TestInstallationWebhookRecordsTheInstallation(t *testing.T) {
	fx := setup(t)
	fx.inbound(t, fx.delivery("installation", map[string]any{"action": "deleted", "installation": map[string]any{"id": 77}, "sender": map[string]any{"login": "octodev"}}, webhookSecret))
	in, _ := db.ExecuteTransaction(func(tx *sql.Tx) (*models.Integration, error) {
		return transactional.IntegrationRepository.FindById(tx, fx.in.Id)
	})
	if fx.host.Describe(in) != "GitHub App created, not installed" {
		t.Fatalf("after delete: %s", fx.host.Describe(in))
	}
	fx.inbound(t, fx.delivery("installation", map[string]any{"action": "created", "installation": map[string]any{"id": 88}, "sender": map[string]any{"login": "octodev"}}, webhookSecret))
	in, _ = db.ExecuteTransaction(func(tx *sql.Tx) (*models.Integration, error) {
		return transactional.IntegrationRepository.FindById(tx, fx.in.Id)
	})
	s, _ := fx.host.settings(in)
	if s.InstallationId != "88" || fx.host.Describe(in) != "GitHub App installed" || s.PrivateKey != fx.fake.pem {
		t.Fatalf("after create: %+v", s)
	}
}

func TestLabelledIssueStartsAnAttemptWithTheIssueAsThread(t *testing.T) {
	fx := setup(t)
	result := fx.inbound(t, fx.delivery("issues", issuePayload("labeled", "traceway", fx.issueBody(), "octodev"), webhookSecret))
	if result.Requests != 1 {
		t.Fatalf("requests = %+v", result)
	}
	attempt := fx.attempt(t)
	if attempt == nil || attempt.RequestedBy == nil || *attempt.RequestedBy != fx.ownerId {
		t.Fatalf("attempt = %+v", attempt)
	}
	links, _ := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.AgentLink, error) {
		return transactional.AgentLinkRepository.FindByAttempt(tx, attempt.Id)
	})
	kinds := map[string]string{}
	for _, link := range links {
		kinds[link.Kind] = link.ExternalRef
	}
	if kinds[models.LinkKindIssue] != "acme/app#9" || kinds[models.LinkKindThread] != "acme/app#9" {
		t.Fatalf("links = %v", kinds)
	}

	for name, payload := range map[string]map[string]any{
		"other label":   issuePayload("labeled", "bug", fx.issueBody(), "octodev"),
		"no hash":       issuePayload("labeled", "traceway", "just words", "octodev"),
		"other project": issuePayload("labeled", "traceway", "Hash: "+testHash+"\nProject: 6f1b3d2e-4c5a-4b8e-9d0f-1a2b3c4d5e6f\n", "octodev"),
	} {
		if r := fx.inbound(t, fx.delivery("issues", payload, webhookSecret)); r.Requests != 0 {
			t.Fatalf("%s started an attempt: %+v", name, r)
		}
	}
	if r := fx.inbound(t, fx.delivery("issues", issuePayload("labeled", "traceway", fx.issueBody(), "stranger"), webhookSecret)); r.Requests != 0 || r.Ignored != 1 {
		t.Fatalf("unmapped login = %+v", r)
	}
}

func TestFixCommentAndReplies(t *testing.T) {
	fx := setup(t)
	comment := func(number int, body string, user string, id int64) map[string]any {
		return map[string]any{
			"action":     "created",
			"issue":      map[string]any{"number": number, "title": "t", "body": fx.issueBody()},
			"comment":    map[string]any{"id": id, "body": body, "user": map[string]any{"login": user}},
			"repository": repoPayload(),
			"sender":     map[string]any{"login": user},
		}
	}
	result := fx.inbound(t, fx.delivery("issue_comment", comment(9, "@traceway fix", "octodev", 1), webhookSecret))
	if result.Requests != 1 {
		t.Fatalf("fix comment = %+v", result)
	}
	attempt := fx.attempt(t)

	reply := fx.inbound(t, fx.delivery("issue_comment", comment(9, "@traceway it is Postgres 16", "octodev", 2), webhookSecret))
	if reply.Messages != 1 {
		t.Fatalf("reply = %+v", reply)
	}
	messages, _ := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.AgentMessage, error) {
		return transactional.AgentMessageRepository.ListAfter(tx, attempt.Id, 0, "", 10)
	})
	if len(messages) != 1 || messages[0].Body != "it is Postgres 16" || messages[0].Provider != Provider || messages[0].ExternalRef != "2" {
		t.Fatalf("messages = %+v", messages)
	}
	if again := fx.inbound(t, fx.delivery("issue_comment", comment(9, "@traceway it is Postgres 16", "octodev", 2), webhookSecret)); again.Messages != 0 {
		t.Fatalf("redelivery accepted: %+v", again)
	}
	if plain := fx.inbound(t, fx.delivery("issue_comment", comment(9, "nice", "octodev", 3), webhookSecret)); plain.Messages != 0 || plain.Ignored != 0 {
		t.Fatalf("comment without mention = %+v", plain)
	}
	if stranger := fx.inbound(t, fx.delivery("issue_comment", comment(9, "@traceway hello", "stranger", 4), webhookSecret)); stranger.Messages != 0 || stranger.Ignored != 1 {
		t.Fatalf("unmapped reply = %+v", stranger)
	}
	if elsewhere := fx.inbound(t, fx.delivery("issue_comment", comment(500, "@traceway hello", "octodev", 5), webhookSecret)); elsewhere.Messages != 0 {
		t.Fatalf("comment on an unlinked issue = %+v", elsewhere)
	}
}

func TestPullRequestClosedMergesAndArchives(t *testing.T) {
	fx := setup(t)
	fx.inbound(t, fx.delivery("issues", issuePayload("labeled", "traceway", fx.issueBody(), "octodev"), webhookSecret))
	attempt := fx.attempt(t)
	if err := telemetry.ExceptionStackTraceRepository.InsertAsync(context.Background(), []models.ExceptionStackTrace{{ProjectId: fx.project.Id, TraceType: "endpoint", ExceptionHash: testHash, StackTrace: "boom", RecordedAt: time.Now().UTC(), Attributes: map[string]string{}}}); err != nil {
		t.Fatal(err)
	}
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		now := time.Now().UTC()
		for _, to := range []string{models.AttemptClaimed, models.AttemptPreparing, models.AttemptRunning, models.AttemptVerifying, models.AttemptPublishing, models.AttemptAwaitingReview} {
			if to == models.AttemptClaimed {
				if _, err := transactional.AgentAttemptRepository.Claim(tx, attempt.Id, "test", now.Add(time.Hour), now); err != nil {
					return struct{}{}, err
				}
				continue
			}
			if err := agent.Transition(tx, attempt.Id, to, now); err != nil {
				return struct{}{}, err
			}
		}
		_, err := agent.RecordLink(tx, attempt.Id, agent.Link{Provider: Provider, Kind: models.LinkKindPR, ExternalRef: "acme/app#42", URL: "https://github.com/acme/app/pull/42", IntegrationId: fx.in.Id}, now)
		return struct{}{}, err
	})
	if err != nil {
		t.Fatal(err)
	}

	closed := func(merged bool) map[string]any {
		return map[string]any{"action": "closed", "pull_request": map[string]any{"number": 42, "title": "Fix", "merged": merged}, "repository": repoPayload(), "sender": map[string]any{"login": "octodev"}}
	}
	result := fx.inbound(t, fx.delivery("pull_request", closed(true), webhookSecret))
	if result.Events != 1 {
		t.Fatalf("merge = %+v", result)
	}
	if row := fx.reload(t, attempt); row.Status != models.AttemptMerged {
		t.Fatalf("status = %s", row.Status)
	}
	archived, err := telemetry.ExceptionStackTraceRepository.IsArchived(context.Background(), fx.project.Id, testHash)
	if err != nil || !archived {
		t.Fatalf("archived = %v, %v", archived, err)
	}
	if again := fx.inbound(t, fx.delivery("pull_request", closed(false), webhookSecret)); again.Events != 0 || again.Ignored != 1 {
		t.Fatalf("second close = %+v", again)
	}
}

func TestInstallationTokensAreMintedAndCached(t *testing.T) {
	fx := setup(t)
	repo := &models.Repository{Owner: "acme", Name: "app"}
	first, err := fx.host.CloneCredential(context.Background(), fx.in, repo)
	if err != nil || !strings.HasPrefix(first.Password, "ghs_") || first.Username != "x-access-token" || first.ExpiresAt.IsZero() {
		t.Fatalf("credential = %+v, %v", first, err)
	}
	second, _ := fx.host.CloneCredential(context.Background(), fx.in, repo)
	if second.Password != first.Password || fx.fake.tokenCalls() != 1 {
		t.Fatalf("token not cached: %s vs %s (%d mints)", first.Password, second.Password, fx.fake.tokenCalls())
	}
	other, _ := fx.host.CloneCredential(context.Background(), fx.in, &models.Repository{Owner: "acme", Name: "other"})
	if other.Password == first.Password || fx.fake.tokenCalls() != 2 {
		t.Fatalf("tokens must be per repository: %d mints", fx.fake.tokenCalls())
	}

	fx.fake.mu.Lock()
	fx.fake.ttl = 30 * time.Second
	fx.fake.mu.Unlock()
	fx.host.tokens = newTokenCache()
	short, _ := fx.host.CloneCredential(context.Background(), fx.in, repo)
	renewed, _ := fx.host.CloneCredential(context.Background(), fx.in, repo)
	if short.Password == renewed.Password {
		t.Fatal("a token inside the refresh margin must be minted again")
	}

	link, err := fx.host.OpenPullRequest(context.Background(), fx.in, repo, agent.PullRequest{Title: "Fix", Head: "traceway/fix-1", Base: "main", Draft: true})
	if err != nil || link.ExternalRef != "acme/app#42" {
		t.Fatalf("pull request = %+v, %v", link, err)
	}
	channel := channelView{fx.host}
	thread, err := channel.Open(context.Background(), fx.in, &models.AgentAttempt{}, &agent.Link{Provider: Provider, ExternalRef: "acme/app#9", URL: "https://github.com/acme/app/issues/9"})
	if err != nil || thread.Kind != models.LinkKindThread || thread.ExternalRef != "acme/app#9" {
		t.Fatalf("open = %+v, %v", thread, err)
	}
	if _, err := channel.Open(context.Background(), fx.in, &models.AgentAttempt{}, nil); err == nil {
		t.Fatal("a GitHub thread needs an origin")
	}
	author := agent.Identity{Display: "Dev", Provider: "web"}
	if _, err := channel.Post(context.Background(), fx.in, thread, agent.Message{Provider: "web", Kind: models.MessageKindAnswer, Body: "Postgres 16", Author: &author}); err != nil {
		t.Fatal(err)
	}
	if _, err := channel.Post(context.Background(), fx.in, thread, agent.Message{Provider: agent.ProviderAgent, Kind: models.MessageKindQuestion, Body: "Which db?"}); err != nil {
		t.Fatal(err)
	}
	fx.fake.mu.Lock()
	comments := append([]string{}, fx.fake.comments...)
	fx.fake.mu.Unlock()
	if len(comments) != 2 || !strings.HasSuffix(comments[0], "_Dev via web_\n\nPostgres 16") || !strings.Contains(comments[1], "**The agent needs an answer.**") {
		t.Fatalf("comments = %v", comments)
	}
	if !strings.HasPrefix(comments[0], "/repos/acme/app/issues/9/comments") {
		t.Fatalf("comment path = %s", comments[0])
	}
}

func TestManifestFlow(t *testing.T) {
	fx := setup(t)
	start, err := StartManifest("https://tw.example.com", fx.in.OrganizationId, 0)
	if err != nil || !strings.HasPrefix(start.URL, "https://tw.example.com/api/integrations/github/manifest/start?state=") {
		t.Fatalf("start = %+v, %v", start, err)
	}
	state := strings.TrimPrefix(start.URL, "https://tw.example.com/api/integrations/github/manifest/start?state=")
	page, err := ManifestPage("https://tw.example.com", state)
	if err != nil || !strings.Contains(page, "https://github.com/settings/apps/new?state=") {
		t.Fatalf("page: %v", err)
	}
	decoded, _ := url.QueryUnescape(page[strings.Index(page, `name="manifest" value="`)+len(`name="manifest" value="`):])
	if !strings.Contains(decoded, "/api/integrations/github/inbound/0") && !strings.Contains(page, "inbound") {
		t.Fatalf("manifest lacks the webhook route: %s", page)
	}
	if _, err := ManifestPage("https://tw.example.com", state+"x"); err == nil {
		t.Fatal("tampered state accepted")
	}
	if _, err := fx.host.CompleteManifest(context.Background(), "code123", state+"x", fx.ownerId); err == nil {
		t.Fatal("tampered state accepted on callback")
	}
	installURL, err := fx.host.CompleteManifest(context.Background(), "code123", state, fx.ownerId)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(installURL, "/apps/traceway-test/installations/new") {
		t.Fatalf("install url = %s", installURL)
	}
	rows, _ := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.Integration, error) {
		return transactional.IntegrationRepository.FindByOrganization(tx, fx.in.OrganizationId)
	})
	var created *models.Integration
	for _, row := range rows {
		if row.Id != fx.in.Id {
			created = row
		}
	}
	if created == nil || created.Name != "GitHub App traceway-test" || created.CreatedBy == nil || *created.CreatedBy != fx.ownerId {
		t.Fatalf("created = %+v", created)
	}
	s, err := fx.host.settings(created)
	if err != nil || s.Mode != ModeApp || s.AppId != "4242" || s.WebhookSecret != webhookSecret || s.ClientSecret != "cs_secret" || s.PrivateKey != fx.fake.pem || s.InstallationId != "" {
		t.Fatalf("stored config = %+v, %v", s, err)
	}
	if fx.host.Describe(created) != "GitHub App created, not installed" {
		t.Fatalf("describe = %s", fx.host.Describe(created))
	}
	if !strings.Contains(string(created.Config), "v1:") || strings.Contains(string(created.Config), webhookSecret) {
		t.Fatalf("secrets stored in plaintext: %s", created.Config)
	}
}
