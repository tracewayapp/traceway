package dart

import (
	"strings"
	"testing"
)

func TestNormalizeArchSanitizesUnsafeInput(t *testing.T) {
	cases := map[string]string{
		"arm64":             "arm64",
		"aarch64":           "arm64",
		"x86_64":            "x64",
		"amd64":             "x64",
		"armv7":             "arm",
		"i386":              "ia32",
		"  ARM64  ":         "arm64",
		"riscv64":           "riscv64",
		"../../../../tmp/x": "tmpx",
		"arm64/../../etc":   "arm64etc",
		`a/b\c`:             "abc",
		"../../arm64":       "arm64",
		"..":                "",
	}
	for in, want := range cases {
		got := NormalizeArch(in)
		if got != want {
			t.Errorf("NormalizeArch(%q) = %q, want %q", in, got, want)
		}
		if strings.ContainsAny(got, `/\.`) {
			t.Errorf("NormalizeArch(%q) = %q still contains a path separator", in, got)
		}
	}
}

func TestIsValidArch(t *testing.T) {
	for _, a := range []string{"arm64", "x64", "x86_64", "ia32", "arm", "riscv64"} {
		if !IsValidArch(a) {
			t.Errorf("IsValidArch(%q) = false, want true", a)
		}
	}
	for _, a := range []string{"", "   ", "../../etc", "arm64/x", "arm-64", "a.b", "arm64\x00", "x64;rm"} {
		if IsValidArch(a) {
			t.Errorf("IsValidArch(%q) = true, want false", a)
		}
	}
}
