package envfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestUpsertCreatesFileWith0600(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")

	result, err := Upsert(path, "TRACEWAY_TOKEN", "secret-value")
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if result != ResultCreated {
		t.Errorf("result = %s, want created", result)
	}
	if got := read(t, path); got != "TRACEWAY_TOKEN=secret-value\n" {
		t.Errorf("content = %q", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestUpsertAppendsWithoutTrailingNewline(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	write(t, path, "OTHER=1", 0o644)

	result, err := Upsert(path, "TRACEWAY_TOKEN", "v")
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if result != ResultAdded {
		t.Errorf("result = %s, want added", result)
	}
	if got := read(t, path); got != "OTHER=1\nTRACEWAY_TOKEN=v\n" {
		t.Errorf("content = %q", got)
	}
}

func TestUpsertReplacesAllUncommentedAssignments(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	write(t, path, strings.Join([]string{
		"# TRACEWAY_TOKEN=commented-out",
		"  # TRACEWAY_TOKEN=also-commented",
		"TRACEWAY_TOKEN=old-1",
		"  export TRACEWAY_TOKEN=old-2",
		"TRACEWAY_TOKEN_SUFFIXED=untouched",
		"OTHER=1",
	}, "\n")+"\n", 0o644)

	result, err := Upsert(path, "TRACEWAY_TOKEN", "new")
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if result != ResultUpdated {
		t.Errorf("result = %s, want updated", result)
	}
	want := strings.Join([]string{
		"# TRACEWAY_TOKEN=commented-out",
		"  # TRACEWAY_TOKEN=also-commented",
		"TRACEWAY_TOKEN=new",
		"  export TRACEWAY_TOKEN=new",
		"TRACEWAY_TOKEN_SUFFIXED=untouched",
		"OTHER=1",
	}, "\n") + "\n"
	if got := read(t, path); got != want {
		t.Errorf("content = %q, want %q", got, want)
	}
}

func TestUpsertUnchangedDoesNotRewrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	write(t, path, "TRACEWAY_TOKEN=same\n", 0o644)

	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	result, err := Upsert(path, "TRACEWAY_TOKEN", "same")
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if result != ResultUnchanged {
		t.Errorf("result = %s, want unchanged", result)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Error("unchanged upsert must not rewrite the file")
	}
}

func TestUpsertPreservesModeAndCRLF(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	write(t, path, "A=1\r\nTRACEWAY_TOKEN=old\r\n", 0o640)

	if _, err := Upsert(path, "TRACEWAY_TOKEN", "new"); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if got := read(t, path); got != "A=1\r\nTRACEWAY_TOKEN=new\r\n" {
		t.Errorf("content = %q", got)
	}

	if _, err := Upsert(path, "B", "2"); err != nil {
		t.Fatalf("append upsert: %v", err)
	}
	if got := read(t, path); got != "A=1\r\nTRACEWAY_TOKEN=new\r\nB=2\r\n" {
		t.Errorf("content = %q", got)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Errorf("mode = %v, want 0640 preserved", info.Mode().Perm())
	}
}

func TestUpsertConnectionStringValueRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	value := "tok_abc@https://traceway.example.com/api/report"

	if _, err := Upsert(path, "PUBLIC_TRACEWAY", value); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if got := read(t, path); got != "PUBLIC_TRACEWAY="+value+"\n" {
		t.Errorf("content = %q", got)
	}
}

func TestUpsertCreatesMissingParentDirs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "apps", "web", ".env.local")
	result, err := Upsert(path, "VITE_TRACEWAY_TOKEN", "v")
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if result != ResultCreated || read(t, path) != "VITE_TRACEWAY_TOKEN=v\n" {
		t.Errorf("result = %s, content = %q", result, read(t, path))
	}
}

func TestUpsertFailsWhenTheParentIsAFile(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "web"), "not a directory", 0o644)
	if _, err := Upsert(filepath.Join(dir, "web", ".env"), "KEY", "v"); err == nil {
		t.Fatal("expected an error when the parent path is a file")
	}
}

func TestUpsertXcconfigKeepsDoubleSlash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Traceway.xcconfig")
	write(t, path, "// Traceway\nTRACEWAY_URL = https://old.example.com\n", 0o644)

	if _, err := Upsert(path, "TRACEWAY_URL", "https://traceway.acme.test"); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if _, err := Upsert(path, "TRACEWAY_TOKEN", "tok"); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	want := "// Traceway\nTRACEWAY_URL=https:/$()/traceway.acme.test\nTRACEWAY_TOKEN=tok\n"
	if got := read(t, path); got != want {
		t.Errorf("content = %q, want %q", got, want)
	}
	if value, found, err := Lookup(path, "TRACEWAY_URL"); err != nil || !found || value != "https://traceway.acme.test" {
		t.Errorf("Lookup = (%q, %v, %v), want the unescaped URL", value, found, err)
	}
	if result, err := Upsert(path, "TRACEWAY_URL", "https://traceway.acme.test"); err != nil || result != ResultUnchanged {
		t.Errorf("rewriting the same URL = (%s, %v), want unchanged", result, err)
	}
}

func TestUpsertPropertiesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "local.properties")
	write(t, path, "sdk.dir=/opt/android\ntraceway.token: old\n", 0o644)

	if _, err := Upsert(path, "traceway.token", "new"); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if got := read(t, path); got != "sdk.dir=/opt/android\ntraceway.token=new\n" {
		t.Errorf("content = %q", got)
	}
}

func TestLookup(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	write(t, path, strings.Join([]string{
		"# VITE_TRACEWAY_CONNECTION_STRING=commented",
		`export VITE_TRACEWAY_CONNECTION_STRING="tok@https://old.example.com/api/report"`,
		"QUOTED='single'",
		"SPACED =  spaced value  ",
	}, "\n")+"\n", 0o644)

	cases := map[string]struct {
		value string
		found bool
	}{
		"VITE_TRACEWAY_CONNECTION_STRING": {"tok@https://old.example.com/api/report", true},
		"QUOTED":                          {"single", true},
		"SPACED":                          {"spaced value", true},
		"MISSING":                         {"", false},
	}
	for key, want := range cases {
		value, found, err := Lookup(path, key)
		if err != nil || value != want.value || found != want.found {
			t.Errorf("Lookup(%s) = (%q, %v, %v), want (%q, %v)", key, value, found, err, want.value, want.found)
		}
	}
	if _, found, err := Lookup(filepath.Join(t.TempDir(), "none.env"), "KEY"); err != nil || found {
		t.Errorf("a missing file has no keys: found = %v, err = %v", found, err)
	}
}

func TestUpsertEquivalentKeepsAnOlderSpelling(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	write(t, path, "CS=tok@https://cloud.tracewayapp.com/api/report\n", 0o644)

	same := func(existing string) bool { return strings.HasPrefix(existing, "tok@https://cloud.tracewayapp.com") }
	result, err := UpsertEquivalent(path, "CS", "tok@https://cloud.tracewayapp.com", same)
	if err != nil || result != ResultUnchanged {
		t.Fatalf("result = (%s, %v), want unchanged", result, err)
	}
	if got := read(t, path); got != "CS=tok@https://cloud.tracewayapp.com/api/report\n" {
		t.Errorf("an equivalent value must be left alone: %q", got)
	}

	result, err = UpsertEquivalent(path, "CS", "other@https://cloud.tracewayapp.com", func(string) bool { return false })
	if err != nil || result != ResultUpdated || read(t, path) != "CS=other@https://cloud.tracewayapp.com\n" {
		t.Errorf("a different value must be written: (%s, %v) %q", result, err, read(t, path))
	}
}
