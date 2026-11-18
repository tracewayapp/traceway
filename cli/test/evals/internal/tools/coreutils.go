package tools

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"mvdan.cc/sh/v3/interp"
)

// In-process text utilities. They implement the commonly used flags of their
// GNU namesakes and read files only through the sandbox, so `cat /etc/passwd`
// fails like any other path outside the workspace.

type uctx struct {
	hc  interp.HandlerContext
	run *shellRun
	// ctx carries the command's timeout (nil for jq and awk).
	ctx context.Context
}

func (u *uctx) errf(format string, a ...any) { fmt.Fprintf(u.hc.Stderr, format+"\n", a...) }

func (u *uctx) open(p string) (io.ReadCloser, error) {
	if p == "-" {
		if in := stdinOf(u.hc); in != nil {
			return io.NopCloser(in), nil
		}
		return io.NopCloser(strings.NewReader("")), nil
	}
	abs, err := u.run.s.Sandbox.Resolve(u.hc.Dir, p)
	if err != nil {
		return nil, err
	}
	return os.Open(abs)
}

// inputs concatenates the named files, or stdin when none are given.
func (u *uctx) inputs(files []string) ([]byte, int) {
	if len(files) == 0 {
		files = []string{"-"}
	}
	var buf bytes.Buffer
	code := 0
	for _, f := range files {
		rc, err := u.open(f)
		if err != nil {
			u.errf("%s: %v", f, err)
			code = 1
			continue
		}
		_, _ = io.Copy(&buf, rc)
		rc.Close()
	}
	return buf.Bytes(), code
}

func splitLines(b []byte) []string {
	s := string(b)
	if s == "" {
		return nil
	}
	s = strings.TrimSuffix(s, "\n")
	return strings.Split(s, "\n")
}

func (u *uctx) writeLines(lines []string) {
	w := bufio.NewWriter(u.hc.Stdout)
	for _, l := range lines {
		w.WriteString(l)
		w.WriteByte('\n')
	}
	w.Flush()
}

// coreutils are the in-process utilities the sandboxed shell offers;
// coreutils_more.go adds the rest at init.
var coreutils = map[string]func(u *uctx, args []string) int{
	"cat": uCat, "head": uHead, "tail": uTail, "wc": uWc, "grep": uGrep, "egrep": func(u *uctx, a []string) int { return uGrep(u, append([]string{"-E"}, a...)) },
	"sort": uSort, "uniq": uUniq, "cut": uCut, "tr": uTr, "ls": uLs, "echo": uEcho,
}

