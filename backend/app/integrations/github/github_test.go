package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/backend/app/agent"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/secrets"
)

func encryptedIntegration(t *testing.T, token string) *models.Integration {
	t.Helper()
	key, _, err := secrets.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	secrets.Init(key)
	t.Cleanup(func() { secrets.Init(nil) })
	config, err := secrets.EncryptFields(json.RawMessage(`{"token":"`+token+`"}`), []string{"token"})
	if err != nil {
		t.Fatal(err)
	}
	return &models.Integration{Id: 4, OrganizationId: 1, Provider: Provider, Config: models.JSONText(config), Enabled: true}
}

func TestReadyRequiresConnectedCredentials(t *testing.T) {
	for _, tc := range []struct {
		name   string
		config string
		ready  bool
	}{
		{"missing token", `{}`, false},
		{"blank token", `{"token":"  "}`, false},
		{"PAT", `{"token":"test-pat"}`, true},
		{"uncreated app", `{"mode":"app"}`, false},
		{"uninstalled app", `{"mode":"app","appId":"1","privateKey":"key"}`, false},
		{"missing app key", `{"mode":"app","appId":"1","installationId":"2"}`, false},
		{"installed app", `{"mode":"app","appId":"1","installationId":"2","privateKey":"key"}`, true},
		{"malformed config", `{`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := New().Ready(&models.Integration{Config: models.JSONText(tc.config)}); got != tc.ready {
				t.Fatalf("Ready = %v, want %v", got, tc.ready)
			}
		})
	}
	integration := encryptedIntegration(t, "test-pat")
	if !New().Ready(integration) {
		t.Fatal("encrypted token must be accepted")
	}
	secrets.Init(nil)
	if New().Ready(integration) {
		t.Fatal("an unreadable token must fail readiness")
	}
}

func TestOpenPullRequestAndComment(t *testing.T) {
	var requests []string
	var bodies []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/repos/acme/app/pulls" {
			w.Write([]byte(`[]`))
			return
		}
		requests = append(requests, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization"))
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		json.Unmarshal(raw, &body)
		bodies = append(bodies, body)
		switch r.URL.Path {
		case "/repos/acme/app/pulls":
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"number":42,"html_url":"https://github.com/acme/app/pull/42"}`))
		case "/repos/acme/app/issues/42/comments":
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id":1}`))
		case "/repos/acme/app/issues":
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"number":7,"html_url":"https://github.com/acme/app/issues/7"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"message":"Not Found"}`))
		}
	}))
	defer server.Close()

	host := New()
	host.APIBase = server.URL
	integration := encryptedIntegration(t, "ghp_secret")
	repo := &models.Repository{Owner: "acme", Name: "app", DefaultBranch: "main"}

	cred, err := host.CloneCredential(context.Background(), integration, repo)
	if err != nil || cred.Username != "x-access-token" || cred.Password != "ghp_secret" {
		t.Fatalf("credential = %+v, %v", cred, err)
	}
	if host.CloneURL(repo) != "https://github.com/acme/app.git" {
		t.Fatalf("clone url = %s", host.CloneURL(repo))
	}

	link, err := host.OpenPullRequest(context.Background(), integration, repo, agent.PullRequest{Title: "Fix", Body: "report", Head: "traceway/fix-1", Base: "main", Draft: true})
	if err != nil {
		t.Fatal(err)
	}
	if link.Kind != models.LinkKindPR || link.ExternalRef != "acme/app#42" || link.URL != "https://github.com/acme/app/pull/42" || link.IntegrationId != 4 {
		t.Fatalf("link = %+v", link)
	}
	if bodies[0]["draft"] != true || bodies[0]["head"] != "traceway/fix-1" || bodies[0]["base"] != "main" {
		t.Fatalf("pull request body = %v", bodies[0])
	}
	if err := host.Comment(context.Background(), integration, link, "finding"); err != nil {
		t.Fatal(err)
	}
	if bodies[1]["body"] != "finding" {
		t.Fatalf("comment body = %v", bodies[1])
	}
	issue, err := host.OpenIssue(context.Background(), integration, repo, agent.Issue{Title: "t", Body: "b"})
	if err != nil || issue.ExternalRef != "acme/app#7" || issue.Kind != models.LinkKindIssue {
		t.Fatalf("issue = %+v, %v", issue, err)
	}
	for _, request := range requests {
		if !strings.HasSuffix(request, "Bearer ghp_secret") {
			t.Fatalf("every call must carry the decrypted token: %s", request)
		}
	}

	if err := host.Comment(context.Background(), integration, agent.Link{ExternalRef: "acme/app#999"}, "x"); err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("a failed call must surface the status: %v", err)
	}
	if err := host.Comment(context.Background(), integration, agent.Link{ExternalRef: "garbage"}, "x"); err == nil {
		t.Fatal("a malformed ref must be refused")
	}
}

func TestValidateAndFields(t *testing.T) {
	host := New()
	if err := host.Validate(map[string]string{"token": " "}); err == nil {
		t.Fatal("an empty token must be refused")
	}
	if err := host.Validate(map[string]string{"token": "ghp_x"}); err != nil {
		t.Fatal(err)
	}
	if fields := agent.SecretFields(host); strings.Join(fields, ",") != "token,clientSecret,privateKey,webhookSecret" {
		t.Fatalf("secret fields = %v", fields)
	}
	if err := host.Validate(map[string]string{"mode": "app", "appId": "x"}); err == nil {
		t.Fatal("a non-numeric app id must be refused")
	}
	if err := host.Validate(map[string]string{"mode": "app"}); err != nil {
		t.Fatalf("an app integration before the manifest flow must be accepted: %v", err)
	}
	if err := host.Validate(map[string]string{"mode": "pat", "token": "ghp_x", "label": "bad label!"}); err == nil {
		t.Fatal("a label with punctuation must be refused")
	}
	if _, err := host.CloneCredential(context.Background(), &models.Integration{Config: models.JSONText(`{}`)}, nil); err == nil {
		t.Fatal("an integration without a token must fail")
	}
}
