// Package tools implements what the agent under test can call: a sandboxed
// shell (an interpreter with an allowlist, never the host shell), confined
// file tools, skills, an MCP bridge, ask_user, and the structured submit
// tools. Every call returns the operations it performed so the graders can
// map CLI and MCP runs onto the same trajectory.
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/record"
)

// Spec is a tool's function definition as the model sees it.
type Spec struct {
	Name        string
	Description string
	Parameters  map[string]any
}

// Result is what one call produced.
type Result struct {
	Output    string
	OK        bool
	Truncated bool
	Ops       []record.OpRecord
	// Final is set when a submit/finish payload validated: the run ends.
	Final      bool
	Submission map[string]any
}

// Tool is one callable tool.
type Tool interface {
	Spec() Spec
	Call(ctx context.Context, args json.RawMessage) Result
}

// MaxOutput caps what one shell or file call returns to the model.
const MaxOutput = 30000

// MaxMCPOutput caps one MCP tool result. An MCP client cannot pipe a result
// through jq, so the cap mirrors Claude Code's MCP output limit (about 25k
// tokens) rather than the shell's.
const MaxMCPOutput = 100000

// Cap truncates s to MaxOutput characters with a marker.
func Cap(s string) (string, bool) { return CapAt(s, MaxOutput) }

// CapAt truncates s to n characters with a marker.
func CapAt(s string, n int) (string, bool) {
	if len(s) <= n {
		return s, false
	}
	more := len(s) - n
	return s[:n] + fmt.Sprintf("\n[output truncated: %d more characters. Narrow the query (smaller window, page size, field selection) or filter the output.]", more), true
}

// Sandbox confines file access to the run's workspace.
type Sandbox struct {
	Root     string   // absolute workspace path
	ReadOnly []string // workspace-relative prefixes that cannot be written (.skills)
}

// ErrOutside is returned for any path that leaves the workspace.
var ErrOutside = errors.New("path is outside the workspace")

// Resolve maps p (absolute, or relative to base) to an absolute path inside
// the workspace. Symlinks pointing out of the workspace are refused.
func (s *Sandbox) Resolve(base, p string) (string, error) {
	if p == "/dev/null" {
		return p, nil
	}
	if base == "" {
		base = s.Root
	}
	var abs string
	if filepath.IsAbs(p) {
		abs = filepath.Clean(p)
	} else {
		abs = filepath.Clean(filepath.Join(base, p))
	}
	if !within(s.Root, abs) || !within(realPath(s.Root), realPath(abs)) {
		return "", fmt.Errorf("%s: %w", p, ErrOutside)
	}
	return abs, nil
}

// Writable reports whether abs may be written. Anything under a .git
// directory is read-only wherever it sits: git reads its config, so a write
// there could make a git command run a program on the host. Both the path and
// what a symlink makes of it are checked, without regard to case.
func (s *Sandbox) Writable(abs string) error {
	if abs == "/dev/null" {
		return nil
	}
	rel, err := filepath.Rel(s.Root, abs)
	if err != nil {
		return err
	}
	if err := s.readOnly(rel); err != nil {
		return err
	}
	if real, err := filepath.Rel(realPath(s.Root), realPath(abs)); err == nil {
		return s.readOnly(real)
	}
	return nil
}

func (s *Sandbox) readOnly(rel string) error {
	rel = filepath.ToSlash(rel)
	lower := strings.ToLower(rel)
	for _, part := range strings.Split(lower, "/") {
		if part == ".git" {
			return fmt.Errorf("%s is read-only", rel)
		}
	}
	for _, ro := range s.ReadOnly {
		ro = strings.ToLower(ro)
		if lower == ro || strings.HasPrefix(lower, ro+"/") {
			return fmt.Errorf("%s is read-only", rel)
		}
	}
	return nil
}

// realPath resolves the symlinks in the longest existing prefix of p.
func realPath(p string) string {
	rest := ""
	for probe := p; ; {
		if real, err := filepath.EvalSymlinks(probe); err == nil {
			return filepath.Join(real, rest)
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return p
		}
		rest = filepath.Join(filepath.Base(probe), rest)
		probe = parent
	}
}

// Rel returns the workspace-relative form of abs.
func (s *Sandbox) Rel(abs string) string {
	rel, err := filepath.Rel(s.Root, abs)
	if err != nil {
		return abs
	}
	return filepath.ToSlash(rel)
}

func within(root, p string) bool {
	if p == root {
		return true
	}
	return strings.HasPrefix(p, strings.TrimSuffix(root, string(filepath.Separator))+string(filepath.Separator))
}

// decodeArgs unmarshals tool arguments, tolerating an empty payload.
func decodeArgs(raw json.RawMessage, v any) error {
	if len(raw) == 0 || string(raw) == "null" {
		raw = []byte("{}")
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("invalid arguments: %v", err)
	}
	return nil
}

func errResult(format string, a ...any) Result {
	return Result{Output: "error: " + fmt.Sprintf(format, a...), OK: false}
}

// readFileCapped reads a workspace file for a tool.
func readFileCapped(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// obj builds a JSON-schema object.
func obj(props map[string]any, required ...string) map[string]any {
	m := map[string]any{"type": "object", "properties": props, "additionalProperties": false}
	if len(required) > 0 {
		m["required"] = required
	}
	return m
}

func strProp(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

func intProp(desc string) map[string]any {
	return map[string]any{"type": "integer", "description": desc}
}
