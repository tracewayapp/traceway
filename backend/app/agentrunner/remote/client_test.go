//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package remote_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/agent"
	"github.com/tracewayapp/traceway/backend/app/agentrunner"
	"github.com/tracewayapp/traceway/backend/app/agentrunner/agents"
	"github.com/tracewayapp/traceway/backend/app/agentrunner/remote"
	"github.com/tracewayapp/traceway/backend/app/agentrunner/workspace"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/controllers"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/dbtest"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/sandbox"
	"github.com/tracewayapp/traceway/backend/app/secrets"
	"github.com/tracewayapp/traceway/backend/app/services"
	"github.com/tracewayapp/traceway/backend/app/storage"
)

const runnerSecret = "runner-secret-for-tests"

type fakeHost struct {
	remote string
	mu     sync.Mutex
	pulls  []agent.PullRequest
}

func (h *fakeHost) Provider() string { return "fakehost" }
func (h *fakeHost) Kinds() []string  { return []string{agent.KindCodeHost} }
func (h *fakeHost) Fields() []agent.Field {
	return []agent.Field{{Key: "token", Kind: agent.FieldSecret}}
}
func (h *fakeHost) SetupFlow() *agent.SetupFlow        { return nil }
func (h *fakeHost) Validate(map[string]string) error   { return nil }
func (h *fakeHost) CloneURL(*models.Repository) string { return h.remote }
func (h *fakeHost) Comment(context.Context, *models.Integration, agent.Link, string) error {
	return nil
}
func (h *fakeHost) Inbound(context.Context, *models.Integration, *http.Request) ([]agent.CodeHostEvent, error) {
	return nil, nil
}
func (h *fakeHost) CloneCredential(context.Context, *models.Integration, *models.Repository) (agent.GitCredential, error) {
	return agent.GitCredential{Username: "x-access-token", Password: "pat-for-one-push", ExpiresAt: time.Now().Add(time.Hour)}, nil
}
func (h *fakeHost) OpenPullRequest(_ context.Context, in *models.Integration, repo *models.Repository, pr agent.PullRequest) (agent.Link, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pulls = append(h.pulls, pr)
	return agent.Link{Provider: "fakehost", Kind: models.LinkKindPR, ExternalRef: repo.Owner + "/" + repo.Name + "#1", URL: "https://fakehost/pull/1", IntegrationId: in.Id}, nil
}

func (h *fakeHost) pullCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.pulls)
}

type fixture struct {
	t       *testing.T
	server  *httptest.Server
	host    *fakeHost
	project *models.Project
	userId  int
	root    string
	script  string
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %v: %s", args, err, out)
	}
}

