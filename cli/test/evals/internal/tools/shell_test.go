package tools

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func newShell(t *testing.T) (*Shell, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "work", ".skills", "traceway"), 0o755); err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(root, "work")
	os.WriteFile(filepath.Join(work, ".skills", "traceway", "SKILL.md"), []byte("---\nname: traceway\ndescription: 'x'\n---\nbody"), 0o644)
	os.WriteFile(filepath.Join(work, "notes.txt"), []byte("alpha 3\nbeta 1\ngamma 2\nbeta 1\n"), 0o644)
	// A stand-in for the traceway binary: prints its args and environment.
	bin := filepath.Join(root, "bin", "traceway")
	os.MkdirAll(filepath.Dir(bin), 0o755)
	os.WriteFile(bin, []byte("#!/bin/sh\nif [ \"$1\" = env ]; then env; exit 0; fi\nif [ \"$1\" = fail ]; then echo '{\"error\":\"not_found\"}' >&2; exit 5; fi\necho '{\"data\":[{\"endpoint\":\"GET /a\",\"p95Duration\":1500000000},{\"endpoint\":\"GET /b\",\"p95Duration\":20000000}]}'\n"), 0o755)
	sb := &Sandbox{Root: work, ReadOnly: []string{".skills"}}
	return &Shell{Sandbox: sb, Bins: map[string]string{"traceway": bin}, BinEnv: []string{"HOME=" + root, "PATH=/usr/bin:/bin", "TZ=UTC"}}, work
}

func run(t *testing.T, s *Shell, cmd string) Result {
	t.Helper()
	raw, _ := json.Marshal(map[string]string{"command": cmd})
	return s.Call(context.Background(), raw)
}

func TestShellUtilities(t *testing.T) {
	s, _ := newShell(t)
	cases := []struct{ cmd, want string }{
		{`echo hi | grep h`, "hi"},
		{`sort -k2 -n notes.txt | head -1`, "beta 1"},
		{`sort notes.txt | uniq -c | sort -rn | head -1 | tr -s ' '`, "2 beta 1"},
		{`cut -d' ' -f1 notes.txt | tail -n 1`, "beta"},
		{`awk '{s+=$2} END {print s}' notes.txt`, "7"},
		{`echo '{"a":[1,2,3]}' | jq '.a | length'`, "3"},
		{`echo '{"a":"x"}' | jq -r .a`, "x"},
		{`grep -c beta notes.txt`, "2"},
		{`wc -l < notes.txt`, "4"},
		{`X=$(echo nested); echo "got $X"`, "got nested"},
		{`printf 'M\nx\nM\ny\nz\n' | grep -A3 M | tr '\n' ,`, "M,x,M,y,z,"},
		{`printf 'M\na\nb\nc\nM\n' | grep -A1 M | tr '\n' ,`, "M,a,--,M,"},
		{`printf 'a\nb\nM\nc\n' | grep -B1 -n M | tr '\n' ,`, "2-b,3:M,"},
		{`grep -e alpha -e gamma notes.txt | wc -l`, "2"},
		{`echo '{"a":"x"}' | jq .a -r`, "x"},
		{`echo '{"a":{"b":1}}' | jq -c .a`, `{"b":1}`},
	}
	for _, c := range cases {
		r := run(t, s, c.cmd)
		if !r.OK || strings.TrimSpace(r.Output) != c.want {
			t.Errorf("%s: got ok=%v %q, want %q", c.cmd, r.OK, r.Output, c.want)
		}
	}
}

func TestShellConfinement(t *testing.T) {
	s, work := newShell(t)
	for _, cmd := range []string{`cat /etc/passwd`, `cat ../../etc/passwd`, `ls /`, `grep root /etc/passwd`, `cd / && cat etc/hostname`} {
		if r := run(t, s, cmd); r.OK || !strings.Contains(r.Output, "outside the workspace") && !strings.Contains(r.Output, "exit status") {
			t.Errorf("%s escaped the sandbox: %q", cmd, r.Output)
		}
	}
	for _, cmd := range []string{`curl https://example.com`, `python3 -c 1`, `bash -c id`, `sh -c id`, `node -e 1`, `go version`} {
		if r := run(t, s, cmd); r.OK || !strings.Contains(r.Output, "not found") {
			t.Errorf("%s should be unavailable: %q", cmd, r.Output)
		}
	}
	if r := run(t, s, `echo hacked > .skills/traceway/SKILL.md`); r.OK {
		t.Errorf("wrote into .skills: %q", r.Output)
	}
	if r := run(t, s, `echo kept > out.txt && cat out.txt`); !r.OK || strings.TrimSpace(r.Output) != "kept" {
		t.Errorf("workspace write failed: %q", r.Output)
	}
	if _, err := os.Stat(filepath.Join(work, "out.txt")); err != nil {
		t.Error(err)
	}
}

