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

func TestNormalizeFilePath(t *testing.T) {
	cases := map[string]string{
		"/Users/dusanstanojevic/Documents/test-app/lib/main.dart":                                  "/test-app/lib/main.dart",
		"/Users/dusanstanojevic/Documents/flutter/packages/flutter/lib/src/material/ink_well.dart": "package:flutter/src/material/ink_well.dart",
		"/Users/x/.pub-cache/hosted/pub.dev/http-1.2.0/lib/src/client.dart":                        "package:http/src/client.dart",
		"/Users/x/.pub-cache/hosted/pub.dev/flutter_quick_video_encoder-1.7.2/lib/enc.dart":        "package:flutter_quick_video_encoder/enc.dart",
		"/Users/x/.pub-cache/hosted/pub.dev/foo_bar-1.0.0-beta.1/lib/a.dart":                       "package:foo_bar/a.dart",
		"lib/ui/hooks.dart":                        "lib/ui/hooks.dart",
		"third_party/dart/sdk/lib/async/zone.dart": "third_party/dart/sdk/lib/async/zone.dart",
		"file:///private/tmp/dartfix/crash.dart":   "file:///private/tmp/dartfix/crash.dart",
		"dart:isolate-patch/isolate_patch.dart":    "dart:isolate-patch/isolate_patch.dart",
		"":                                         "",
	}
	for in, want := range cases {
		if got := normalizeFilePath(in); got != want {
			t.Errorf("normalizeFilePath(%q) = %q, want %q", in, got, want)
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
