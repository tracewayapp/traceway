package tools

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func init() {
	for name, f := range map[string]func(u *uctx, args []string) int{
		"date": uDate, "seq": uSeq, "sed": uSed, "basename": uBasename, "dirname": uDirname,
		"tee": uTee, "find": uFind, "nl": uNl, "rev": uRev, "sleep": uSleep,
	} {
		coreutils[name] = f
	}
}

var agoRe = regexp.MustCompile(`^\s*(-?\d+)\s*(second|sec|minute|min|hour|day|week)s?\s*(ago)?\s*$`)

// parseDateArg handles the -d forms agents use: now, @epoch, RFC 3339,
// "N units ago", "+N units".
func parseDateArg(s string, now time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "" || s == "now":
		return now, nil
	case strings.HasPrefix(s, "@"):
		n, err := strconv.ParseInt(s[1:], 10, 64)
		return time.Unix(n, 0).UTC(), err
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	if m := agoRe.FindStringSubmatch(strings.TrimPrefix(s, "+")); m != nil {
		n, _ := strconv.Atoi(m[1])
		unit := map[string]time.Duration{"second": time.Second, "sec": time.Second, "minute": time.Minute, "min": time.Minute, "hour": time.Hour, "day": 24 * time.Hour, "week": 7 * 24 * time.Hour}[m[2]]
		d := time.Duration(n) * unit
		if m[3] == "ago" {
			d = -d
		}
		return now.Add(d), nil
	}
	return time.Time{}, fmt.Errorf("invalid date %q", s)
}

func strftime(t time.Time, f string) string {
	r := strings.NewReplacer(
		"%Y", t.Format("2006"), "%m", t.Format("01"), "%d", t.Format("02"), "%H", t.Format("15"),
		"%M", t.Format("04"), "%S", t.Format("05"), "%s", strconv.FormatInt(t.Unix(), 10), "%z", t.Format("-0700"),
		"%Z", t.Format("MST"), "%F", t.Format("2006-01-02"), "%T", t.Format("15:04:05"), "%N", fmt.Sprintf("%09d", t.Nanosecond()),
		"%3N", fmt.Sprintf("%03d", t.Nanosecond()/1e6), "%y", t.Format("06"), "%b", t.Format("Jan"), "%a", t.Format("Mon"), "%%", "%",
	)
	return r.Replace(f)
}

func uDate(u *uctx, args []string) int {
	now := time.Now().UTC()
	t := now
	format := "%a %b %d %H:%M:%S UTC %Y"
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-u" || a == "--utc":
		case a == "-d" && i+1 < len(args):
			var err error
			if t, err = parseDateArg(args[i+1], now); err != nil {
				u.errf("date: %v", err)
				return 1
			}
			i++
		case strings.HasPrefix(a, "--date="):
			var err error
			if t, err = parseDateArg(strings.TrimPrefix(a, "--date="), now); err != nil {
				u.errf("date: %v", err)
				return 1
			}
		case a == "-Iseconds" || a == "--iso-8601=seconds":
			format = "%Y-%m-%dT%H:%M:%S+00:00"
		case a == "-I" || a == "--iso-8601":
			format = "%Y-%m-%d"
		case strings.HasPrefix(a, "+"):
			format = a[1:]
		default:
			u.errf("date: unsupported argument %s", a)
			return 1
		}
	}
	fmt.Fprintln(u.hc.Stdout, strftime(t.UTC(), format))
	return 0
}

func uSeq(u *uctx, args []string) int {
	nums := make([]float64, 0, 3)
	for _, a := range args {
		n, err := strconv.ParseFloat(a, 64)
		if err != nil {
			u.errf("seq: invalid number %s", a)
			return 1
		}
		nums = append(nums, n)
	}
	first, step, last := 1.0, 1.0, 0.0
	switch len(nums) {
	case 1:
		last = nums[0]
	case 2:
		first, last = nums[0], nums[1]
	case 3:
		first, step, last = nums[0], nums[1], nums[2]
	default:
		u.errf("seq: missing operand")
		return 1
	}
	for n, i := first, 0; (step > 0 && n <= last) || (step < 0 && n >= last); n, i = n+step, i+1 {
		if i > 100000 {
			break
		}
		fmt.Fprintln(u.hc.Stdout, strconv.FormatFloat(n, 'f', -1, 64))
	}
	return 0
}

