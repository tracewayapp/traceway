package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joho/godotenv"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/services"
)

func TestAllInOneStartupPreservesEnvironment(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is required for the All-in-One entrypoint")
	}
	script, err := os.ReadFile("../../docker/start-backend.sh")
	if err != nil {
		t.Fatal(err)
	}
	defaults, err := os.ReadFile("../.env.docker")
	if err != nil {
		t.Fatal(err)
	}
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	const operatorSecret = "operator-test-key-with-$literal-characters"
	const oldSecret = "Xt7Kj2mP9qLwNc4rVb8sYd3hFgAeZuCpDnRoWxMvBa5k"
	for _, tc := range []struct {
		name       string
		env        []string
		fileSecret string
		wantSecret string
		wantError  string
	}{
		{"operator overrides old file", []string{"JWT_SECRET=" + operatorSecret, "CLICKHOUSE_SERVER=external:9000", "POSTGRES_HOST=external"}, oldSecret, operatorSecret, ""},
		{"mounted file key", nil, operatorSecret, operatorSecret, ""},
		{"missing key", nil, "", "", "JWT_SECRET environment variable is not set"},
		{"explicit empty key", []string{"JWT_SECRET="}, operatorSecret, "", "JWT_SECRET environment variable is not set"},
		{"public environment key", []string{"JWT_SECRET=" + oldSecret}, "", "", "publicly known default"},
		{"old mounted file", nil, oldSecret, "", "publicly known default"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			envFile := filepath.Join(dir, ".env")
			contents := string(defaults)
			if tc.fileSecret != "" {
				contents += "\nJWT_SECRET='" + tc.fileSecret + "'\n"
			}
			for name, content := range map[string]string{
				".env":                   contents,
				"traceway":               "#!/bin/sh\nexec \"$TRACEWAY_TEST_BINARY\" -test.run '^TestDockerBackendHelper$'\n",
				"wait-for-clickhouse.sh": "#!/bin/sh\nexit 0\n",
				"pg_isready":             "#!/bin/sh\nexit 0\n",
			} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0700); err != nil {
					t.Fatal(err)
				}
			}
			// Map container paths to fixtures while executing the production script.
			entrypoint := strings.ReplaceAll(string(script), "/usr/local/bin/", dir+"/")
			entrypoint = strings.ReplaceAll(entrypoint, "/app/.env", envFile)
			command := exec.Command(bash, "-c", entrypoint)
			command.Dir = dir
			command.Env = append([]string{
				"PATH=" + dir + ":" + os.Getenv("PATH"),
				"TRACEWAY_TEST_BINARY=" + testBinary,
				"TRACEWAY_TEST_DOCKER_BACKEND=1",
				"TRACEWAY_TEST_WANT_SECRET=" + tc.wantSecret,
			}, tc.env...)
			output, err := command.CombinedOutput()
			if tc.wantError != "" {
				if err == nil || !strings.Contains(string(output), tc.wantError) {
					t.Fatalf("expected startup rejection %q: %v\n%s", tc.wantError, err, output)
				}
			} else if err != nil {
				t.Fatalf("startup failed: %v\n%s", err, output)
			}
		})
	}
}

func TestDockerBackendHelper(t *testing.T) {
	if os.Getenv("TRACEWAY_TEST_DOCKER_BACKEND") != "1" {
		return
	}
	if err := godotenv.Load(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	config.Init(config.LoadFromEnv())
	if err := services.InitJWT(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if config.Config.JWTSecret != os.Getenv("TRACEWAY_TEST_WANT_SECRET") {
		fmt.Fprintln(os.Stderr, "startup replaced the configured signing key")
		os.Exit(1)
	}
	os.Exit(0)
}