// setup starts a backend in remote mode with the runner routes mounted, a
// project bound to a local bare repository through the fake host, and a
// default profile on the scripted command agent.
func setup(t *testing.T) *fixture {
	t.Helper()
	config.LoggingEnabled = false
	t.Cleanup(func() { config.LoggingEnabled = true })
	dbtest.SetupSQLite(t)
	config.Config.AgentMode = "remote"
	config.Config.AgentRunnerSecret = runnerSecret
	config.Config.JWTSecret = "test-secret-that-is-long-enough-for-jwt-0123"
	t.Cleanup(func() {
		config.Config.AgentMode = ""
		config.Config.AgentRunnerSecret = ""
	})
	if err := services.InitJWT(); err != nil {
		t.Fatal(err)
	}
	key, _, _ := secrets.GenerateKey()
	secrets.Init(key)
	t.Cleanup(func() { secrets.Init(nil) })
	local, err := storage.NewLocalStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	prev := storage.Store
	storage.Store = local
	t.Cleanup(func() { storage.Store = prev })

	root := t.TempDir()
	work := filepath.Join(root, "work")
	run(t, "", "git", "init", "--quiet", "-b", "main", work)
	run(t, work, "git", "config", "user.email", "t@example.com")
	run(t, work, "git", "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(work, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, work, "git", "add", "-A")
	run(t, work, "git", "commit", "--quiet", "-m", "init")
	remoteRepo := filepath.Join(root, "upstream.git")
	run(t, "", "git", "clone", "--quiet", "--bare", work, remoteRepo)
	host := &fakeHost{remote: remoteRepo}
	agent.RegisterCodeHost(host)

	fx := &fixture{t: t, host: host, root: root, script: filepath.Join(root, "agent.sh")}
	_, err = db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		user, err := transactional.UserRepository.Create(tx, "dev@example.com", "Dev", "hashed")
		if err != nil {
			return struct{}{}, err
		}
		org, err := transactional.OrganizationRepository.Create(tx, "Org", "UTC")
		if err != nil {
			return struct{}{}, err
		}
		if _, err := transactional.OrganizationRepository.AddUser(tx, org.Id, user.Id, "owner"); err != nil {
			return struct{}{}, err
		}
		project, err := transactional.ProjectRepository.CreateWithOrganization(tx, "Project", "opentelemetry", org.Id)
		if err != nil {
			return struct{}{}, err
		}
		now := time.Now().UTC()
		integrationId, err := transactional.IntegrationRepository.Create(tx, &models.Integration{OrganizationId: org.Id, Provider: "fakehost", Kinds: models.StringSlice{agent.KindCodeHost}, Name: "fake", Config: models.JSONText(`{}`), Enabled: true, CreatedAt: now, UpdatedAt: now})
		if err != nil {
			return struct{}{}, err
		}
		if _, err := transactional.RepositoryRepository.Create(tx, &models.Repository{ProjectId: project.Id, IntegrationId: &integrationId, Owner: "acme", Name: "app", DefaultBranch: "main", CreatedAt: now, UpdatedAt: now}); err != nil {
			return struct{}{}, err
		}
		credential, _ := secrets.Encrypt([]byte("provider-key"))
		profileId, err := transactional.AgentProfileRepository.Create(tx, &models.AgentProfile{OrganizationId: org.Id, Name: "Scripted", Agent: "command", Credential: credential, NetworkPolicy: models.JSONText(`{}`), AllowedTools: models.StringSlice{}, IsDefault: true, CreatedAt: now, UpdatedAt: now})
		if err != nil {
			return struct{}{}, err
		}
		if err := transactional.AgentProfileRepository.SetDefault(tx, org.Id, profileId, now); err != nil {
			return struct{}{}, err
		}
		fx.project, fx.userId = project, user.Id
		return struct{}{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	middleware.InitUseAgentRunnerAuth()
	controllers.AgentRunnerPollWindow = 300 * time.Millisecond
	t.Cleanup(func() { controllers.AgentRunnerPollWindow = 25 * time.Second })
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	controllers.RegisterAgentRunnerRoutes(engine.Group("/api"))
	fx.server = httptest.NewServer(engine)
	t.Cleanup(fx.server.Close)
	return fx
}

func (fx *fixture) client(name string) *remote.Client {
	return remote.New(fx.server.URL, runnerSecret, name, "test", remote.Capabilities{SchemaVersion: 1, Agents: []string{"command"}, Sandbox: "off", Workers: 2})
}

func (fx *fixture) runner(name string) *agentrunner.Runner {
	client := fx.client(name)
	return &agentrunner.Runner{
		Claimer:     client,
		Reporter:    client,
		Backend:     sandbox.Off{},
		Mirrors:     &workspace.Mirrors{Root: filepath.Join(fx.root, "mirrors-"+name)},
		WorkRoot:    filepath.Join(fx.root, "work-"+name),
		Agents:      map[string]agents.Agent{"command": agents.Command{Argv: []string{"/bin/sh", fx.script}}},
		InstanceURL: fx.server.URL,
		HostTools:   true,
		Limits:      agentrunner.DefaultLimits,
	}
}

func (fx *fixture) scriptAgent(body string) {
	fx.t.Helper()
	script := "#!/bin/sh\nset -e\nprompt=$(cat)\n" + body + "\n"
	if err := os.WriteFile(fx.script, []byte(script), 0o755); err != nil {
		fx.t.Fatal(err)
	}
}

func (fx *fixture) start(hash string) *models.AgentAttempt {
	fx.t.Helper()
	started, err := db.ExecuteTransaction(func(tx *sql.Tx) (*agent.StartResult, error) {
		return agent.StartAttempt(tx, fx.project, agent.Subject{Kind: models.SubjectKindTracewayException, Ref: hash, ProjectId: fx.project.Id}, agent.StartOptions{RequestedBy: &fx.userId})
	})
	if err != nil {
		fx.t.Fatal(err)
	}
	return started.Attempt
}

func (fx *fixture) reload(id uuid.UUID) *models.AgentAttempt {
	fx.t.Helper()
	loaded, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.AgentAttempt, error) {
		return transactional.AgentAttemptRepository.FindById(tx, id)
	})
	if err != nil || loaded == nil {
		fx.t.Fatalf("reload %s: %v", id, err)
	}
	return loaded
}

func (fx *fixture) eventKinds(id uuid.UUID) []string {
	fx.t.Helper()
	rows, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.AgentAttemptEvent, error) {
		return transactional.AgentAttemptEventRepository.ListAfter(tx, id, 0, 500)
	})
	if err != nil {
		fx.t.Fatal(err)
	}
	kinds := make([]string, 0, len(rows))
	for _, row := range rows {
		kinds = append(kinds, row.Kind)
	}
	return kinds
}

func (fx *fixture) blob(id uuid.UUID, name string) string {
	content, err := storage.Store.Read(context.Background(), agent.BlobKey(id, name))
	if err != nil {
		return ""
	}
	return string(content)
}

