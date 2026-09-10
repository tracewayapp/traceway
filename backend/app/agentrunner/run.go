// Package agentrunner is the harness: it turns a claimed attempt into a
// draft pull request, a report, or a question. Per attempt, in order: fetch
// the mirror and check the repository out, confine the coding agent in the
// sandbox with a run token and the provider key, stream its events to the
// control plane while renewing the lease, verify the change with the
// scratch guard, the diff limit, the secret scan and the repository's tests,
// and publish through the code host. Embedded mode runs it inside the
// backend; a remote runner runs the same pipeline behind the runner protocol.
package agentrunner

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/agent"
	"github.com/tracewayapp/traceway/backend/app/agentrunner/agents"
	"github.com/tracewayapp/traceway/backend/app/agentrunner/verify"
	"github.com/tracewayapp/traceway/backend/app/agentrunner/workspace"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/sandbox"
	traceway "go.tracewayapp.com"
)

// Limits are the harness's own guards, independent of the agent profile.
type Limits struct {
	DiffMaxBytes    int
	DiffMaxFiles    int
	ScratchPatterns []string
	VerifyTimeout   time.Duration
	DefaultTimeout  time.Duration
}

var DefaultLimits = Limits{
	DiffMaxBytes:    512 << 10,
	DiffMaxFiles:    60,
	ScratchPatterns: workspace.DefaultScratchPatterns,
	VerifyTimeout:   20 * time.Minute,
	DefaultTimeout:  45 * time.Minute,
}

// Runner executes attempts. One Runner serves every worker of an executor;
// the embedded executor and a remote runner differ only in their Claimer
// and Reporter.
type Runner struct {
	Claimer  Claimer
	Reporter Reporter
	Backend  sandbox.Backend
	Mirrors  *workspace.Mirrors
	// WorkRoot holds one directory per attempt: the checkout, a home for
	// the agent's state and a scratch dir.
	WorkRoot string
	Agents   map[string]agents.Agent
	// InstanceURL is the Traceway origin the sandboxed CLI talks to.
	InstanceURL string
	// SkillDir and CLIPath are bound read-only into the sandbox. HostTools
	// says the agent and the CLI are installed on this host and their
	// directories get bound; off when the sandbox image carries them.
	SkillDir  string
	CLIPath   string
	HostTools bool
	// Image is the docker backend's fallback when the repository names none.
	Image  string
	Limits Limits
}

// sandboxPath is the PATH inside a sandbox image: the traceway CLI first,
// then the image's own tools.
const sandboxPath = "/opt/traceway/bin:/usr/local/bin:/usr/bin:/bin"

const (
	eventFlushInterval = 2 * time.Second
	eventFlushSize     = 50
	leaseRenewInterval = 30 * time.Second
	verifyRounds       = 2
	branchPrefix       = "traceway/fix-"
)

// Run drives one claimed attempt to a terminal or waiting status. Every
// failure ends the attempt as failed with the reason on the row; the lease
// is renewed while it runs and released by the final transition.
func (r *Runner) Run(ctx context.Context, attempt *models.AgentAttempt) {
	if scoped, ok := r.Claimer.(scopedControl); ok {
		bound := *r
		bound.Claimer, bound.Reporter = scoped.ForAttempt(attempt)
		r = &bound
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stopLease := r.renewLease(ctx, cancel, attempt)
	defer stopLease()

	recorder := newRecorder(attempt.Id, r.Reporter, cancel)
	defer recorder.flush()

	if err := r.run(ctx, attempt, recorder); err != nil {
		r.fail(ctx, attempt, recorder, err)
	}
}

func (r *Runner) run(ctx context.Context, attempt *models.AgentAttempt, recorder *recorder) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err := r.transition(ctx, attempt, models.AttemptPreparing); err != nil {
		return err
	}
	prep, err := r.prepare(ctx, attempt)
	if err != nil {
		return err
	}
	defer prep.credentialsGone()
	defer func() {
		if prep.snapshot != nil {
			prep.snapshot.Close()
		}
	}()
	stopToken := r.renewRunToken(ctx, cancel, attempt, prep)
	defer stopToken()

	if err := r.transition(ctx, attempt, models.AttemptRunning); err != nil {
		return err
	}
	inv := agents.Invocation{Prompt: prep.prompt, SessionId: prep.sessionId, Workdir: prep.repoDir, Profile: prep.profile}
	var usage agents.Usage
	for round := 0; round < verifyRounds; round++ {
		if prep.profile.BudgetUSD > 0 {
			inv.Profile.BudgetUSD = prep.profile.BudgetUSD - attempt.CostUSD - usage.CostUSD
			if inv.Profile.BudgetUSD <= 0 {
				return errors.New("attempt budget exhausted")
			}
		}
		result, err := r.runAgent(ctx, attempt, prep, inv, recorder)
		if err != nil {
			return err
		}
		usage.InputTokens += result.Usage.InputTokens
		usage.OutputTokens += result.Usage.OutputTokens
		usage.CostUSD += result.Usage.CostUSD
		usage.Turns += result.Usage.Turns
		result.Usage = usage
		if result.Status == agents.StatusFixed {
			verified, retryPrompt, err := r.verifyChange(ctx, attempt, prep, result, recorder)
			if err != nil {
				return err
			}
			if verified {
				return r.publish(ctx, attempt, prep, result, recorder)
			}
			if retryPrompt != "" && round == 0 {
				inv = agents.Invocation{Prompt: retryPrompt, SessionId: result.SessionId, Workdir: prep.repoDir, Profile: prep.profile}
				continue
			}
			return r.finishAnalyzed(ctx, attempt, prep, result, recorder, "The change did not pass verification and was not published.")
		}
		return r.finishWithoutFix(ctx, attempt, prep, result, recorder)
	}
	return nil
}

