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
	a := strings.ToLower(strings.TrimSpace(arch))
	switch a {
	case "x86_64", "x64", "amd64":
		return "x64"
	case "aarch64", "arm64":
		return "arm64"
	case "armv7", "arm":
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

func IsValidArch(arch string) bool {
	a := strings.TrimSpace(arch)
	if a == "" {
		return false
	}
	for _, r := range a {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
		default:
			return false
		}
	}
	return true
}

func normalizeFilePath(path string) string {
	if path == "" {
		return path
	}
	if pkg, rest, ok := packageFromHosted(path); ok {
		return "package:" + pkg + "/" + rest
	}
	if pkg, rest, ok := packageFromPackagesDir(path); ok {
		return "package:" + pkg + "/" + rest
	}
	if strings.HasPrefix(path, "/") {
		if i := strings.LastIndex(path, "/lib/"); i != -1 {
			if j := strings.LastIndex(path[:i], "/"); j >= 0 {
				return path[j:]
			}
			return path[i:]
		}
	}
	return path
}

func packageFromPackagesDir(p string) (string, string, bool) {
	_, after, ok := strings.Cut(p, "/packages/")
	if !ok {
		return "", "", false
	}
	pkg, rest, ok := strings.Cut(after, "/")
	if !ok || pkg == "" || !strings.HasPrefix(rest, "lib/") {
		return "", "", false
	}
	return pkg, rest[len("lib/"):], true
}

func packageFromHosted(p string) (string, string, bool) {
	_, after, ok := strings.Cut(p, "/hosted/")
	if !ok {
		return "", "", false
	}
	_, after, ok = strings.Cut(after, "/")
	if !ok {
		return "", "", false
	}
	dir, rest, ok := strings.Cut(after, "/")
	if !ok || dir == "" || !strings.HasPrefix(rest, "lib/") {
		return "", "", false
	}
	pkg, _, ok := strings.Cut(dir, "-")
	if !ok || pkg == "" {
		return "", "", false
	}
	return pkg, rest[len("lib/"):], true
}
