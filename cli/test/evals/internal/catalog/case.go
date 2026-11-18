// Package catalog loads the eval cases (cases/<suite>/<id>.yaml), the presets
// that select them, and the model configuration. It owns the case schema; every
// other package reads cases through these types.
package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Suites, in report order.
var Suites = []string{"perf", "debug", "write", "setup"}

// Arms. An arm fixes the surface the agent works through and what it is given.
const (
	ArmCLISkill      = "cli+skill"       // traceway CLI in the shell, traceway skill available
	ArmCLINoSkill    = "cli-noskill"     // traceway CLI in the shell, no skills (baseline)
	ArmMCP           = "mcp"             // /mcp tools, server instructions, no traceway CLI
	ArmCLIWriteSkill = "cli-write+skill" // traceway-write CLI, traceway-write skill
	ArmMCPWrite      = "mcp-write"       // /mcp/write tools
	ArmCLISetupSkill = "cli+setup-skill" // container shell, traceway-setup skill
)

// AllArms lists every arm the runner knows.
var AllArms = []string{ArmCLISkill, ArmCLINoSkill, ArmMCP, ArmCLIWriteSkill, ArmMCPWrite, ArmCLISetupSkill}

// IsMCPArm reports whether the arm talks to Traceway through MCP tools.
func IsMCPArm(arm string) bool { return arm == ArmMCP || arm == ArmMCPWrite }

// IsWriteArm reports whether the arm carries a write-capable surface.
func IsWriteArm(arm string) bool { return arm == ArmCLIWriteSkill || arm == ArmMCPWrite }

// Case is one eval scenario. Strings in Prompt, Expect, Oracle and Hooks may
// contain {{seed.x}} / {{env.x}} placeholders, resolved after seeding.
type Case struct {
	ID         string   `yaml:"id"`
	Suite      string   `yaml:"suite"`
	Title      string   `yaml:"title"`
	Tags       []string `yaml:"tags"`
	Seed       string   `yaml:"seed"`
	Arms       []string `yaml:"arms"`
	Invoke     string   `yaml:"invoke"` // slash | natural
	Distractor string   `yaml:"distractor"`

	Workspace   Workspace         `yaml:"workspace"`
	Prompt      string            `yaml:"prompt"`
	Budget      Budget            `yaml:"budget"`
	User        []UserRule        `yaml:"user"`
	Credential  Credential        `yaml:"credential"`
	AgentAccess *bool             `yaml:"agent_access"`
	Setup       *SetupSpec        `yaml:"setup"`
	Hooks       []Hook            `yaml:"hooks"`
	Expect      Expect            `yaml:"expect"`
	Judge       JudgeSpec         `yaml:"judge"`
	Oracle      []Step            `yaml:"oracle"`
	OracleByArm map[string][]Step `yaml:"oracle_by_arm"`

	// Path is the YAML file the case was loaded from.
	Path string `yaml:"-"`
}

// Workspace names the fixture repo copied into the agent's work dir.
type Workspace struct {
	Repo string `yaml:"repo"`
}

// Budget bounds one run. Zero values take the defaults in Defaults().
type Budget struct {
	MaxTurns        int      `yaml:"max_turns"`
	MaxToolCalls    int      `yaml:"max_tool_calls"`
	MaxOutputTokens int      `yaml:"max_output_tokens"`
	Timeout         Duration `yaml:"timeout"`
}

// Defaults fills unset budget fields.
func (b Budget) Defaults() Budget {
	if b.MaxTurns == 0 {
		b.MaxTurns = 40
	}
	if b.MaxToolCalls == 0 {
		b.MaxToolCalls = 30
	}
	if b.MaxOutputTokens == 0 {
		b.MaxOutputTokens = 12000
	}
	if b.Timeout == 0 {
		b.Timeout = Duration(10 * time.Minute)
	}
	return b
}

// UserRule scripts one ask_user answer. The first rule whose Match regexp
// matches the question wins; a rule with Default set matches anything.
type UserRule struct {
	Match   string `yaml:"match"`
	Reply   string `yaml:"reply"`
	Default string `yaml:"default"`
	// Once makes the rule apply to its first match only.
	Once bool `yaml:"once"`
}

