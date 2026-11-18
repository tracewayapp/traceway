package main

import (
	"net/http"
	"testing"
)

func TestAlertingReadCommandsBothEditions(t *testing.T) {
	calls := 0
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; _, _ = w.Write([]byte(`{}`)) }))
	defer srv.Close()
	seedSessionFor(t, srv.URL)
	for _, args := range [][]string{
		{"channels", "list"}, {"teams", "list", "--organization", "1"}, {"schedules", "show", "1", "--organization", "1"},
		{"contact-methods", "list"}, {"notification-chains", "show"}, {"oncall", "current"}, {"organization-members", "list", "--organization", "1"},
	} {
		if _, stderr, err := runCmd(t, "", args...); err != nil {
			t.Fatalf("%v: %v %s", args, err, stderr)
		}
	}
	if calls != 7 {
		t.Fatalf("requests=%d", calls)
	}
}

func TestAlertingOrganizationAndPersonalReadsWithoutProject(t *testing.T) {
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{}`)) }))
	defer srv.Close()
	seedProfileFor(t, srv.URL)
	for _, args := range [][]string{{"teams", "list", "--organization", "1"}, {"contact-methods", "list"}, {"notification-chains", "show"}} {
		if _, stderr, err := runCmd(t, "", args...); err != nil {
			t.Fatalf("%v: %v %s", args, err, stderr)
		}
	}
	if _, _, err := runCmd(t, "", "channels", "list"); err == nil {
		t.Fatal("project-scoped read must still require a project")
	}
}
