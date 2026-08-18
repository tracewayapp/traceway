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

func TestUpsertMissingParentDirErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", ".env")
	if _, err := Upsert(path, "KEY", "v"); err == nil {
		t.Fatal("expected an error for a missing parent directory")
	}
}
