package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// paidRunRe matches a command line that starts a paid eval run, also when a
// backslash continues it onto the next line.
var paidRunRe = regexp.MustCompile(`twevals\S*[\s\\]+(run|resume)\b|eval-local`)

// dispatchRe matches the other ways to start the evals workflow or reach its
// key: the recipe that dispatches it and the secret's name.
var dispatchRe = regexp.MustCompile(`eval-dispatch|OPENROUTER_API_KEY`)

// CheckWorkflows enforces the dispatch-only rule over a .github/workflows
// directory: cli-evals.yml exists, its `on:` is exactly workflow_dispatch, the
// approval flag is set only by a job behind the evals environment, and no
// other workflow (or composite action next to it) references it, dispatches
// it, uses the evals environment, or starts a paid run.
func CheckWorkflows(dir string) error {
	evalsPath := filepath.Join(dir, filepath.Base(WorkflowFile))
	raw, err := os.ReadFile(evalsPath)
	if err != nil {
		return fmt.Errorf("%s must exist: %w", WorkflowFile, err)
	}
	evalsName, err := checkEvalsWorkflow(raw)
	if err != nil {
		return fmt.Errorf("%s: %w", WorkflowFile, err)
	}
	var files []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() && e.Name() != filepath.Base(WorkflowFile) && isYAML(e.Name()) {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	actions := filepath.Join(dir, "..", "actions")
	_ = filepath.WalkDir(actions, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && isYAML(d.Name()) {
			files = append(files, p)
		}
		return nil
	})
	for _, path := range files {
		name, _ := filepath.Rel(filepath.Dir(dir), path)
		other, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(other)
		if strings.Contains(text, "cli-evals.yml") || strings.Contains(text, "cli-evals.yaml") ||
			(evalsName != "" && strings.Contains(text, evalsName)) || dispatchRe.MatchString(text) {
			return fmt.Errorf("%s references the evals workflow, its dispatch recipe or its key; no workflow may call or dispatch the evals", name)
		}
		if paidRunRe.MatchString(text) || strings.Contains(text, "TWEVALS_APPROVED") {
			return fmt.Errorf("%s starts a paid eval run; only %s may, and only when dispatched by hand", name, WorkflowFile)
		}
		if usesEvalsEnvironment(other) {
			return fmt.Errorf("%s uses the evals environment, which holds the OpenRouter key; only %s may", name, WorkflowFile)
		}
	}
	return nil
}

var (
	justHeaderRe = regexp.MustCompile(`^@?([A-Za-z_][A-Za-z0-9_-]*)[^:=]*:([^=].*|)$`)
	justAliasRe  = regexp.MustCompile(`^alias\s+([A-Za-z0-9_-]+)\s*:=\s*([A-Za-z0-9_-]+)`)
	justCallRe   = regexp.MustCompile(`\bjust\s+(?:-\S+\s+)*([A-Za-z_][A-Za-z0-9_-]*)`)
)

// CheckJustfile enforces that no recipe reachable from default, check or
// test (through dependencies or a `just <recipe>` line) starts a paid eval
// run or dispatches the evals workflow.
func CheckJustfile(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	type recipe struct {
		deps []string
		body string
	}
	recipes := map[string]*recipe{}
	alias := map[string]string{}
	var cur *recipe
	for _, line := range strings.Split(string(raw), "\n") {
		if line != "" && line[0] != ' ' && line[0] != '\t' {
			cur = nil
			if m := justAliasRe.FindStringSubmatch(line); m != nil {
				alias[m[1]] = m[2]
				continue
			}
			if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
				continue
			}
			if m := justHeaderRe.FindStringSubmatch(line); m != nil {
				cur = &recipe{}
				recipes[m[1]] = cur
				for _, d := range strings.Fields(m[2]) {
					if d == "&&" {
						continue
					}
					cur.deps = append(cur.deps, strings.SplitN(d, "(", 2)[0])
				}
			}
			continue
		}
		if cur != nil {
			cur.body += line + "\n"
		}
	}
	for _, root := range []string{"default", "check", "test"} {
		seen := map[string]bool{}
		var walk func(name string) error
		walk = func(name string) error {
			if a, ok := alias[name]; ok {
				name = a
			}
			r, ok := recipes[name]
			if seen[name] || !ok {
				return nil
			}
			seen[name] = true
			if name == "eval-local" || name == "eval-dispatch" || paidRunRe.MatchString(r.body) || dispatchRe.MatchString(r.body) ||
				strings.Contains(r.body, "gh workflow run") {
				return fmt.Errorf("just %s reaches the recipe %s, which starts a paid eval run or dispatches the evals", root, name)
			}
			next := r.deps
			for _, m := range justCallRe.FindAllStringSubmatch(r.body, -1) {
				next = append(next, m[1])
			}
			for _, d := range next {
				if err := walk(d); err != nil {
					return err
				}
			}
			return nil
		}
		if err := walk(root); err != nil {
			return err
		}
	}
	return nil
}