// preparation is everything Run resolves before the agent starts.
type preparation struct {
	inputs     *Inputs
	repository *models.Repository
	cred       agent.GitCredential
	profile    agents.Profile
	adapter    agents.Agent
	prompt     string
	sessionId  string
	runToken   string
	workDir    string
	repoDir    string
	homeDir    string
	scratchDir string
	mirror     string
	ref        string
	snapshot   *workspace.Snapshot
	resumed    bool
}

// credentialsGone drops the plaintext credentials once the run is over.
func (p *preparation) credentialsGone() {
	p.cred = agent.GitCredential{}
	p.profile.Credential = ""
	p.inputs.Profile.Credential = ""
	p.runToken = ""
}

func (r *Runner) prepare(ctx context.Context, attempt *models.AgentAttempt) (*preparation, error) {
	inputs, err := r.Claimer.Inputs(ctx, attempt)
	if err != nil {
		return nil, err
	}
	if inputs.Repository == nil {
		return nil, errors.New("the control plane sent no repository")
	}
	prep := &preparation{inputs: inputs, repository: inputs.Repository, profile: inputs.Profile, runToken: inputs.RunToken}
	adapter, ok := r.Agents[attempt.Agent]
	if !ok {
		return nil, fmt.Errorf("no adapter for agent %q", attempt.Agent)
	}
	prep.adapter = adapter
	cred, err := r.Claimer.GitCredential(ctx, attempt)
	if err != nil {
		return nil, err
	}
	prep.cred = cred
	integrationId := 0
	if prep.repository.IntegrationId != nil {
		integrationId = *prep.repository.IntegrationId
	}
	mirror, err := r.Mirrors.Update(ctx, integrationId, prep.repository.Owner, prep.repository.Name, inputs.CloneURL, &cred)
	if err != nil {
		return nil, err
	}
	prep.mirror = mirror
	prep.ref = attempt.BaseBranch
	if prep.ref == "" {
		prep.ref = prep.repository.DefaultBranch
	}
	if attempt.FixBranch != "" && workspace.HasBranch(ctx, mirror, attempt.FixBranch) {
		prep.ref = attempt.FixBranch
	}

	prep.workDir = filepath.Join(r.WorkRoot, attempt.Id.String())
	prep.repoDir = filepath.Join(prep.workDir, "repo")
	prep.homeDir = filepath.Join(prep.workDir, "home")
	prep.scratchDir = filepath.Join(prep.workDir, "tmp")
	if _, err := os.Stat(filepath.Join(prep.repoDir, ".git")); err == nil && attempt.Resume {
		prep.resumed = true
		prep.sessionId = inputs.SessionId
	} else {
		if err := os.RemoveAll(prep.workDir); err != nil {
			return nil, err
		}
		if err := workspace.Checkout(ctx, mirror, inputs.CloneURL, prep.ref, prep.repoDir); err != nil {
			return nil, err
		}
	}
	for _, dir := range []string{prep.homeDir, prep.scratchDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	if err := writeCLIProfile(prep.homeDir, r.InstanceURL, prep.runToken, attempt.ProjectId.String()); err != nil {
		return nil, err
	}
	prep.prompt = Prompt(inputs.Pack, r.SkillDir, prep.scratchDir, r.Limits.ScratchPatterns)
	return prep, nil
}

// latestSessionId finds the session the last run left, so a resumed attempt
// continues the conversation instead of starting over.
func latestSessionId(tx *sql.Tx, attemptId uuid.UUID) (string, error) {
	events, err := transactional.AgentAttemptEventRepository.ListAfter(tx, attemptId, 0, 100000)
	if err != nil {
		return "", err
	}
	session := ""
	for _, event := range events {
		if event.Kind != agents.EventResult {
			continue
		}
		var payload struct {
			SessionId string `json:"sessionId"`
		}
		if json.Unmarshal(event.Payload, &payload) == nil && payload.SessionId != "" {
			session = payload.SessionId
		}
	}
	return session, nil
}

// writeCLIProfile logs the sandboxed traceway CLI in: a profile pointing at
// this instance with the run token as a non-refreshing credential and the
// project preselected, under the sandbox's own XDG directories.
func writeCLIProfile(homeDir, instanceURL, runToken, projectId string) error {
	root, err := os.OpenRoot(filepath.Dir(homeDir))
	if err != nil {
		return err
	}
	defer root.Close()
	configDir := filepath.Join(filepath.Base(homeDir), ".config", "traceway")
	stateDir := filepath.Join(filepath.Base(homeDir), ".local", "state", "traceway")
	for _, dir := range []string{configDir, stateDir} {
		if err := root.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	config, _ := json.MarshalIndent(map[string]any{"profiles": map[string]any{"default": map[string]string{"url": instanceURL, "username": "agent"}}}, "", "  ")
	state, _ := json.MarshalIndent(map[string]any{"current_profile": "default", "profiles": map[string]any{"default": map[string]string{"jwt": runToken, "credential_kind": "run", "current_project_id": projectId}}}, "", "  ")
	if err := writeRootFile(root, filepath.Join(configDir, "config.json"), config); err != nil {
		return err
	}
	return writeRootFile(root, filepath.Join(stateDir, "state.json"), state)
}

func writeRootFile(root *os.Root, path string, content []byte) error {
	temporary := path + "." + uuid.NewString()
	file, err := root.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer root.Remove(temporary)
	_, err = file.Write(content)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return root.Rename(temporary, path)
}

// runAgent runs one agent session inside the sandbox and streams its
// events; the transcript is appended to storage when the session ends.
func (r *Runner) runAgent(ctx context.Context, attempt *models.AgentAttempt, prep *preparation, inv agents.Invocation, recorder *recorder) (agents.Result, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	spec := r.agentSpec(prep, inv)
	cmd, cleanup, err := r.Backend.Build(ctx, spec)
	if err != nil {
		return agents.Result{}, err
	}
	defer cleanup()
	cmd.Stdin = strings.NewReader(inv.Prompt)
	var stderr tailBuffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return agents.Result{}, err
	}
	cmd.WaitDelay = 10 * time.Second
	if err := cmd.Start(); err != nil {
		return agents.Result{}, fmt.Errorf("start %s: %w", prep.adapter.Name(), err)
	}

	parser := prep.adapter.NewParser()
	var transcript bytes.Buffer
	lastFlush := time.Now()
	total := 0
	flushTranscript := func() error {
		if transcript.Len() == 0 {
			return nil
		}
		if err := r.Reporter.Blob(ctx, attempt.Id, agent.BlobTranscript, transcript.Bytes(), true); err != nil {
			return err
		}
		transcript.Reset()
		lastFlush = time.Now()
		return nil
	}
	var streamErr error
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1<<20), 16<<20)
	for scanner.Scan() {
		line := []byte(redactOutput(scanner.Text(), prep.profile.Credential, prep.inputs.RunToken))
		total += len(line)
		if total > 16<<20 {
			streamErr = errors.New("agent output exceeded the 16 MiB session limit")
			cancel()
			break
		}
		transcript.Write(line)
		transcript.WriteByte('\n')
		events, err := parser.Parse(line)
		if err != nil {
			recorder.add(agents.Event{SchemaVersion: agents.EventSchemaVersion, Kind: "malformed", Text: err.Error(), At: time.Now().UTC()})
			continue
		}
		recorder.add(events...)
		if transcript.Len() >= 64<<10 || time.Since(lastFlush) >= eventFlushInterval {
			if err := flushTranscript(); err != nil {
				streamErr = err
				cancel()
				break
			}
		}
	}
	if err := scanner.Err(); err != nil {
		streamErr = fmt.Errorf("read agent output: %w", err)
		cancel()
	}
	waitErr := cmd.Wait()
	recorder.flush()
	if streamErr != nil {
		return agents.Result{}, streamErr
	}
	if err := flushTranscript(); err != nil {
		return agents.Result{}, err
	}

	result := parser.Result()
	if waitErr != nil {
		result.Status = agents.StatusError
		result.Error = redactOutput(strings.TrimSpace(result.Error+" ("+waitErr.Error()+"): "+tail(stderr.String(), 2000)), prep.profile.Credential, prep.inputs.RunToken)
	}
	return result, nil
}

