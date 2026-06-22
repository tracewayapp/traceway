package android

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var (
	oClassLineRe  = regexp.MustCompile(`^(\S+) -> (\S+):$`)
	oSourceFileRe = regexp.MustCompile(`"id":"sourceFile","fileName":"([^"]+)"`)
	oMethodLineRe = regexp.MustCompile(`^\s+(?:(\d+):(\d+):)?[^ ]+ ([^ (]+)\([^)]*\)(?::(\d+)(?::(\d+))?)? -> (\S+)$`)
	oAtFrameRe    = regexp.MustCompile(`^(\s*)at\s+(.+)\((.*)\)\s*$`)
	oClassTokenRe = regexp.MustCompile(`[A-Za-z_$][\w$]*(?:\.[A-Za-z_$][\w$]*)*`)
)

func parseMappingRegex(text string) *Mapping {
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
				if sm := oSourceFileRe.FindStringSubmatch(line); sm != nil {
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
		if cm := oClassLineRe.FindStringSubmatch(line); cm != nil {
			curOrig, curObf, lastMethod = cm[1], cm[2], ""
			m.classOrig[curObf] = curOrig
			if m.methods[curObf] == nil {
				m.methods[curObf] = map[string][]methodMapping{}
			}
			continue
		}
		mm := oMethodLineRe.FindStringSubmatch(line)
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

func retraceRegex(src r8Source, text string) string {
	var b strings.Builder
	emitted := 0
	for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		if fm := oAtFrameRe.FindStringSubmatch(line); fm != nil && emitted < maxFrames {
			if out, n, ok := retraceFrameWith(src, fm[1], fm[2], fm[3]); ok {
				b.WriteString(out)
				emitted += n
				continue
			}
		}
		b.WriteString(replaceClassesRegex(src, line))
		b.WriteByte('\n')
	}
	return b.String()
}

func replaceClassesRegex(src r8Source, line string) string {
	return oClassTokenRe.ReplaceAllStringFunc(line, func(tok string) string {
		if orig, _, ok := src.classInfo(tok); ok {
			return orig
		}
		return tok
	})
}

const edgeCaseMapping = `# compiler: R8
# pg_map_id: deadbeef
com.example.Foo -> a.a:
# {"id":"sourceFile","fileName":"Foo.kt"}
    int $r8$clinit -> a
    # {"id":"com.android.tools.r8.synthesized"}
    1:1:void <init>():8:8 -> <init>
    1:1:void crashKotlin():33:33 -> a
    2:2:double com.example.Bar.checkout(double[]):15:15 -> a
    3:3:double com.example.Bar.applyTax(double):10 -> a
    void render() -> b
    com.example.Foo INSTANCE -> c
com.example.Removed -> R8$$REMOVED$$CLASS$$0:
# {"id":"sourceFile","fileName":"Removed.kt"}
    1:1:void a.MainActivity$$ExternalSyntheticLambda0.onClick(android.view.View):0 -> onClick
    # {"id":"com.android.tools.r8.synthesized"}
    1:1:void onCreate$lambda$4$lambda$3():22:22 -> onClick
    1:1:void onClick(android.view.View):0 -> onClick
    # {"id":"com.android.tools.r8.synthesized"}

orphan.method():1 -> x
   not a real line here
`

func bigMapping(tb testing.TB) string {
	tb.Helper()
	base, err := os.ReadFile(filepath.Join("fixtures", "r8-app", "mapping.txt"))
	if err != nil {
		tb.Fatal(err)
	}
	var b strings.Builder
	b.Write(base)
	if len(base) > 0 && base[len(base)-1] != '\n' {
		b.WriteByte('\n')
	}
	for i := 0; b.Len() < 1<<20; i++ {
		fmt.Fprintf(&b, "benchpad.S%dClass%d -> twbp_%d:\n", i, i, i)
		b.WriteString("    1:1:void run():1:1 -> a\n")
		b.WriteString("    2:2:int compute(int):2:2 -> b\n")
	}
	return b.String()
}

func TestParseMappingMatchesRegex(t *testing.T) {
	inputs := map[string]string{
		"r8":     mustRead(t, filepath.Join("fixtures", "r8", "mapping.txt")),
		"r8-app": mustRead(t, filepath.Join("fixtures", "r8-app", "mapping.txt")),
		"big":    bigMapping(t),
		"edges":  edgeCaseMapping,
	}
	for name, in := range inputs {
		t.Run(name, func(t *testing.T) {
			if !reflect.DeepEqual(ParseMapping(in), parseMappingRegex(in)) {
				t.Errorf("manual ParseMapping differs from the regex oracle for %s", name)
			}
		})
	}
}

func TestRetraceMatchesRegex(t *testing.T) {
	for _, dir := range []string{"r8", "r8-app"} {
		mapping := mustRead(t, filepath.Join("fixtures", dir, "mapping.txt"))
		obf := mustRead(t, filepath.Join("fixtures", dir, "obfuscated.txt"))
		m := parseMappingRegex(mapping)
		if got, want := m.Retrace(obf), retraceRegex(m, obf); got != want {
			t.Errorf("%s: manual retrace differs from regex retrace\n got: %q\nwant: %q", dir, got, want)
		}
	}
}

func FuzzParseMapping(f *testing.F) {
	for _, dir := range []string{"r8", "r8-app"} {
		if b, err := os.ReadFile(filepath.Join("fixtures", dir, "mapping.txt")); err == nil {
			f.Add(string(b))
		}
	}
	f.Add(edgeCaseMapping)
	f.Fuzz(func(t *testing.T, data string) {
		if !reflect.DeepEqual(ParseMapping(data), parseMappingRegex(data)) {
			t.Fatalf("manual and regex parsers diverge for input %q", data)
		}
	})
}

func BenchmarkParseMapping(b *testing.B) {
	in := bigMapping(b)
	b.Run("regex", func(b *testing.B) {
		b.SetBytes(int64(len(in)))
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = parseMappingRegex(in)
		}
	})
	b.Run("manual", func(b *testing.B) {
		b.SetBytes(int64(len(in)))
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = ParseMapping(in)
		}
	})
}
