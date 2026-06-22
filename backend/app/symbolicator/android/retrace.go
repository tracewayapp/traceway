package android

import (
	"regexp"
	"strconv"
	"strings"
)

const maxFrames = 50

type methodMapping struct {
	hasObf      bool
	obfStart    int
	obfEnd      int
	origClass   string
	method      string
	hasOrig     bool
	origStart   int
	origEnd     int
	synthesized bool
}

type Mapping struct {
	classOrig  map[string]string
	classFile  map[string]string
	methods    map[string]map[string][]methodMapping
	approxSize int64
}

func (m *Mapping) ApproxSize() int64 { return m.approxSize }

var (
	classLineRe  = regexp.MustCompile(`^(\S+) -> (\S+):$`)
	sourceFileRe = regexp.MustCompile(`"id":"sourceFile","fileName":"([^"]+)"`)
	methodLineRe = regexp.MustCompile(`^\s+(?:(\d+):(\d+):)?[^ ]+ ([^ (]+)\([^)]*\)(?::(\d+)(?::(\d+))?)? -> (\S+)$`)
	atFrameRe    = regexp.MustCompile(`^(\s*)at\s+(.+)\((.*)\)\s*$`)
	classTokenRe = regexp.MustCompile(`[A-Za-z_$][\w$]*(?:\.[A-Za-z_$][\w$]*)*`)
)

func LooksLikeR8Mapping(data []byte) bool {
	text := string(data)
	if len(text) > 1<<16 {
		text = text[:1<<16]
	}
	n := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "# compiler: R8") {
			return true
		}
		if classLineRe.MatchString(line) {
			return true
		}
		if n++; n > 400 {
			break
		}
	}
	return false
}

func (m *Mapping) IsEmpty() bool { return len(m.classOrig) == 0 }

func ParseMapping(text string) *Mapping {
	m := &Mapping{
		classOrig: map[string]string{},
		classFile: map[string]string{},
		methods:   map[string]map[string][]methodMapping{},
	}
	var curObf, curOrig, lastMethod string
	for _, line := range strings.Split(text, "\n") {
		if line == "" {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			if curOrig != "" {
				if sm := sourceFileRe.FindStringSubmatch(line); sm != nil {
					m.classFile[curOrig] = sm[1]
				}
			}
			if strings.Contains(line, "com.android.tools.r8.synthesized") && lastMethod != "" {
				if s := m.methods[curObf][lastMethod]; len(s) > 0 {
					s[len(s)-1].synthesized = true
				}
			}
			continue
		}
		if cm := classLineRe.FindStringSubmatch(line); cm != nil {
			curOrig, curObf, lastMethod = cm[1], cm[2], ""
			m.classOrig[curObf] = curOrig
			if m.methods[curObf] == nil {
				m.methods[curObf] = map[string][]methodMapping{}
			}
			continue
		}
		mm := methodLineRe.FindStringSubmatch(line)
		if mm == nil || curObf == "" {
			lastMethod = ""
			continue
		}
		entry := methodMapping{}
		if mm[1] != "" {
			entry.hasObf = true
			entry.obfStart, _ = strconv.Atoi(mm[1])
			entry.obfEnd, _ = strconv.Atoi(mm[2])
		}
		qn := mm[3]
		if i := strings.LastIndex(qn, "."); i >= 0 {
			entry.origClass = qn[:i]
			entry.method = qn[i+1:]
		} else {
			entry.origClass = curOrig
			entry.method = qn
		}
		if mm[4] != "" {
			entry.hasOrig = true
			entry.origStart, _ = strconv.Atoi(mm[4])
			if mm[5] != "" {
				entry.origEnd, _ = strconv.Atoi(mm[5])
			} else {
				entry.origEnd = entry.origStart
			}
		}
		m.methods[curObf][mm[6]] = append(m.methods[curObf][mm[6]], entry)
		lastMethod = mm[6]
	}

	size := int64(len(m.classOrig)) * 96
	for _, byMethod := range m.methods {
		for _, entries := range byMethod {
			size += int64(len(entries))*96 + 64
		}
	}
	m.approxSize = size + 1

	return m
}