const fixedScript = `printf 'package main\n\nfunc main() {}\n' > main.go
echo "---REPORT---"
echo "STATUS: fixed"
echo "SUBJECT: $SUBJECT"
echo "Fixed the nil dereference in main."`

func TestTwoRunnersNeverClaimTheSameAttempt(t *testing.T) {
	fx := setup(t)
	ids := map[uuid.UUID]bool{}
	for _, hash := range []string{"0123456789abcdef", "123456789abcdef0", "23456789abcdef01"} {
		ids[fx.start(hash).Id] = true
	}
	var mu sync.Mutex
	claimed := map[uuid.UUID]string{}
	var wg sync.WaitGroup
	for _, name := range []string{"alpha", "beta"} {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			got, err := fx.client(name).Claim(context.Background(), 2)
			if err != nil {
				t.Errorf("%s: %v", name, err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			for _, attempt := range got {
				if previous, dup := claimed[attempt.Id]; dup {
					t.Errorf("attempt %s claimed by both %s and %s", attempt.Id, previous, name)
				}
				claimed[attempt.Id] = name
			}
		}(name)
	}
	wg.Wait()
	for id := range claimed {
		if !ids[id] {
			t.Fatalf("unknown attempt %s claimed", id)
		}
		row := fx.reload(id)
		if row.Status != models.AttemptClaimed || !strings.HasPrefix(row.ClaimedBy, "runner:"+claimed[id]+"/") {
			t.Fatalf("attempt %s = %s by %q, want claimed by runner:%s", id, row.Status, row.ClaimedBy, claimed[id])
		}
	}
	if len(claimed) != 3 {
		t.Fatalf("claimed %d of 3", len(claimed))
	}
	empty, err := fx.client("gamma").Claim(context.Background(), 1)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty queue poll = %v, %v", empty, err)
	}
}

func TestPollClampsAndRefusesOutsideRemoteMode(t *testing.T) {
	fx := setup(t)
	for i := 0; i < 9; i++ {
		fx.start(strings.Repeat(string(rune('a'+i)), 16))
	}
	got, err := fx.client("big").Claim(context.Background(), 50)
	if err != nil || len(got) != 8 {
		t.Fatalf("claim(50) = %d, %v; want the cap of 8", len(got), err)
	}
	if _, err := remote.New(fx.server.URL, "wrong", "x", "test", remote.Capabilities{}).Claim(context.Background(), 1); err != remote.ErrUnauthorized {
		t.Fatalf("wrong secret = %v", err)
	}
	config.Config.AgentMode = "embedded"
	_, err = fx.client("late").Claim(context.Background(), 1)
	config.Config.AgentMode = "remote"
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("embedded mode poll = %v", err)
	}
}

func TestEventsRenewTheLeaseAndReclaimEndsIt(t *testing.T) {
	fx := setup(t)
	attempt := fx.start("0123456789abcdef")
	alpha := fx.client("alpha")
	got, err := alpha.Claim(context.Background(), 1)
	if err != nil || len(got) != 1 {
		t.Fatalf("claim = %v, %v", got, err)
	}
	before := fx.reload(attempt.Id).LeaseExpiresAt
	time.Sleep(1100 * time.Millisecond)
	if err := alpha.Events(context.Background(), attempt.Id, []agents.Event{{SchemaVersion: 1, Kind: agents.EventAssistantText, Text: "looking", At: time.Now().UTC()}}); err != nil {
		t.Fatal(err)
	}
	after := fx.reload(attempt.Id).LeaseExpiresAt
	if before == nil || after == nil || !after.After(*before) {
		t.Fatalf("lease not renewed: %v -> %v", before, after)
	}
	if kinds := fx.eventKinds(attempt.Id); kinds[len(kinds)-1] != agents.EventAssistantText {
		t.Fatalf("events = %v", kinds)
	}
	held, err := alpha.Renew(context.Background(), attempt.Id)
	if err != nil || !held {
		t.Fatalf("renew = %v, %v", held, err)
	}

	reclaimed, err := db.ExecuteTransaction(func(tx *sql.Tx) (int64, error) {
		return agent.ReclaimStale(tx, time.Now().UTC().Add(agent.LeaseDuration+time.Minute))
	})
	if err != nil || reclaimed != 1 {
		t.Fatalf("reclaim = %d, %v", reclaimed, err)
	}
	if err := alpha.Events(context.Background(), attempt.Id, nil); err != agentrunner.ErrClaimLost {
		t.Fatalf("events after reclaim = %v", err)
	}
	if held, err := alpha.Renew(context.Background(), attempt.Id); err != nil || held {
		t.Fatalf("renew after reclaim = %v, %v", held, err)
	}
	if err := alpha.Finish(context.Background(), attempt.Id, agentrunner.Outcome{Status: agents.StatusError, Error: "late"}); err != nil {
		t.Fatalf("late result should be a no-op, got %v", err)
	}
	row := fx.reload(attempt.Id)
	if row.Status != models.AttemptQueued || !row.Resume || row.ClaimedBy != "" {
		t.Fatalf("reclaimed attempt = %+v", row)
	}
}

