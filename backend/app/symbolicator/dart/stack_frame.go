package dart

import (
	"regexp"
	"strconv"
	"strings"
)

type StackFrame struct {
	Index   int
	Section string
	Offset  uint64
	Raw     string
}

type StackTrace struct {
	BuildID string
	OS      string
	Arch    string
	Frames  []StackFrame
}

var (
	buildIDRe = regexp.MustCompile(`build_id:\s*'([0-9a-fA-F]+)'`)
	osArchRe  = regexp.MustCompile(`os:\s*(\S+)\s+arch:\s*(\S+)`)

	frameRe = regexp.MustCompile(
		`^\s*#(\d+)\s+abs\s+[0-9a-fA-F]+(?:\s+virt\s+[0-9a-fA-F]+)?\s+(_kDart(?:Isolate|Vm)SnapshotInstructions)\+0x([0-9a-fA-F]+)`,
	)
)

func ParseTrace(text string) StackTrace {
	var t StackTrace
	for _, line := range strings.Split(text, "\n") {
		if t.BuildID == "" {
			if m := buildIDRe.FindStringSubmatch(line); m != nil {
				t.BuildID = strings.ToLower(m[1])
			}
		}
		if t.OS == "" {
			if m := osArchRe.FindStringSubmatch(line); m != nil {
				t.OS, t.Arch = m[1], m[2]
			}
		}
		if m := frameRe.FindStringSubmatch(line); m != nil {
			idx, _ := strconv.Atoi(m[1])
			off, _ := strconv.ParseUint(m[3], 16, 64)
			section := "isolate"
			if strings.Contains(m[2], "Vm") {
				section = "vm"
			}
			t.Frames = append(t.Frames, StackFrame{
				Index:   idx,
				Section: section,
				Offset:  off,
				Raw:     strings.TrimSpace(line),
			})
		}
	}
	return t
}

func IsNonSymbolic(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		if frameRe.MatchString(line) {
			return true
		}
	}
	return false
}
