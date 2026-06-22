package android

import "strings"

func NormalizeProguardUUID(s string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(s) {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F', r == '-':
			b.WriteRune(r)
		}
	}
	return b.String()
}
