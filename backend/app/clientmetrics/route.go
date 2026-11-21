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
	// A percent-encoded ASCII character (a space, a slash, a brace) is a value somebody
	// typed or a name passed through encodeURIComponent, never a route literal. Encoded
	// UTF-8 bytes (%C3%A9) stay, since localized paths have them.
	encodedASCII = regexp.MustCompile(`(?i)%[0-7][0-9a-f]`)
)

func isIDSegment(segment string) bool {
	if numberSegment.MatchString(segment) || uuidSegment.MatchString(segment) {
		return true
	}
	if encodedASCII.MatchString(segment) {
		return true
	}
	if !strings.ContainsAny(segment, "0123456789") {
		return false
	}
	return hexSegment.MatchString(segment) || tokenSegment.MatchString(segment)
}

// queryStart is where the query string or fragment of a path begins, or -1. The value is a
// route template when the app or its router named the view, and there a "?" ends an
// optional parameter (/:lang?/about): cutting it off made every such route /:lang. A
// template's "?" is told from a query's by the parameter it ends and the segment after it.
func queryStart(path string) int {
	for i := 0; i < len(path); i++ {
		if path[i] == '#' {
			return i
		}
		if path[i] != '?' {
			continue
		}
		segment := strings.LastIndexByte(path[:i], '/') + 1
		optionalParam := path[segment] == ':' && i+1 < len(path) && path[i+1] == '/'
		if !optionalParam {
			return i
		}
	}
	return -1
}

// NormalizePath turns a raw path into something close to its route template, so
// /orders/8841 and /orders/8842 are one series: segments that are numbers, UUIDs, long
// hex or token strings, or that carry a percent-encoded ASCII character become :id. It
// is the same idea as the exception hash normalization, and the same rules the browser
// SDK applies on the device (js-client packages/core/src/metrics/route.ts), so the two
// must stay in step.
func NormalizePath(path string) string {
	if i := queryStart(path); i >= 0 {
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
