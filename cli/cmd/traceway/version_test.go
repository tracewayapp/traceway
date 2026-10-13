package main

import (
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/cli/internal/appdir"
)

func TestVersion_table(t *testing.T) {
	stdout, _, err := runCmd(t, "", "version", "--output", "table")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), appdir.Binary+" version "+version) {
		t.Errorf("expected version line, got: %s", stdout.String())
	}
}

func TestVersion_json(t *testing.T) {
	stdout, _, err := runCmd(t, "", "version", "--output", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), `"version"`) {
		t.Errorf("expected 'version' field in JSON, got: %s", stdout.String())
	}
}

func TestVersion_flag(t *testing.T) {
	stdout, _, err := runCmd(t, "", "--version")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), version) {
		t.Errorf("expected version in --version output, got: %s", stdout.String())
	}
}