func coreutilNames() []string {
	var out []string
	for k := range coreutils {
		if k != "egrep" && k != "echo" {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return append(out, "xargs", "echo", "printf", "test")
}

// flags is a tiny getopt: single-dash letters (bundled), --long, and
// options that take a value. set keeps an option's last value, vals every
// value of a repeatable one (grep -e A -e B).
type flags struct {
	set  map[string]string
	vals map[string][]string
	args []string
}

func parseFlags(args []string, withValue string) flags {
	f := flags{set: map[string]string{}, vals: map[string][]string{}}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			f.args = append(f.args, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "--") && len(a) > 2 {
			k, v, ok := strings.Cut(a[2:], "=")
			if !ok {
				v = "true"
			}
			f.set["-"+k] = v
			continue
		}
		if strings.HasPrefix(a, "-") && len(a) > 1 && !isNumber(a) {
			for j := 1; j < len(a); j++ {
				c := string(a[j])
				if strings.Contains(withValue, c) {
					v := a[j+1:]
					if v == "" && i+1 < len(args) {
						i++
						v = args[i]
					}
					f.set[c] = v
					f.vals[c] = append(f.vals[c], v)
					break
				}
				f.set[c] = "true"
			}
			continue
		}
		f.args = append(f.args, a)
	}
	return f
}

func (f flags) has(k string) bool { _, ok := f.set[k]; return ok }

func (f flags) int(k string, def int) int {
	if v, ok := f.set[k]; ok {
		n, err := strconv.Atoi(strings.TrimPrefix(v, "+"))
		if err == nil {
			return n
		}
	}
	return def
}

func isNumber(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

func uCat(u *uctx, args []string) int {
	f := parseFlags(args, "")
	data, code := u.inputs(f.args)
	if f.has("n") {
		lines := splitLines(data)
		for i, l := range lines {
			lines[i] = fmt.Sprintf("%6d\t%s", i+1, l)
		}
		u.writeLines(lines)
		return code
	}
	u.hc.Stdout.Write(data)
	return code
}

// numericShorthand turns `head -20` into `head -n 20`.
func numericShorthand(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if len(a) > 1 && a[0] == '-' && isNumber(a[1:]) {
			out = append(out, "-n", a[1:])
			continue
		}
		out = append(out, a)
	}
	return out
}

func uHead(u *uctx, args []string) int {
	f := parseFlags(numericShorthand(args), "nc")
	data, code := u.inputs(f.args)
	if f.has("c") {
		n := f.int("c", 0)
		if n < len(data) {
			data = data[:n]
		}
		u.hc.Stdout.Write(data)
		return code
	}
	n := f.int("n", 10)
	lines := splitLines(data)
	if n < 0 {
		n = max(0, len(lines)+n)
	}
	if n < len(lines) {
		lines = lines[:n]
	}
	u.writeLines(lines)
	return code
}

func uTail(u *uctx, args []string) int {
	f := parseFlags(numericShorthand(args), "nc")
	data, code := u.inputs(f.args)
	lines := splitLines(data)
	v := f.set["n"]
	if strings.HasPrefix(v, "+") {
		from := f.int("n", 1)
		if from-1 < len(lines) {
			lines = lines[max(0, from-1):]
		} else {
			lines = nil
		}
	} else {
		n := f.int("n", 10)
		if n < len(lines) {
			lines = lines[len(lines)-n:]
		}
	}
	u.writeLines(lines)
	return code
}

func uWc(u *uctx, args []string) int {
	f := parseFlags(args, "")
	data, code := u.inputs(f.args)
	lines := bytes.Count(data, []byte("\n"))
	words := len(strings.Fields(string(data)))
	chars := len(data)
	var parts []string
	all := !f.has("l") && !f.has("w") && !f.has("c") && !f.has("m")
	if all || f.has("l") {
		parts = append(parts, strconv.Itoa(lines))
	}
	if all || f.has("w") {
		parts = append(parts, strconv.Itoa(words))
	}
	if all || f.has("c") || f.has("m") {
		parts = append(parts, strconv.Itoa(chars))
	}
	fmt.Fprintln(u.hc.Stdout, strings.Join(parts, " "))
	return code
}

// breToERE converts a POSIX basic regexp to Go syntax.
func breToERE(p string) string {
	var b strings.Builder
	for i := 0; i < len(p); i++ {
		c := p[i]
		if c == '\\' && i+1 < len(p) {
			n := p[i+1]
			switch n {
			case '(', ')', '|', '+', '?', '{', '}':
				b.WriteByte(n)
			default:
				b.WriteByte('\\')
				b.WriteByte(n)
			}
			i++
			continue
		}
		switch c {
		case '(', ')', '|', '+', '?', '{', '}':
			b.WriteByte('\\')
		}
		b.WriteByte(c)
	}
	return b.String()
}

func uGrep(u *uctx, args []string) int {
	f := parseFlags(args, "eABCm")
	patterns := f.vals["e"]
	rest := f.args
	if len(patterns) == 0 {
		if len(rest) == 0 {
			u.errf("grep: no pattern")
			return 2
		}
		patterns, rest = []string{rest[0]}, rest[1:]
	}
	var res []*regexp.Regexp
	for _, p := range patterns {
		switch {
		case f.has("F"):
			p = regexp.QuoteMeta(p)
		case f.has("E") || f.has("P"):
		default:
			p = breToERE(p)
		}
		if f.has("w") {
			p = `\b(?:` + p + `)\b`
		}
		if f.has("x") {
			p = `^(?:` + p + `)$`
		}
		if f.has("i") {
			p = "(?i)" + p
		}
		re, err := regexp.Compile(p)
		if err != nil {
			u.errf("grep: %v", err)
			return 2
		}
		res = append(res, re)
	}
	match := func(l string) bool {
		for _, re := range res {
			if re.MatchString(l) {
				return !f.has("v")
			}
		}
		return f.has("v")
	}
	files := rest
	if f.has("r") || f.has("R") {
		if len(files) == 0 {
			files = []string{"."}
		}
		var expanded []string
		for _, root := range files {
			abs, err := u.run.s.Sandbox.Resolve(u.hc.Dir, root)
			if err != nil {
				u.errf("grep: %v", err)
				continue
			}
			_ = filepath.WalkDir(abs, func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules") {
					return filepath.SkipDir
				}
				if !d.IsDir() {
					rel, _ := filepath.Rel(u.hc.Dir, p)
					expanded = append(expanded, rel)
				}
				return nil
			})
		}
		files = expanded
	}
	multi := len(files) > 1 || f.has("r") || f.has("R")
	if f.has("h") {
		multi = false
	}
	if f.has("H") {
		multi = true
	}
	if len(files) == 0 {
		files = []string{"-"}
	}
	after, before := f.int("A", 0), f.int("B", 0)
	if c := f.int("C", 0); c > 0 {
		after, before = c, c
	}
	maxCount := f.int("m", -1)
	found := false
	w := bufio.NewWriter(u.hc.Stdout)
	defer w.Flush()
	for _, file := range files {
		rc, err := u.open(file)
		if err != nil {
			u.errf("grep: %s: %v", file, err)
			continue
		}
		data, _ := io.ReadAll(rc)
		rc.Close()
		lines := splitLines(data)
		count := 0
		lastPrinted := -1
		afterLeft := 0
		prefix := func(i int, sep string) string {
			p := ""
			if multi {
				p = file + sep
			}
			if f.has("n") {
				p += strconv.Itoa(i+1) + sep
			}
			return p
		}
		// printLine prints line i once, with grep's "--" between context
		// groups that are not adjacent.
		printLine := func(i int, sep string) {
			if (before > 0 || after > 0) && lastPrinted >= 0 && i > lastPrinted+1 {
				fmt.Fprintln(w, "--")
			}
			fmt.Fprintln(w, prefix(i, sep)+lines[i])
			lastPrinted = i
		}
		for i, l := range lines {
			if !match(l) {
				if afterLeft > 0 && !f.has("c") && !f.has("o") {
					printLine(i, "-")
					afterLeft--
				}
				continue
			}
			if maxCount >= 0 && count >= maxCount {
				break
			}
			found = true
			count++
			if f.has("q") {
				return 0
			}
			if f.has("l") {
				fmt.Fprintln(w, file)
				break
			}
			if f.has("c") {
				continue
			}
			if f.has("o") && !f.has("v") {
				for _, re := range res {
					for _, m := range re.FindAllString(l, -1) {
						fmt.Fprintln(w, prefix(i, ":")+m)
					}
				}
				continue
			}
			for b := max(lastPrinted+1, i-before); b < i; b++ {
				printLine(b, "-")
			}
			printLine(i, ":")
			afterLeft = after
		}
		if f.has("c") {
			if multi {
				fmt.Fprintf(w, "%s:%d\n", file, count)
			} else {
				fmt.Fprintln(w, count)
			}
		}
	}
	if !found {
		return 1
	}
	return 0
}

