package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/ops"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/record"
)

// File tools, confined to the workspace. .skills/ is readable but not
// writable.

func fileOp(name, rel string, ok bool) []record.OpRecord {
	return []record.OpRecord{{Op: ops.Op{Op: name, Args: map[string]any{"path": rel}}, OK: ok}}
}

// ReadFile reads a file with line numbers.
type ReadFile struct{ Sandbox *Sandbox }

func (t *ReadFile) Spec() Spec {
	return Spec{Name: "read_file", Description: "Read a text file from the workspace. Lines are numbered. Use offset/limit (1-based lines) for long files.",
		Parameters: obj(map[string]any{
			"path":   strProp("Workspace-relative path."),
			"offset": intProp("First line to return (1-based)."),
			"limit":  intProp("Maximum number of lines (default 2000)."),
		}, "path")}
}

func (t *ReadFile) Call(_ context.Context, raw json.RawMessage) Result {
	var in struct {
		Path   string `json:"path"`
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
	}
	if err := decodeArgs(raw, &in); err != nil {
		return errResult("%v", err)
	}
	abs, err := t.Sandbox.Resolve("", in.Path)
	if err != nil {
		return Result{Output: "error: " + err.Error(), Ops: fileOp(ops.OpReadFile, in.Path, false)}
	}
	rel := t.Sandbox.Rel(abs)
	info, err := os.Stat(abs)
	if err != nil {
		return Result{Output: fmt.Sprintf("error: %s: no such file", in.Path), Ops: fileOp(ops.OpReadFile, rel, false)}
	}
	if info.IsDir() {
		return Result{Output: fmt.Sprintf("error: %s is a directory; use list_files", in.Path), Ops: fileOp(ops.OpReadFile, rel, false)}
	}
	text, err := readFileCapped(abs)
	if err != nil {
		return Result{Output: "error: " + err.Error(), Ops: fileOp(ops.OpReadFile, rel, false)}
	}
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	start := max(1, in.Offset)
	limit := in.Limit
	if limit <= 0 {
		limit = 2000
	}
	var b strings.Builder
	for i := start; i <= len(lines) && i < start+limit; i++ {
		fmt.Fprintf(&b, "%6d\t%s\n", i, lines[i-1])
	}
	if start+limit <= len(lines) {
		fmt.Fprintf(&b, "[%d more lines; continue with offset %d]\n", len(lines)-(start+limit)+1, start+limit)
	}
	out, trunc := Cap(b.String())
	// The window is part of the operation, so reading a long file in chunks
	// is not the same call repeated.
	args := map[string]any{"path": rel}
	if start > 1 {
		args["offset"] = start
	}
	if in.Limit > 0 {
		args["limit"] = in.Limit
	}
	return Result{Output: out, OK: true, Truncated: trunc, Ops: []record.OpRecord{{Op: ops.Op{Op: ops.OpReadFile, Args: args}, OK: true, Output: text}}}
}

// ListFiles lists files under a directory.
type ListFiles struct{ Sandbox *Sandbox }

func (t *ListFiles) Spec() Spec {
	return Spec{Name: "list_files", Description: "List files under a workspace directory (recursive, .git skipped). Optional glob filters file names, e.g. *.go.",
		Parameters: obj(map[string]any{
			"path": strProp("Directory, default the workspace root."),
			"glob": strProp("Optional file-name glob."),
		})}
}

func (t *ListFiles) Call(_ context.Context, raw json.RawMessage) Result {
	var in struct {
		Path string `json:"path"`
		Glob string `json:"glob"`
	}
	if err := decodeArgs(raw, &in); err != nil {
		return errResult("%v", err)
	}
	if in.Path == "" {
		in.Path = "."
	}
	abs, err := t.Sandbox.Resolve("", in.Path)
	if err != nil {
		return errResult("%v", err)
	}
	var files []string
	_ = filepath.WalkDir(abs, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == "__pycache__") {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		if in.Glob != "" {
			if ok, _ := path.Match(in.Glob, d.Name()); !ok {
				return nil
			}
		}
		files = append(files, t.Sandbox.Rel(p))
		return nil
	})
	sort.Strings(files)
	out, trunc := Cap(strings.Join(files, "\n"))
	return Result{Output: out, OK: true, Truncated: trunc, Ops: fileOp(ops.OpListFiles, t.Sandbox.Rel(abs), true)}
}

// Search greps the workspace with a regular expression.
type Search struct{ Sandbox *Sandbox }

func (t *Search) Spec() Spec {
	return Spec{Name: "search", Description: "Search workspace files for a regular expression (RE2 syntax). Returns path:line: text.",
		Parameters: obj(map[string]any{
			"pattern": strProp("Regular expression."),
			"path":    strProp("Directory or file to search, default the workspace root."),
			"glob":    strProp("Optional file-name glob, e.g. *.py."),
		}, "pattern")}
}

