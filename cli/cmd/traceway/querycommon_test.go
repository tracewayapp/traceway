package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/internal/exitcode"
)

// newTestCmd returns a minimal cobra command pre-wired with the global flags
// confirmMutation cares about (--yes is the only one).
func newTestCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().BoolVar(&flagYes, "yes", false, "")
	return cmd
}

func TestConfirmMutation_yesFlagBypassesPrompt(t *testing.T) {
	t.Cleanup(func() { flagYes = false })

	cmd := newTestCmd()
	flagYes = true // set after newTestCmd so BoolVar default doesn't reset it
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetIn(strings.NewReader("")) // no input — must not be consulted

	if err := confirmMutation(cmd, []string{"about to do thing"}); err != nil {
		t.Fatalf("confirmMutation = %v, want nil", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output with --yes, got %q", out.String())
	}
}

func TestConfirmMutation_envVarBypassesPrompt(t *testing.T) {
	t.Setenv("TRACEWAY_ASSUME_YES", "1")
	cmd := newTestCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetIn(strings.NewReader(""))

	if err := confirmMutation(cmd, []string{"summary"}); err != nil {
		t.Fatalf("confirmMutation = %v, want nil", err)
	}
}

func TestConfirmMutation_nonTTYWithoutYes_returnsUsageError(t *testing.T) {
	cmd := newTestCmd()
	stderr := &bytes.Buffer{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(stderr)
	cmd.SetIn(strings.NewReader("")) // io.Reader, not *os.File → not a TTY

	err := confirmMutation(cmd, []string{"summary"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var ce *cliError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *cliError, got %T", err)
	}
	if ce.code != exitcode.Usage {
		t.Errorf("exit code = %d, want %d", ce.code, exitcode.Usage)
	}
	if !strings.Contains(stderr.String(), "usage_error") {
		t.Errorf("stderr should contain usage_error envelope, got %q", stderr.String())
	}
}

func TestResolvePagination_defaults(t *testing.T) {
	cmd := &cobra.Command{Use: "fake"}
	addPaginationFlags(cmd)
	if err := cmd.ParseFlags(nil); err != nil {
		t.Fatal(err)
	}
	p := resolvePagination(cmd)
	if p.Page != 1 {
		t.Errorf("Page = %d, want 1", p.Page)
	}
	if p.PageSize != 50 {
		t.Errorf("PageSize = %d, want 50", p.PageSize)
	}
}

func TestAddPaginationFlags_pageDefaultIsOne(t *testing.T) {
	cmd := &cobra.Command{Use: "fake"}
	addPaginationFlags(cmd)
	f := cmd.Flags().Lookup("page")
	if f == nil {
		t.Fatal("--page flag not registered")
	}
	if f.DefValue != "1" {
		t.Errorf("--page default = %q, want \"1\"", f.DefValue)
	}
}

func TestResolvePagination_explicit(t *testing.T) {
	cmd := &cobra.Command{Use: "fake"}
	addPaginationFlags(cmd)
	if err := cmd.ParseFlags([]string{"--page", "3", "--page-size", "100"}); err != nil {
		t.Fatal(err)
	}
	p := resolvePagination(cmd)
	if p.Page != 3 {
		t.Errorf("Page = %d, want 3", p.Page)
	}
	if p.PageSize != 100 {
		t.Errorf("PageSize = %d, want 100", p.PageSize)
	}
}

func TestValidatePaginationFlags(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"defaults", nil, false},
		{"in-range", []string{"--page", "2", "--page-size", "50"}, false},
		{"max-page-size", []string{"--page-size", "100"}, false},
		{"min-page-size", []string{"--page-size", "1"}, false},
		{"page-zero", []string{"--page", "0"}, true},
		{"page-negative", []string{"--page", "-1"}, true},
		{"page-size-zero", []string{"--page-size", "0"}, true},
		{"page-size-over-max", []string{"--page-size", "101"}, true},
		{"page-size-way-over", []string{"--page-size", "200"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "fake"}
			addPaginationFlags(cmd)
			if err := cmd.ParseFlags(tc.args); err != nil {
				t.Fatal(err)
			}
			err := validatePaginationFlags(cmd)
			if tc.wantErr && err == nil {
				t.Errorf("validatePaginationFlags(%v) = nil, want error", tc.args)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("validatePaginationFlags(%v) = %v, want nil", tc.args, err)
			}
		})
	}
}