func uBasename(u *uctx, args []string) int {
	if len(args) == 0 {
		return 1
	}
	b := filepath.Base(args[0])
	if len(args) > 1 {
		b = strings.TrimSuffix(b, args[1])
	}
	fmt.Fprintln(u.hc.Stdout, b)
	return 0
}

func uDirname(u *uctx, args []string) int {
	if len(args) == 0 {
		return 1
	}
	fmt.Fprintln(u.hc.Stdout, filepath.Dir(args[0]))
	return 0
}

func uTee(u *uctx, args []string) int {
	f := parseFlags(args, "")
	data, _ := u.inputs(nil)
	u.hc.Stdout.Write(data)
	for _, p := range f.args {
		abs, err := u.run.s.Sandbox.Resolve(u.hc.Dir, p)
		if err == nil {
			err = u.run.s.Sandbox.Writable(abs)
		}
		if err != nil {
			u.errf("tee: %v", err)
			return 1
		}
		flag := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
		if f.has("a") {
			flag = os.O_CREATE | os.O_WRONLY | os.O_APPEND
		}
		fh, err := os.OpenFile(abs, flag, 0o644)
		if err != nil {
			u.errf("tee: %v", err)
			return 1
		}
		fh.Write(data)
		fh.Close()
	}
	return 0
}

func uFind(u *uctx, args []string) int {
	roots := []string{}
	i := 0
	for ; i < len(args) && !strings.HasPrefix(args[i], "-"); i++ {
		roots = append(roots, args[i])
	}
	if len(roots) == 0 {
		roots = []string{"."}
	}
	var name, pathGlob, typ string
	fold := false
	maxDepth := -1
	for ; i < len(args); i++ {
		switch args[i] {
		case "-name", "-iname":
			if i+1 < len(args) {
				name = args[i+1]
				fold = args[i] == "-iname"
				i++
			}
		case "-path":
			if i+1 < len(args) {
				pathGlob = args[i+1]
				i++
			}
		case "-type":
			if i+1 < len(args) {
				typ = args[i+1]
				i++
			}
			if typ != "f" && typ != "d" && typ != "l" {
				u.errf("find: unsupported -type %s (f, d or l)", typ)
				return 1
			}
		case "-maxdepth":
			if i+1 < len(args) {
				maxDepth, _ = strconv.Atoi(args[i+1])
				i++
			}
		case "-print":
		default:
			u.errf("find: unsupported predicate %s", args[i])
			return 1
		}
	}
	for _, root := range roots {
		abs, err := u.run.s.Sandbox.Resolve(u.hc.Dir, root)
		if err != nil {
			u.errf("find: %v", err)
			return 1
		}
		_ = filepath.WalkDir(abs, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(abs, p)
			depth := 0
			if rel != "." {
				depth = strings.Count(rel, string(filepath.Separator)) + 1
			}
			if maxDepth >= 0 && depth > maxDepth {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() && d.Name() == ".git" {
				return filepath.SkipDir
			}
			// As GNU find prints it: ./x under ".", the root itself as given.
			shown := root
			if rel != "." {
				shown = strings.TrimSuffix(root, "/") + "/" + filepath.ToSlash(rel)
			}
			switch typ {
			case "f":
				if !d.Type().IsRegular() {
					return nil
				}
			case "d":
				if !d.IsDir() {
					return nil
				}
			case "l":
				if d.Type()&fs.ModeSymlink == 0 {
					return nil
				}
			}
			if name != "" {
				pat, base := name, d.Name()
				if fold {
					pat, base = strings.ToLower(pat), strings.ToLower(base)
				}
				if ok, _ := filepath.Match(pat, base); !ok {
					return nil
				}
			}
			if pathGlob != "" {
				if ok, _ := path.Match(pathGlob, filepath.ToSlash(shown)); !ok {
					return nil
				}
			}
			fmt.Fprintln(u.hc.Stdout, filepath.ToSlash(shown))
			return nil
		})
	}
	return 0
}

func uNl(u *uctx, args []string) int {
	data, code := u.inputs(parseFlags(args, "").args)
	lines := splitLines(data)
	for i, l := range lines {
		lines[i] = fmt.Sprintf("%6d\t%s", i+1, l)
	}
	u.writeLines(lines)
	return code
}

