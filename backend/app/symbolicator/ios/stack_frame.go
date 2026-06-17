package ios

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const maxFrames = 50

type StackFrame struct {
	UUID   string
	Offset uint64
	Image  string
	Raw    string
}

type StackTrace struct {
	OS     string
	Arch   string
	Frames []StackFrame
}

var (
	osArchRe = regexp.MustCompile(`os:\s*(\S+)\s+arch:\s*(\S+)`)
	frameRe  = regexp.MustCompile(`^\s*#(\d+)\s+([0-9a-fA-F]{32})\s+0x([0-9a-fA-F]+)\s*(.*)$`)
)

func ParseTrace(text string) StackTrace {
	var t StackTrace
	for _, line := range strings.Split(text, "\n") {
		if t.OS == "" {
			if m := osArchRe.FindStringSubmatch(line); m != nil {
				t.OS, t.Arch = m[1], m[2]
			}
		}
		if m := frameRe.FindStringSubmatch(line); m != nil {
			off, _ := strconv.ParseUint(m[3], 16, 64)
			t.Frames = append(t.Frames, StackFrame{
				UUID:   strings.ToLower(m[2]),
				Offset: off,
				Image:  strings.TrimSpace(m[4]),
				Raw:    strings.TrimSpace(line),
			})
		}
	}
	return t
}

func IsIOSTrace(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		if frameRe.MatchString(line) {
			return true
		}
	}
	return false
}

func IsIOSLanguage(lang string) bool {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "swift", "ios":
		return true
	default:
		return false
	}
}

func RenderResolved(trace StackTrace, preamble string, lookup func(uuid string, off uint64) []SymFrame) string {
	var b strings.Builder
	if preamble != "" {
		b.WriteString(preamble)
		b.WriteByte('\n')
	}
	n := 0
	for _, f := range trace.Frames {
		if n >= maxFrames {
			break
		}
		resolved := lookup(f.UUID, f.Offset)
		if len(resolved) == 0 {
			fmt.Fprintf(&b, "#%d  %s+0x%x\n", n, frameLabel(f), f.Offset)
			n++
			continue
		}
		for _, sf := range resolved {
			if n >= maxFrames {
				break
			}
			fmt.Fprintf(&b, "#%d  %s (%s)\n", n, sf.Function, sf.Location())
			n++
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func frameLabel(f StackFrame) string {
	if f.Image != "" {
		return f.Image
	}
	if len(f.UUID) >= 8 {
		return f.UUID[:8]
	}
	return f.UUID
}

var (
	honeycombFrameRe = regexp.MustCompile(`^\s*[0-9]+\s+([\w .+-]+?)\s+0x[0-9a-fA-F]+\s+([\w .+-]*?)\s+\+\s+([0-9]+)\s*$`)
	honeycombUUIDRe  = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

func IsHoneycombTrace(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		if honeycombFrameRe.MatchString(line) {
			return true
		}
	}
	return false
}

func ParseHoneycombTrace(text, buildUUID, appExecutable string) StackTrace {
	var t StackTrace
	for _, line := range strings.Split(text, "\n") {
		if t.OS == "" {
			if m := osArchRe.FindStringSubmatch(line); m != nil {
				t.OS, t.Arch = m[1], m[2]
			}
		}
		m := honeycombFrameRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		binary := strings.TrimSpace(m[1])
		ref := strings.TrimSpace(m[2])
		off, err := strconv.ParseUint(m[3], 10, 64)
		if err != nil {
			continue
		}
		var uuid, image string
		switch {
		case honeycombUUIDRe.MatchString(ref):
			uuid, image = NormalizeUUID(ref), binary
		case ref != "" && ref == appExecutable:
			uuid, image = NormalizeUUID(buildUUID), ref
		default:
			continue
		}
		if uuid == "" {
			continue
		}
		t.Frames = append(t.Frames, StackFrame{
			UUID:   uuid,
			Offset: off,
			Image:  image,
			Raw:    strings.TrimSpace(line),
		})
	}
	return t
}
