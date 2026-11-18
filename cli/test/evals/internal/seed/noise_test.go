package seed

import (
	"strings"
	"testing"
	"time"
)

// TestBackgroundRouteSyntax: background routes use the parameter syntax of
// the service's router, and their queries the driver's placeholders.
func TestBackgroundRouteSyntax(t *testing.T) {
	cases := []struct {
		stack       Stack
		param, bind string
	}{
		{Go("nethttp", "pgx"), "{id}", "$1"},
		{Go("chi", "sql"), "{id}", "$1"},
		{Go("gin", "sql"), ":id", "$1"},
		{Node("express", "pg"), ":id", "$1"},
		{Python("fastapi", "psycopg"), "{id}", "%s"},
		{Python("flask", "psycopg"), "<id>", "%s"},
		{Java("spring-boot"), "{id}", "?"},
	}
	for _, c := range cases {
		s := NewScenario("bg", time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC), NewDrySink())
		svc := s.Project("p", OTel).Service("svc", c.stack)
		s.Background(svc, TypicalTraffic(20))
		seen := false
		for _, e := range svc.endpoints {
			for _, style := range []string{":", "{", "<"} {
				if strings.Contains(e.Route, style) && !strings.HasPrefix(c.param, style) {
					t.Errorf("%s: route %s, want %s parameters", c.stack.Framework, e.Route, c.param)
				}
			}
			seen = seen || strings.Contains(e.Route, c.param)
		}
		if !seen {
			t.Errorf("%s: no background route has a %s parameter", c.stack.Framework, c.param)
		}
		if q := placeholders(c.stack, selectFor("GET", "users")); !strings.HasSuffix(q, "id = "+c.bind) {
			t.Errorf("%s: query %q, want the %s placeholder", c.stack.Framework, q, c.bind)
		}
	}
}
