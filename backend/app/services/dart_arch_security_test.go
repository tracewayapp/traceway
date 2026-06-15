package services

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestDartSymbolsKeyConfinesArch(t *testing.T) {
	pid := uuid.New()
	prefix := "dartsymbols/" + pid.String() + "/"
	hostile := []string{
		"../../../../../../tmp/pwned",
		"../../" + uuid.New().String() + "/aaaa-arm64",
		"arm64/../../../etc/cron.d/x",
		"..",
		"a/b",
		`back\slash`,
	}
	for _, arch := range hostile {
		key := DartSymbolsKey(pid, fixtureBuildID, arch)
		seg, ok := strings.CutPrefix(key, prefix)
		if !ok {
			t.Errorf("arch %q escaped the project prefix: key=%q", arch, key)
			continue
		}
		if strings.ContainsAny(seg, `/\`) || strings.Contains(seg, "..") {
			t.Errorf("arch %q produced traversal chars in key segment %q (full key %q)", arch, seg, key)
		}
		if cleaned := filepath.ToSlash(filepath.Clean(key)); cleaned != key {
			t.Errorf("arch %q produced a non-clean key %q (clean=%q)", arch, key, cleaned)
		}
	}
}