// agentSpec confines the agent: its workspace writable, sibling workspaces
// hidden, the skill and the CLI read-only, exactly the allowlisted
// environment, egress to the provider and this instance.
func (r *Runner) agentSpec(prep *preparation, inv agents.Invocation) sandbox.Spec {
	argv := prep.adapter.Command(inv)
	readOnly := []sandbox.Bind{}
	if r.HostTools {
		if resolved, err := exec.LookPath(argv[0]); err == nil {
			argv[0] = resolved
			if dir, ok := sandbox.ParentDir(resolved); ok {
				readOnly = append(readOnly, sandbox.Bind{Path: dir, Optional: true})
			}
		}
	}
	if r.SkillDir != "" {
		readOnly = append(readOnly, sandbox.Bind{Path: r.SkillDir, Optional: true})
	}
	if r.HostTools && r.CLIPath != "" {
		if dir, ok := sandbox.ParentDir(r.CLIPath); ok {
			readOnly = append(readOnly, sandbox.Bind{Path: dir, Optional: true})
		}
	}
	env := map[string]string{
		"PATH":             r.path(),
		"HOME":             prep.homeDir,
		"TMPDIR":           prep.scratchDir,
		"XDG_CONFIG_HOME":  filepath.Join(prep.homeDir, ".config"),
		"XDG_STATE_HOME":   filepath.Join(prep.homeDir, ".local", "state"),
		"LANG":             "C.UTF-8",
		"TERM":             "dumb",
		"TRACEWAY_URL":     r.InstanceURL,
		"TRACEWAY_PROJECT": prep.repository.ProjectId.String(),
	}
	for name, value := range prep.adapter.Env(inv) {
		env[name] = value
	}
	timeout := r.Limits.DefaultTimeout
	if prep.profile.TimeoutMinutes > 0 {
		timeout = time.Duration(prep.profile.TimeoutMinutes) * time.Minute
	}
	network := sandbox.NetworkPolicy{Mode: sandbox.NetworkAllow, Allow: prep.adapter.Egress(inv)}
	if instance := instanceHost(r.InstanceURL); instance.Host != "" {
		network.Allow = append(network.Allow, instance)
	}
	return sandbox.Spec{
		Command:  argv,
		Workdir:  prep.repoDir,
		Image:    r.image(prep),
		ReadOnly: readOnly,
		Hidden:   []string{r.WorkRoot},
		Writable: []sandbox.Bind{{Path: prep.workDir}},
		Env:      env,
		Limits:   sandbox.Limits{WallClock: timeout, CPU: 2, MemoryMB: 2048, PIDs: 256},
		Network:  network,
	}
}