func uSort(u *uctx, args []string) int {
	f := parseFlags(args, "kt")
	data, code := u.inputs(f.args)
	lines := splitLines(data)
	sep := f.set["t"]
	keyStart, keyEnd := 0, 0
	if k, ok := f.set["k"]; ok {
		a, b, _ := strings.Cut(k, ",")
		keyStart, _ = strconv.Atoi(strings.TrimRight(a, "nrh"))
		keyEnd, _ = strconv.Atoi(strings.TrimRight(b, "nrh"))
		if strings.ContainsAny(a+b, "n") {
			f.set["n"] = "true"
		}
		if strings.ContainsAny(a+b, "r") {
			f.set["r"] = "true"
		}
	}
	key := func(l string) string {
		if keyStart == 0 {
			return l
		}
		var fields []string
		if sep != "" {
			fields = strings.Split(l, sep)
		} else {
			fields = strings.Fields(l)
		}
		if keyStart-1 >= len(fields) {
			return ""
		}
		end := len(fields)
		if keyEnd > 0 && keyEnd < end {
			end = keyEnd
		}
		j := " "
		if sep != "" {
			j = sep
		}
		return strings.Join(fields[keyStart-1:end], j)
	}
	numeric := f.has("n") || f.has("g") || f.has("h")
	less := func(a, b string) bool {
		ka, kb := key(a), key(b)
		if numeric {
			na, _ := strconv.ParseFloat(strings.TrimSpace(firstNumber(ka)), 64)
			nb, _ := strconv.ParseFloat(strings.TrimSpace(firstNumber(kb)), 64)
			if na != nb {
				return na < nb
			}
			return ka < kb
		}
		if f.has("f") {
			return strings.ToLower(ka) < strings.ToLower(kb)
		}
		return ka < kb
	}
	sort.SliceStable(lines, func(i, j int) bool {
		if f.has("r") {
			return less(lines[j], lines[i])
		}
		return less(lines[i], lines[j])
	})
	if f.has("u") {
		var out []string
		for i, l := range lines {
			if i == 0 || key(l) != key(lines[i-1]) {
				out = append(out, l)
			}
		}
		lines = out
	}
	u.writeLines(lines)
	return code
}

