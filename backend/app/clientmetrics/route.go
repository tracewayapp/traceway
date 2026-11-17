package clientmetrics

import (
	"regexp"
	"strings"
)

const (
	maxPathSegments = 6
	// MaxDimensionLength is the longest dimension value kept; longer ones become
	// `other`.
	MaxDimensionLength = 64
	idSegment          = ":id"
)

var (
	numberSegment = regexp.MustCompile(`^\d+$`)
	uuidSegment   = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	hexSegment    = regexp.MustCompile(`(?i)^[0-9a-f]{8,}$`)
	tokenSegment  = regexp.MustCompile(`^[A-Za-z0-9_\-=.~]{20,}$`)
)

func isIDSegment(segment string) bool {
	if numberSegment.MatchString(segment) || uuidSegment.MatchString(segment) {
		return true
	}
	if !strings.ContainsAny(segment, "0123456789") {
		return false
	}
	return hexSegment.MatchString(segment) || tokenSegment.MatchString(segment)
}

// NormalizePath turns a raw path into something close to its route template, so
// /orders/8841 and /orders/8842 are one series: segments that are numbers, UUIDs, or
// long hex or token strings become :id. It is the same idea as the exception hash
// normalization, and the same rules the browser SDK applies on the device
// (js-client packages/core/src/metrics/route.ts), so the two must stay in step.
func NormalizePath(path string) string {
	if i := strings.IndexAny(path, "?#"); i >= 0 {
		path = path[:i]
	}
	path = strings.TrimRight(path, "/")
	if path == "" {
		return "/"
	}
	var segments []string
	for _, segment := range strings.Split(path, "/") {
		if segment != "" {
			segments = append(segments, segment)
		}
	}
	truncated := len(segments) > maxPathSegments
	if truncated {
		segments = segments[:maxPathSegments]
	}
	for i, segment := range segments {
		if isIDSegment(segment) {
			segments[i] = idSegment
		}
	}
	out := "/" + strings.Join(segments, "/")
	for len(segments) > 1 {
		extra := 0
		if truncated {
			extra = 2
		}
		if len(out)+extra <= MaxDimensionLength {
			break
		}
		segments = segments[:len(segments)-1]
		truncated = true
		out = "/" + strings.Join(segments, "/")
	}
	if truncated {
		out += "/*"
	}
	if len(out) > MaxDimensionLength {
		out = out[:MaxDimensionLength]
	}
	return out
}