func (r *Runner) path() string {
	if r.HostTools {
		return os.Getenv("PATH")
	}
	return sandboxPath
}

func (r *Runner) image(prep *preparation) string {
	if prep.repository.Image != "" {
		return prep.repository.Image
	}
	return r.Image
}

// verifyChange runs the harness guards and the repository's tests on the
// agent's change. It returns verified when the change may be published, or
// the prompt for one more round when the tests failed the first time.
func (r *Runner) verifyChange(ctx context.Context, attempt *models.AgentAttempt, prep *preparation, result agents.Result, recorder *recorder) (bool, string, error) {
	if err := r.transition(ctx, attempt, models.AttemptVerifying); err != nil {
		return false, "", err
	}
	if prep.snapshot != nil {
		prep.snapshot.Close()
		prep.snapshot = nil
	}
	snapshot, err := workspace.NewSnapshot(ctx, prep.mirror, prep.inputs.CloneURL, prep.ref, prep.repoDir)
	if err != nil {
		return false, "", err
	}
	prep.snapshot = snapshot
	changes, err := snapshot.Changes(ctx)
	if err != nil {
		return false, "", err
	}
	if len(changes) == 0 {
		r.finding(ctx, attempt, "The agent reported a fix but left the working tree unchanged; nothing was published.")
		return false, "", nil
	}
	if err := workspace.ScratchGuard(changes, r.Limits.ScratchPatterns); err != nil {
		r.finding(ctx, attempt, err.Error()+"; nothing was published.")
		return false, "", nil
	}
	patch, err := snapshot.Diff(ctx)
	if err != nil {
		return false, "", err
	}
	if err := workspace.DiffLimit(patch, changes, r.Limits.DiffMaxBytes, r.Limits.DiffMaxFiles); err != nil {
		r.finding(ctx, attempt, err.Error()+"; nothing was published.")
		return false, "", nil
	}
	if err := workspace.SecretScan(patch); err != nil {
		r.finding(ctx, attempt, err.Error()+"; nothing was published.")
		return false, "", nil
	}
	r.blob(ctx, attempt.Id, agent.BlobDiff, []byte(patch), false)

	binds := append(r.verifyBinds(prep), sandbox.Bind{Path: snapshot.GitDir})
	outcome, err := verify.Run(ctx, r.Backend, r.image(prep), prep.repository.TestCommand, snapshot.Dir, binds, map[string]string{"PATH": r.path(), "HOME": "/tmp", "TMPDIR": "/tmp", "LANG": "C.UTF-8"}, r.Limits.VerifyTimeout)
	if err != nil {
		return false, "", err
	}
	if outcome.Passed {
		verifiedPatch, err := snapshot.Diff(ctx)
		if err != nil {
			return false, "", err
		}
		if verifiedPatch != patch {
			r.finding(ctx, attempt, "The test command changed the proposed patch; nothing was published.")
			return false, "", nil
		}
		return true, "", nil
	}
	body := "The test command failed:\n\n```\n" + outcome.Output + "\n```"
	r.finding(ctx, attempt, body)
	if err := r.transition(ctx, attempt, models.AttemptRunning); err != nil {
		return false, "", err
	}
	return false, verifyFailureMessage + "\n\n<<<DATA test output\n" + strings.ReplaceAll(outcome.Output, ">>>END DATA", "[data delimiter removed]") + "\n>>>END DATA\n", nil
}

