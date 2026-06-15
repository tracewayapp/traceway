package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type corpus struct {
	Language string   `json:"language"`
	Urls     []string `json:"urls"`
}

type dartBuild struct {
	BuildID string `json:"buildId"`
	Trace   string `json:"trace"`
}

type dartCorpus struct {
	Language string      `json:"language"`
	Builds   []dartBuild `json:"builds"`
}

const vlqChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

var vlqIndex = func() [128]int8 {
	var t [128]int8
	for i := range t {
		t[i] = -1
	}
	for i, c := range vlqChars {
		t[c] = int8(i)
	}
	return t
}()

func vlqEncode(b *strings.Builder, v int) {
	u := v << 1
	if v < 0 {
		u = (-v << 1) | 1
	}
	for {
		digit := u & 31
		u >>= 5
		if u > 0 {
			digit |= 32
		}
		b.WriteByte(vlqChars[digit])
		if u == 0 {
			break
		}
	}
}

func mappingsEndState(mappings string) (srcIdx, srcLine, srcCol, nameIdx int) {
	vals := make([]int, 0, 5)
	cur, shift := 0, 0
	flush := func() {
		if len(vals) >= 4 {
			srcIdx += vals[1]
			srcLine += vals[2]
			srcCol += vals[3]
		}
		if len(vals) >= 5 {
			nameIdx += vals[4]
		}
		vals = vals[:0]
	}
	for _, c := range mappings {
		if c == ';' || c == ',' {
			flush()
			continue
		}
		d := vlqIndex[c]
		cur |= int(d&31) << shift
		if d&32 != 0 {
			shift += 5
			continue
		}
		v := cur >> 1
		if cur&1 != 0 {
			v = -v
		}
		vals = append(vals, v)
		cur, shift = 0, 0
	}
	flush()
	return
}

func padMap(mapBytes []byte, kb, mappingsKB, seed int) []byte {
	var m map[string]any
	if err := json.Unmarshal(mapBytes, &m); err != nil {
		panic(err)
	}
	sources, _ := m["sources"].([]any)
	content, _ := m["sourcesContent"].([]any)
	for len(content) < len(sources) {
		content = append(content, nil)
	}
	padIdx := len(sources)
	var pad strings.Builder
	line := fmt.Sprintf("const benchPadValue%d = %d;\n", seed, seed)
	for pad.Len() < kb*1024 {
		pad.WriteString(line)
	}
	m["sources"] = append(sources, "__benchpad.js")
	m["sourcesContent"] = append(content, pad.String())

	if mappingsKB > 0 {
		mappings, _ := m["mappings"].(string)
		names, _ := m["names"].([]any)
		nameBase := len(names)
		for i := 0; i < 64; i++ {
			names = append(names, fmt.Sprintf("__benchFn%d_%d", seed, i))
		}
		m["names"] = names

		srcIdx, srcLine, srcCol, nameIdx := mappingsEndState(mappings)
		var b strings.Builder
		b.WriteString(mappings)
		dSrc := padIdx - srcIdx
		dLine := -srcLine
		dCol := -srcCol
		dName := nameBase - nameIdx
		genLine := 0
		for b.Len() < len(mappings)+mappingsKB*1024 {
			b.WriteByte(';')
			genLine++
			col := 0
			for s := 0; s < 8; s++ {
				if s > 0 {
					b.WriteByte(',')
				}
				vlqEncode(&b, col)
				vlqEncode(&b, dSrc)
				vlqEncode(&b, dLine)
				vlqEncode(&b, dCol)
				vlqEncode(&b, dName)
				dSrc, dCol = 0, 1
				dLine = 0
				dName = 1
				if s == 7 {
					dLine, dCol = 1, -7
					dName = -7
				}
				col = 10
			}
		}
		m["mappings"] = b.String()
	}

	out, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return out
}