// Credential picks the identity a write-suite run acts as.
type Credential struct {
	Scope  string `yaml:"scope"`  // write (default on write arms) | read
	Member string `yaml:"member"` // admin (default) | user | readonly
}

// SetupSpec configures a setup-suite case.
type SetupSpec struct {
	// Path names the setup skill's path the case exercises, fast or plan.
	// The harness does not read it; Token and ApproveDrafts set the run up.
	Path string `yaml:"path"`
	// Token is what the prompt hands the agent: ingest | setup.
	Token string `yaml:"token"`
	// ApproveDrafts makes the harness approve every pending setup draft.
	ApproveDrafts bool `yaml:"approve_drafts"`
	// Env is the container environment of the agent's shell and of the
	// verify step. Values: "@token:<project>" (that project's ingest
	// token), "@url" (the instance), "@port" (a free port for the app),
	// or a literal.
	Env map[string]string `yaml:"env"`
	// Verify is what the harness does after the agent finishes: build and
	// start the app against the backend and hit its routes, so state
	// assertions can check that telemetry arrived.
	Verify *SetupVerify `yaml:"verify"`
}

// SetupVerify builds and starts the instrumented app.
type SetupVerify struct {
	Dir   string   `yaml:"dir"`
	Build string   `yaml:"build"`
	Start string   `yaml:"start"`
	Hit   []string `yaml:"hit"`
	// EnvFiles are sourced (if present) before start, so env files the
	// agent wrote are honored.
	EnvFiles []string `yaml:"env_files"`
	Wait     Duration `yaml:"wait"`
}

// HookActions are the actions a hook may name (runner/hooks.go).
var HookActions = []string{"edit_post_mortem", "stub_healthy", "stub_unhealthy", "sleep", "approve_setup_draft"}

// Hook is a harness action triggered after a matching agent call.
type Hook struct {
	After  CallMatcher    `yaml:"after"`
	Action string         `yaml:"action"`
	Args   map[string]any `yaml:"args"`
	// Times bounds how often the hook fires (default 1).
	Times int `yaml:"times"`
}

// Expect is everything the deterministic graders check.
type Expect struct {
	Answer    *Answer       `yaml:"answer"`
	Grounding []string      `yaml:"grounding"`
	Calls     *Trajectory   `yaml:"calls"`
	State     []StateAssert `yaml:"state"`
	Code      *CodeSpec     `yaml:"code"`
	// Report holds regexps the final report text must (or must not) match.
	Report *ReportSpec `yaml:"report"`
}

// ReportSpec checks the final free-text report.
type ReportSpec struct {
	Contains    []string `yaml:"contains"`
	NotContains []string `yaml:"not_contains"`
}

// Answer is the expected structured answer. Perf fields apply to
// submit_perf_diagnosis, debug fields to submit_debug_diagnosis.
type Answer struct {
	// perf
	ChecklistItem string   `yaml:"checklist_item"`
	AnyOf         []string `yaml:"any_of"`
	Not           []string `yaml:"not"`
	Locus         *Locus   `yaml:"locus"`
	Onset         *Onset   `yaml:"onset"`
	// debug
	IssueHash            string   `yaml:"issue_hash"`
	OccurrenceID         string   `yaml:"occurrence_id"`
	Category             string   `yaml:"category"`
	Culprit              *Culprit `yaml:"culprit"`
	NeedsInstrumentation *bool    `yaml:"needs_instrumentation"`
	// MentionsAll lists regexps that must all match somewhere in the answer.
	MentionsAll []string `yaml:"mentions_all"`
}

// Locus pins where the bottleneck is. Every set field must match.
type Locus struct {
	Endpoint     string `yaml:"endpoint"`
	SpanContains string `yaml:"span_contains"`
	Service      string `yaml:"service"`
	Host         string `yaml:"host"`
}

// Onset is the expected start of a regression.
type Onset struct {
	At               string `yaml:"at"`
	ToleranceMinutes int    `yaml:"tolerance_minutes"`
}

// Culprit is the expected root-cause location.
type Culprit struct {
	File   string `yaml:"file"`
	Symbol string `yaml:"symbol"`
	Line   int    `yaml:"line"`
	// LineTolerance defaults to 3.
	LineTolerance int `yaml:"line_tolerance"`
}

