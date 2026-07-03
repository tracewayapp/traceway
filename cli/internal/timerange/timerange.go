// Package timerange resolves user-supplied time-range inputs (a relative
// "since" duration or an explicit RFC3339 from/to pair) into a
// client.TimeRange. It is shared by the CLI flag layer and the MCP server so
// both surfaces accept the exact same grammar.
package timerange

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/cli/pkg/client"
)

// ErrInvalid is wrapped by every validation failure in Resolve so callers can
// map any malformed combination to a single "invalid time range" error path.
var ErrInvalid = errors.New("invalid time range")

// Resolve validates the since/from/to combination (empty string = unset) and
// returns the resulting TimeRange. The default (all empty) is "last 1 hour".
func Resolve(since, from, to string) (client.TimeRange, error) {
	if since != "" && (from != "" || to != "") {
		return client.TimeRange{}, fmt.Errorf("%w: since cannot be combined with from/to", ErrInvalid)
	}
	if (from != "") != (to != "") {
		return client.TimeRange{}, fmt.Errorf("%w: from and to must be used together", ErrInvalid)
	}

	if from != "" {
		fromT, err := time.Parse(time.RFC3339, from)
		if err != nil {
			return client.TimeRange{}, fmt.Errorf("%w: from: %v", ErrInvalid, err)
		}
		toT, err := time.Parse(time.RFC3339, to)
		if err != nil {
			return client.TimeRange{}, fmt.Errorf("%w: to: %v", ErrInvalid, err)
		}
		if !toT.After(fromT) {
			return client.TimeRange{}, fmt.Errorf("%w: from must be before to", ErrInvalid)
		}
		return client.TimeRangeFromExplicit(fromT, toT), nil
	}

	dur := time.Hour
	if since != "" {
		d, err := ParseRelativeDuration(since)
		if err != nil {
			return client.TimeRange{}, fmt.Errorf("%w: since: %v", ErrInvalid, err)
		}
		dur = d
	}
	return client.TimeRangeFromSince(dur), nil
}

// ParseRelativeDuration accepts time.ParseDuration's standard input plus a
// simple "Nd" form (positive integer days) which time.ParseDuration rejects.
// Compound forms like "7d2h" or "7D" are not supported on purpose.
func ParseRelativeDuration(s string) (time.Duration, error) {
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
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	if d <= 0 {
		return 0, fmt.Errorf("invalid duration %q: must be positive", s)
	}
	return d, nil
}
