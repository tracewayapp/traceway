package geoip

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type mmdbWriter struct{ buf []byte }

func (w *mmdbWriter) control(kind, size int) {
	if kind > 7 {
		w.buf = append(w.buf, byte(size), byte(kind-7))
		return
	}
	w.buf = append(w.buf, byte(kind<<5|size))
}

func (w *mmdbWriter) str(s string) {
	w.control(2, len(s))
	w.buf = append(w.buf, s...)
}

func (w *mmdbWriter) uint(kind int, value uint64) {
	var raw [8]byte
	binary.BigEndian.PutUint64(raw[:], value)
	start := 0
	for start < 7 && raw[start] == 0 {
		start++
	}
	w.control(kind, 8-start)
	w.buf = append(w.buf, raw[start:]...)
}

// countryDatabase builds a MaxMind DB file with one IPv4 network per entry, each a
// /8, mapped to a country. It is the smallest file the reader accepts.
func countryDatabase(t *testing.T, networks map[byte]string) []byte {
	t.Helper()
	var data mmdbWriter
	offsets := map[byte]int{}
	for first, code := range networks {
		offsets[first] = len(data.buf)
		data.control(7, 1)
		data.str("country")
		data.control(7, 1)
		data.str("iso_code")
		data.str(code)
	}

	type node struct{ next [2]int }
	nodes := []node{{[2]int{-1, -1}}}
	for first, offset := range offsets {
		at := 0
		for bit := 7; bit >= 0; bit-- {
			side := int(first>>bit) & 1
			if bit == 0 {
				nodes[at].next[side] = -2 - offset
				break
			}
			if nodes[at].next[side] < 0 {
				nodes = append(nodes, node{[2]int{-1, -1}})
				nodes[at].next[side] = len(nodes) - 1
			}
			at = nodes[at].next[side]
		}
	}
	var out []byte
	for _, n := range nodes {
		for _, next := range n.next {
			value := len(nodes)
			switch {
			case next >= 0:
				value = next
			case next <= -2:
				value = len(nodes) + 16 + (-2 - next)
			}
			out = append(out, byte(value>>16), byte(value>>8), byte(value))
		}
	}
	out = append(out, make([]byte, 16)...)
	out = append(out, data.buf...)
	out = append(out, "\xab\xcd\xefMaxMind.com"...)

	var meta mmdbWriter
	meta.control(7, 9)
	meta.str("binary_format_major_version")
	meta.uint(5, 2)
	meta.str("binary_format_minor_version")
	meta.uint(5, 0)
	meta.str("build_epoch")
	meta.uint(9, uint64(time.Now().Unix()))
	meta.str("database_type")
	meta.str("Test-Country")
	meta.str("description")
	meta.control(7, 0)
	meta.str("ip_version")
	meta.uint(5, 4)
	meta.str("languages")
	meta.control(11, 0)
	meta.str("node_count")
	meta.uint(6, uint64(len(nodes)))
	meta.str("record_size")
	meta.uint(5, 24)
	return append(out, meta.buf...)
}

func TestCountryFromDatabaseAndHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "country.mmdb")
	if err := os.WriteFile(path, countryDatabase(t, map[byte]string{81: "DE", 203: "au"}), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := New("CF-IPCountry", path)
	if err != nil {
		t.Fatal(err)
	}
	header := func(value string) func(string) string {
		return func(name string) string {
			if name == "CF-IPCountry" {
				return value
			}
			return ""
		}
	}
	cases := []struct {
		header, ip, want string
	}{
		{"", "81.2.69.160", "DE"},
		{"", "203.0.113.9", "AU"},
		{"", "::ffff:81.2.69.160", "DE"},
		{"", "8.8.8.8", ""},
		{"", "not an address", ""},
		{"fr", "81.2.69.160", "FR"},
		{"XX", "81.2.69.160", "DE"},
		{"T1", "8.8.8.8", ""},
		{"<script>", "203.0.113.9", "AU"},
	}
	for _, c := range cases {
		if got := r.Country(header(c.header), c.ip); got != c.want {
			t.Errorf("header %q ip %q: %q, want %q", c.header, c.ip, got, c.want)
		}
	}
}

func TestDatabaseIsReloadedWhenReplaced(t *testing.T) {
	path := filepath.Join(t.TempDir(), "country.mmdb")
	if err := os.WriteFile(path, countryDatabase(t, map[byte]string{81: "DE"}), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := New("", path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, countryDatabase(t, map[byte]string{81: "NL"}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, time.Now().Add(time.Minute), time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := r.load(); err != nil {
		t.Fatal(err)
	}
	if got := r.Country(nil, "81.2.69.160"); got != "NL" {
		t.Errorf("after replacing the file: %q", got)
	}
}

func TestUnconfiguredAndBroken(t *testing.T) {
	r, err := New("", "")
	if err != nil || r.Enabled() || r.Country(nil, "81.2.69.160") != "" {
		t.Errorf("unconfigured resolver: %v %v", err, r.Enabled())
	}
	var none *Resolver
	if none.Country(nil, "81.2.69.160") != "" || none.Enabled() {
		t.Error("nil resolver")
	}
	if _, err := New("", filepath.Join(t.TempDir(), "missing.mmdb")); err == nil {
		t.Error("a missing database must be an error")
	}
	bad := filepath.Join(t.TempDir(), "bad.mmdb")
	_ = os.WriteFile(bad, []byte("not a database"), 0o600)
	if _, err := New("", bad); err == nil {
		t.Error("a file that is not a database must be an error")
	}
}
