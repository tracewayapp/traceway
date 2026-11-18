package ops

import (
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/env"
)

// TestOpsMatchToolMap cross-checks the static table against tool-map.md,
// the published CLI-to-MCP translation table.
func TestOpsMatchToolMap(t *testing.T) {
	pairs := ParseToolMap()
	if len(pairs) < 25 {
		t.Fatalf("parsed only %d rows from tool-map.md", len(pairs))
	}
	for _, p := range pairs {
		op := ParseCLI(p.CLI)
		spec, ok := SpecFor(op.Op)
		if !ok {
			t.Errorf("tool-map.md row %v -> %s maps to no op (%s)", p.CLI, p.MCP, op.Op)
			continue
		}
		if spec.MCP != p.MCP {
			t.Errorf("%v: tool-map.md says %s, the ops table says %s", p.CLI, p.MCP, spec.MCP)
		}
	}
}

// TestEveryCLICommandMaps walks both built editions' help: every leaf
// command must map to an op, and exactly the commands only traceway-write
// has are marked WriteEdition, as writes.
func TestEveryCLICommandMaps(t *testing.T) {
	bins := env.BinariesForTest(t)
	inRead := map[string]bool{}
	for _, path := range leafCommands(t, bins.CLI, nil) {
		inRead[strings.Join(path, " ")] = true
		op := ParseCLI(path)
		if op.Op == OpUnknownCLI {
			t.Errorf("traceway %v maps to no op", path)
			continue
		}
		if spec, ok := SpecFor(op.Op); ok && spec.WriteEdition {
			t.Errorf("traceway %v exists in the read edition, but %s is marked WriteEdition", path, op.Op)
		}
	}
	for _, path := range leafCommands(t, bins.CLIWrite, nil) {
		op := ParseCLI(path)
		if op.Op == OpUnknownCLI {
			t.Errorf("traceway-write %v maps to no op", path)
			continue
		}
		if inRead[strings.Join(path, " ")] {
			continue
		}
		if spec, ok := SpecFor(op.Op); !ok || !spec.WriteEdition || !op.Write {
			t.Errorf("traceway-write %v exists only in the write edition, so %s must be WriteEdition and Write", path, op.Op)
		}
	}
}

func leafCommands(t *testing.T, bin string, prefix []string) [][]string {
	out, _ := exec.Command(bin, append(append([]string{}, prefix...), "--help")...).CombinedOutput()
	var subs []string
	in := false
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "Available Commands:") {
			in = true
			continue
		}
		if in {
			if strings.TrimSpace(line) == "" {
				break
			}
			name := strings.Fields(line)[0]
			if name != "help" && name != "completion" {
				subs = append(subs, name)
			}
		}
	}
	if len(subs) == 0 {
		if len(prefix) == 0 {
			t.Fatalf("%s --help listed no commands", bin)
		}
		return [][]string{prefix}
	}
	var leaves [][]string
	for _, s := range subs {
		leaves = append(leaves, leafCommands(t, bin, append(append([]string{}, prefix...), s))...)
	}
	return leaves
}

func TestParseCLI(t *testing.T) {
	cases := []struct {
		argv string
		want Op
	}{
		{"endpoints show 7e3 --recorded-at 2026-10-17T10:00:00Z -o json", Op{Op: "endpoints.show", Args: map[string]any{"id": "7e3", "recorded_at": "2026-10-17T10:00:00Z", "output": "json"}}},
		{"exceptions archive aa bb --yes", Op{Op: "exceptions.archive", Args: map[string]any{"hashes": []any{"aa", "bb"}, "yes": true}, Write: true}},
		{"logs query --service api --min-severity=17", Op{Op: "logs.query", Args: map[string]any{"service_name": "api", "min_severity": "17"}}},
		{"metrics query --name x --tag a=b --tag c=d", Op{Op: "metrics.query", Args: map[string]any{"name": "x", "tag_filters": []any{"a=b", "c=d"}}}},
		{"endpoints mark-slow 'GET /x' --offset-ms 200", Op{Op: "endpoints.mark_slow", Args: map[string]any{"endpoint": "'GET", "offset_ms": "200"}, Write: true}},
		{"post-mortems update 4 --content-file - --expected-updated-at T", Op{Op: "post_mortems.update", Args: map[string]any{"id": "4", "content_md": "-", "expected_updated_at": "T"}, Write: true}},
		// Reading a command's usage is not running it.
		{"endpoints chart --help", Op{Op: OpHelp, Args: map[string]any{"command": "endpoints chart"}}},
		{"exceptions archive -h", Op{Op: OpHelp, Args: map[string]any{"command": "exceptions archive"}}},
	}
	for _, c := range cases {
		got := ParseCLI(strings.Fields(c.argv))
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s:\n got %+v\nwant %+v", c.argv, got, c.want)
		}
	}
}

func TestRenderCLI(t *testing.T) {
	argv, err := RenderCLI("alerts.snooze", map[string]any{"id": 12, "duration_minutes": 60})
	if err != nil || strings.Join(argv, " ") != "alerts snooze 12 --minutes 60" {
		t.Fatalf("RenderCLI = %v, %v", argv, err)
	}
}

func TestInvocationsUnquote(t *testing.T) {
	inv, err := Invocations(`traceway logs query --search "/api/orders " --since 6h | jq -r '.data[] | .body'; echo a\ b`)
	if err != nil {
		t.Fatal(err)
	}
	if len(inv) != 3 || inv[0][4] != "/api/orders " || inv[1][2] != ".data[] | .body" || inv[2][1] != "a b" {
		t.Fatalf("%q", inv)
	}
	op := ParseCLI(inv[0][1:])
	if op.Args["search"] != "/api/orders " {
		t.Fatalf("%+v", op)
	}
}