type memberFrame struct {
	cls    string
	method string
	file   string
	line   int
}

type r8Source interface {
	classInfo(obfClass string) (orig, file string, ok bool)
	matched(obfClass, obfMethod string, obfLine int) []memberFrame
}

func (m *Mapping) Retrace(text string) string { return retraceWith(m, text) }

func retraceWith(src r8Source, text string) string {
	var b strings.Builder
	emitted := 0
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	for _, line := range lines {
		if fm := atFrameRe.FindStringSubmatch(line); fm != nil && emitted < maxFrames {
			if out, n, ok := retraceFrameWith(src, fm[1], fm[2], fm[3]); ok {
				b.WriteString(out)
				emitted += n
				continue
			}
		}
		b.WriteString(replaceClassesWith(src, line))
		b.WriteByte('\n')
	}
	return b.String()
}

func retraceFrameWith(src r8Source, indent, ref, loc string) (string, int, bool) {
	dot := strings.LastIndex(ref, ".")
	if dot < 0 {
		return "", 0, false
	}
	obfClass, obfMethod := ref[:dot], ref[dot+1:]
	origClass, file, ok := src.classInfo(obfClass)
	if !ok {
		return "", 0, false
	}
	lineStr := ""
	if c := strings.LastIndex(loc, ":"); c >= 0 {
		lineStr = loc[c+1:]
	}
	obfLine, _ := strconv.Atoi(lineStr)

	members := src.matched(obfClass, obfMethod, obfLine)
	var b strings.Builder
	if len(members) == 0 {
		b.WriteString(indent + "at " + origClass + "." + obfMethod + "(" + file + lineSuffix(obfLine) + ")\n")
		return b.String(), 1, true
	}
	for _, mf := range members {
		b.WriteString(indent + "at " + mf.cls + "." + mf.method + "(" + mf.file + ":" + strconv.Itoa(mf.line) + ")\n")
	}
	return b.String(), len(members), true
}

func replaceClassesWith(src r8Source, line string) string {
	return classTokenRe.ReplaceAllStringFunc(line, func(tok string) string {
		if orig, _, ok := src.classInfo(tok); ok {
			return orig
		}
		return tok
	})
}

func (m *Mapping) classInfo(obfClass string) (string, string, bool) {
	orig, ok := m.classOrig[obfClass]
	if !ok {
		return "", "", false
	}
	return orig, m.fileFor(orig, ""), true
}

func (m *Mapping) matched(obfClass, obfMethod string, obfLine int) []memberFrame {
	var out []memberFrame
	for _, e := range m.methods[obfClass][obfMethod] {
		if e.synthesized {
			continue
		}
		if !e.hasObf || (obfLine >= e.obfStart && obfLine <= e.obfEnd) {
			out = append(out, memberFrame{
				cls:    e.origClass,
				method: e.method,
				file:   m.fileFor(e.origClass, ""),
				line:   computeLine(e, obfLine),
			})
		}
	}
	return out
}

func computeLine(e methodMapping, obfLine int) int {
	if !e.hasOrig {
		return obfLine
	}
	if e.hasObf && e.obfEnd > e.obfStart && e.origEnd > e.origStart {
		return e.origStart + (obfLine - e.obfStart)
	}
	return e.origStart
}

func (m *Mapping) fileFor(origClass, fallback string) string {
	if f, ok := m.classFile[origClass]; ok {
		return f
	}
	simple := origClass
	if i := strings.LastIndex(simple, "."); i >= 0 {
		simple = simple[i+1:]
	}
	if i := strings.Index(simple, "$"); i >= 0 {
		simple = simple[:i]
	}
	if simple != "" {
		return simple + ".java"
	}
	return fallback
}

func lineSuffix(line int) string {
	if line > 0 {
		return ":" + strconv.Itoa(line)
	}
	return ""
}
