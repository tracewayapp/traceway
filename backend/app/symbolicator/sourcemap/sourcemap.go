package sourcemap

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const NoID int32 = -1
const unmapped uint32 = ^uint32(0)

type Token struct {
	GenLine, GenCol uint32
	SrcID           int32
	SrcLine, SrcCol uint32
	NameID          int32
}

type ParsedMap struct {
	Sources []string
	Names   []string
	Tokens  []Token // sorted by (GenLine, GenCol)
}

type rawMap struct {
	Version    int               `json:"version"`
	SourceRoot string            `json:"sourceRoot"`
	Sources    []*string         `json:"sources"`
	Names      []json.RawMessage `json:"names"`
	Mappings   string            `json:"mappings"`
	Sections   []rawSection      `json:"sections"`
}

type rawSection struct {
	Offset struct {
		Line   uint32 `json:"line"`
		Column uint32 `json:"column"`
	} `json:"offset"`
	Map *json.RawMessage `json:"map"`
}

func Parse(data []byte) (*ParsedMap, error) {
	var raw rawMap
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("sourcemap: invalid JSON: %w", err)
	}
	if raw.Sections != nil {
		return parseIndexed(&raw)
	}
	return parseRegular(&raw)
}

func parseRegular(raw *rawMap) (*ParsedMap, error) {
	m := &ParsedMap{
		Sources: make([]string, len(raw.Sources)),
		Names:   make([]string, 0, len(raw.Names)),
	}
	for i, s := range raw.Sources {
		if s == nil {
			continue
		}
		m.Sources[i] = joinSourceRoot(raw.SourceRoot, *s)
	}
	for _, n := range raw.Names {
		m.Names = append(m.Names, decodeName(n))
	}
	if err := m.decodeMappings(raw.Mappings); err != nil {
		return nil, err
	}
	sortTokens(m.Tokens)
	return m, nil
}

func parseIndexed(raw *rawMap) (*ParsedMap, error) {
	out := &ParsedMap{}
	for i, section := range raw.Sections {
		if section.Map == nil {
			return nil, fmt.Errorf("sourcemap: section %d has no embedded map", i)
		}
		inner, err := Parse(*section.Map)
		if err != nil {
			return nil, fmt.Errorf("sourcemap: section %d: %w", i, err)
		}
		srcOffset := int32(len(out.Sources))
		nameOffset := int32(len(out.Names))
		out.Sources = append(out.Sources, inner.Sources...)
		out.Names = append(out.Names, inner.Names...)
		for _, t := range inner.Tokens {
			if t.GenLine == 0 {
				t.GenCol += section.Offset.Column
			}
			t.GenLine += section.Offset.Line
			if t.SrcID != NoID {
				t.SrcID += srcOffset
			}
			if t.NameID != NoID {
				t.NameID += nameOffset
			}
			out.Tokens = append(out.Tokens, t)
		}
	}
	sortTokens(out.Tokens)
	return out, nil
}

func (m *ParsedMap) decodeMappings(mappings string) error {
	var genLine uint32
	var genCol, srcID, srcLine, srcCol, nameID int64
	nums := make([]int64, 0, 5)

	for _, line := range strings.Split(mappings, ";") {
		genCol = 0
		for _, seg := range strings.Split(line, ",") {
			if seg == "" {
				continue
			}
			nums = nums[:0]
			var err error
			nums, err = decodeVLQ(seg, nums)
			if err != nil {
				return err
			}
			genCol += nums[0]
			t := Token{
				GenLine: genLine,
				GenCol:  uint32(genCol),
				SrcID:   NoID,
				SrcLine: unmapped,
				SrcCol:  unmapped,
				NameID:  NoID,
			}
			if len(nums) >= 4 {
				srcID += nums[1]
				srcLine += nums[2]
				srcCol += nums[3]
				t.SrcID = int32(srcID)
				t.SrcLine = uint32(srcLine)
				t.SrcCol = uint32(srcCol)
				if int(srcID) >= len(m.Sources) || srcID < 0 {
					t.SrcID = NoID
				}
			}
			if len(nums) >= 5 {
				nameID += nums[4]
				if nameID >= 0 && int(nameID) < len(m.Names) {
					t.NameID = int32(nameID)
				}
			}
			m.Tokens = append(m.Tokens, t)
		}
		genLine++
	}
	return nil
}

// FloorToken returns the token at the greatest (GenLine, GenCol) <= (line, col),
// walking back to the first of several tokens that share that position
// (rust-sourcemap greatest_lower_bound). Returns nil before the first token.
func (m *ParsedMap) FloorToken(line, col uint32) *Token {
	idx := sort.Search(len(m.Tokens), func(i int) bool {
		t := m.Tokens[i]
		return t.GenLine > line || (t.GenLine == line && t.GenCol > col)
	})
	if idx == 0 {
		return nil
	}
	idx--
	for idx > 0 {
		a, b := m.Tokens[idx-1], m.Tokens[idx]
		if a.GenLine == b.GenLine && a.GenCol == b.GenCol {
			idx--
		} else {
			break
		}
	}
	return &m.Tokens[idx]
}

func sortTokens(tokens []Token) {
	sort.SliceStable(tokens, func(i, j int) bool {
		a, b := tokens[i], tokens[j]
		if a.GenLine != b.GenLine {
			return a.GenLine < b.GenLine
		}
		return a.GenCol < b.GenCol
	})
}

func decodeName(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return strings.Trim(string(raw), `"`)
}

func joinSourceRoot(root, source string) string {
	if root == "" || strings.Contains(source, "://") || strings.HasPrefix(source, "/") {
		return source
	}
	return strings.TrimSuffix(root, "/") + "/" + source
}
