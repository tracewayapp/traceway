package android

import "strings"

func IsR8Trace(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		if atFrameRe.MatchString(line) {
			return true
		}
	}
	return false
}

func IsAndroidTrace(text string) bool {
	return IsR8Trace(text)
}