func (r *Runner) verifyBinds(prep *preparation) []sandbox.Bind {
	var binds []sandbox.Bind
	if r.HostTools && r.CLIPath != "" {
		if dir, ok := sandbox.ParentDir(r.CLIPath); ok {
			binds = append(binds, sandbox.Bind{Path: dir, Optional: true})
		}
	}
	return binds
}

// publish commits the change on the fix branch, pushes it, and hands the
// control plane the outcome; opening the pull request is its job, so the
// harness never needs more than the push credential.
func (r *Runner) publish(ctx context.Context, attempt *models.AgentAttempt, prep *preparation, result agents.Result, recorder *recorder) error {
	if err := r.transition(ctx, attempt, models.AttemptPublishing); err != nil {
		return err
	}
	branch := attempt.FixBranch
	if branch == "" {
		branch = branchPrefix + attempt.Id.String()
	}
	cred, err := r.Claimer.GitCredential(ctx, attempt)
	if err != nil {
		return err
	}
	if err := prep.snapshot.Commit(ctx, branch, fmt.Sprintf("Fix %s (attempt %d)", attempt.SubjectRef, attempt.Number), &cred); err != nil {
		return err
	}
	return r.Reporter.Finish(ctx, attempt.Id, Outcome{SchemaVersion: ProtocolVersion, Status: agents.StatusFixed, Branch: branch, Report: result.Report, Usage: result.Usage})
}

