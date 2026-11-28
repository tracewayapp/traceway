// Package connstr reads Traceway connection strings (`<token>@<server>`) and server URLs with the rules every
// Traceway SDK uses, so a value copied from any SDK version, old env file or old plan means the
// same server here.
package connstr

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

// CloudURL is the server SDKs use when no baseUrl is given.
const CloudURL = "https://cloud.tracewayapp.com"

var schemeRegex = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*://`)

var plainHTTPHosts = map[string]bool{"localhost": true, "127.0.0.1": true, "10.0.2.2": true, "[::1]": true}

// Parse splits a connection string into the project token and the server's base URL.
func Parse(s string) (token, baseURL string, err error) {
	s = strings.TrimSpace(s)
	at := strings.IndexByte(s, '@')
	if at < 0 {
		return "", "", errors.New("a connection string looks like <token>@<server URL>")
	}
	token = strings.TrimSpace(s[:at])
	if token == "" {
		return "", "", errors.New("the connection string has no token before the @")
	}
	baseURL, err = NormalizeBaseURL(s[at+1:])
	if err != nil {
		return "", "", err
	}
	return token, baseURL, nil
}

// NormalizeBaseURL turns any form of a server URL (no scheme, a trailing slash, the 1.x /api/report suffix, a
// query string) into the base URL the SDKs send to. A path other than /api or /api/report is kept, so a server
// under a sub-path stays reachable.
func NormalizeBaseURL(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", errors.New("the server URL is empty")
	}
	if !schemeRegex.MatchString(s) {
		if plainHTTPHosts[strings.ToLower(hostOf(s))] {
			s = "http://" + s
		} else {
			s = "https://" + s
		}
	}
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return "", errors.New("the server URL must be an http(s) URL with a host")
	}
	// The authority can itself be named "api".
	schemeEnd := strings.Index(s, "://") + 3
	host, tail, _ := strings.Cut(s[schemeEnd:], "/")
	authority := s[:schemeEnd] + host
	path := "/" + tail
	path = strings.TrimRight(path, "/")
	if strings.HasSuffix(path, "/api/report") {
		path = strings.TrimSuffix(path, "/api/report")
	} else {
		path = strings.TrimSuffix(path, "/api")
	}
	path = strings.TrimRight(path, "/")
	return authority + path, nil
}

// IsCloud reports whether a base URL is Traceway Cloud, where SDKs need no baseUrl.
func IsCloud(baseURL string) bool {
	u, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Scheme, "https") && strings.EqualFold(u.Hostname(), "cloud.tracewayapp.com") &&
		(u.Port() == "" || u.Port() == "443") && strings.Trim(u.Path, "/") == ""
}

// SameHost reports whether two base URLs name the same host and port, whatever their scheme and path.
func SameHost(a, b string) bool {
	ua, errA := url.Parse(a)
	ub, errB := url.Parse(b)
	return errA == nil && errB == nil && ua.Host != "" && strings.EqualFold(ua.Host, ub.Host)
}

// hostOf is the host of a URL written without a scheme, without its port.
func hostOf(s string) string {
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
