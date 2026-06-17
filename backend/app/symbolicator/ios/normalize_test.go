package ios

import (
	"strings"
	"testing"
)

func TestNormalizeUUID(t *testing.T) {
	cases := map[string]string{
		"2DD71042-1184-32BE-8F92-DD4E3D3FE24A": "2dd71042118432be8f92dd4e3d3fe24a",
		"  FE66-4295  ":                        "fe664295",
		"abcdef0123456789":                     "abcdef0123456789",
		"GHIJ":                                 "",
	}
	for in, want := range cases {
		if got := NormalizeUUID(in); got != want {
			t.Errorf("NormalizeUUID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeArch(t *testing.T) {
	cases := map[string]string{
		"arm64":   "arm64",
		"arm64e":  "arm64",
		"aarch64": "arm64",
		"  ARM64": "arm64",
		"x86_64":  "x64",
		"amd64":   "x64",
		"armv7s":  "arm",
		"i386":    "ia32",
		"../../x": "x",
	}
	for in, want := range cases {
		if got := NormalizeArch(in); got != want {
			t.Errorf("NormalizeArch(%q) = %q, want %q", in, got, want)
		}
		if strings.ContainsAny(NormalizeArch(in), "/.") {
			t.Errorf("NormalizeArch(%q) leaked a path separator", in)
		}
	}
}
