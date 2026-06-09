package services

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseSourceMapSections(t *testing.T) {
	data := []byte(`{"version":3,"sections":[
		{"offset":{"line":0,"column":0},"map":{"version":3,"sources":["first.js"],"names":[],"mappings":"AAAA"}},
		{"offset":{"line":5,"column":0},"map":{"version":3,"sources":["second.js"],"names":[],"mappings":"AAAA"}}]}`)
	p, err := parseSourceMap(data)
	if err != nil {
		t.Fatal(err)
	}

	file, _, line, col, ok := p.source(1, 0)
	if !ok || file != "first.js" || line != 1 || col != 0 {
		t.Errorf("line 1 should resolve via the first section, got file=%q line=%d col=%d ok=%v", file, line, col, ok)
	}

	file, _, line, col, ok = p.source(6, 0)
	if !ok || file != "second.js" || line != 1 || col != 0 {
		t.Errorf("line 6 should resolve via the second section (offset 5), got file=%q line=%d col=%d ok=%v", file, line, col, ok)
	}

	if _, _, _, _, ok = p.source(7, 0); ok {
		t.Error("line past the second section's mappings should not resolve")
	}
}

func TestParseSourceMapErrors(t *testing.T) {
	cases := []struct {
		name    string
		data    string
		wantErr string
	}{
		{"version rejected", `{"version":2,"sources":[],"names":[],"mappings":"AAAA"}`, "version"},
		{"empty mappings", `{"version":3,"sources":[],"names":[],"mappings":""}`, "mappings are empty"},
		{"truncated vlq", `{"version":3,"sources":[],"names":[],"mappings":"g"}`, "unexpected end of mappings"},
		{"section without map", `{"version":3,"sections":[{"offset":{"line":0,"column":0}}]}`, "section without map"},
		{"section version rejected", `{"version":3,"sections":[{"offset":{"line":0,"column":0},"map":{"version":2,"sources":[],"names":[],"mappings":"AAAA"}}]}`, "version"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseSourceMap([]byte(tc.data))
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestParseSourceMapNonStringNames(t *testing.T) {
	p, err := parseSourceMap([]byte(`{"version":3,"sources":["a.js"],"names":[42],"mappings":"AAAAA"}`))
	if err != nil {
		t.Fatal(err)
	}
	_, name, _, _, ok := p.source(1, 0)
	if !ok || name != "42" {
		t.Errorf("expected numeric name decoded to %q, got %q ok=%v", "42", name, ok)
	}
}

func TestParseSourceMapSourceIndexOutOfBounds(t *testing.T) {
	p, err := parseSourceMap([]byte(`{"version":3,"sources":["a.js"],"names":[],"mappings":"ACAA"}`))
	if err != nil {
		t.Fatal(err)
	}
	file, _, line, col, ok := p.source(1, 0)
	if !ok || file != "" || line != 1 || col != 0 {
		t.Errorf("out-of-bounds source index should resolve with empty file, got file=%q line=%d col=%d ok=%v", file, line, col, ok)
	}
}

func TestParseSourceMapSourceRoot(t *testing.T) {
	cases := []struct {
		name       string
		sourceRoot string
		source     string
		want       string
	}{
		{"path join", "src", "a.js", "src/a.js"},
		{"url join", "http://example.com/assets/", "a.js", "http://example.com/assets/a.js"},
		{"absolute path kept", "src", "/abs/a.js", "/abs/a.js"},
		{"absolute url kept", "src", "webpack:///./foo.js", "webpack:///./foo.js"},
		{"no root", "", "../src/a.js", "../src/a.js"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := fmt.Sprintf(`{"version":3,"sourceRoot":%q,"sources":[%q],"names":[],"mappings":"AAAA"}`, tc.sourceRoot, tc.source)
			p, err := parseSourceMap([]byte(data))
			if err != nil {
				t.Fatal(err)
			}
			file, _, _, _, ok := p.source(1, 0)
			if !ok || file != tc.want {
				t.Errorf("expected source %q, got %q ok=%v", tc.want, file, ok)
			}
		})
	}
}
