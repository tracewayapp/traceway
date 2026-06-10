package symbolicator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tracewayapp/traceway/backend/app/symbolicator/scopes"
)

func fixture(t testing.TB, parts ...string) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "symbolic"))
	if err == nil {
		if _, statErr := os.Stat(root); statErr == nil {
			return filepath.Join(append([]string{root, "symbolic-testutils", "fixtures"}, parts...)...)
		}
	}
	return filepath.Join(append([]string{"..", "services", "testdata"}, parts...)...)
}

func mustRead(t testing.TB, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	return data
}

func readIfSet(t testing.TB, parts []string) []byte {
	if parts == nil {
		return nil
	}
	return mustRead(t, fixture(t, parts...))
}

type expectation struct {
	line, col uint32

	wantNone bool
	file     string
	srcLine  uint32
	srcCol   uint32
	fn       string
}

type parityCase struct {
	name         string
	minifiedPath []string
	mapPath      []string
	expectations []expectation
}

var parityCases = []parityCase{
	{
		name:         "resolves_inlined_function",
		minifiedPath: []string{"sourcemapcache", "inlining", "module.js"},
		mapPath:      []string{"sourcemapcache", "inlining", "module.js.map"},
		expectations: []expectation{
			{line: 0, col: 62, file: "../src/app.js", srcLine: 2, srcCol: 29, fn: ""},
			{line: 0, col: 46, file: "../src/bar.js", srcLine: 3, srcCol: 2, fn: "bar"},
			{line: 0, col: 33, file: "../src/foo.js", srcLine: 1, srcCol: 8, fn: ""},
			{line: 1, col: 17, wantNone: true},
		},
	},
	{
		name:         "writes_simple_cache",
		minifiedPath: []string{"sourcemapcache", "simple", "minified.js"},
		mapPath:      []string{"sourcemapcache", "simple", "minified.js.map"},
		expectations: []expectation{
			{line: 0, col: 10, file: "tests/fixtures/simple/original.js", srcLine: 1, srcCol: 9, fn: "abcd"},
		},
	},
	{
		name:         "resolves_location_from_cache",
		minifiedPath: []string{"sourcemapcache", "preact.module.js"},
		mapPath:      []string{"sourcemapcache", "preact.module.js.map"},
		expectations: []expectation{
			{line: 0, col: 49, file: "../src/constants.js", srcLine: 2, srcCol: 34, fn: ""},
			{line: 0, col: 132, file: "../src/util.js", srcLine: 11, srcCol: 22, fn: "assign"},
			{line: 0, col: 481, file: "../src/create-element.js", srcLine: 39, srcCol: 8, fn: "createElement"},
			{line: 0, col: 9779, file: "../src/component.js", srcLine: 181, srcCol: 4, fn: ""},
			{line: 0, col: 9794, file: "../src/create-context.js", srcLine: 2, srcCol: 11, fn: ""},
		},
	},
	{
		name:         "missing_source_names",
		minifiedPath: []string{"sourcemapcache", "nofiles.js"},
		mapPath:      []string{"sourcemapcache", "nofiles.js.map"},
		expectations: []expectation{
			{line: 0, col: 38, srcLine: 2, srcCol: 8, fn: "add"},
		},
	},
	{
		name:    "hermes_scope_lookup",
		mapPath: []string{"sourcemapcache", "hermes-metro", "react-native-hermes.map"},
		expectations: []expectation{
			{line: 0, col: 11939, file: "module.js", srcLine: 1, srcCol: 10, fn: ""},
			{line: 0, col: 11857, file: "input.js", srcLine: 2, srcCol: 0, fn: ""},
		},
	},
	{
		name:         "metro_scope_lookup",
		minifiedPath: []string{"sourcemapcache", "hermes-metro", "react-native-metro.js"},
		mapPath:      []string{"sourcemapcache", "hermes-metro", "react-native-metro.js.map"},
		expectations: []expectation{
			{line: 6, col: 100, file: "module.js", srcLine: 1, srcCol: 10, fn: ""},
			{line: 5, col: 43, file: "input.js", srcLine: 2, srcCol: 0, fn: ""},
		},
	},
	{
		name:         "webpack_scope_lookup",
		minifiedPath: []string{"sourcemapcache", "webpack", "bundle.js"},
		mapPath:      []string{"sourcemapcache", "webpack", "bundle.js.map"},
		expectations: []expectation{
			{line: 0, col: 84, file: "webpack:///./foo.js", srcLine: 1, srcCol: 8, fn: ""},
			{line: 0, col: 43, file: "webpack:///./bar.js", srcLine: 1, srcCol: 2, fn: ""},
		},
	},
}

func TestSymbolicParity(t *testing.T) {
	original := scopes.ActiveParser()
	defer func() { _ = scopes.SetParser(original) }()

	for _, parserName := range scopes.AvailableParsers() {
		t.Run(parserName, func(t *testing.T) {
			if err := scopes.SetParser(parserName); err != nil {
				t.Fatalf("SetParser(%q): %v", parserName, err)
			}
			runParityCases(t)
		})
	}
}

func runParityCases(t *testing.T) {
	for _, tc := range parityCases {
		t.Run(tc.name, func(t *testing.T) {
			mapBytes := mustRead(t, fixture(t, tc.mapPath...))
			bundle := readIfSet(t, tc.minifiedPath)

			r, err := NewResolver(mapBytes, bundle)
			if err != nil {
				t.Fatalf("NewResolver: %v", err)
			}

			for _, exp := range tc.expectations {
				frame, ok := r.Lookup(exp.line, exp.col)

				if exp.wantNone {
					if ok {
						t.Errorf("lookup(%d,%d): got %+v, want no mapping", exp.line, exp.col, frame)
					}
					continue
				}
				if !ok {
					t.Errorf("lookup(%d,%d): got no mapping, want a result", exp.line, exp.col)
					continue
				}

				if exp.file != "" && frame.File != exp.file {
					t.Errorf("lookup(%d,%d) file: got %q, want %q", exp.line, exp.col, frame.File, exp.file)
				}
				if frame.Line != exp.srcLine+1 {
					t.Errorf("lookup(%d,%d) line: got %d, want %d", exp.line, exp.col, frame.Line, exp.srcLine+1)
				}
				if frame.Col != exp.srcCol+1 {
					t.Errorf("lookup(%d,%d) col: got %d, want %d", exp.line, exp.col, frame.Col, exp.srcCol+1)
				}
				if frame.Fn != exp.fn {
					t.Errorf("lookup(%d,%d) fn: got %q, want %q", exp.line, exp.col, frame.Fn, exp.fn)
				}
			}
		})
	}
}