func isYAML(name string) bool {
	return strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml")
}

// usesEvalsEnvironment reports whether a workflow has a job with
// environment: evals.
func usesEvalsEnvironment(raw []byte) bool {
	var doc yaml.Node
	if yaml.Unmarshal(raw, &doc) != nil || len(doc.Content) == 0 {
		return false
	}
	jobs := mapValue(doc.Content[0], "jobs")
	if jobs == nil || jobs.Kind != yaml.MappingNode {
		return false
	}
	for i := 1; i < len(jobs.Content); i += 2 {
		if environmentOf(jobs.Content[i]) == "evals" {
			return true
		}
	}
	return false
}

func environmentOf(job *yaml.Node) string {
	envNode := mapValue(job, "environment")
	if envNode == nil {
		return ""
	}
	if envNode.Kind == yaml.ScalarNode {
		return envNode.Value
	}
	if n := mapValue(envNode, "name"); n != nil {
		return n.Value
	}
	return ""
}

// checkEvalsWorkflow checks cli-evals.yml and returns its name.
func checkEvalsWorkflow(raw []byte) (string, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return "", err
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return "", fmt.Errorf("not a YAML mapping")
	}
	root := doc.Content[0]
	if err := noDuplicateKeys(root); err != nil {
		return "", err
	}
	name := ""
	if n := mapValue(root, "name"); n != nil && n.Kind == yaml.ScalarNode {
		name = n.Value
	}
	if env := mapValue(root, "env"); env != nil && strings.Contains(encode(env), "TWEVALS_APPROVED") {
		return "", fmt.Errorf("the workflow-level env sets TWEVALS_APPROVED; only the job behind environment: evals may")
	}
	on := mapValue(root, "on")
	if on == nil {
		// YAML 1.1 readers turn a bare `on` key into true; accept that
		// spelling too so the check cannot be dodged by quoting.
		on = mapValue(root, "true")
	}
	if on == nil {
		return "", fmt.Errorf("has no on: block")
	}
	switch on.Kind {
	case yaml.ScalarNode:
		if on.Value != "workflow_dispatch" {
			return "", fmt.Errorf("on: must be exactly workflow_dispatch, got %q", on.Value)
		}
	case yaml.SequenceNode:
		if len(on.Content) != 1 || on.Content[0].Value != "workflow_dispatch" {
			return "", fmt.Errorf("on: must be exactly [workflow_dispatch]")
		}
	case yaml.MappingNode:
		if err := noDuplicateKeys(on); err != nil {
			return "", err
		}
		var keys []string
		for i := 0; i < len(on.Content); i += 2 {
			keys = append(keys, on.Content[i].Value)
		}
		if len(keys) != 1 || keys[0] != "workflow_dispatch" {
			return "", fmt.Errorf("on: must have exactly one trigger, workflow_dispatch; found %v", keys)
		}
	default:
		return "", fmt.Errorf("on: has an unexpected shape")
	}

	jobs := mapValue(root, "jobs")
	if jobs == nil || jobs.Kind != yaml.MappingNode {
		return "", fmt.Errorf("has no jobs")
	}
	gated := 0
	for i := 0; i < len(jobs.Content); i += 2 {
		jobName, job := jobs.Content[i].Value, jobs.Content[i+1]
		body := encode(job)
		approves := strings.Contains(body, "TWEVALS_APPROVED")
		runsPaid := paidRunRe.MatchString(body)
		if !approves && !runsPaid {
			continue
		}
		if environmentOf(job) != "evals" {
			return "", fmt.Errorf("job %q starts a paid run or sets TWEVALS_APPROVED without environment: evals", jobName)
		}
		gated++
	}
	if gated == 0 {
		return "", fmt.Errorf("no job runs the evals behind environment: evals")
	}
	return name, nil
}

func encode(n *yaml.Node) string {
	var buf strings.Builder
	enc := yaml.NewEncoder(&buf)
	_ = enc.Encode(n)
	return buf.String()
}

// noDuplicateKeys rejects a mapping that repeats a key: a reader that keeps
// the last one would see a different workflow than this check does.
func noDuplicateKeys(m *yaml.Node) error {
	seen := map[string]bool{}
	for i := 0; i+1 < len(m.Content); i += 2 {
		k := m.Content[i].Value
		if seen[k] {
			return fmt.Errorf("the key %q appears twice", k)
		}
		seen[k] = true
	}
	return nil
}

func mapValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}