// finishWithoutFix handles a question or an analysis: the report reaches
// the thread and the attempt waits for input or ends analyzed.
func (r *Runner) finishWithoutFix(ctx context.Context, attempt *models.AgentAttempt, prep *preparation, result agents.Result, recorder *recorder) error {
	if result.Status == agents.StatusError {
		return errors.New(result.Error)
	}
	if result.Status == agents.StatusQuestion {
		return r.Reporter.Finish(ctx, attempt.Id, Outcome{SchemaVersion: ProtocolVersion, Status: agents.StatusQuestion, Report: result.Report, Usage: result.Usage})
	}
	return r.finishAnalyzed(ctx, attempt, prep, result, recorder, "")
}

func (r *Runner) finishAnalyzed(ctx context.Context, attempt *models.AgentAttempt, prep *preparation, result agents.Result, recorder *recorder, note string) error {
	report := result.Report
	if note != "" {
		report = note + "\n\n" + report
	}
	return r.Reporter.Finish(ctx, attempt.Id, Outcome{SchemaVersion: ProtocolVersion, Status: agents.StatusAnalysis, Report: report, Usage: result.Usage})
}

// finding posts a harness message to the thread: a refused change, a test
// failure, anything a reviewer should see next to the agent's own words.
func (r *Runner) finding(ctx context.Context, attempt *models.AgentAttempt, body string) {
	if err := r.Reporter.Finding(ctx, attempt.Id, body); err != nil {
		traceway.CaptureException(fmt.Errorf("agentrunner: post finding on %s: %w", attempt.Id, err))
	}
}

func (r *Runner) fail(ctx context.Context, attempt *models.AgentAttempt, recorder *recorder, cause error) {
	recorder.add(agents.Event{SchemaVersion: agents.EventSchemaVersion, Kind: agent.EventError, Text: cause.Error(), At: time.Now().UTC()})
	recorder.flush()
	if errors.Is(cause, ErrClaimLost) {
		return
	}
	err := r.Reporter.Finish(context.WithoutCancel(ctx), attempt.Id, Outcome{SchemaVersion: ProtocolVersion, Status: agents.StatusError, Error: cause.Error()})
	if err != nil && !errors.Is(err, ErrClaimLost) {
		traceway.CaptureException(fmt.Errorf("agentrunner: mark attempt %s failed: %w", attempt.Id, err))
	}
	traceway.CaptureException(fmt.Errorf("agentrunner: attempt %s failed: %w", attempt.Id, cause))
}

func (r *Runner) transition(ctx context.Context, attempt *models.AgentAttempt, to string) error {
	return r.Reporter.Transition(ctx, attempt.Id, to)
}

// renewLease keeps the lease alive while the run lasts and cancels the run
// when the lease is lost, so a reclaimed attempt never runs twice.
func (r *Runner) renewLease(ctx context.Context, cancel context.CancelFunc, attempt *models.AgentAttempt) func() {
	done := make(chan struct{})
	go func() {
		defer traceway.Recover()
		ticker := time.NewTicker(leaseRenewInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			held, err := r.Reporter.Renew(ctx, attempt.Id)
			if errors.Is(err, ErrClaimLost) {
				held = false
			} else if err != nil {
				traceway.CaptureException(fmt.Errorf("agentrunner: renew lease for %s: %w", attempt.Id, err))
				cancel()
				return
			}
			if !held {
				cancel()
				return
			}
		}
	}()
	return func() { close(done) }
}

func (r *Runner) blob(ctx context.Context, attemptId uuid.UUID, name string, content []byte, appendTo bool) {
	if err := r.Reporter.Blob(ctx, attemptId, name, content, appendTo); err != nil {
		traceway.CaptureException(fmt.Errorf("agentrunner: store %s for %s: %w", name, attemptId, err))
	}
}

func instanceHost(instanceURL string) sandbox.HostPort {
	trimmed := strings.TrimPrefix(strings.TrimPrefix(instanceURL, "https://"), "http://")
	host, port, _ := strings.Cut(strings.SplitN(trimmed, "/", 2)[0], ":")
	if host == "" {
		return sandbox.HostPort{}
	}
	number := 443
	if strings.HasPrefix(instanceURL, "http://") {
		number = 80
	}
	if port != "" {
		fmt.Sscanf(port, "%d", &number)
	}
	return sandbox.HostPort{Host: host, Port: number}
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
