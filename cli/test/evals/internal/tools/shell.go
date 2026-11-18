package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/ops"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/record"
)

// Shell runs bash-syntax command lines in an interpreter (mvdan.cc/sh), not
// the host shell. Only allowlisted commands exist: the arm's traceway binary,
// jq and awk (in-process), read-only text utilities (in-process, confined to
// the workspace), and read-only git. Each traceway invocation inside a
// pipeline is recorded as its own operation.
type Shell struct {
	Sandbox *Sandbox
	// Bins maps an allowed command name to the executable it runs
	// (traceway or traceway-write).
	Bins map[string]string
	// BinEnv is the environment of those executables (scrubbed: HOME, XDG,
	// PATH, TZ, LANG only).
	BinEnv []string
	// Git enables read-only git subcommands in the workspace.
	Git bool
	// Timeout bounds one command line.
	Timeout time.Duration
	// Denied names commands that exist on other arms, to explain refusals.
	Denied map[string]string
}

// Spec describes the tool.
func (s *Shell) Spec() Spec {
	var names []string
	for n := range s.Bins {
		names = append(names, n)
	}
	sort.Strings(names)
	allowed := append(names, "jq", "awk")
	allowed = append(allowed, coreutilNames()...)
	if s.Git {
		allowed = append(allowed, "git ("+strings.Join(gitAllowed, ", ")+")")
	}
	return Spec{
		Name: "shell",
		Description: "Run a bash command line in the repository workspace (pipes, redirects, $(...), && and here-docs work). " +
			"Available commands: " + strings.Join(allowed, ", ") + ". " +
			"There is no network access other than through the commands above, and no interpreters or package managers. " +
			"Output is capped at 30000 characters.",
		Parameters: obj(map[string]any{
			"command": strProp("The command line to run."),
		}, "command"),
	}
}

type shellRun struct {
	s      *Shell
	mu     sync.Mutex
	ops    []record.OpRecord
	ctx    context.Context
	runner *interp.Runner
}

// Call runs one command line.
func (s *Shell) Call(ctx context.Context, raw json.RawMessage) Result {
	var in struct {
		Command string `json:"command"`
	}
	if err := decodeArgs(raw, &in); err != nil {
		return errResult("%v", err)
	}
	if strings.TrimSpace(in.Command) == "" {
		return errResult("command is empty")
	}
	file, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(in.Command), "")
	if err != nil {
		return errResult("syntax error: %v", err)
	}
	timeout := s.Timeout
	if timeout == 0 {
		timeout = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var out lockedBuffer
	run := &shellRun{s: s, ctx: ctx}
	env := []string{"HOME=" + s.Sandbox.Root, "PATH=/usr/bin:/bin", "TZ=UTC", "LANG=C.UTF-8", "PWD=" + s.Sandbox.Root}
	runner, err := interp.New(
		interp.StdIO(nil, &out, &out),
		interp.Dir(s.Sandbox.Root),
		interp.Env(expand.ListEnviron(env...)),
		interp.ExecHandlers(func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc { return run.exec }),
		interp.OpenHandler(run.open),
		interp.ReadDirHandler2(run.readDir),
		interp.StatHandler(run.stat),
	)
	if err != nil {
		return errResult("interpreter: %v", err)
	}
	run.runner = runner
	runErr := runner.Run(ctx, file)
	exit := 0
	if runErr != nil {
		if st, ok := interp.IsExitStatus(runErr); ok {
			exit = int(st)
		} else if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			out.WriteString(fmt.Sprintf("\n[command timed out after %s]", timeout))
			exit = 124
		} else {
			out.WriteString("\n" + runErr.Error())
			exit = 1
		}
	}
	text := out.String()
	if exit != 0 {
		text += fmt.Sprintf("\n[exit status %d]", exit)
	}
	capped, trunc := Cap(text)
	return Result{Output: capped, OK: exit == 0, Truncated: trunc, Ops: run.ops}
}

func (r *shellRun) record(op record.OpRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ops = append(r.ops, op)
}

func (r *shellRun) exec(ctx context.Context, args []string) error {
	hc := interp.HandlerCtx(ctx)
	name := args[0]
	if strings.Contains(name, "/") {
		name = filepath.Base(name)
	}
	if bin, ok := r.s.Bins[name]; ok {
		return r.execTraceway(ctx, hc, bin, args)
	}
	if name == "git" && r.s.Git {
		return r.execGit(ctx, hc, args)
	}
	if name == "jq" {
		return exitStatus(runJQ(hc, r, args[1:]))
	}
	if name == "awk" {
		return exitStatus(runAWK(hc, r, args[1:]))
	}
	if name == "xargs" {
		return exitStatus(r.xargs(ctx, hc, args[1:]))
	}
	if f, ok := coreutils[name]; ok {
		return exitStatus(f(&uctx{hc: hc, run: r, ctx: ctx}, args[1:]))
	}
	msg := fmt.Sprintf("%s: command not found in this sandbox", name)
	if why, ok := r.s.Denied[name]; ok {
		msg = fmt.Sprintf("%s: %s", name, why)
	}
	fmt.Fprintln(hc.Stderr, msg)
	return interp.NewExitStatus(127)
}