var numRe = regexp.MustCompile(`-?[0-9]+(\.[0-9]+)?([eE][-+]?[0-9]+)?`)

func firstNumber(s string) string {
	if m := numRe.FindString(s); m != "" {
		return m
	}
	return "0"
}

func uUniq(u *uctx, args []string) int {
	f := parseFlags(args, "")
	data, code := u.inputs(f.args)
	lines := splitLines(data)
	type run struct {
		line string
		n    int
	}
	var runs []run
	for _, l := range lines {
		cmp := l
		if f.has("i") {
			cmp = strings.ToLower(l)
		}
		if len(runs) > 0 {
			last := runs[len(runs)-1].line
			if f.has("i") {
				last = strings.ToLower(last)
			}
			if last == cmp {
				runs[len(runs)-1].n++
				continue
			}
		}
		runs = append(runs, run{l, 1})
	}
	var out []string
	for _, r := range runs {
		if f.has("d") && r.n < 2 || f.has("u") && r.n > 1 {
			continue
		}
		if f.has("c") {
			out = append(out, fmt.Sprintf("%7d %s", r.n, r.line))
		} else {
			out = append(out, r.line)
		}
	}
	u.writeLines(out)
	return code
}

func parseRanges(spec string) [][2]int {
	var out [][2]int
	for _, part := range strings.Split(spec, ",") {
		a, b, isRange := strings.Cut(part, "-")
		lo, _ := strconv.Atoi(a)
		hi := lo
		if isRange {
			if b == "" {
				hi = 1 << 30
			} else {
				hi, _ = strconv.Atoi(b)
			}
			if a == "" {
				lo = 1
			}
		}
		out = append(out, [2]int{lo, hi})
	}
	return out
}

func inRanges(i int, rs [][2]int) bool {
	for _, r := range rs {
		if i >= r[0] && i <= r[1] {
			return true
		}
	}
	return false
}

func uCut(u *uctx, args []string) int {
	f := parseFlags(args, "dfc")
	data, code := u.inputs(f.args)
	var out []string
	for _, l := range splitLines(data) {
		if c, ok := f.set["c"]; ok {
			rs := parseRanges(c)
			var b strings.Builder
			for i, r := range []rune(l) {
				if inRanges(i+1, rs) {
					b.WriteRune(r)
				}
			}
			out = append(out, b.String())
			continue
		}
		d := "\t"
		if v, ok := f.set["d"]; ok {
			d = v
		}
		fields := strings.Split(l, d)
		rs := parseRanges(f.set["f"])
		var keep []string
		for i, x := range fields {
			if inRanges(i+1, rs) {
				keep = append(keep, x)
			}
		}
		out = append(out, strings.Join(keep, d))
	}
	u.writeLines(out)
	return code
}

