//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package agentrunner

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/app/agent"
	"github.com/tracewayapp/traceway/backend/app/agentrunner/agents"
	"github.com/tracewayapp/traceway/backend/app/agentrunner/workspace"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/dbtest"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/sandbox"
	"github.com/tracewayapp/traceway/backend/app/secrets"
	"github.com/tracewayapp/traceway/backend/app/storage"
)

// fakeHost is the code host of the tests: clone URLs are local bare
// repositories and pull requests are recorded instead of opened.
type fakeHost struct {
	remote string
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
	return agent.GitCredential{}, nil
}
func (h *fakeHost) OpenPullRequest(_ context.Context, in *models.Integration, repo *models.Repository, pr agent.PullRequest) (agent.Link, error) {
	h.pulls = append(h.pulls, pr)
	return agent.Link{Provider: "fakehost", Kind: models.LinkKindPR, ExternalRef: repo.Owner + "/" + repo.Name + "#1", URL: "https://fakehost/pull/1", IntegrationId: in.Id}, nil
}

type harness struct {
	t       *testing.T
	host    *fakeHost
	project *models.Project
	userId  int
	runner  *Runner
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

// setupHarness builds a bare upstream with one commit, a project bound to it
// through the fake host, a default profile on the command agent, and a
// Runner on the unconfined backend.
func setupHarness(t *testing.T, testCommand string) *harness {
	t.Helper()
	config.LoggingEnabled = false
	t.Cleanup(func() { config.LoggingEnabled = true })
	dbtest.SetupSQLite(t)
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
	remote := filepath.Join(root, "upstream.git")
	run(t, "", "git", "clone", "--quiet", "--bare", work, remote)

	host := &fakeHost{remote: remote}
	agent.RegisterCodeHost(host)

	h := &harness{t: t, host: host, script: filepath.Join(root, "agent.sh")}
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
		if _, err := transactional.RepositoryRepository.Create(tx, &models.Repository{ProjectId: project.Id, IntegrationId: &integrationId, Owner: "acme", Name: "app", DefaultBranch: "main", TestCommand: testCommand, CreatedAt: now, UpdatedAt: now}); err != nil {
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
		h.project, h.userId = project, user.Id
		return struct{}{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	control := Local{Executor: "test", InstanceURL: "http://localhost:18082"}
	h.runner = &Runner{
		Claimer:     control,
		Reporter:    control,
		Backend:     sandbox.Off{},
		Mirrors:     &workspace.Mirrors{Root: filepath.Join(root, "mirrors")},
		WorkRoot:    filepath.Join(root, "workspaces"),
		Agents:      map[string]agents.Agent{"command": agents.Command{Argv: []string{"/bin/sh", h.script}}},
		InstanceURL: "http://localhost:18082",
		HostTools:   true,
		Limits:      DefaultLimits,
	}
	return h
}

// scriptAgent writes the shell the fake agent runs: it reads the prompt from
// stdin and prints a transcript followed by the report.
func (h *harness) scriptAgent(body string) {
	h.t.Helper()
	script := "#!/bin/sh\nset -e\nprompt=$(cat)\n" + body + "\n"
	if err := os.WriteFile(h.script, []byte(script), 0o755); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) start(hash string) *models.AgentAttempt {
	h.t.Helper()
	started, err := db.ExecuteTransaction(func(tx *sql.Tx) (*agent.StartResult, error) {
		return agent.StartAttempt(tx, h.project, agent.Subject{Kind: models.SubjectKindTracewayException, Ref: hash, ProjectId: h.project.Id}, agent.StartOptions{RequestedBy: &h.userId})
	})
	if err != nil {
		h.t.Fatal(err)
	}
	return started.Attempt
}

// claimAndRun takes the next queued attempt the way a worker does and runs
// the pipeline to its end.
func (h *harness) claimAndRun() *models.AgentAttempt {
	h.t.Helper()
	claimed, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.AgentAttempt, error) {
		return agent.Claim(tx, "test", 1, time.Now().UTC())
	})
	if err != nil || len(claimed) != 1 {
		h.t.Fatalf("claim = %d, %v", len(claimed), err)
	}
	h.runner.Run(context.Background(), claimed[0])
	return h.reload(claimed[0])
}

func (h *harness) reload(attempt *models.AgentAttempt) *models.AgentAttempt {
	h.t.Helper()
	loaded, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.AgentAttempt, error) {
		return transactional.AgentAttemptRepository.FindById(tx, attempt.Id)
	})
	if err != nil {
		h.t.Fatal(err)
	}
	return loaded
}