func exitStatus(code int) error {
	if code == 0 {
		return nil
	}
	return interp.NewExitStatus(uint8(code))
}

// limitedBuffer keeps the first n bytes it is given.
type limitedBuffer struct {
	buf   bytes.Buffer
	n     int
	trunc bool
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
	if room := l.n - l.buf.Len(); room > 0 {
		if len(p) > room {
			l.buf.Write(p[:room])
			l.trunc = true
		} else {
			l.buf.Write(p)
		}
	} else if len(p) > 0 {
		l.trunc = true
	}
	return len(p), nil
}

func stdinOf(hc interp.HandlerContext) io.Reader {
	if hc.Stdin == nil {
		return nil
	}
	if f, ok := hc.Stdin.(*os.File); ok && f == nil {
		return nil
	}
	return hc.Stdin
}

func (r *shellRun) execTraceway(ctx context.Context, hc interp.HandlerContext, bin string, args []string) error {
	cmd := exec.CommandContext(ctx, bin, args[1:]...)
	cmd.Dir = hc.Dir
	cmd.Env = r.s.BinEnv
	if in := stdinOf(hc); in != nil {
		cmd.Stdin = in
	}
	so := &limitedBuffer{n: 200000}
	se := &limitedBuffer{n: 20000}
	cmd.Stdout = io.MultiWriter(hc.Stdout, so)
	cmd.Stderr = io.MultiWriter(hc.Stderr, se)
	err := cmd.Run()
	exit := 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exit = ee.ExitCode()
		} else {
			fmt.Fprintln(hc.Stderr, err)
			exit = 126
		}
	}
	output := so.buf.String()
	if se.buf.Len() > 0 {
		output += "\n" + se.buf.String()
	}
	op := ops.ParseCLI(args[1:])
	r.record(record.OpRecord{Op: op, Argv: append([]string{filepath.Base(bin)}, args[1:]...), Exit: exit, OK: exit == 0, Output: output, Truncated: so.trunc})
	return exitStatus(exit)
}

var gitAllowed = []string{"log", "show", "diff", "status", "blame", "ls-files", "rev-parse", "shortlog"}

// gitDenied are options of the allowed subcommands that write a file or run
// a program.
var gitDenied = []string{"--output", "--ext-diff"}

// execGit runs a read-only git subcommand on the workspace repository. GIT_DIR
// is pinned, so git never discovers a repository (and its config) written
// elsewhere in the workspace; the workspace's .git is read-only to every tool.
func (r *shellRun) execGit(ctx context.Context, hc interp.HandlerContext, args []string) error {
	refuse := func(format string, a ...any) error {
		fmt.Fprintf(hc.Stderr, "git: "+format+"\n", a...)
		return interp.NewExitStatus(2)
	}
	if _, err := r.s.Sandbox.Resolve("", hc.Dir); err != nil {
		return refuse("only the workspace repository is available")
	}
	rest := args[1:]
	for len(rest) > 0 && (rest[0] == "--no-pager" || rest[0] == "-P") {
		rest = rest[1:]
	}
	if len(rest) > 0 && strings.HasPrefix(rest[0], "-") {
		return refuse("global options are not allowed in this sandbox")
	}
	sub := ""
	if len(rest) > 0 {
		sub = rest[0]
	}
	if !slices.Contains(gitAllowed, sub) {
		return refuse("%s: only read-only git (%s) is available", sub, strings.Join(gitAllowed, ", "))
	}
	for _, a := range rest[1:] {
		if err := r.gitArg(hc.Dir, a); err != nil {
			return refuse("%v", err)
		}
	}
	root := r.s.Sandbox.Root
	cmd := exec.CommandContext(ctx, "git", append([]string{"--no-pager"}, rest...)...)
	cmd.Dir = hc.Dir
	cmd.Env = []string{"HOME=" + root, "PATH=/usr/bin:/bin", "GIT_DIR=" + filepath.Join(root, ".git"), "GIT_WORK_TREE=" + root,
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_PAGER=cat", "PAGER=cat", "TZ=UTC", "LANG=C.UTF-8", "GIT_OPTIONAL_LOCKS=0"}
	so := &limitedBuffer{n: 100000}
	cmd.Stdout = io.MultiWriter(hc.Stdout, so)
	cmd.Stderr = hc.Stderr
	err := cmd.Run()
	exit := 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exit = ee.ExitCode()
		} else {
			exit = 126
		}
	}
	r.record(record.OpRecord{Op: ops.Op{Op: ops.OpGit, Args: map[string]any{"subcommand": sub}}, Argv: args, Exit: exit, OK: exit == 0, Output: so.buf.String()})
	return exitStatus(exit)
}

