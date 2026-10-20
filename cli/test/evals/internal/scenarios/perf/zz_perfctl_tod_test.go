package perf

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func TestZZPerfctlTOD(t *testing.T) {
	cases := map[string]string{
		"perf-ctl-alert-handoff":         "perf/ctl-alert-handoff",
		"perf-ctl-healthy":               "perf/ctl-healthy",
		"perf-ctl-marked-slow-ok":        "perf/ctl-marked-slow-ok",
		"perf-ctl-marked-slow-regressed": "perf/ctl-marked-slow-regressed",
		"perf-ctl-metric-p95-trap":       "perf/ctl-metric-p95-trap",
		"perf-ctl-onset-bisect":          "perf/ctl-onset-bisect",
		"perf-app-sync-background":       "perf/app-sync-background",
		"perf-xs-android-timeouts":       "perf/xs-android-timeouts",
		"perf-xs-frontend-timeouts":      "perf/xs-frontend-timeouts",
		"perf-xs-ios-watchdog":           "perf/xs-ios-watchdog",
	}
	ids := make([]string, 0, len(cases))
	for id := range cases {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	base := time.Date(2026, 10, 20, 0, 13, 0, 0, time.UTC)
	for _, id := range ids {
		vals := map[string][]string{}
		for i := 0; i < 48; i++ {
			now := base.Add(time.Duration(i) * 30 * time.Minute)
			s := seed.NewScenario(id, now, seed.NewDrySink())
			if err := scenarios.Run(cases[id], s); err != nil {
				t.Fatal(err)
			}
			_ = s.Flush()
			for k, v := range s.Exposed() {
				var str string
				switch x := v.(type) {
				case float64:
					str = fmt.Sprintf("%.0f", x)
				case int:
					str = fmt.Sprint(x)
				case nil:
					str = "NIL"
				default:
					b, _ := json.Marshal(x)
					if strings.Contains(string(b), "durationMs") {
						var m map[string]any
						_ = json.Unmarshal(b, &m)
						str = fmt.Sprintf("dur=%.0f", m["durationMs"])
					} else if len(b) > 30 {
						continue
					} else {
						str = string(b)
					}
				}
				vals[k] = append(vals[k], str)
			}
		}
		keys := make([]string, 0, len(vals))
		for k := range vals {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if strings.Contains(k, "Ago") || strings.HasPrefix(k, "ago") || strings.HasSuffix(k, "At") || strings.HasSuffix(k, "From") || strings.HasSuffix(k, "To") || strings.HasPrefix(k, "deploy") || strings.HasPrefix(k, "link") {
				continue
			}
			t.Logf("%s %s: %s", id, k, strings.Join(vals[k], " "))
		}
	}
}