func (h *harness) messages(attempt *models.AgentAttempt) []string {
	h.t.Helper()
	rows, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.AgentMessage, error) {
		return transactional.AgentMessageRepository.ListAfter(tx, attempt.Id, 0, "", 50)
	})
	if err != nil {
		h.t.Fatal(err)
	}
	var out []string
	for _, row := range rows {
		out = append(out, row.Direction+"/"+row.Kind+": "+row.Body)
	}
	return out
}

func (h *harness) eventKinds(attempt *models.AgentAttempt) []string {
	h.t.Helper()
	rows, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.AgentAttemptEvent, error) {
		return transactional.AgentAttemptEventRepository.ListAfter(tx, attempt.Id, 0, 500)
	})
	if err != nil {
		h.t.Fatal(err)
	}
	var kinds []string
	for _, row := range rows {
		kind := row.Kind
		if kind == agent.EventStatus {
			var payload struct {
				Status string `json:"status"`
			}
			json.Unmarshal(row.Payload, &payload)
			kind += ":" + payload.Status
		}
		kinds = append(kinds, kind)
	}
	return kinds
}

func (h *harness) blob(attempt *models.AgentAttempt, name string) string {
	content, err := storage.Store.Read(context.Background(), agent.BlobKey(attempt.Id, name))
	if err != nil {
		return ""
	}
	return string(content)
}

func (h *harness) remoteHasBranch(branch string) bool {
	return workspace.HasBranch(context.Background(), h.host.remote, branch)
}

const fixedReport = `echo "reading the trace"
printf 'package main\n\nfunc main() {}\n' > main.go
echo ---REPORT---
echo "STATUS: fixed"
echo "SUBJECT: 0123456789abcdef"
echo "Propagated the context."`

func TestPipelineFixedBecomesDraftPullRequest(t *testing.T) {
	h := setupHarness(t, "test -f main.go")
	h.scriptAgent(fixedReport)
	h.start("0123456789abcdef")
	attempt := h.claimAndRun()
	if attempt.Status != models.AttemptAwaitingReview {
		t.Fatalf("status = %s, error = %s", attempt.Status, attempt.Error)
	}
	if attempt.FixBranch != branchPrefix+attempt.Id.String() || attempt.Executor != "test" || attempt.ReportKey == "" {
		t.Fatalf("outcome = %+v", attempt)
	}
	if !h.remoteHasBranch(attempt.FixBranch) {
		t.Fatal("the fix branch must be pushed to the remote")
	}
	if len(h.host.pulls) != 1 || !h.host.pulls[0].Draft || h.host.pulls[0].Head != attempt.FixBranch || h.host.pulls[0].Base != "main" || !strings.Contains(h.host.pulls[0].Body, "Propagated the context.") {
		t.Fatalf("pull request = %+v", h.host.pulls)
	}
	if h.blob(attempt, agent.BlobReport) != "Propagated the context." || !strings.Contains(h.blob(attempt, agent.BlobDiff), "+func main() {}") || !strings.Contains(h.blob(attempt, agent.BlobTranscript), "reading the trace") {
		t.Fatalf("blobs: report=%q diff=%q transcript=%q", h.blob(attempt, agent.BlobReport), h.blob(attempt, agent.BlobDiff), h.blob(attempt, agent.BlobTranscript))
	}
	kinds := strings.Join(h.eventKinds(attempt), " ")
	for _, want := range []string{"created", "status:claimed", "status:preparing", "status:running", "assistant_text", "status:verifying", "status:publishing", "message", "status:awaiting_review"} {
		if !strings.Contains(kinds, want) {
			t.Errorf("events lack %s: %s", want, kinds)
		}
	}
	messages := h.messages(attempt)
	if len(messages) != 1 || !strings.HasPrefix(messages[0], "out/pr: Opened a draft pull request: https://fakehost/pull/1") {
		t.Fatalf("messages = %v", messages)
	}
	links, _ := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.AgentLink, error) {
		return transactional.AgentLinkRepository.FindByAttempt(tx, attempt.Id)
	})
	var kindsOfLinks []string
	for _, link := range links {
		kindsOfLinks = append(kindsOfLinks, link.Kind)
	}
	if strings.Join(kindsOfLinks, ",") != "pr" {
		t.Fatalf("links = %v", kindsOfLinks)
	}
}

