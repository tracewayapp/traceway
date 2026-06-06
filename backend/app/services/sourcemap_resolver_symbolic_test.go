package services

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/backend/app/cache"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/storage"

	"github.com/google/uuid"
)

var recordSymbolic = flag.Bool("record-symbolic", false, "write testdata/symbolic_parity_results.txt")

type symbolicFrame struct {
	inFile     string
	inLine     int
	inCol      int
	file       string
	line       int
	col        int
	name       string
	unresolved bool
}

type symbolicCase struct {
	name   string
	maps   []string
	frames []symbolicFrame
}

type symbolicResult struct {
	caseName         string
	input            string
	expectedLocation string
	actualLocation   string
	locationPass     bool
	expectedName     string
	actualName       string
	namePass         bool
	nameChecked      bool
}

var symbolicCases = []symbolicCase{
	{
		name: "resolves_inlined_function",
		maps: []string{"sourcemapcache/inlining/module.js.map"},
		frames: []symbolicFrame{
			{inFile: "module.js", inLine: 1, inCol: 63, file: "../src/app.js", line: 3, col: 30, name: "buttonCallback"},
			{inFile: "module.js", inLine: 1, inCol: 47, file: "../src/bar.js", line: 4, col: 3, name: "bar"},
			{inFile: "module.js", inLine: 1, inCol: 34, file: "../src/foo.js", line: 2, col: 9},
			{inFile: "module.js", inLine: 2, inCol: 18, unresolved: true},
		},
	},
	{
		name: "writes_simple_cache",
		maps: []string{"sourcemapcache/simple/minified.js.map"},
		frames: []symbolicFrame{
			{inFile: "minified.js", inLine: 1, inCol: 11, file: "tests/fixtures/simple/original.js", line: 2, col: 10, name: "abcd"},
		},
	},
	{
		name: "resolves_location_from_cache",
		maps: []string{"sourcemapcache/preact.module.js.map"},
		frames: []symbolicFrame{
			{inFile: "preact.module.js", inLine: 1, inCol: 50, file: "../src/constants.js", line: 3, col: 35},
			{inFile: "preact.module.js", inLine: 1, inCol: 133, file: "../src/util.js", line: 12, col: 23, name: "assign"},
			{inFile: "preact.module.js", inLine: 1, inCol: 482, file: "../src/create-element.js", line: 40, col: 9, name: "createElement"},
			{inFile: "preact.module.js", inLine: 1, inCol: 9780, file: "../src/component.js", line: 182, col: 5},
			{inFile: "preact.module.js", inLine: 1, inCol: 9795, file: "../src/create-context.js", line: 3, col: 12},
		},
	},
	{
		name: "missing_source_names",
		maps: []string{"sourcemapcache/nofiles.js.map"},
		frames: []symbolicFrame{
			{inFile: "nofiles.js", inLine: 1, inCol: 39, file: "<unknown>", line: 3, col: 9, name: "add"},
		},
	},
	{
		name: "missing_source_contents",
		maps: []string{"sourcemapcache/preact-missing-source-contents.module.js.map"},
		frames: []symbolicFrame{
			{inFile: "preact-missing-source-contents.module.js", inLine: 1, inCol: 133, file: "../src/util.js", line: 12, col: 23},
		},
	},
	{
		name: "hermes_scope_lookup",
		maps: []string{"sourcemapcache/hermes-metro/react-native-hermes.map"},
		frames: []symbolicFrame{
			{inFile: "react-native-hermes", inLine: 1, inCol: 11940, file: "module.js", line: 2, col: 11, name: "foo"},
			{inFile: "react-native-hermes", inLine: 1, inCol: 11858, file: "input.js", line: 3, col: 1, name: "<global>"},
		},
	},
	{
		name: "metro_scope_lookup",
		maps: []string{"sourcemapcache/hermes-metro/react-native-metro.js.map"},
		frames: []symbolicFrame{
			{inFile: "react-native-metro.js", inLine: 7, inCol: 101, file: "module.js", line: 2, col: 11, name: "foo"},
			{inFile: "react-native-metro.js", inLine: 6, inCol: 44, file: "input.js", line: 3, col: 1, name: "<global>"},
		},
	},
	{
		name: "webpack_scope_lookup",
		maps: []string{"sourcemapcache/webpack/bundle.js.map"},
		frames: []symbolicFrame{
			{inFile: "bundle.js", inLine: 1, inCol: 85, file: "webpack:///./foo.js", line: 2, col: 9, name: "module.exports"},
			{inFile: "bundle.js", inLine: 1, inCol: 44, file: "webpack:///./bar.js", line: 2, col: 3, name: "module.exports"},
		},
	},
}

