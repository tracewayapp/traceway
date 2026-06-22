package android

import (
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

func isWS(b byte) bool { return b == ' ' || b == '\t' || b == '\n' || b == '\f' || b == '\r' }

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func isIdentStart(b byte) bool {
	return b == '_' || b == '$' || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

func isIdentPart(b byte) bool { return isIdentStart(b) || isDigit(b) }

func hasWS(s string) bool {
	for i := 0; i < len(s); i++ {
		if isWS(s[i]) {
			return true
		}
	}
	return false
}

func scanUint(s string, i int) (val, next int, ok bool) {
	j := i
	for j < len(s) && isDigit(s[j]) {
		val = val*10 + int(s[j]-'0')
		j++
	}
	return val, j, j > i
}

const sourceFileMarker = `"id":"sourceFile","fileName":"`

func sourceFileFrom(line string) (string, bool) {
	s := line
	for {
		idx := strings.Index(s, sourceFileMarker)
		if idx < 0 {
			return "", false
		}
		start := idx + len(sourceFileMarker)
		if end := strings.IndexByte(s[start:], '"'); end > 0 {
			return s[start : start+end], true
		}
		s = s[idx+1:]
	}
}

func parseClassLine(line string) (orig, obf string, ok bool) {
	idx := strings.Index(line, " -> ")
	if idx <= 0 {
		return "", "", false
	}
	orig = line[:idx]
	rest := line[idx+4:]
	if rest == "" || rest[len(rest)-1] != ':' {
		return "", "", false
	}
	obf = rest[:len(rest)-1]
	if obf == "" || hasWS(orig) || hasWS(obf) {
		return "", "", false
	}
	return orig, obf, true
}

func parseMethodLine(line, curOrig string) (methodMapping, string, bool) {
	i := 0
	for i < len(line) && isWS(line[i]) {
		i++
	}
	if i == 0 || i >= len(line) {
		return methodMapping{}, "", false
	}

	// The obfStart:obfEnd: prefix is an optional regex group; the engine tries it
	// present first, then backtracks to absent if the rest fails to parse.
	if v1, j, ok1 := scanUint(line, i); ok1 && j < len(line) && line[j] == ':' {
		if v2, k, ok2 := scanUint(line, j+1); ok2 && k < len(line) && line[k] == ':' {
			e := methodMapping{hasObf: true, obfStart: v1, obfEnd: v2}
			if om, ok := parseMethodRest(line, k+1, curOrig, &e); ok {
				return e, om, true
			}
		}
	}
	var e methodMapping
	if om, ok := parseMethodRest(line, i, curOrig, &e); ok {
		return e, om, true
	}
	return methodMapping{}, "", false
}

func parseMethodRest(line string, i int, curOrig string, entry *methodMapping) (string, bool) {
	sp := strings.IndexByte(line[i:], ' ')
	if sp <= 0 {
		return "", false
	}
	i += sp + 1

	ms := i
	for i < len(line) && line[i] != ' ' && line[i] != '(' {
		i++
	}
	if i == ms || i >= len(line) || line[i] != '(' {
		return "", false
	}
	method := line[ms:i]

	cp := strings.IndexByte(line[i+1:], ')')
	if cp < 0 {
		return "", false
	}
	i = i + 1 + cp + 1

	if i < len(line) && line[i] == ':' {
		v1, j, ok1 := scanUint(line, i+1)
		if !ok1 {
			return "", false
		}
		entry.hasOrig = true
		entry.origStart = v1
		entry.origEnd = v1
		i = j
		if i < len(line) && line[i] == ':' {
			v2, k, ok2 := scanUint(line, i+1)
			if !ok2 {
				return "", false
			}
			entry.origEnd = v2
			i = k
		}
	}

	if i+4 > len(line) || line[i:i+4] != " -> " {
		return "", false
	}
	i += 4
	obfMethod := line[i:]
	if obfMethod == "" || hasWS(obfMethod) {
		return "", false
	}

	if dot := strings.LastIndex(method, "."); dot >= 0 {
		entry.origClass = method[:dot]
		entry.method = method[dot+1:]
	} else {
		entry.origClass = curOrig
		entry.method = method
	}
	return obfMethod, true
}

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
		if _, _, ok := parseClassLine(line); ok {
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
	rest := text
	for len(rest) > 0 {
		var line string
		if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
			line, rest = rest[:nl], rest[nl+1:]
		} else {
			line, rest = rest, ""
		}
		if line == "" {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			if curOrig != "" {
				if fn, ok := sourceFileFrom(line); ok {
					m.classFile[curOrig] = fn
				}
			}
			if lastMethod != "" && strings.Contains(line, "com.android.tools.r8.synthesized") {
				if s := m.methods[curObf][lastMethod]; len(s) > 0 {
					s[len(s)-1].synthesized = true
				}
			}
			continue
		}
		if !isWS(line[0]) {
			if orig, obf, ok := parseClassLine(line); ok {
				curOrig, curObf, lastMethod = orig, obf, ""
				m.classOrig[curObf] = curOrig
				if m.methods[curObf] == nil {
					m.methods[curObf] = map[string][]methodMapping{}
				}
			} else {
				lastMethod = ""
			}
			continue
		}
		if curObf == "" {
			lastMethod = ""
			continue
		}
		entry, obfMethod, ok := parseMethodLine(line, curOrig)
		if !ok {
			lastMethod = ""
			continue
		}
		m.methods[curObf][obfMethod] = append(m.methods[curObf][obfMethod], entry)
		lastMethod = obfMethod
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
	for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		if indent, ref, loc, ok := scanFrame(line); ok && emitted < maxFrames {
			if out, n, ok := retraceFrameWith(src, indent, ref, loc); ok {
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

func scanFrame(line string) (indent, ref, loc string, ok bool) {
	i := 0
	for i < len(line) && isWS(line[i]) {
		i++
	}
	indent = line[:i]
	if i+2 > len(line) || line[i] != 'a' || line[i+1] != 't' {
		return "", "", "", false
	}
	j := i + 2
	k := j
	for k < len(line) && isWS(line[k]) {
		k++
	}
	if k == j {
		return "", "", "", false
	}
	end := len(line)
	for end > k && isWS(line[end-1]) {
		end--
	}
	if end <= k || line[end-1] != ')' {
		return "", "", "", false
	}
	content := line[k : end-1]
	lp := strings.LastIndexByte(content, '(')
	if lp <= 0 {
		return "", "", "", false
	}
	return indent, content[:lp], content[lp+1:], true
}

func isAtFrame(line string) bool {
	_, _, _, ok := scanFrame(line)
	return ok
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
	var b strings.Builder
	i := 0
	for i < len(line) {
		if !isIdentStart(line[i]) {
			b.WriteByte(line[i])
			i++
			continue
		}
		start := i
		i++
		for i < len(line) {
			if isIdentPart(line[i]) {
				i++
			} else if line[i] == '.' && i+1 < len(line) && isIdentStart(line[i+1]) {
				i += 2
			} else {
				break
			}
		}
		tok := line[start:i]
		if orig, _, ok := src.classInfo(tok); ok {
			b.WriteString(orig)
		} else {
			b.WriteString(tok)
		}
	}
	return b.String()
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