func TestPipelineAnalysisEndsAnalyzed(t *testing.T) {
	h := setupHarness(t, "")
	h.scriptAgent(`echo "looked everywhere"
echo ---REPORT---
echo "STATUS: analysis"
echo "SUBJECT: 0123456789abcdef"
echo "Not a code bug."`)
	h.start("0123456789abcdef")
	attempt := h.claimAndRun()
	if attempt.Status != models.AttemptAnalyzed || attempt.FixBranch != "" || len(h.host.pulls) != 0 {
		t.Fatalf("attempt = %+v pulls=%d", attempt, len(h.host.pulls))
	}
	if messages := h.messages(attempt); len(messages) != 1 || messages[0] != "out/finding: Not a code bug." {
		t.Fatalf("messages = %v", messages)
	}
	if h.blob(attempt, agent.BlobReport) != "Not a code bug." {
		t.Fatalf("report = %q", h.blob(attempt, agent.BlobReport))
	}
}

func TestPipelineQuestionThenAnswerResumesToPullRequest(t *testing.T) {
	h := setupHarness(t, "")
	h.scriptAgent(`case "$prompt" in
  *Postgres*)
    printf 'package main\n\n// answered\n' > main.go
    echo ---REPORT---
    echo "STATUS: fixed"
    echo "SUBJECT: 0123456789abcdef"
    echo "Used the answer."
    ;;
  *)
    echo ---REPORT---
    echo "STATUS: question"
    echo "SUBJECT: 0123456789abcdef"
    echo "Which database backs checkout?"
    ;;
esac`)
	h.start("0123456789abcdef")
	attempt := h.claimAndRun()
	if attempt.Status != models.AttemptNeedsInput {
		t.Fatalf("status = %s, error = %s", attempt.Status, attempt.Error)
	}
	if messages := h.messages(attempt); len(messages) != 1 || messages[0] != "out/question: Which database backs checkout?" {
		t.Fatalf("messages = %v", messages)
	}
	author := agent.Identity{UserId: h.userId, Provider: "web"}
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.AgentMessage, error) {
		return agent.Post(tx, attempt, agent.Message{Direction: models.MessageInbound, Provider: "web", Kind: models.MessageKindAnswer, Body: "It is Postgres 16.", Author: &author}, nil, "", time.Now().UTC())
	})
	if err != nil {
		t.Fatal(err)
	}
	if requeued := h.reload(attempt); requeued.Status != models.AttemptQueued || !requeued.Resume {
		t.Fatalf("the answer must requeue the attempt with resume: %+v", requeued)
	}
	attempt = h.claimAndRun()
	if attempt.Status != models.AttemptAwaitingReview || len(h.host.pulls) != 1 {
		t.Fatalf("resumed run = %s (%s), pulls=%d", attempt.Status, attempt.Error, len(h.host.pulls))
	}
	if !strings.Contains(h.blob(attempt, agent.BlobTranscript), "Which database") {
		t.Fatal("the transcript of the first session must be kept across the resume")
	}
	messages := h.messages(attempt)
	if len(messages) != 3 || !strings.HasPrefix(messages[1], "in/answer: It is Postgres 16.") || !strings.HasPrefix(messages[2], "out/pr:") {
		t.Fatalf("messages = %v", messages)
	}
}