func TestResolveStackTraceSymbolicParity(t *testing.T) {
	cache.InitSourceMapCache(100, 64<<20)
	store, err := storage.NewLocalStorage("testdata")
	if err != nil {
		t.Fatal(err)
	}
	storage.Store = store
	projectId := uuid.New()

	var results []symbolicResult

	for _, tc := range symbolicCases {
		t.Run(tc.name, func(t *testing.T) {
			sourceMaps := make([]*models.SourceMap, 0, len(tc.maps))
			for _, mapPath := range tc.maps {
				sourceMaps = append(sourceMaps, &models.SourceMap{
					ProjectId:  projectId,
					Version:    "1.0.0",
					FileName:   filepath.Base(mapPath),
					StorageKey: mapPath,
				})
			}

			lines := []string{"Error: symbolic fixture test"}
			for _, f := range tc.frames {
				lines = append(lines, "    anonymous()")
				lines = append(lines, fmt.Sprintf("    %s:%d:%d", f.inFile, f.inLine, f.inCol))
			}
			input := strings.Join(lines, "\n")

			resolved := ResolveStackTrace(context.Background(), projectId, input, sourceMaps)
			outLines := strings.Split(resolved, "\n")
			if len(outLines) != len(lines) {
				t.Fatalf("expected %d output lines, got %d:\n%s", len(lines), len(outLines), resolved)
			}

			for i, f := range tc.frames {
				nameLine := outLines[1+2*i]
				locationLine := outLines[2+2*i]
				inputFrame := fmt.Sprintf("%s:%d:%d", f.inFile, f.inLine, f.inCol)

				var expectedLocation string
				var expectedName string
				nameChecked := true
				if f.unresolved {
					expectedLocation = inputFrame
					expectedName = "anonymous"
				} else {
					expectedLocation = fmt.Sprintf("%s:%d:%d", f.file, f.line, f.col)
					expectedName = f.name
					nameChecked = f.name != ""
				}

				actualLocation := strings.TrimPrefix(locationLine, "    ")
				actualName := strings.TrimSuffix(strings.TrimPrefix(nameLine, "    "), "()")

				locationPass := actualLocation == expectedLocation
				namePass := actualName == expectedName

				if !locationPass {
					t.Errorf("frame %s: location expected %q, got %q", inputFrame, expectedLocation, actualLocation)
				}
				if nameChecked && !namePass {
					t.Errorf("frame %s: name expected %q, got %q", inputFrame, expectedName, actualName)
				}

				results = append(results, symbolicResult{
					caseName:         tc.name,
					input:            inputFrame,
					expectedLocation: expectedLocation,
					actualLocation:   actualLocation,
					locationPass:     locationPass,
					expectedName:     expectedName,
					actualName:       actualName,
					namePass:         namePass,
					nameChecked:      nameChecked,
				})
			}
		})
	}

	if *recordSymbolic {
		if err := writeSymbolicResults(results); err != nil {
			t.Fatalf("failed to write results file: %v", err)
		}
	}
}

func writeSymbolicResults(results []symbolicResult) error {
	var b strings.Builder
	passed := 0
	failed := 0
	for _, r := range results {
		if r.locationPass && (!r.nameChecked || r.namePass) {
			passed++
		} else {
			failed++
		}
	}

	b.WriteString("symbolic parity results\n")
	b.WriteString("fixtures: getsentry/symbolic 925230e878ea25f6a0af88171fce93b6272cd30d\n")
	b.WriteString("regenerate: go test ./app/services/ -run TestResolveStackTraceSymbolicParity -record-symbolic\n")
	fmt.Fprintf(&b, "frames: %d passed, %d failed, %d total\n\n", passed, failed, len(results))

	for _, r := range results {
		status := "PASS"
		if !r.locationPass || (r.nameChecked && !r.namePass) {
			status = "FAIL"
		}
		fmt.Fprintf(&b, "[%s] %s :: %s\n", status, r.caseName, r.input)
		fmt.Fprintf(&b, "  location expected: %s\n", r.expectedLocation)
		fmt.Fprintf(&b, "  location actual:   %s\n", r.actualLocation)
		if r.nameChecked {
			fmt.Fprintf(&b, "  name expected:     %s\n", r.expectedName)
		} else {
			b.WriteString("  name expected:     (not asserted)\n")
		}
		fmt.Fprintf(&b, "  name actual:       %s\n", r.actualName)
		b.WriteString("\n")
	}

	return os.WriteFile("testdata/symbolic_parity_results.txt", []byte(b.String()), 0644)
}
