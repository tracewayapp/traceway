package traceway

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var schemeRegex = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*://`)

var plainHTTPHosts = map[string]bool{"localhost": true, "127.0.0.1": true, "10.0.2.2": true, "[::1]": true}

// ParseConnectionString reads a self-monitoring connection string (`<project token>@<instance URL>`) with the
// rules every Traceway SDK uses, so any spelling an SDK accepts works here too.
func ParseConnectionString(s string) (token, base string, err error) {
	s = strings.TrimSpace(s)
	at := strings.IndexByte(s, '@')
	if at < 0 || strings.TrimSpace(s[:at]) == "" {
		return "", "", errors.New("self-monitoring connection string must look like <project token>@<instance URL>")
	}
	base, err = normalizeInstanceURL(s[at+1:], true)
	if err != nil {
		return "", "", fmt.Errorf("self-monitoring connection string has an invalid instance URL %q", s[at+1:])
	}
	return strings.TrimSpace(s[:at]), base, nil
}

// NormalizeBaseURL turns any spelling of a server URL (no scheme, a trailing slash, a /api or /api/report suffix,
// a query string) into the base URL; a path other than /api or /api/report is kept (a server under a sub-path).
func NormalizeBaseURL(raw string) (string, error) {
	return normalizeInstanceURL(raw, false)
}

// The backend's own connection strings were also documented with an /api/otel suffix, so the self-monitoring
// parser drops that one as well.
func normalizeInstanceURL(raw string, dropOtelSuffix bool) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", errors.New("the instance URL is empty")
	}
	if !schemeRegex.MatchString(s) {
		if plainHTTPHosts[strings.ToLower(hostWithoutPort(s))] {
			s = "http://" + s
		} else {
			s = "https://" + s
		}
	}
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimRight(s, "/")
	switch {
	case strings.HasSuffix(s, "/api/report"):
		s = strings.TrimSuffix(s, "/api/report")
	case dropOtelSuffix && strings.HasSuffix(s, "/api/otel"):
		s = strings.TrimSuffix(s, "/api/otel")
	default:
		s = strings.TrimSuffix(s, "/api")
	}
	s = strings.TrimRight(s, "/")

	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", errors.New("the instance URL must be an http(s) URL with a host")
	}
	return s, nil
}

func hostWithoutPort(s string) string {
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	if strings.HasPrefix(s, "[") {
		if end := strings.IndexByte(s, ']'); end >= 0 {
			return s[:end+1]
		}
		return s
	}
	if i := strings.LastIndexByte(s, ':'); i >= 0 {
		return s[:i]
	}
	return s
}