func main() {
	language := flag.String("language", "js", "corpus language: js or dart")
	bundle := flag.String("bundle", "../../testing/symbolication/node-app/dist/app.mjs", "")
	mapFile := flag.String("map", "../../testing/symbolication/node-app/dist/app.mjs.map", "")
	symbols := flag.String("symbols", "seeds/dart/app.debug.elf", "dart: seed .symbols/.elf")
	traceFile := flag.String("trace", "seeds/dart/trace.txt", "dart: seed non-symbolic trace")
	entries := flag.Int("entries", 1, "")
	padKB := flag.Int("pad-kb", 256, "")
	mapPadKB := flag.Int("map-pad-kb", 0, "")
	mappingsPadKB := flag.Int("mappings-pad-kb", 0, "")
	out := flag.String("out", "./corpus", "")
	flag.Parse()

	if err := os.MkdirAll(*out, 0o755); err != nil {
		panic(err)
	}
	if *language == "dart" {
		generateDart(*out, *symbols, *traceFile, *entries)
		return
	}

	bundleBytes, err := os.ReadFile(*bundle)
	if err != nil {
		panic(err)
	}
	mapBytes, err := os.ReadFile(*mapFile)
	if err != nil {
		panic(err)
	}
	firstLine := strings.SplitN(string(bundleBytes), "\n", 2)[0]

	c := corpus{Language: "js"}
	for n := 0; n < *entries; n++ {
		var pad strings.Builder
		chunk := "function __benchPad%d_%d(a,b){var c=a*b+%d;for(var i=0;i<3;i++){c+=i*a-b}return c}\n"
		i := 0
		for pad.Len() < *padKB*1024 {
			pad.WriteString(fmt.Sprintf(chunk, n, i, i+n))
			i++
		}
		name := fmt.Sprintf("app%d.mjs", n)
		content := firstLine + "\n" + pad.String() + "//# sourceMappingURL=" + name + ".map\n"
		if err := os.WriteFile(filepath.Join(*out, name), []byte(content), 0o644); err != nil {
			panic(err)
		}
		entryMap := mapBytes
		if *mapPadKB > 0 || *mappingsPadKB > 0 {
			entryMap = padMap(mapBytes, *mapPadKB, *mappingsPadKB, n)
		}
		if err := os.WriteFile(filepath.Join(*out, name+".map"), entryMap, 0o644); err != nil {
			panic(err)
		}
		c.Urls = append(c.Urls, name)
	}
	data, _ := json.MarshalIndent(c, "", "  ")
	if err := os.WriteFile(filepath.Join(*out, "corpus.json"), data, 0o644); err != nil {
		panic(err)
	}
	fmt.Printf("wrote %d entries (%d KB padding each) to %s\n", *entries, *padKB, *out)
}

var archRe = regexp.MustCompile(`os:\s*\S+\s+arch:\s*(\S+)`)
var buildIDLineRe = regexp.MustCompile(`build_id:\s*'[0-9a-fA-F]+'`)

func generateDart(out, symbolsPath, tracePath string, entries int) {
	elf, err := os.ReadFile(symbolsPath)
	if err != nil {
		panic(fmt.Errorf("reading dart seed symbols %q: %w", symbolsPath, err))
	}
	template, err := os.ReadFile(tracePath)
	if err != nil {
		panic(fmt.Errorf("reading dart seed trace %q: %w", tracePath, err))
	}
	absSeed, err := filepath.Abs(symbolsPath)
	if err != nil {
		panic(err)
	}
	arch := "arm64"
	if m := archRe.FindStringSubmatch(string(template)); m != nil {
		arch = m[1]
	}

	c := dartCorpus{Language: "dart"}
	for n := 0; n < entries; n++ {
		buildID := fmt.Sprintf("%032x", n)
		dest := filepath.Join(out, buildID+"-"+arch+".symbols")
		_ = os.Remove(dest)
		if err := os.Link(absSeed, dest); err != nil {

			if werr := os.WriteFile(dest, elf, 0o644); werr != nil {
				panic(werr)
			}
		}
		c.Builds = append(c.Builds, dartBuild{BuildID: buildID, Trace: substituteBuildID(string(template), buildID)})
	}
	data, _ := json.MarshalIndent(c, "", "  ")
	if err := os.WriteFile(filepath.Join(out, "corpus.json"), data, 0o644); err != nil {
		panic(err)
	}
	fmt.Printf("wrote %d dart builds (arch %s, seed %s) to %s\n", entries, arch, symbolsPath, out)
}

func substituteBuildID(trace, buildID string) string {
	if buildIDLineRe.MatchString(trace) {
		return buildIDLineRe.ReplaceAllString(trace, "build_id: '"+buildID+"'")
	}
	return "build_id: '" + buildID + "'\n" + trace
}
