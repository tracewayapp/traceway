package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_missingFile_returnsEmpty(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil cfg")
	}
	if len(cfg.Profiles) != 0 {
		t.Errorf("expected empty Profiles, got %v", cfg.Profiles)
	}
}

func TestLoad_existingFile_readsJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Join(dir, "traceway"), 0o700); err != nil {
		t.Fatal(err)
	}
	body := `{
		"profiles": {
			"stormwind": {
				"url": "https://traceway.stormwind.local",
				"username": "fred@example.com"
			}
		}
	}`
	if err := os.WriteFile(filepath.Join(dir, "traceway", "config.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	p, ok := cfg.Profiles["stormwind"]
	if !ok {
		t.Fatal("stormwind profile not loaded")
	}
	if p.URL != "https://traceway.stormwind.local" {
		t.Errorf("URL = %q", p.URL)
	}
	if p.Username != "fred@example.com" {
		t.Errorf("Username = %q", p.Username)
	}
}

func TestLoad_corruptFile_returnsError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Join(dir, "traceway"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "traceway", "config.json"), []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(); err == nil {
		t.Fatal("expected Load() to fail on corrupt JSON")
	}
}

func TestSave_writesAtomicallyWith0600(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	cfg := &Config{
		Profiles: map[string]Profile{
			"default": {
				URL:      "https://cloud.tracewayapp.com",
				Username: "fred@example.com",
			},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	path := filepath.Join(dir, "traceway", "config.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file perm = %o, want 0600", perm)
	}

	dirInfo, err := os.Stat(filepath.Join(dir, "traceway"))
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Errorf("dir perm = %o, want 0700", perm)
	}
}

func TestSave_overwritesExisting(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	first := &Config{
		Profiles: map[string]Profile{"a": {URL: "https://a"}},
	}
	if err := first.Save(); err != nil {
		t.Fatalf("first Save: %v", err)
	}

	second := &Config{
		Profiles: map[string]Profile{"b": {URL: "https://b"}},
	}
	if err := second.Save(); err != nil {
		t.Fatalf("second Save: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := loaded.Profiles["a"]; ok {
		t.Error("profile 'a' should have been overwritten")
	}
	if _, ok := loaded.Profiles["b"]; !ok {
		t.Error("profile 'b' should exist after second Save")
	}
}

func TestSave_thenLoad_roundTrips(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	want := &Config{
		Profiles: map[string]Profile{
			"stormwind": {
				URL:      "https://traceway.stormwind.local",
				Username: "fred@example.com",
			},
		},
	}
	if err := want.Save(); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Profiles["stormwind"] != want.Profiles["stormwind"] {
		t.Errorf("Profile mismatch: got %+v want %+v", got.Profiles["stormwind"], want.Profiles["stormwind"])
	}
}

func TestLoad_migratesLegacyShape(t *testing.T) {
	cfgDir := t.TempDir()
	stateDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)
	t.Setenv("XDG_STATE_HOME", stateDir)

	if err := os.MkdirAll(filepath.Join(cfgDir, "traceway"), 0o700); err != nil {
		t.Fatal(err)
	}

	// Write a config.json in the OLD shape (has current_profile, jwt, current_project_id).
	legacy := `{
		"current_profile": "stormwind",
		"profiles": {
			"stormwind": {
				"url": "https://traceway.stormwind.local",
				"username": "fred@example.com",
				"jwt": "abc.def.ghi",
				"current_project_id": "proj-1"
			}
		}
	}`
	if err := os.WriteFile(filepath.Join(cfgDir, "traceway", "config.json"), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Config should have only url+username; no jwt/current_project_id/current_profile.
	p, ok := cfg.Profiles["stormwind"]
	if !ok {
		t.Fatal("stormwind profile not present after migration")
	}
	if p.URL != "https://traceway.stormwind.local" {
		t.Errorf("URL = %q", p.URL)
	}
	if p.Username != "fred@example.com" {
		t.Errorf("Username = %q", p.Username)
	}

	// Verify the on-disk config.json has no legacy fields.
	rawCfg, err := os.ReadFile(filepath.Join(cfgDir, "traceway", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(rawCfg, &raw); err != nil {
		t.Fatal(err)
	}
	if _, has := raw["current_profile"]; has {
		t.Error("config.json still has current_profile after migration")
	}
	profiles, _ := raw["profiles"].(map[string]any)
	sw, _ := profiles["stormwind"].(map[string]any)
	if _, has := sw["jwt"]; has {
		t.Error("config.json still has jwt after migration")
	}
	if _, has := sw["current_project_id"]; has {
		t.Error("config.json still has current_project_id after migration")
	}

	// Verify state.json was created with the moved fields.
	stateFile := filepath.Join(stateDir, "traceway", "state.json")
	rawState, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("state.json not created: %v", err)
	}
	var rawSt map[string]any
	if err := json.Unmarshal(rawState, &rawSt); err != nil {
		t.Fatal(err)
	}
	if rawSt["current_profile"] != "stormwind" {
		t.Errorf("state current_profile = %v, want stormwind", rawSt["current_profile"])
	}
	stateProfiles, _ := rawSt["profiles"].(map[string]any)
	swState, _ := stateProfiles["stormwind"].(map[string]any)
	if swState["jwt"] != "abc.def.ghi" {
		t.Errorf("state jwt = %v, want abc.def.ghi", swState["jwt"])
	}
	if swState["current_project_id"] != "proj-1" {
		t.Errorf("state current_project_id = %v, want proj-1", swState["current_project_id"])
	}
}
