package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/tracewayapp/traceway/cli/internal/exitcode"
	"github.com/tracewayapp/traceway/cli/internal/output"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

// addPaginationFlags registers --page and --page-size on the given command.
// Defaults: page=1, page-size=50.
func addPaginationFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.Int("page", 1, "Page number (1-indexed)")
	f.Int("page-size", 50, "Page size (max records per response)")
}

// resolvePagination reads the --page/--page-size flags from cmd and returns
// a PaginationParams. Assumes addPaginationFlags was called on the command.
func resolvePagination(cmd *cobra.Command) client.PaginationParams {
	page, _ := cmd.Flags().GetInt("page")
	pageSize, _ := cmd.Flags().GetInt("page-size")
	return client.PaginationParams{Page: page, PageSize: pageSize}
}

// validatePaginationFlags rejects --page/--page-size values the backend would
// refuse (it binds page min=1, pageSize min=1,max=100). On violation it returns
// an error formatted for the usage_error envelope's Message field; callers pass
// it to renderUsageError. Mirrors validateEnumFlag so out-of-range pagination
// surfaces a local usage error instead of a raw backend 400.
func validatePaginationFlags(cmd *cobra.Command) error {
	page, _ := cmd.Flags().GetInt("page")
	pageSize, _ := cmd.Flags().GetInt("page-size")
	if page < 1 {
		return fmt.Errorf("--page must be 1 or greater")
	}
	if pageSize < 1 || pageSize > 100 {
		return fmt.Errorf("--page-size must be between 1 and 100")
	}
	return nil
}

// paginationHint returns a usage_error hint showing the valid pagination bounds
// for a command, e.g. "traceway exceptions list --page <1+> --page-size <1-100>".
func paginationHint(cmdPath string) string {
	return fmt.Sprintf("%s --page <1+> --page-size <1-100>", cmdPath)
}

// firstLine returns the first line of s, useful for fitting a stack trace
// into a table column.
func firstLine(s string) string {
	for i, ch := range s {
		if ch == '\n' || ch == '\r' {
			return s[:i]
		}
	}
	return s
}

// pickStr returns alt if s is empty, else s. Saves a guard at every callsite.
func pickStr(s, alt string) string {
	if s == "" {
		return alt
	}
	return s
}

// pickDefault returns alt if v is zero, else v.
func pickDefault(v, alt int) int {
	if v == 0 {
		return alt
	}
	return v
}

// validateUUIDArg checks that id parses as a UUID. On failure it renders a
// usage_error envelope to the command's stderr and returns the resulting
// *cliError (callers should return it directly); on success it returns nil.
// label names the argument in the message, e.g. "exception id" or "task id".
func validateUUIDArg(cmd *cobra.Command, mode output.Mode, id, label string) error {
	if _, err := uuid.Parse(id); err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode,
			fmt.Sprintf("invalid %s %q: must be a UUID", label, id), "")
	}
	return nil
}

// renderUsageError writes a usage_error envelope and returns a *cliError
// with exitcode.Usage. Used when a command receives invalid flag combinations
// or values that aren't covered by other validators.
func renderUsageError(errOut io.Writer, mode output.Mode, message, hint string) error {
	_ = output.RenderError(errOut, mode, output.ErrorEnvelope{
		Code:     "usage_error",
		Message:  message,
		Hint:     hint,
		ExitCode: exitcode.Usage,
	})
	return newCLIError(exitcode.Usage, "usage_error")
}

// confirmMutation gates a destructive action behind one of three approvals,
// in priority order:
//
//  1. --yes flag is set
//  2. TRACEWAY_ASSUME_YES env var is set to a truthy value (1, true, yes)
//  3. stdin is a TTY → print summary + prompt, accept y/yes
//
// If none apply (non-TTY caller without an opt-in), a usage_error envelope
// is rendered to errOut and a *cliError(Usage) is returned. Callers should
// return that error directly so main() exits 2.
func confirmMutation(cmd *cobra.Command, summaryLines []string) error {
	if flagYes {
		return nil
	}
	if assume := strings.ToLower(strings.TrimSpace(os.Getenv("TRACEWAY_ASSUME_YES"))); assume == "1" || assume == "true" || assume == "yes" {
		return nil
	}

	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())

	in := cmd.InOrStdin()
	f, ok := in.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		return renderUsageError(cmd.ErrOrStderr(), mode,
			"refusing to perform mutation without confirmation",
			"pass --yes or set TRACEWAY_ASSUME_YES=1")
	}

	out := cmd.OutOrStdout()
	for _, line := range summaryLines {
		_, _ = fmt.Fprintln(out, line)
	}
	_, _ = fmt.Fprint(out, "Continue? [y/N] ")

	r := bufio.NewReader(in)
	line, err := r.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return renderUsageError(cmd.ErrOrStderr(), mode,
			"failed to read confirmation: "+err.Error(),
			"pass --yes to skip the prompt")
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	if answer != "y" && answer != "yes" {
		return renderUsageError(cmd.ErrOrStderr(), mode,
			"confirmation declined", "")
	}
	return nil
}