// gitArg refuses a denied option, and any argument or option value that
// names an existing file outside the workspace: git diff reads two such paths
// even without --no-index, and blame --contents, -O and the like read one.
// Values that name nothing on disk (a pickaxe string such as -S/api/orders)
// pass.
func (r *shellRun) gitArg(dir, a string) error {
	var values []string
	switch {
	case a == "--":
	case strings.HasPrefix(a, "--"):
		name, value, _ := strings.Cut(a, "=")
		if slices.Contains(gitDenied, name) {
			return fmt.Errorf("%s is not available in this sandbox", name)
		}
		values = []string{value}
	case strings.HasPrefix(a, "-"):
		// A short option's value is attached after its letter (-O../x), and
		// letters can be bundled before it (-pO../x).
		for i := 2; i < len(a); i++ {
			values = append(values, a[i:])
		}
	default:
		values = []string{a}
	}
	for _, v := range values {
		if v == "" {
			continue
		}
		if _, err := r.s.Sandbox.Resolve(dir, v); err == nil {
			continue
		}
		p := v
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		if _, err := os.Lstat(p); err == nil {
			return fmt.Errorf("%s: %w", v, ErrOutside)
		}
	}
	return nil
}

func (r *shellRun) open(ctx context.Context, path string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	hc := interp.HandlerCtx(ctx)
	abs, err := r.s.Sandbox.Resolve(hc.Dir, path)
	if err != nil {
		return nil, err
	}
	if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_APPEND|os.O_TRUNC) != 0 && abs != "/dev/null" {
		if err := r.s.Sandbox.Writable(abs); err != nil {
			return nil, err
		}
		r.record(record.OpRecord{Op: ops.Op{Op: ops.OpWriteFile, Args: map[string]any{"path": r.s.Sandbox.Rel(abs)}, Write: false}, OK: true})
	}
	return os.OpenFile(abs, flag, perm)
}

func (r *shellRun) readDir(ctx context.Context, path string) ([]fs.DirEntry, error) {
	hc := interp.HandlerCtx(ctx)
	abs, err := r.s.Sandbox.Resolve(hc.Dir, path)
	if err != nil {
		return nil, err
	}
	return os.ReadDir(abs)
}

func (r *shellRun) stat(ctx context.Context, name string, follow bool) (fs.FileInfo, error) {
	hc := interp.HandlerCtx(ctx)
	abs, err := r.s.Sandbox.Resolve(hc.Dir, name)
	if err != nil {
		return nil, os.ErrNotExist
	}
	if follow {
		return os.Stat(abs)
	}
	return os.Lstat(abs)
}

// xargs builds command lines from stdin and runs them through the same
// allowlisted handler.
func (r *shellRun) xargs(ctx context.Context, hc interp.HandlerContext, args []string) int {
	replace := ""
	perCall := 0
	i := 0
	for ; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-I" && i+1 < len(args):
			replace = args[i+1]
			i++
		case strings.HasPrefix(a, "-I") && len(a) > 2:
			replace = a[2:]
		case a == "-n" && i+1 < len(args):
			fmt.Sscanf(args[i+1], "%d", &perCall)
			i++
		case a == "-r" || a == "--no-run-if-empty" || a == "-0":
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(hc.Stderr, "xargs: unsupported option %s\n", a)
			return 2
		default:
			goto done
		}
	}
done:
	cmdArgs := args[i:]
	if len(cmdArgs) == 0 {
		cmdArgs = []string{"echo"}
	}
	in := stdinOf(hc)
	if in == nil {
		return 0
	}
	data, _ := io.ReadAll(in)
	var items []string
	if replace != "" {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(line) != "" {
				items = append(items, strings.TrimSpace(line))
			}
		}
	} else {
		items = strings.Fields(string(data))
	}
	code := 0
	runOne := func(argv []string) {
		if err := r.exec(ctx, argv); err != nil {
			if st, ok := interp.IsExitStatus(err); ok {
				code = 123
				_ = st
			} else {
				code = 1
			}
		}
	}
	switch {
	case replace != "":
		for _, it := range items {
			argv := make([]string, len(cmdArgs))
			for k, a := range cmdArgs {
				argv[k] = strings.ReplaceAll(a, replace, it)
			}
			runOne(argv)
		}
	case perCall > 0:
		for k := 0; k < len(items); k += perCall {
			end := min(k+perCall, len(items))
			runOne(append(append([]string{}, cmdArgs...), items[k:end]...))
		}
	default:
		if len(items) > 0 {
			runOne(append(append([]string{}, cmdArgs...), items...))
		}
	}
	return code
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *lockedBuffer) WriteString(s string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buf.WriteString(s)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}
