package main

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/internal/exitcode"
	"github.com/tracewayapp/traceway/cli/internal/output"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

// errInvalidTimeRange is returned for any malformed combination of
// --since / --from / --to. Callers map this to the invalid_time_range
// error envelope code with exit 2 (usage).
var errInvalidTimeRange = errors.New("invalid time range")

// addTimeRangeFlags registers --since, --from, --to on the given command.
// The default (no flags) is "since 1h".
func addTimeRangeFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.String("since", "", "Relative time range, e.g. 1h, 24h, 7d (default: 1h, mutually exclusive with --from/--to)")
	f.String("from", "", "Start of explicit time range, RFC3339 (mutually exclusive with --since)")
	f.String("to", "", "End of explicit time range, RFC3339 (required with --from)")
}

// resolveTimeRange validates the combination of --since/--from/--to on cmd
// and returns the resulting TimeRange. The default (none of the flags) is
// "last 1 hour".
func resolveTimeRange(cmd *cobra.Command) (client.TimeRange, error) {
	since, _ := cmd.Flags().GetString("since")
	from, _ := cmd.Flags().GetString("from")
	to, _ := cmd.Flags().GetString("to")

	if since != "" && (from != "" || to != "") {
		return client.TimeRange{}, fmt.Errorf("%w: --since cannot be combined with --from/--to", errInvalidTimeRange)
	}
	if (from != "") != (to != "") {
		return client.TimeRange{}, fmt.Errorf("%w: --from and --to must be used together", errInvalidTimeRange)
	}

	if from != "" {
		fromT, err := time.Parse(time.RFC3339, from)
		if err != nil {
			return client.TimeRange{}, fmt.Errorf("%w: --from: %v", errInvalidTimeRange, err)
		}
		toT, err := time.Parse(time.RFC3339, to)
		if err != nil {
			return client.TimeRange{}, fmt.Errorf("%w: --to: %v", errInvalidTimeRange, err)
		}
		return client.TimeRangeFromExplicit(fromT, toT), nil
	}

	dur := time.Hour
	if since != "" {
		d, err := parseRelativeDuration(since)
		if err != nil {
			return client.TimeRange{}, fmt.Errorf("%w: --since: %v", errInvalidTimeRange, err)
		}
		dur = d
	}
	return client.TimeRangeFromSince(dur), nil
}

// parseRelativeDuration accepts time.ParseDuration's standard input plus a
// simple "Nd" form (positive integer days) which time.ParseDuration rejects.
// Compound forms like "7d2h" or "7D" are not supported on purpose.
func parseRelativeDuration(s string) (time.Duration, error) {
	if prefix, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.Atoi(prefix)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		if n <= 0 {
			return 0, fmt.Errorf("invalid duration %q: must be positive", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

// renderTimeRangeError maps errInvalidTimeRange (from resolveTimeRange) to an envelope.
func renderTimeRangeError(errOut io.Writer, mode output.Mode, err error) error {
	_ = output.RenderError(errOut, mode, output.ErrorEnvelope{
		Code:     "invalid_time_range",
		Message:  err.Error(),
		Hint:     "use --since DURATION (e.g. 1h, 24h, 7d) or --from RFC3339 --to RFC3339",
		ExitCode: exitcode.Usage,
	})
	return newCLIError(exitcode.Usage, "invalid_time_range")
}

// Sentinels for resolveTimestamp. errTimestampMissing means the required flag
// was omitted; errTimestampInvalid means it wasn't RFC3339. renderTimestampError
// maps them to the right envelope.
var (
	errTimestampMissing = errors.New("timestamp required")
	errTimestampInvalid = errors.New("invalid timestamp")
)

// addTimestampFlag registers a single required RFC3339 timestamp flag (e.g.
// --recorded-at, --started-at) on a by-id detail command. The flag is enforced
// in resolveTimestamp rather than via MarkFlagRequired so a miss renders our
// usage envelope (the root command silences cobra's own errors).
func addTimestampFlag(cmd *cobra.Command, name, usage string) {
	cmd.Flags().String(name, "", usage)
}

// resolveTimestamp reads the named flag and parses it as RFC3339. It is
// mandatory: by-id lookups must bound their query to a window around the
// record's timestamp so ClickHouse prunes daily partitions. Returns
// errTimestampMissing when empty, errTimestampInvalid when malformed.
func resolveTimestamp(cmd *cobra.Command, name string) (time.Time, error) {
	raw, _ := cmd.Flags().GetString(name)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("%w: --%s is required", errTimestampMissing, name)
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: --%s: %v", errTimestampInvalid, name, err)
	}
	return t, nil
}

// renderTimestampError maps a resolveTimestamp failure to an envelope. A missing
// flag is a usage_error; a malformed value is invalid_timestamp. Both exit 2.
func renderTimestampError(errOut io.Writer, mode output.Mode, name string, err error) error {
	code := "invalid_timestamp"
	if errors.Is(err, errTimestampMissing) {
		code = "usage_error"
	}
	_ = output.RenderError(errOut, mode, output.ErrorEnvelope{
		Code:     code,
		Message:  err.Error(),
		Hint:     "pass --" + name + " RFC3339 (e.g. 2026-06-23T14:30:00Z); copy it from the dashboard URL's t= param or a notification's \"Occurred at\"",
		ExitCode: exitcode.Usage,
	})
	return newCLIError(exitcode.Usage, code)
}