// TestShellDevNullIsNotAWrite: discarding stderr is not a file operation,
// so it never counts toward the trajectory grader's repeat limit.
func TestShellDevNullIsNotAWrite(t *testing.T) {
	s, _ := newShell(t)
	if r := run(t, s, `grep beta notes.txt 2>/dev/null >/dev/null`); !r.OK || len(r.Ops) != 0 {
		t.Fatalf("ok=%v ops=%+v", r.OK, r.Ops)
	}
}

func TestShellRecordsTracewayInPipelines(t *testing.T) {
	s, _ := newShell(t)
	r := run(t, s, `traceway endpoints list --since 24h --order-by p95 --output json | jq -r '.data[0].endpoint'`)
	if !r.OK || strings.TrimSpace(r.Output) != "GET /a" {
		t.Fatalf("pipeline output %q", r.Output)
	}
	if len(r.Ops) != 1 || r.Ops[0].Op.Op != "endpoints.list" || r.Ops[0].Args["since"] != "24h" || r.Ops[0].Args["order_by"] != "p95" {
		t.Fatalf("ops %+v", r.Ops)
	}
	if !strings.Contains(r.Ops[0].Output, "p95Duration") {
		t.Error("the raw output before the pipe must be kept for grounding")
	}
	r = run(t, s, `traceway fail x; traceway exceptions show abc`)
	if len(r.Ops) != 2 || r.Ops[0].OK || r.Ops[0].Exit != 5 || !r.Ops[1].OK || r.Ops[1].Args["hash"] != "abc" {
		t.Fatalf("ops %+v", r.Ops)
	}
}

// TestToolEnvIsScrubbed asserts that secrets in the harness's own
// environment never reach tool subprocesses.
func TestToolEnvIsScrubbed(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "sk-or-secret")
	t.Setenv("GITHUB_TOKEN", "ghp_secret")
	t.Setenv("JWT_SECRET", "jwt-secret-value-that-is-long-enough")
	s, _ := newShell(t)
	r := run(t, s, `traceway env`)
	for _, leak := range []string{"sk-or-secret", "ghp_secret", "jwt-secret-value", "OPENROUTER", "GITHUB_TOKEN", "JWT_SECRET"} {
		if strings.Contains(r.Output, leak) {
			t.Errorf("tool environment leaked %s:\n%s", leak, r.Output)
		}
	}
	r = run(t, s, `echo "$OPENROUTER_API_KEY$GITHUB_TOKEN$JWT_SECRET"`)
	if strings.TrimSpace(r.Output) != "" {
		t.Errorf("interpreter environment leaked: %q", r.Output)
	}
}

func TestEditAndReadFile(t *testing.T) {
	s, _ := newShell(t)
	ed := &EditFile{Sandbox: s.Sandbox}
	res := ed.Call(context.Background(), json.RawMessage(`{"path":"notes.txt","old_string":"gamma 2","new_string":"gamma 9"}`))
	if !res.OK {
		t.Fatal(res.Output)
	}
	rd := &ReadFile{Sandbox: s.Sandbox}
	res = rd.Call(context.Background(), json.RawMessage(`{"path":"notes.txt"}`))
	if !strings.Contains(res.Output, "gamma 9") {
		t.Fatal(res.Output)
	}
	res = ed.Call(context.Background(), json.RawMessage(`{"path":".skills/traceway/SKILL.md","old_string":"body","new_string":"x"}`))
	if res.OK || !strings.Contains(res.Output, "read-only") {
		t.Fatalf("edited .skills: %q", res.Output)
	}
	res = rd.Call(context.Background(), json.RawMessage(`{"path":"/etc/passwd"}`))
	if res.OK {
		t.Fatal("read outside the workspace")
	}
}