func (t *Search) Call(_ context.Context, raw json.RawMessage) Result {
	var in struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
		Glob    string `json:"glob"`
	}
	if err := decodeArgs(raw, &in); err != nil {
		return errResult("%v", err)
	}
	re, err := regexp.Compile(in.Pattern)
	if err != nil {
		return errResult("invalid pattern: %v", err)
	}
	if in.Path == "" {
		in.Path = "."
	}
	abs, err := t.Sandbox.Resolve("", in.Path)
	if err != nil {
		return errResult("%v", err)
	}
	var b strings.Builder
	hits := 0
	_ = filepath.WalkDir(abs, func(p string, d fs.DirEntry, err error) error {
		if err != nil || hits > 500 {
			return nil
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		// Only regular files: a symlink the setup container planted could
		// point anywhere on the host.
		if !d.Type().IsRegular() {
			return nil
		}
		if in.Glob != "" {
			if ok, _ := path.Match(in.Glob, d.Name()); !ok {
				return nil
			}
		}
		data, err := os.ReadFile(p)
		if err != nil || isBinary(data) {
			return nil
		}
		for i, l := range strings.Split(string(data), "\n") {
			if re.MatchString(l) {
				fmt.Fprintf(&b, "%s:%d: %s\n", t.Sandbox.Rel(p), i+1, l)
				hits++
			}
		}
		return nil
	})
	if hits == 0 {
		b.WriteString("no matches")
	}
	out, trunc := Cap(b.String())
	return Result{Output: out, OK: true, Truncated: trunc, Ops: []record.OpRecord{{Op: ops.Op{Op: ops.OpSearch, Args: map[string]any{"pattern": in.Pattern, "path": t.Sandbox.Rel(abs)}}, OK: true}}}
}

func isBinary(b []byte) bool {
	n := min(len(b), 8000)
	for _, c := range b[:n] {
		if c == 0 {
			return true
		}
	}
	return false
}

// EditFile replaces an exact string in a file.
type EditFile struct{ Sandbox *Sandbox }

func (t *EditFile) Spec() Spec {
	return Spec{Name: "edit_file", Description: "Replace an exact string in a workspace file. old_string must match exactly once unless replace_all is true.",
		Parameters: obj(map[string]any{
			"path":        strProp("Workspace-relative path."),
			"old_string":  strProp("Exact text to replace."),
			"new_string":  strProp("Replacement text."),
			"replace_all": map[string]any{"type": "boolean", "description": "Replace every occurrence."},
		}, "path", "old_string", "new_string")}
}

func (t *EditFile) Call(_ context.Context, raw json.RawMessage) Result {
	var in struct {
		Path       string `json:"path"`
		Old        string `json:"old_string"`
		New        string `json:"new_string"`
		ReplaceAll bool   `json:"replace_all"`
	}
	if err := decodeArgs(raw, &in); err != nil {
		return errResult("%v", err)
	}
	abs, err := t.Sandbox.Resolve("", in.Path)
	if err == nil {
		err = t.Sandbox.Writable(abs)
	}
	if err != nil {
		return Result{Output: "error: " + err.Error(), Ops: fileOp(ops.OpEditFile, in.Path, false)}
	}
	rel := t.Sandbox.Rel(abs)
	data, err := os.ReadFile(abs)
	if err != nil {
		return Result{Output: fmt.Sprintf("error: %s: no such file", in.Path), Ops: fileOp(ops.OpEditFile, rel, false)}
	}
	text := string(data)
	n := strings.Count(text, in.Old)
	switch {
	case in.Old == "":
		return Result{Output: "error: old_string is empty", Ops: fileOp(ops.OpEditFile, rel, false)}
	case n == 0:
		return Result{Output: "error: old_string not found in " + rel, Ops: fileOp(ops.OpEditFile, rel, false)}
	case n > 1 && !in.ReplaceAll:
		return Result{Output: fmt.Sprintf("error: old_string matches %d times in %s; add context or set replace_all", n, rel), Ops: fileOp(ops.OpEditFile, rel, false)}
	}
	if in.ReplaceAll {
		text = strings.ReplaceAll(text, in.Old, in.New)
	} else {
		text = strings.Replace(text, in.Old, in.New, 1)
	}
	if err := os.WriteFile(abs, []byte(text), 0o644); err != nil {
		return Result{Output: "error: " + err.Error(), Ops: fileOp(ops.OpEditFile, rel, false)}
	}
	return Result{Output: fmt.Sprintf("edited %s (%d replacement(s))", rel, max(1, n*boolToInt(in.ReplaceAll))), OK: true, Ops: fileOp(ops.OpEditFile, rel, true)}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// WriteFile writes a whole file.
type WriteFile struct{ Sandbox *Sandbox }

func (t *WriteFile) Spec() Spec {
	return Spec{Name: "write_file", Description: "Create or overwrite a workspace file with the given content.",
		Parameters: obj(map[string]any{
			"path":    strProp("Workspace-relative path."),
			"content": strProp("Full file content."),
		}, "path", "content")}
}

func (t *WriteFile) Call(_ context.Context, raw json.RawMessage) Result {
	var in struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := decodeArgs(raw, &in); err != nil {
		return errResult("%v", err)
	}
	abs, err := t.Sandbox.Resolve("", in.Path)
	if err == nil {
		err = t.Sandbox.Writable(abs)
	}
	if err != nil {
		return Result{Output: "error: " + err.Error(), Ops: fileOp(ops.OpWriteFile, in.Path, false)}
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return errResult("%v", err)
	}
	if err := os.WriteFile(abs, []byte(in.Content), 0o644); err != nil {
		return errResult("%v", err)
	}
	return Result{Output: fmt.Sprintf("wrote %s (%d bytes)", t.Sandbox.Rel(abs), len(in.Content)), OK: true, Ops: fileOp(ops.OpWriteFile, t.Sandbox.Rel(abs), true)}
}
