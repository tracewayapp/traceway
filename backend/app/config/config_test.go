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
		{"default is loopback", "", []string{"127.0.0.1", "::1"}},
		{"whitespace only is loopback", "  ", []string{"127.0.0.1", "::1"}},
		{"wildcard trusts everything", "*", []string{"0.0.0.0/0", "::/0"}},
		{"list with spaces and empty entries", " 10.0.0.1, 192.168.0.0/16,, ", []string{"10.0.0.1", "192.168.0.0/16"}},
		{"only separators trusts nobody", ",,", nil},
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
