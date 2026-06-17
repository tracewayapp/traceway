package main

import (
	"bytes"
	"debug/macho"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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
	Language   string      `json:"language"`
	BinaryName string      `json:"binaryName,omitempty"`
	Builds     []dartBuild `json:"builds"`
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
	dsym := flag.String("dsym", "seeds/ios/app.dsym", "ios: seed dSYM (Mach-O DWARF)")
	binary := flag.String("binary", "sample", "honeycomb-ios: Mach-O name inside the .dSYM bundle")
	traceFile := flag.String("trace", "seeds/dart/trace.txt", "dart/ios: seed non-symbolic trace")
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
	if *language == "ios" {
		generateIOS(*out, *dsym, *traceFile, *entries)
		return
	}
	if *language == "honeycomb-ios" {
		generateHoneycombIOS(*out, *dsym, *traceFile, *binary, *entries)
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

var iosFrameUUIDRe = regexp.MustCompile(`(#\d+\s+)[0-9a-fA-F]{32}(\s+0x)`)

func generateIOS(out, dsymPath, tracePath string, entries int) {
	dsym, err := os.ReadFile(dsymPath)
	if err != nil {
		panic(fmt.Errorf("reading ios seed dSYM %q: %w", dsymPath, err))
	}
	template, err := os.ReadFile(tracePath)
	if err != nil {
		panic(fmt.Errorf("reading ios seed trace %q: %w", tracePath, err))
	}
	absSeed, err := filepath.Abs(dsymPath)
	if err != nil {
		panic(err)
	}
	arch := "arm64"
	if m := archRe.FindStringSubmatch(string(template)); m != nil {
		arch = m[1]
	}

	c := dartCorpus{Language: "ios"}
	for n := 0; n < entries; n++ {
		uuid := fmt.Sprintf("%032x", n)
		dest := filepath.Join(out, uuid+".dsym")
		_ = os.Remove(dest)
		if err := os.Link(absSeed, dest); err != nil {
			if werr := os.WriteFile(dest, dsym, 0o644); werr != nil {
				panic(werr)
			}
		}
		c.Builds = append(c.Builds, dartBuild{BuildID: uuid, Trace: substituteIOSUUID(string(template), uuid)})
	}
	data, _ := json.MarshalIndent(c, "", "  ")
	if err := os.WriteFile(filepath.Join(out, "corpus.json"), data, 0o644); err != nil {
		panic(err)
	}
	fmt.Printf("wrote %d ios builds (arch %s, seed %s) to %s\n", entries, arch, dsymPath, out)
}

func substituteIOSUUID(trace, uuid string) string {
	return iosFrameUUIDRe.ReplaceAllString(trace, "${1}"+uuid+"${2}")
}

var twFrameRe = regexp.MustCompile(`^\s*#(\d+)\s+([0-9a-fA-F]{32})\s+0x([0-9a-fA-F]+)\s+(\S+)\s*$`)

func hyphenateUUID(hex32 string) string {
	h := strings.ToUpper(hex32)
	if len(h) != 32 {
		return h
	}
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

func seedMachoUUID(data []byte) []byte {
	f, err := macho.NewFile(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	defer f.Close()
	for _, l := range f.Loads {
		lb, ok := l.(macho.LoadBytes)
		if !ok {
			continue
		}
		raw := lb.Raw()
		if len(raw) >= 24 && f.ByteOrder.Uint32(raw[0:4]) == 0x1b {
			return append([]byte(nil), raw[8:24]...)
		}
	}
	return nil
}

func patchMachoUUID(seed, oldUUID, newUUID []byte) []byte {
	out := make([]byte, len(seed))
	copy(out, seed)
	if i := bytes.Index(out, oldUUID); i >= 0 {
		copy(out[i:i+16], newUUID)
	}
	return out
}

type iosSeedFrame struct {
	idx    int
	uuid32 string
	image  string
	offset uint64
}

func generateHoneycombIOS(out, dsymPath, tracePath, binaryName string, entries int) {
	seed, err := os.ReadFile(dsymPath)
	if err != nil {
		panic(fmt.Errorf("reading ios seed dSYM %q: %w", dsymPath, err))
	}
	template, err := os.ReadFile(tracePath)
	if err != nil {
		panic(fmt.Errorf("reading ios seed trace %q: %w", tracePath, err))
	}
	oldUUID := seedMachoUUID(seed)
	if oldUUID == nil {
		panic(fmt.Errorf("seed dSYM %q has no LC_UUID", dsymPath))
	}

	arch := "arm64"
	if m := archRe.FindStringSubmatch(string(template)); m != nil {
		arch = m[1]
	}

	var frames []iosSeedFrame
	for _, line := range strings.Split(string(template), "\n") {
		m := twFrameRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		idx, _ := strconv.Atoi(m[1])
		off, _ := strconv.ParseUint(m[3], 16, 64)
		frames = append(frames, iosSeedFrame{idx: idx, uuid32: strings.ToLower(m[2]), image: m[4], offset: off})
	}
	if len(frames) == 0 {
		panic(fmt.Errorf("ios seed trace %q has no frames", tracePath))
	}

	c := dartCorpus{Language: "honeycomb-ios", BinaryName: binaryName}
	for n := 0; n < entries; n++ {
		buildHex := fmt.Sprintf("%032x", n)
		buildUUID := hyphenateUUID(buildHex)
		newUUID, _ := hex.DecodeString(buildHex)

		dwarfDir := filepath.Join(out, buildUUID+".dSYM", "Contents", "Resources", "DWARF")
		if err := os.MkdirAll(dwarfDir, 0o755); err != nil {
			panic(err)
		}
		if err := os.WriteFile(filepath.Join(dwarfDir, binaryName), patchMachoUUID(seed, oldUUID, newUUID), 0o644); err != nil {
			panic(err)
		}

		var b strings.Builder
		fmt.Fprintf(&b, "os: ios arch: %s\n", arch)
		for _, f := range frames {
			uuid := buildUUID
			if f.uuid32 != frames[0].uuid32 {
				uuid = hyphenateUUID(f.uuid32)
			}
			fmt.Fprintf(&b, "%d   %s   0x%016x   %s + %d\n", f.idx, f.image, 0x100000000+f.offset, uuid, f.offset)
		}
		c.Builds = append(c.Builds, dartBuild{BuildID: buildUUID, Trace: b.String()})
	}
	data, _ := json.MarshalIndent(c, "", "  ")
	if err := os.WriteFile(filepath.Join(out, "corpus.json"), data, 0o644); err != nil {
		panic(err)
	}
	fmt.Printf("wrote %d honeycomb-ios builds (arch %s, binary %s, seed %s) to %s\n", entries, arch, binaryName, dsymPath, out)
}