func TestAskUserRules(t *testing.T) {
	a := &AskUser{}
	if got := a.Answer("anything"); !strings.Contains(got, "anything to add") {
		t.Fatal(got)
	}
}

// TestShellGitIsReadOnly holds git to the workspace: no config written into
// the workspace can make git run a command on the host, and no argument reads
// or writes a file outside it (the run directory around it holds seed.json).
func TestShellGitIsReadOnly(t *testing.T) {
	s, work := newShell(t)
	s.Git = true
	outside := filepath.Dir(work)
	os.WriteFile(filepath.Join(outside, "seed.json"), []byte("answer-key"), 0o644)
	for _, args := range [][]string{{"init", "-q"}, {"add", "notes.txt"}, {"-c", "user.email=x@y", "-c", "user.name=x", "commit", "-qm", "init"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = work
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("git unavailable: %v %s", err, out)
		}
	}
	const hook = `[core]\n\tfsmonitor = \"echo x > ` + "`" + `pwd` + "`" + `/../PWNED; false\"\n[diff]\n\texternal = \"echo x > ../PWNED; false\"\n`
	write := &WriteFile{Sandbox: s.Sandbox}
	for _, path := range []string{".git/config", ".GIT/config", "sub/.git/config", "bare/config"} {
		res := write.Call(context.Background(), json.RawMessage(`{"path":"`+path+`","content":"`+hook+`"}`))
		if strings.HasSuffix(path, ".git/config") || strings.HasSuffix(path, ".GIT/config") {
			if res.OK {
				t.Errorf("write_file wrote %s", path)
			}
		}
	}
	if r := run(t, s, `printf '[core]\n\tfsmonitor = "echo x > ../PWNED; false"\n' >> .git/config`); r.OK {
		t.Errorf("the shell wrote .git/config: %q", r.Output)
	}
	// A bare repository's layout in a subdirectory, which git would otherwise
	// discover and take its config from.
	for _, f := range []string{"HEAD", "objects/x", "refs/x"} {
		write.Call(context.Background(), json.RawMessage(`{"path":"bare/`+f+`","content":"ref: refs/heads/main\n"}`))
	}
	if r := run(t, s, `cd bare && git rev-parse --absolute-git-dir`); strings.TrimSpace(r.Output) != filepath.Join(work, ".git") {
		t.Errorf("git in bare/ used %q, want the workspace repository", r.Output)
	}
	for _, cmd := range []string{`git status`, `git diff`, `git log -p -1`, `git show HEAD`, `cd sub && git status`} {
		run(t, s, cmd)
	}
	if _, err := os.Stat(filepath.Join(outside, "PWNED")); err == nil {
		t.Error("a git command ran a command written into the workspace")
	}
	for _, cmd := range []string{
		`git diff --no-index ../seed.json /dev/null`,
		`git diff --no-index -- ../seed.json /dev/null`,
		`git diff ../seed.json /dev/null`,
		`git diff -- ` + filepath.Join(outside, "seed.json") + ` /dev/null`,
		`git blame --contents ../seed.json notes.txt`,
		`git blame --contents=../seed.json notes.txt`,
		`git log -pO../seed.json -1`,
		`git log --output=../out.txt -1`,
		`git diff --output ../out.txt HEAD`,
		`git log --ext-diff -p -1`,
		`git --exec-path=.. status`,
		`git --config-env=core.fsmonitor=HOME status`,
		`git -c core.pager=cat log`,
		`cd .. && git diff seed.json /dev/null`,
	} {
		r := run(t, s, cmd)
		if r.OK || strings.Contains(r.Output, "answer-key") {
			t.Errorf("%s: ok=%v %q", cmd, r.OK, r.Output)
		}
	}
	if _, err := os.Stat(filepath.Join(outside, "out.txt")); err == nil {
		t.Error("git wrote a file outside the workspace")
	}
	for _, cmd := range []string{`git log --oneline | wc -l`, `git --no-pager log --oneline -S/api/orders | wc -l`, `git log --format=%s -- notes.txt`, `cd .skills && git status --short`} {
		if r := run(t, s, cmd); !r.OK {
			t.Errorf("%s: %q", cmd, r.Output)
		}
	}
}