func TestPipelineVerifyFailureRetriesOnceThenAnalyzes(t *testing.T) {
	h := setupHarness(t, "echo 'FAIL: TestX'; exit 1")
	h.scriptAgent(`printf 'package main\n\nfunc main() {}\n' > main.go
echo "round"
echo ---REPORT---
echo "STATUS: fixed"
echo "SUBJECT: 0123456789abcdef"
echo "Fixed."`)
	h.start("0123456789abcdef")
	attempt := h.claimAndRun()
	if attempt.Status != models.AttemptAnalyzed || len(h.host.pulls) != 0 {
		t.Fatalf("status = %s (%s), pulls=%d", attempt.Status, attempt.Error, len(h.host.pulls))
	}
	if h.remoteHasBranch(branchPrefix + attempt.Id.String()) {
		t.Fatal("a change that failed verification must not be pushed")
	}
	messages := h.messages(attempt)
	if len(messages) != 3 || !strings.Contains(messages[0], "FAIL: TestX") || !strings.Contains(messages[1], "FAIL: TestX") || !strings.HasPrefix(messages[2], "out/finding: The change did not pass verification") {
		t.Fatalf("messages = %v", messages)
	}
	if transcript := h.blob(attempt, agent.BlobTranscript); strings.Count(transcript, "round") != 2 {
		t.Fatalf("the agent must have run twice: %q", transcript)
	}
}

func TestPipelineRefusesScratchFilesOversizedDiffsAndSecrets(t *testing.T) {
	cases := []struct {
		name   string
		script string
		limits func(*Limits)
		want   string
	}{
		{"scratch file", `printf 'x\n' > main.go; echo 'echo hi' > helper.sh`, nil, "scratch file"},
		{"oversized diff", `printf 'package main\n%s\n' "$(head -c 3000 /dev/zero | tr '\0' 'x')" > main.go`, func(l *Limits) { l.DiffMaxBytes = 100 }, "larger than the attempt allows"},
		{"secret", `printf 'package main\nconst k = "AKIAIOSFODNN7EXAMPLE"\n' > main.go`, nil, "looks like a credential"},
		{"no change", `true`, nil, "left the working tree unchanged"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := setupHarness(t, "")
			if tc.limits != nil {
				tc.limits(&h.runner.Limits)
			}
			h.scriptAgent(tc.script + `
echo ---REPORT---
echo "STATUS: fixed"
echo "SUBJECT: 0123456789abcdef"
echo "Done."`)
			h.start("0123456789abcdef")
			attempt := h.claimAndRun()
			if attempt.Status != models.AttemptAnalyzed || len(h.host.pulls) != 0 || h.remoteHasBranch(branchPrefix+attempt.Id.String()) {
				t.Fatalf("status = %s (%s), pulls=%d", attempt.Status, attempt.Error, len(h.host.pulls))
			}
			messages := strings.Join(h.messages(attempt), "\n")
			if !strings.Contains(messages, tc.want) {
				t.Fatalf("finding must say why: %s", messages)
			}
		})
	}
}

func TestPipelineAgentErrorFailsTheAttempt(t *testing.T) {
	h := setupHarness(t, "")
	h.scriptAgent(`echo "crashing"; exit 3`)
	h.start("0123456789abcdef")
	attempt := h.claimAndRun()
	if attempt.Status != models.AttemptFailed || !strings.Contains(attempt.Error, "no report") || attempt.FinishedAt == nil {
		t.Fatalf("attempt = %+v", attempt)
	}
	if kinds := strings.Join(h.eventKinds(attempt), " "); !strings.Contains(kinds, "error") || !strings.Contains(kinds, "status:failed") {
		t.Fatalf("events = %s", kinds)
	}
}

func TestWriteCLIProfileLogsTheSandboxedCLIIn(t *testing.T) {
	home := t.TempDir()
	if err := writeCLIProfile(home, "https://traceway.example.com", "run-token", "project-1"); err != nil {
		t.Fatal(err)
	}
	config, _ := os.ReadFile(filepath.Join(home, ".config", "traceway", "config.json"))
	state, _ := os.ReadFile(filepath.Join(home, ".local", "state", "traceway", "state.json"))
	if !strings.Contains(string(config), `"url": "https://traceway.example.com"`) {
		t.Fatalf("config = %s", config)
	}
	if !strings.Contains(string(state), `"jwt": "run-token"`) || !strings.Contains(string(state), `"credential_kind": "run"`) || !strings.Contains(string(state), `"current_project_id": "project-1"`) {
		t.Fatalf("state = %s", state)
	}
	if got := instanceHost("https://traceway.example.com"); got.Host != "traceway.example.com" || got.Port != 443 {
		t.Fatalf("instanceHost = %+v", got)
	}
	if got := instanceHost("http://localhost:18082"); got.Host != "localhost" || got.Port != 18082 {
		t.Fatalf("instanceHost = %+v", got)
	}
}