// Trajectory is the tool-call DSL.
type Trajectory struct {
	Required            []CallMatcher   `yaml:"required"`
	AnyOf               [][]CallMatcher `yaml:"any_of"`
	Forbidden           []CallMatcher   `yaml:"forbidden"`
	Order               [][]string      `yaml:"order"`
	MaxCalls            int             `yaml:"max_calls"`
	MaxFailedCalls      int             `yaml:"max_failed_calls"`
	MaxIdenticalRepeats int             `yaml:"max_identical_repeats"`
}

// CallMatcher matches one recorded operation.
type CallMatcher struct {
	Op    string         `yaml:"op"`
	Args  map[string]any `yaml:"args"`
	Write *bool          `yaml:"write"`
	OK    *bool          `yaml:"ok"`
}

// StateAssert is one post-run assertion against the backend or workspace.
// Kind selects which fields apply; see grade/state.go.
type StateAssert struct {
	Kind string `yaml:"kind"`
	// Common addressing.
	ID       string `yaml:"id"`
	Endpoint string `yaml:"endpoint"`
	Path     string `yaml:"path"`
	// Expected values.
	Status         string            `yaml:"status"`
	Enabled        *bool             `yaml:"enabled"`
	SnoozedMinutes *Range            `yaml:"snoozed_minutes"`
	Snoozed        *bool             `yaml:"snoozed"`
	Equals         []string          `yaml:"equals"`
	Among          []string          `yaml:"among"`
	Count          *Range            `yaml:"count"`
	OffsetMs       *Range            `yaml:"offset_ms"`
	Reason         string            `yaml:"reason"`
	Contains       []string          `yaml:"contains"`
	NotContains    []string          `yaml:"not_contains"`
	Title          string            `yaml:"title"`
	Tags           []string          `yaml:"tags"`
	IncidentID     string            `yaml:"incident_id"`
	Exists         *bool             `yaml:"exists"`
	Route          string            `yaml:"route"`
	HTTPStatus     *Range            `yaml:"http_status"`
	Projects       []string          `yaml:"projects"`
	Project        string            `yaml:"project"`
	Frameworks     map[string]string `yaml:"frameworks"`
	Extra          map[string]any    `yaml:"extra"`
}

// Range is an inclusive numeric interval; a scalar YAML value sets both ends.
type Range struct {
	Min *float64 `yaml:"min"`
	Max *float64 `yaml:"max"`
}

// UnmarshalYAML accepts `5` as {min: 5, max: 5}. A bound that is still a
// {{template}} (the case is loaded before seeding) is left unset; Resolve
// decodes the case again after filling it in.
func (r *Range) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode && strings.Contains(n.Value, "{{") {
		return nil
	}
	if n.Kind == yaml.MappingNode {
		var raw map[string]yaml.Node
		if err := n.Decode(&raw); err != nil {
			return err
		}
		for k, v := range raw {
			if v.Kind == yaml.ScalarNode && strings.Contains(v.Value, "{{") {
				continue
			}
			var f float64
			if err := v.Decode(&f); err != nil {
				return fmt.Errorf("range %s: %w", k, err)
			}
			switch k {
			case "min":
				r.Min = &f
			case "max":
				r.Max = &f
			default:
				return fmt.Errorf("range: unknown key %q", k)
			}
		}
		return nil
	}
	if n.Kind == yaml.ScalarNode {
		var v float64
		if err := n.Decode(&v); err != nil {
			return err
		}
		r.Min, r.Max = &v, &v
		return nil
	}
	type plain Range
	return n.Decode((*plain)(r))
}

// Contains reports whether v lies in the range.
func (r *Range) Contains(v float64) bool {
	if r == nil {
		return true
	}
	if r.Min != nil && v < *r.Min {
		return false
	}
	if r.Max != nil && v > *r.Max {
		return false
	}
	return true
}

func (r *Range) String() string {
	if r == nil {
		return "any"
	}
	lo, hi := "-inf", "+inf"
	if r.Min != nil {
		lo = fmt.Sprint(*r.Min)
	}
	if r.Max != nil {
		hi = fmt.Sprint(*r.Max)
	}
	return "[" + lo + ", " + hi + "]"
}