func expandSet(s string) []rune {
	s = strings.NewReplacer(`\n`, "\n", `\t`, "\t", `\\`, `\`,
		"[:upper:]", "ABCDEFGHIJKLMNOPQRSTUVWXYZ", "[:lower:]", "abcdefghijklmnopqrstuvwxyz",
		"[:digit:]", "0123456789", "[:space:]", " \t\n\r\f\v", "[:punct:]", "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~").Replace(s)
	rs := []rune(s)
	var out []rune
	for i := 0; i < len(rs); i++ {
		if i+2 < len(rs) && rs[i+1] == '-' {
			for c := rs[i]; c <= rs[i+2]; c++ {
				out = append(out, c)
			}
			i += 2
			continue
		}
		out = append(out, rs[i])
	}
	return out
}

func uTr(u *uctx, args []string) int {
	f := parseFlags(args, "")
	data, _ := u.inputs(nil)
	in := []rune(string(data))
	if len(f.args) == 0 {
		u.errf("tr: missing operand")
		return 1
	}
	set1 := expandSet(f.args[0])
	contains := func(set []rune, r rune) int {
		for i, x := range set {
			if x == r {
				return i
			}
		}
		return -1
	}
	var out []rune
	switch {
	case f.has("d"):
		for _, r := range in {
			if contains(set1, r) < 0 {
				out = append(out, r)
			}
		}
	case len(f.args) >= 2:
		set2 := expandSet(f.args[1])
		for _, r := range in {
			if i := contains(set1, r); i >= 0 && len(set2) > 0 {
				out = append(out, set2[min(i, len(set2)-1)])
			} else {
				out = append(out, r)
			}
		}
	default:
		out = in
	}
	if f.has("s") {
		sq := set1
		if len(f.args) >= 2 && !f.has("d") {
			sq = expandSet(f.args[1])
		}
		var squeezed []rune
		for i, r := range out {
			if i > 0 && r == out[i-1] && contains(sq, r) >= 0 {
				continue
			}
			squeezed = append(squeezed, r)
		}
		out = squeezed
	}
	io.WriteString(u.hc.Stdout, string(out))
	return 0
}

func uLs(u *uctx, args []string) int {
	f := parseFlags(args, "")
	targets := f.args
	if len(targets) == 0 {
		targets = []string{"."}
	}
	code := 0
	for _, t := range targets {
		abs, err := u.run.s.Sandbox.Resolve(u.hc.Dir, t)
		if err != nil {
			u.errf("ls: %v", err)
			code = 2
			continue
		}
		info, err := os.Stat(abs)
		if err != nil {
			u.errf("ls: %s: no such file or directory", t)
			code = 2
			continue
		}
		if !info.IsDir() {
			fmt.Fprintln(u.hc.Stdout, t)
			continue
		}
		var walk func(dir, label string)
		walk = func(dir, label string) {
			entries, _ := os.ReadDir(dir)
			if f.has("R") || len(targets) > 1 {
				fmt.Fprintf(u.hc.Stdout, "%s:\n", label)
			}
			var subdirs []string
			for _, e := range entries {
				if !f.has("a") && !f.has("A") && strings.HasPrefix(e.Name(), ".") {
					continue
				}
				name := e.Name()
				if e.IsDir() {
					subdirs = append(subdirs, name)
				}
				if f.has("l") {
					fi, _ := e.Info()
					mode, size := "-rw-r--r--", int64(0)
					if fi != nil {
						mode, size = fi.Mode().String(), fi.Size()
					}
					fmt.Fprintf(u.hc.Stdout, "%s %8d %s\n", mode, size, name)
				} else {
					if e.IsDir() && f.has("p") {
						name += "/"
					}
					fmt.Fprintln(u.hc.Stdout, name)
				}
			}
			if f.has("R") {
				for _, d := range subdirs {
					fmt.Fprintln(u.hc.Stdout)
					walk(filepath.Join(dir, d), path.Join(label, d))
				}
			}
		}
		walk(abs, t)
	}
	return code
}

// uEcho is only reached through xargs; the interpreter's builtin echo
// handles every direct use.
func uEcho(u *uctx, args []string) int {
	nl := true
	if len(args) > 0 && args[0] == "-n" {
		nl, args = false, args[1:]
	}
	io.WriteString(u.hc.Stdout, strings.Join(args, " "))
	if nl {
		io.WriteString(u.hc.Stdout, "\n")
	}
	return 0
}
