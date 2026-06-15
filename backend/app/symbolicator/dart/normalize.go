package dart

import "strings"

func NormalizeDebugID(debugID string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(debugID)) {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func NormalizeArch(arch string) string {
	switch strings.ToLower(strings.TrimSpace(arch)) {
	case "x86_64", "x64", "amd64":
		return "x64"
	case "aarch64", "arm64":
		return "arm64"
	case "armv7", "arm":
		return "arm"
	case "ia32", "x86", "i386":
		return "ia32"
	default:
		return strings.ToLower(strings.TrimSpace(arch))
	}
}
