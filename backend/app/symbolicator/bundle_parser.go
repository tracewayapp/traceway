package symbolicator

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf16"
)

type bundleParser func(bundle []byte) (*functionScopes, error)

var bundleParsers = map[string]bundleParser{
	"goja": parseFunctionScopesGoja,
}

var activeBundleParser = "goja"

func SetParser(name string) error {
	if _, ok := bundleParsers[name]; !ok {
		return fmt.Errorf("symbolicator: unknown bundle parser %q (available: %s)", name, strings.Join(AvailableParsers(), ", "))
	}
	activeBundleParser = name
	return nil
}

func ActiveParser() string {
	return activeBundleParser
}

func AvailableParsers() []string {
	names := make([]string, 0, len(bundleParsers))
	for name := range bundleParsers {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func parseFunctionScopes(bundle []byte) (*functionScopes, error) {
	return bundleParsers[activeBundleParser](bundle)
}

type rawScope struct {
	start, end, namePos uint32
}

func scopesFromRaw(src string, raw []rawScope) *functionScopes {
	offsets := make([]uint32, 0, len(raw)*3)
	for _, s := range raw {
		offsets = append(offsets, s.start, s.end, s.namePos)
	}
	pos := convertOffsets(src, offsets)

	scopes := make([]genScope, len(raw))
	for i, s := range raw {
		start, end, name := pos[s.start], pos[s.end], pos[s.namePos]
		scopes[i] = genScope{
			startLine: start.line, startCol: start.col,
			endLine: end.line, endCol: end.col,
			nameLine: name.line, nameCol: name.col,
		}
	}
	return &functionScopes{transitions: buildTransitions(scopes)}
}

type genScope struct {
	startLine, startCol uint32
	endLine, endCol     uint32
	nameLine, nameCol   uint32
}

type functionScopes struct {
	transitions []scopeTransition // sorted by generated position
}

// scopeTransition marks that, from this generated position onward, the
// innermost enclosing function's name token is at (nameLine, nameCol). has is
// false when no function encloses the range (global scope).
type scopeTransition struct {
	line, col         uint32
	nameLine, nameCol uint32
	has               bool
}

type scopeEvent struct {
	line, col uint32
	start     bool
	scope     int
}

// buildTransitions flattens the well-nested function scopes into a sorted list
// of transitions in a single sweep, so the resolver can find the enclosing
// function of any token with a linear merge instead of scanning every scope per
// token.
func buildTransitions(scopes []genScope) []scopeTransition {
	events := make([]scopeEvent, 0, len(scopes)*2)
	for i := range scopes {
		s := scopes[i]
		if !less(s.startLine, s.startCol, s.endLine, s.endCol) {
			continue
		}
		events = append(events,
			scopeEvent{line: s.startLine, col: s.startCol, start: true, scope: i},
			scopeEvent{line: s.endLine, col: s.endCol, start: false, scope: i},
		)
	}
	slices.SortFunc(events, func(a, b scopeEvent) int {
		if a.line != b.line {
			return int(a.line) - int(b.line)
		}
		if a.col != b.col {
			return int(a.col) - int(b.col)
		}
		if a.start == b.start {
			return 0
		}
		if !a.start { // a scope ending here closes before another opens
			return -1
		}
		return 1
	})

	var transitions []scopeTransition
	stack := make([]int, 0, 16)
	var lastNameLine, lastNameCol uint32
	lastHas, haveLast := false, false

	for i := 0; i < len(events); {
		line, col := events[i].line, events[i].col
		for i < len(events) && events[i].line == line && events[i].col == col {
			if events[i].start {
				stack = append(stack, events[i].scope)
			} else {
				stack = removeFromStack(stack, events[i].scope)
			}
			i++
		}

		var nameLine, nameCol uint32
		has := false
		if len(stack) > 0 {
			top := scopes[stack[len(stack)-1]]
			nameLine, nameCol, has = top.nameLine, top.nameCol, true
		}
		if !haveLast || has != lastHas || nameLine != lastNameLine || nameCol != lastNameCol {
			transitions = append(transitions, scopeTransition{line: line, col: col, nameLine: nameLine, nameCol: nameCol, has: has})
			lastNameLine, lastNameCol, lastHas, haveLast = nameLine, nameCol, has, true
		}
	}
	return transitions
}

func removeFromStack(stack []int, scope int) []int {
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i] == scope {
			return append(stack[:i], stack[i+1:]...)
		}
	}
	return stack
}

func less(aLine, aCol, bLine, bCol uint32) bool {
	return aLine < bLine || (aLine == bLine && aCol < bCol)
}

type genPos struct {
	line, col uint32
}

func convertOffsets(src string, offsets []uint32) map[uint32]genPos {
	sorted := append([]uint32(nil), offsets...)
	slices.Sort(sorted)

	out := make(map[uint32]genPos, len(sorted))
	var line, col uint32
	oi := 0
	for i, r := range src {
		for oi < len(sorted) && sorted[oi] == uint32(i) {
			out[sorted[oi]] = genPos{line, col}
			oi++
		}
		if oi >= len(sorted) {
			return out
		}
		if r == '\n' {
			line++
			col = 0
		} else {
			col += uint32(utf16.RuneLen(r))
		}
	}
	for oi < len(sorted) {
		out[sorted[oi]] = genPos{line, col}
		oi++
	}
	return out
}
