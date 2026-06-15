package jsstack

import (
	"regexp"
	"strings"
)

type jsStackFrame struct {
	fn  string
	loc string
}

var jsLocRe = regexp.MustCompile(`^.+:\d+:\d+$`)
var jsEvalLocRe = regexp.MustCompile(`\(([^()]+:\d+:\d+)\)`)
var geckoEvalLocRe = regexp.MustCompile(`^(.*?) line (\d+) > (?:eval|Function)(?: line \d+ > (?:eval|Function))*:\d+:\d+$`)

func Canonicalize(trace string) (string, bool) {
	lines := strings.Split(trace, "\n")

	parse := detectJsFrameParser(lines)
	if parse == nil {
		return trace, false
	}

	out := make([]string, 0, len(lines)*2)
	converted := false
	for _, line := range lines {
		f, ok := parse(line)
		if !ok {
			out = append(out, line)
			continue
		}
		// Gecko chains engine mechanisms onto frame names with stars:
		// "async*loadRate" is the real loadRate reached via async machinery
		// (Chrome prints "at async loadRate"), while a bare "handleEvent*" or
		// "async*" is purely synthetic. Other engines never emit either form,
		// so the marker is stripped and synthetic-only frames are dropped to
		// keep grouping engine-independent.
		if i := strings.LastIndex(f.fn, "*"); i != -1 {
			f.fn = f.fn[i+1:]
			if f.fn == "" {
				converted = true
				continue
			}
		}
		if f.loc == "" {
			if f.fn != "" {
				out = append(out, "    "+f.fn+" [native code]")
			}
			continue
		}
		if f.fn != "" {
			out = append(out, f.fn+"()")
		}
		out = append(out, "    "+f.loc)
		converted = true
	}
	if !converted {
		return trace, false
	}
	return strings.Join(out, "\n"), true
}

func detectJsFrameParser(lines []string) func(string) (jsStackFrame, bool) {
	v8, gecko := 0, 0
	for _, line := range lines {
		if f, ok := parseV8Frame(line); ok && f.loc != "" {
			v8++
		} else if f, ok := parseGeckoFrame(line); ok && f.loc != "" {
			gecko++
		}
	}
	if v8 > 0 && v8 >= gecko {
		return parseV8Frame
	}
	if gecko > 0 {
		return parseGeckoFrame
	}
	return nil
}

func parseV8Frame(line string) (jsStackFrame, bool) {
	s := strings.TrimSpace(line)
	if !strings.HasPrefix(s, "at ") {
		return jsStackFrame{}, false
	}
	s = strings.TrimSpace(s[3:])
	s = strings.TrimPrefix(s, "async ")
	s = strings.TrimPrefix(s, "new ")

	name, paren := s, ""
	hasParen := false
	if strings.HasSuffix(s, ")") {
		if i := strings.Index(s, " ("); i != -1 {
			name, paren, hasParen = s[:i], s[i+2:len(s)-1], true
		}
	}
	if !hasParen {
		if jsLocRe.MatchString(name) {
			return jsStackFrame{loc: name}, true
		}
		return jsStackFrame{}, false
	}

	if i := strings.Index(name, " [as "); i != -1 {
		name = name[:i]
	}
	loc := ""
	if strings.HasPrefix(paren, "eval at ") {
		if m := jsEvalLocRe.FindStringSubmatch(paren); m != nil {
			loc = m[1]
		}
	} else if jsLocRe.MatchString(paren) {
		loc = paren
	}
	return jsStackFrame{fn: name, loc: loc}, true
}

func parseGeckoFrame(line string) (jsStackFrame, bool) {
	s := strings.TrimSpace(line)
	i := strings.LastIndex(s, "@")
	if i == -1 {
		return jsStackFrame{}, false
	}
	name, loc := s[:i], s[i+1:]
	if loc == "[native code]" {
		return jsStackFrame{fn: name}, true
	}
	if !jsLocRe.MatchString(loc) {
		return jsStackFrame{}, false
	}
	if m := geckoEvalLocRe.FindStringSubmatch(loc); m != nil {
		loc = m[1] + ":" + m[2] + ":1"
	}
	return jsStackFrame{fn: name, loc: loc}, true
}
