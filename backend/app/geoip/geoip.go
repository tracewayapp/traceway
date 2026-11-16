// Package geoip answers one question: which country a request came from. It has two
// sources, both optional: a header a trusted proxy sets (a CDN knows the country of
// every request) and a MaxMind-format database file. Without either the answer is
// empty and client metrics carry no country.
package geoip

import (
	"fmt"
	"net/netip"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oschwald/maxminddb-golang/v2"
)

const reloadCheck = time.Hour

type Resolver struct {
	header string
	path   string

	reader   atomic.Pointer[maxminddb.Reader]
	mu       sync.Mutex
	modified time.Time
	checked  atomic.Int64
	now      func() time.Time
}

// New builds a resolver. header is the name of a header that carries an ISO 3166-1
// alpha-2 code on every request (CF-IPCountry, CloudFront-Viewer-Country, or one the
// operator's proxy sets); it is trusted as it is, so it is only safe when every
// request passes a proxy that overwrites it. path is a country database in MaxMind DB
// format (GeoLite2-Country, DB-IP's country lite). An unreadable database is an error:
// a configured source that silently does nothing would look like "no traffic from
// anywhere".
func New(header, path string) (*Resolver, error) {
	r := &Resolver{header: strings.TrimSpace(header), path: strings.TrimSpace(path), now: time.Now}
	if r.path != "" {
		if err := r.load(); err != nil {
			return nil, err
		}
		r.checked.Store(r.now().Unix())
	}
	return r, nil
}

// Enabled reports whether any source is configured.
func (r *Resolver) Enabled() bool {
	return r != nil && (r.header != "" || r.path != "")
}

func (r *Resolver) load() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	info, err := os.Stat(r.path)
	if err != nil {
		return fmt.Errorf("GEOIP_DB_PATH: %w", err)
	}
	if r.reader.Load() != nil && info.ModTime().Equal(r.modified) {
		return nil
	}
	// Read into memory, not mapped: an operator replaces the file in place, and the
	// previous reader stays valid for the lookups still using it.
	data, err := os.ReadFile(r.path)
	if err != nil {
		return fmt.Errorf("GEOIP_DB_PATH: %w", err)
	}
	reader, err := maxminddb.OpenBytes(data)
	if err != nil {
		return fmt.Errorf("GEOIP_DB_PATH %s is not a MaxMind DB file: %w", r.path, err)
	}
	r.reader.Store(reader)
	r.modified = info.ModTime()
	return nil
}

func normalize(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) != 2 || code[0] < 'A' || code[0] > 'Z' || code[1] < 'A' || code[1] > 'Z' {
		return ""
	}
	// XX is Cloudflare's "unknown", T1 its Tor marker (not two letters, already gone).
	if code == "XX" {
		return ""
	}
	return code
}

// Country returns the ISO 3166-1 alpha-2 code of a request, or "" when it is not
// known. headerValue reads a request header; ip is the client address.
func (r *Resolver) Country(headerValue func(string) string, ip string) string {
	if r == nil {
		return ""
	}
	if r.header != "" && headerValue != nil {
		if code := normalize(headerValue(r.header)); code != "" {
			return code
		}
	}
	reader := r.reader.Load()
	if reader == nil {
		return ""
	}
	if now := r.now().Unix(); now-r.checked.Load() >= int64(reloadCheck/time.Second) {
		r.checked.Store(now)
		go func() { _ = r.load() }()
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return ""
	}
	var code string
	if err := reader.Lookup(addr.Unmap()).DecodePath(&code, "country", "iso_code"); err != nil {
		return ""
	}
	return normalize(code)
}
