package ios

import "strings"

func NormalizeUUID(uuid string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(uuid)) {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func NormalizeArch(arch string) string {
	a := strings.ToLower(strings.TrimSpace(arch))
	switch a {
	case "x86_64", "x64", "amd64":
		return "x64"
	case "aarch64", "arm64", "arm64e":
		return "arm64"
	case "armv7", "armv7s", "arm":
		return "arm"
	case "ia32", "x86", "i386":
		return "ia32"
	}
	var b strings.Builder
	for _, r := range a {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
