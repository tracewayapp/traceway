package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"
)

type smMapping struct {
	genLine int32
	genCol  int32
	srcInd  int32
	srcLine int32
	srcCol  int32
	nameInd int32
}

const smMappingBytes = 24

type smSection struct {
	offsetLine     int
	offsetCol      int
	sources        []string
	sourcesContent []string
	names          []string
	mappings       []smMapping
}

type parsedSourceMap struct {
	sections []smSection
	size     int64
}

type smSourceMapJSON struct {
	Version        int               `json:"version"`
	SourceRoot     string            `json:"sourceRoot"`
	Sources        []string          `json:"sources"`
	SourcesContent []string          `json:"sourcesContent"`
	Names          []json.RawMessage `json:"names"`
	Mappings       string            `json:"mappings"`
}

type smSectionJSON struct {
	Offset struct {
		Line   int `json:"line"`
		Column int `json:"column"`
	} `json:"offset"`
	Map *smSourceMapJSON `json:"map"`
}

type smFileJSON struct {
	smSourceMapJSON
	Sections []smSectionJSON `json:"sections"`
}

func parseSourceMap(data []byte) (*parsedSourceMap, error) {
	var f smFileJSON
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	if err := checkSourceMapVersion(f.Version); err != nil {
		return nil, err
	}

	sectionsJSON := f.Sections
	if len(sectionsJSON) == 0 {
		sectionsJSON = []smSectionJSON{{Map: &f.smSourceMapJSON}}
	}

	p := &parsedSourceMap{sections: make([]smSection, 0, len(sectionsJSON))}
	for _, sj := range sectionsJSON {
		if sj.Map == nil {
			return nil, errors.New("sourcemap: section without map")
		}
		s, err := parseSourceMapSection(sj)
		if err != nil {
			return nil, err
		}
		p.sections = append(p.sections, s)
	}
	for i, j := 0, len(p.sections)-1; i < j; i, j = i+1, j-1 {
		p.sections[i], p.sections[j] = p.sections[j], p.sections[i]
	}
	p.computeSize()
	return p, nil
}

func parseSourceMapSection(sj smSectionJSON) (smSection, error) {
	m := sj.Map
	if err := checkSourceMapVersion(m.Version); err != nil {
		return smSection{}, err
	}

	var rootURL *url.URL
	if m.SourceRoot != "" {
		u, err := url.Parse(m.SourceRoot)
		if err != nil {
			return smSection{}, err
		}
		if u.IsAbs() {
			rootURL = u
		}
	}

	sources := make([]string, len(m.Sources))
	for i, src := range m.Sources {
		sources[i] = absSourceName(rootURL, m.SourceRoot, src)
	}

	names := make([]string, len(m.Names))
	for i, raw := range m.Names {
		names[i] = decodeSourceMapName(raw)
	}

	mappings, err := parseSourceMapMappings(m.Mappings)
	if err != nil {
		return smSection{}, err
	}

	return smSection{
		offsetLine:     sj.Offset.Line,
		offsetCol:      sj.Offset.Column,
		sources:        sources,
		sourcesContent: m.SourcesContent,
		names:          names,
		mappings:       mappings,
	}, nil
}

func checkSourceMapVersion(version int) error {
	if version == 3 || version == 0 {
		return nil
	}
	return fmt.Errorf("sourcemap: got version=%d, but only 3rd version is supported", version)
}

func absSourceName(rootURL *url.URL, sourceRoot, source string) string {
	if path.IsAbs(source) {
		return source
	}
	if u, err := url.Parse(source); err == nil && u.IsAbs() {
		return source
	}
	if rootURL != nil {
		u := *rootURL
		u.Path = path.Join(u.Path, source)
		return u.String()
	}
	if sourceRoot != "" {
		return path.Join(sourceRoot, source)
	}
	return source
}

func decodeSourceMapName(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	if raw[0] == '"' && raw[len(raw)-1] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s
		}
	}
	return string(raw)
}

var vlqDecodeMap [256]byte

func init() {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	for i := range len(alphabet) {
		vlqDecodeMap[alphabet[i]] = byte(i)
	}
}