func uRev(u *uctx, args []string) int {
	data, code := u.inputs(args)
	lines := splitLines(data)
	for i, l := range lines {
		r := []rune(l)
		for a, b := 0, len(r)-1; a < b; a, b = a+1, b-1 {
			r[a], r[b] = r[b], r[a]
		}
		lines[i] = string(r)
	}
	u.writeLines(lines)
	return code
}

func uSleep(u *uctx, args []string) int {
	if len(args) == 0 {
		return 1
	}
	d, err := strconv.ParseFloat(strings.TrimSuffix(args[0], "s"), 64)
	if err != nil {
		return 1
	}
	if d > 30 {
		d = 30
	}
	ctx := u.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return 124
	case <-time.After(time.Duration(d * float64(time.Second))):
	}
	return 0
}

// sed supports -n, -E/-r, -e, and the commands s///[gpi], p, d, q with line,
// $, /re/ and range addresses, separated by ; or newlines.
func uSed(u *uctx, args []string) int {
	quiet, ere := false, false
	var scripts, files []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-n" || a == "--quiet":
			quiet = true
		case a == "-E" || a == "-r":
			ere = true
		case a == "-e" && i+1 < len(args):
			scripts = append(scripts, args[i+1])
			i++
		case a == "-i" || strings.HasPrefix(a, "-i"):
			u.errf("sed: in-place editing is not available; use edit_file")
			return 1
		case strings.HasPrefix(a, "-") && len(a) > 1:
			for _, c := range a[1:] {
				switch c {
				case 'n':
					quiet = true
				case 'E', 'r':
					ere = true
				default:
					u.errf("sed: unsupported option -%c", c)
					return 1
				}
			}
		default:
			if len(scripts) == 0 {
				scripts = append(scripts, a)
			} else {
				files = append(files, a)
			}
		}
	}
	if len(scripts) == 0 {
		u.errf("sed: no script")
		return 1
	}
	cmds, err := parseSed(strings.Join(scripts, "\n"), ere)
	if err != nil {
		u.errf("sed: %v", err)
		return 1
	}
	data, code := u.inputs(files)
	lines := splitLines(data)
	var out []string
	inRange := make([]bool, len(cmds))
	for idx, line := range lines {
		n := idx + 1
		last := n == len(lines)
		deleted, quit := false, false
		for ci, c := range cmds {
			if !c.matches(n, last, line, &inRange[ci]) {
				continue
			}
			switch c.op {
			case 'p':
				out = append(out, line)
			case 'd':
				deleted = true
			case 'q':
				quit = true
			case 's':
				// The Nth match (default the first), and with g every match
				// from it on.
				locs := c.re.FindAllStringSubmatchIndex(line, -1)
				from := max(c.nth, 1) - 1
				if from >= len(locs) {
					continue
				}
				to := from + 1
				if c.global {
					to = len(locs)
				}
				var b []byte
				prev := 0
				for _, loc := range locs[from:to] {
					b = append(b, line[prev:loc[0]]...)
					b = c.re.ExpandString(b, c.repl, line, loc)
					prev = loc[1]
				}
				line = string(append(b, line[prev:]...))
				if c.print {
					out = append(out, line)
				}
			}
			if deleted || quit {
				break
			}
		}
		if !deleted && !quiet {
			out = append(out, line)
		}
		if quit {
			break
		}
	}
	u.writeLines(out)
	return code
}

type sedAddr struct {
	line  int // 0 = none
	last  bool
	re    *regexp.Regexp
	isSet bool
}

type sedCmd struct {
	a1, a2 sedAddr
	op     byte
	re     *regexp.Regexp
	repl   string
	global bool
	print  bool
	// nth is the s command's numeric flag (0 = none).
	nth int
}

func (a sedAddr) hit(n int, last bool, line string) bool {
	switch {
	case a.last:
		return last
	case a.re != nil:
		return a.re.MatchString(line)
	default:
		return a.line == n
	}
}

func (c sedCmd) matches(n int, last bool, line string, active *bool) bool {
	if !c.a1.isSet {
		return true
	}
	if !c.a2.isSet {
		return c.a1.hit(n, last, line)
	}
	if *active {
		if c.a2.hit(n, last, line) || (c.a2.line > 0 && n >= c.a2.line) {
			*active = false
		}
		return true
	}
	if c.a1.hit(n, last, line) {
		*active = !(c.a2.line > 0 && n >= c.a2.line)
		return true
	}
	return false
}