// CodeSpec verifies a code change with hidden tests.
type CodeSpec struct {
	HiddenTests string   `yaml:"hidden_tests"` // testdata/hidden/<name>
	Command     string   `yaml:"command"`
	Gating      bool     `yaml:"gating"`
	Timeout     Duration `yaml:"timeout"`
	// TouchesCulprit requires the diff to change the culprit file.
	TouchesCulprit bool `yaml:"touches_culprit"`
	// MustChange / MustNotChange are path globs checked against the diff.
	MustChange    []string `yaml:"must_change"`
	MustNotChange []string `yaml:"must_not_change"`
	// Static marks a fixture that cannot run here; only the diff is graded.
	Static bool `yaml:"static"`
}

// JudgeSpec is the LLM judge's rubric. Expected is shown to the judge only.
type JudgeSpec struct {
	Rubric   []string `yaml:"rubric"`
	Expected string   `yaml:"expected"`
}

// Step is one oracle action, played by the fake model.
type Step struct {
	Shell     string         `yaml:"shell"`
	Op        string         `yaml:"op"`
	Args      map[string]any `yaml:"args"`
	Stdin     string         `yaml:"stdin"`
	ReadFile  string         `yaml:"read_file"`
	ListFiles string         `yaml:"list_files"`
	Search    map[string]any `yaml:"search"`
	WriteFile *FileStep      `yaml:"write_file"`
	EditFile  *EditStep      `yaml:"edit_file"`
	LoadSkill string         `yaml:"load_skill"`
	AskUser   string         `yaml:"ask_user"`
	Resource  string         `yaml:"read_resource"`
	Submit    map[string]any `yaml:"submit"`
	Finish    map[string]any `yaml:"finish"`
	Say       string         `yaml:"say"`
}

// FileStep writes a whole file.
type FileStep struct {
	Path    string `yaml:"path"`
	Content string `yaml:"content"`
	// From copies the content of a testdata file instead.
	From string `yaml:"from"`
}

// EditStep replaces one string in a file.
type EditStep struct {
	Path string `yaml:"path"`
	Old  string `yaml:"old"`
	New  string `yaml:"new"`
}

// Duration is a time.Duration that unmarshals from "10m".
type Duration time.Duration

// UnmarshalYAML parses Go duration strings.
func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	var s string
	if err := n.Decode(&s); err != nil {
		return err
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("duration %q: %w", s, err)
	}
	*d = Duration(v)
	return nil
}

// D returns the time.Duration.
func (d Duration) D() time.Duration { return time.Duration(d) }

var idRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)+$`)

// suiteArms are the arms a suite can run: write cases need a write surface,
// and the setup arm (the container shell) belongs to the setup suite alone.
// perf and debug take any other arm (a debug case may check that an agent
// with write access writes nothing).
var suiteArms = map[string][]string{
	"perf":  {ArmCLISkill, ArmCLINoSkill, ArmMCP, ArmCLIWriteSkill, ArmMCPWrite},
	"debug": {ArmCLISkill, ArmCLINoSkill, ArmMCP, ArmCLIWriteSkill, ArmMCPWrite},
	"write": {ArmCLIWriteSkill, ArmMCPWrite},
	"setup": {ArmCLISetupSkill},
}

// Validate checks a case for structural errors that YAML decoding cannot.
func (c *Case) Validate() error {
	var errs []string
	add := func(f string, a ...any) { errs = append(errs, fmt.Sprintf(f, a...)) }
	if !idRe.MatchString(c.ID) {
		add("id %q must be kebab-case", c.ID)
	}
	if !slices.Contains(Suites, c.Suite) {
		add("suite %q must be one of %v", c.Suite, Suites)
	} else if !strings.HasPrefix(c.ID, c.Suite+"-") {
		add("id %q must start with %q", c.ID, c.Suite+"-")
	}
	if c.Path != "" && filepath.Base(c.Path) != c.ID+".yaml" {
		add("file %s must be named %s.yaml", c.Path, c.ID)
	}
	if c.Seed == "" {
		add("seed is required")
	}
	if len(c.Arms) == 0 {
		add("arms is required")
	}
	for _, a := range c.Arms {
		if !slices.Contains(AllArms, a) {
			add("unknown arm %q", a)
		}
	}
	switch c.Invoke {
	case "", "slash", "natural":
	default:
		add("invoke must be slash or natural, got %q", c.Invoke)
	}
	// The runner preloads the skill for slash cases, and the prompt is what
	// the agent reads, so the two must agree.
	if slash := strings.HasPrefix(strings.TrimSpace(c.Prompt), "/"); slash != (c.Invoke == "slash") {
		add("invoke: slash goes with a prompt that starts with /<skill>, and only with one")
	}
	if strings.TrimSpace(c.Prompt) == "" {
		add("prompt is required")
	}
	if len(c.Oracle) == 0 {
		add("oracle is required")
	}
	for arm := range c.OracleByArm {
		if !slices.Contains(c.Arms, arm) {
			add("oracle_by_arm names arm %q that the case does not run", arm)
		}
	}
	if c.Suite == "perf" || c.Suite == "debug" {
		if c.Expect.Answer == nil {
			add("%s cases need expect.answer", c.Suite)
		}
	}
	if (c.Suite == "write" || c.Suite == "setup") && len(c.Expect.State) == 0 {
		add("%s cases need expect.state", c.Suite)
	}
	if c.Suite == "setup" && c.Setup == nil {
		add("setup cases need a setup block")
	}
	if c.Setup != nil {
		if c.Setup.Path != "" && c.Setup.Path != "fast" && c.Setup.Path != "plan" {
			add("setup.path must be fast or plan, got %q", c.Setup.Path)
		}
		if c.Setup.Token != "" && c.Setup.Token != "ingest" && c.Setup.Token != "setup" {
			add("setup.token must be ingest or setup, got %q", c.Setup.Token)
		}
	}
	for _, a := range c.Arms {
		if ok := suiteArms[c.Suite]; ok != nil && !slices.Contains(ok, a) {
			add("arm %q does not run the %s suite (its arms: %v)", a, c.Suite, ok)
		}
	}
	if c.Suite == "debug" && c.Expect.Answer != nil {
		if ans := c.Expect.Answer; ans.Category == "" && ans.IssueHash == "" && ans.Culprit == nil {
			add("debug cases need expect.answer.category, issue_hash or culprit")
		}
	}
	for i, h := range c.Hooks {
		if !slices.Contains(HookActions, h.Action) {
			add("hooks[%d]: unknown action %q (one of %v)", i, h.Action, HookActions)
		}
	}
	if c.Suite != "setup" && c.Distractor == "" && !slices.Contains(c.Tags, "control") {
		add("distractor is required (or tag the case control)")
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s: %s", c.ID, strings.Join(errs, "; "))
	}
	return nil
}

// HasTag reports whether the case carries tag.
func (c *Case) HasTag(tag string) bool { return slices.Contains(c.Tags, tag) }

// OracleFor returns the oracle steps for an arm.
func (c *Case) OracleFor(arm string) []Step {
	if s, ok := c.OracleByArm[arm]; ok {
		return s
	}
	return c.Oracle
}

// LoadCases reads every cases/<suite>/*.yaml under root. TWEVALS_ONLY (a
// comma-separated list of ids) limits loading to those cases, so work in
// progress on other cases cannot break a run.
func LoadCases(root string) ([]*Case, error) {
	var out []*Case
	seen := map[string]string{}
	only := map[string]bool{}
	for _, id := range strings.Split(os.Getenv("TWEVALS_ONLY"), ",") {
		if id = strings.TrimSpace(id); id != "" {
			only[id] = true
		}
	}
	for _, suite := range Suites {
		files, err := filepath.Glob(filepath.Join(root, "cases", suite, "*.yaml"))
		if err != nil {
			return nil, err
		}
		sort.Strings(files)
		for _, f := range files {
			if len(only) > 0 && !only[strings.TrimSuffix(filepath.Base(f), ".yaml")] {
				continue
			}
			c, err := LoadCase(f)
			if err != nil {
				return nil, err
			}
			if c.Suite != suite {
				return nil, fmt.Errorf("%s: suite %q does not match directory %q", f, c.Suite, suite)
			}
			if prev, dup := seen[c.ID]; dup {
				return nil, fmt.Errorf("duplicate case id %s in %s and %s", c.ID, prev, f)
			}
			seen[c.ID] = f
			out = append(out, c)
		}
	}
	return out, nil
}

// LoadCase reads and validates one case file. Unknown keys are errors.
func LoadCase(path string) (*Case, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	dec.KnownFields(true)
	var c Case
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	c.Path = path
	if c.Invoke == "" {
		c.Invoke = "natural"
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}