func decodeVLQ(s string, i int) (int32, int, error) {
	var n int32
	shift := uint(0)
	for {
		if i >= len(s) {
			return 0, i, errors.New("sourcemap: unexpected end of mappings")
		}
		c := vlqDecodeMap[s[i]]
		i++
		n += int32(c&31) << shift
		shift += 5
		if c&32 == 0 {
			break
		}
	}
	if n&1 != 0 {
		return -(n >> 1), i, nil
	}
	return n >> 1, i, nil
}

func parseSourceMapMappings(s string) ([]smMapping, error) {
	if s == "" {
		return nil, errors.New("sourcemap: mappings are empty")
	}

	values := make([]smMapping, 0, strings.Count(s, ",")+strings.Count(s, ";"))
	cur := smMapping{genLine: 1, srcLine: 1}
	hasValue := false
	hasName := false
	field := 0

	push := func() {
		if !hasValue {
			return
		}
		hasValue = false
		m := cur
		if hasName {
			hasName = false
		} else {
			m.nameInd = -1
		}
		values = append(values, m)
	}

	i := 0
	for i < len(s) {
		switch s[i] {
		case ',':
			push()
			field = 0
			i++
		case ';':
			push()
			cur.genLine++
			cur.genCol = 0
			field = 0
			i++
		default:
			n, next, err := decodeVLQ(s, i)
			if err != nil {
				return nil, err
			}
			i = next
			switch field {
			case 0:
				cur.genCol += n
			case 1:
				cur.srcInd += n
			case 2:
				cur.srcLine += n
			case 3:
				cur.srcCol += n
			case 4:
				cur.nameInd += n
				hasName = true
			}
			if field == 4 {
				field = 0
			} else {
				field++
			}
			hasValue = true
		}
	}
	push()
	return values, nil
}

func (p *parsedSourceMap) source(genLine, genCol int) (string, string, int, int, bool) {
	for i := range p.sections {
		s := &p.sections[i]
		if s.offsetLine < genLine || (s.offsetLine+1 == genLine && s.offsetCol <= genCol) {
			return s.lookup(genLine-s.offsetLine, genCol-s.offsetCol)
		}
	}
	return "", "", 0, 0, false
}

func (s *smSection) lookup(genLine, genCol int) (string, string, int, int, bool) {
	if len(s.mappings) == 0 {
		return "", "", 0, 0, false
	}

	i := sort.Search(len(s.mappings), func(i int) bool {
		m := &s.mappings[i]
		if int(m.genLine) == genLine {
			return int(m.genCol) >= genCol
		}
		return int(m.genLine) >= genLine
	})

	var match *smMapping
	if i == len(s.mappings) {
		match = &s.mappings[i-1]
		if int(match.genLine) != genLine {
			return "", "", 0, 0, false
		}
	} else {
		match = &s.mappings[i]
		if int(match.genLine) > genLine || int(match.genCol) > genCol {
			if i == 0 {
				return "", "", 0, 0, false
			}
			match = &s.mappings[i-1]
		}
	}

	source := ""
	if match.srcInd >= 0 && int(match.srcInd) < len(s.sources) {
		source = s.sources[match.srcInd]
	}
	name := ""
	if match.nameInd >= 0 && int(match.nameInd) < len(s.names) {
		name = s.names[match.nameInd]
	}
	return source, name, int(match.srcLine), int(match.srcCol), true
}

func (p *parsedSourceMap) sourceContent(file string) string {
	for i := range p.sections {
		s := &p.sections[i]
		for j, src := range s.sources {
			if src == file {
				if j < len(s.sourcesContent) {
					return s.sourcesContent[j]
				}
				break
			}
		}
	}
	return ""
}

func (p *parsedSourceMap) computeSize() {
	size := int64(64)
	for i := range p.sections {
		s := &p.sections[i]
		size += 64 + int64(len(s.mappings))*smMappingBytes
		for _, v := range s.sources {
			size += int64(len(v)) + 16
		}
		for _, v := range s.sourcesContent {
			size += int64(len(v)) + 16
		}
		for _, v := range s.names {
			size += int64(len(v)) + 16
		}
	}
	p.size = size
}