func parseSed(script string, ere bool) ([]sedCmd, error) {
	var cmds []sedCmd
	compile := func(p string) (*regexp.Regexp, error) {
		if !ere {
			p = breToERE(p)
		}
		return regexp.Compile(p)
	}
	s := script
	parseAddr := func() (sedAddr, error) {
		s = strings.TrimLeftFunc(s, unicode.IsSpace)
		switch {
		case strings.HasPrefix(s, "$"):
			s = s[1:]
			return sedAddr{last: true, isSet: true}, nil
		case strings.HasPrefix(s, "/"):
			end := strings.Index(s[1:], "/")
			if end < 0 {
				return sedAddr{}, fmt.Errorf("unterminated address regex")
			}
			re, err := compile(s[1 : end+1])
			s = s[end+2:]
			return sedAddr{re: re, isSet: true}, err
		default:
			j := 0
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			if j == 0 {
				return sedAddr{}, nil
			}
			n, _ := strconv.Atoi(s[:j])
			s = s[j:]
			return sedAddr{line: n, isSet: true}, nil
		}
	}
	for {
		s = strings.TrimLeft(s, " \t\n;")
		if s == "" {
			return cmds, nil
		}
		var c sedCmd
		var err error
		if c.a1, err = parseAddr(); err != nil {
			return nil, err
		}
		if strings.HasPrefix(s, ",") {
			s = s[1:]
			if c.a2, err = parseAddr(); err != nil {
				return nil, err
			}
		}
		s = strings.TrimLeft(s, " \t")
		if s == "" {
			return nil, fmt.Errorf("missing command")
		}
		c.op = s[0]
		s = s[1:]
		switch c.op {
		case 'p', 'd', 'q':
		case 's':
			if s == "" {
				return nil, fmt.Errorf("unterminated s command")
			}
			delim := s[0]
			parts := splitUnescaped(s[1:], delim, 3)
			if len(parts) < 2 {
				return nil, fmt.Errorf("unterminated s command")
			}
			if c.re, err = compile(parts[0]); err != nil {
				return nil, err
			}
			c.repl = sedReplacement(parts[1])
			flags := ""
			if len(parts) == 3 {
				flags = parts[2]
			}
			k := 0
			for k < len(flags) && strings.ContainsRune("gpiI0123456789", rune(flags[k])) {
				switch f := flags[k]; {
				case f == 'g':
					c.global = true
				case f == 'p':
					c.print = true
				case f == 'i' || f == 'I':
					c.re = regexp.MustCompile("(?i)" + c.re.String())
				default:
					c.nth = c.nth*10 + int(f-'0')
				}
				k++
			}
			s = flags[k:]
		default:
			return nil, fmt.Errorf("unsupported command %q", c.op)
		}
		cmds = append(cmds, c)
	}
}

// splitUnescaped splits s on delim (not preceded by a backslash) into at most
// n parts, unescaping the delimiter in all but the last; the last part keeps
// the remainder verbatim (it holds the rest of the script).
func splitUnescaped(s string, delim byte, n int) []string {
	var parts []string
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if len(parts) == n-1 {
			b.WriteString(s[i:])
			break
		}
		if s[i] == '\\' && i+1 < len(s) && s[i+1] == delim {
			b.WriteByte(delim)
			i++
			continue
		}
		if s[i] == delim {
			parts = append(parts, b.String())
			b.Reset()
			continue
		}
		b.WriteByte(s[i])
	}
	return append(parts, b.String())
}

// sedReplacement converts \1 and & to Go's ${1} and ${0}.
func sedReplacement(r string) string {
	var b strings.Builder
	for i := 0; i < len(r); i++ {
		switch {
		case r[i] == '\\' && i+1 < len(r) && r[i+1] >= '0' && r[i+1] <= '9':
			b.WriteString("${" + string(r[i+1]) + "}")
			i++
		case r[i] == '\\' && i+1 < len(r) && r[i+1] == 'n':
			b.WriteByte('\n')
			i++
		case r[i] == '\\' && i+1 < len(r):
			b.WriteByte(r[i+1])
			i++
		case r[i] == '&':
			b.WriteString("${0}")
		case r[i] == '$':
			b.WriteString("$$")
		default:
			b.WriteByte(r[i])
		}
	}
	return b.String()
}
