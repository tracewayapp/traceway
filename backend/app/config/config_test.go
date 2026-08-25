package config

import (
	"slices"
	"testing"
)

func TestTrustedProxyList(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  []string
	}{
		{"default is the private ranges", "", DefaultTrustedProxies},
		{"whitespace only is the default", "  ", DefaultTrustedProxies},
		{"wildcard trusts everything", "*", []string{"0.0.0.0/0", "::/0"}},
		{"none trusts nobody", "none", nil},
		{"none is case-insensitive and padded", "  NONE  ", nil},
		{"single entry", "10.0.0.0/8", []string{"10.0.0.0/8"}},
		{"list with spaces and empty entries", " 10.0.0.1, 192.168.0.0/16,, ", []string{"10.0.0.1", "192.168.0.0/16"}},
		{"trailing comma", "10.0.0.0/8,", []string{"10.0.0.0/8"}},
		{"trailing newline", "10.0.0.0/8\n", []string{"10.0.0.0/8"}},
		{"only separators falls back to the default", ",,", DefaultTrustedProxies},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Cfg{TrustedProxies: tt.value}
			if got := c.TrustedProxyList(); !slices.Equal(got, tt.want) {
				t.Errorf("TrustedProxyList(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