func TestRemoteRunnerCompletesAnAttemptThroughTheProtocol(t *testing.T) {
	fx := setup(t)
	fx.scriptAgent(strings.ReplaceAll(fixedScript, "$SUBJECT", "0123456789abcdef"))
	attempt := fx.start("0123456789abcdef")
	runner := fx.runner("alpha")
	claimed, err := runner.Claimer.Claim(context.Background(), 1)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim = %v, %v", claimed, err)
	}
	runner.Run(context.Background(), claimed[0])

	row := fx.reload(attempt.Id)
	if row.Status != models.AttemptAwaitingReview || !strings.HasPrefix(row.ClaimedBy, "runner:alpha/") || row.Executor != "runner:alpha" {
		t.Fatalf("attempt = %s by %q executor %q (error %q)", row.Status, row.ClaimedBy, row.Executor, row.Error)
	}
	if row.FixBranch != "traceway/fix-"+row.Id.String() || row.ReportKey == "" {
		t.Fatalf("outcome = %+v", row)
	}
	if fx.host.pullCount() != 1 || !fx.host.pulls[0].Draft || fx.host.pulls[0].Head != row.FixBranch {
		t.Fatalf("pulls = %+v", fx.host.pulls)
	}
	if !workspace.HasBranch(context.Background(), fx.host.remote, row.FixBranch) {
		t.Fatal("fix branch was not pushed")
	}
	if !strings.Contains(fx.blob(attempt.Id, agent.BlobDiff), "func main() {}") || !strings.Contains(fx.blob(attempt.Id, agent.BlobReport), "nil dereference") || fx.blob(attempt.Id, agent.BlobTranscript) == "" {
		t.Fatalf("blobs missing: diff=%q report=%q", fx.blob(attempt.Id, agent.BlobDiff), fx.blob(attempt.Id, agent.BlobReport))
	}
	kinds := strings.Join(fx.eventKinds(attempt.Id), ",")
	for _, want := range []string{"created", "status", "message"} {
		if !strings.Contains(kinds, want) {
			t.Fatalf("events %s lack %s", kinds, want)
		}
	}
	messages, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.AgentMessage, error) {
		return transactional.AgentMessageRepository.ListAfter(tx, attempt.Id, 0, "", 10)
	})
	if err != nil || len(messages) != 1 || messages[0].Kind != models.MessageKindPR {
		t.Fatalf("messages = %+v, %v", messages, err)
	}
}

func TestSecondRunnerFinishesWhatADeadRunnerLeft(t *testing.T) {
	fx := setup(t)
	fx.scriptAgent(strings.ReplaceAll(fixedScript, "$SUBJECT", "0123456789abcdef"))
	attempt := fx.start("0123456789abcdef")

	dead := fx.client("dead")
	claimed, err := dead.Claim(context.Background(), 1)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim = %v, %v", claimed, err)
	}
	if err := dead.Transition(context.Background(), attempt.Id, models.AttemptPreparing); err != nil {
		t.Fatal(err)
	}
	if err := dead.Events(context.Background(), attempt.Id, []agents.Event{{SchemaVersion: 1, Kind: agents.EventAssistantText, Text: "halfway", At: time.Now().UTC()}}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecuteTransaction(func(tx *sql.Tx) (int64, error) {
		return agent.ReclaimStale(tx, time.Now().UTC().Add(agent.LeaseDuration+time.Minute))
	}); err != nil {
		t.Fatal(err)
	}

	survivor := fx.runner("survivor")
	again, err := survivor.Claimer.Claim(context.Background(), 1)
	if err != nil || len(again) != 1 || !again[0].Resume {
		t.Fatalf("survivor claim = %+v, %v", again, err)
	}
	survivor.Run(context.Background(), again[0])

	row := fx.reload(attempt.Id)
	if row.Status != models.AttemptAwaitingReview || !strings.HasPrefix(row.ClaimedBy, "runner:survivor/") {
		t.Fatalf("attempt = %s by %q (error %q)", row.Status, row.ClaimedBy, row.Error)
	}
	kinds := strings.Join(fx.eventKinds(attempt.Id), ",")
	if !strings.Contains(kinds, "assistant_text,reclaimed") || !strings.HasSuffix(kinds, "message,status") {
		t.Fatalf("events = %s", kinds)
	}
	if err := dead.Transition(context.Background(), attempt.Id, models.AttemptRunning); err != agentrunner.ErrClaimLost {
		t.Fatalf("dead runner transition = %v", err)
	}
}
