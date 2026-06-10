package scopes

import (
	"slices"
	"unicode/utf16"
)

type Transition struct {
	Line, Col         uint32
	NameLine, NameCol uint32
	Named             bool
}

type rawScope struct {
	start, end, namePos uint32
}

func scopesFromRaw(src string, raw []rawScope) []Transition {
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
	return buildTransitions(scopes)
}

type genScope struct {
	startLine, startCol uint32
	endLine, endCol     uint32
	nameLine, nameCol   uint32
}

type scopeEvent struct {
	line, col uint32
	start     bool
	scope     int
}

func buildTransitions(scopes []genScope) []Transition {
	events := make([]scopeEvent, 0, len(scopes)*2)
	for i := range scopes {
		s := scopes[i]
		if s.startLine > s.endLine || (s.startLine == s.endLine && s.startCol >= s.endCol) {
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
		if !a.start {
			return -1
		}
		return 1
	})

	var transitions []Transition
	stack := make([]int, 0, 16)
	var lastNameLine, lastNameCol uint32
	lastNamed, haveLast := false, false

	for i := 0; i < len(events); {
		line, col := events[i].line, events[i].col
		for i < len(events) && events[i].line == line && events[i].col == col {
			if events[i].start {
				stack = append(stack, events[i].scope)
			} else {
				for j := len(stack) - 1; j >= 0; j-- {
					if stack[j] == events[i].scope {
						stack = slices.Delete(stack, j, j+1)
						break
					}
				}
			}
			i++
		}

		var nameLine, nameCol uint32
		named := false
		if len(stack) > 0 {
			top := scopes[stack[len(stack)-1]]
			nameLine, nameCol, named = top.nameLine, top.nameCol, true
		}
		if !haveLast || named != lastNamed || nameLine != lastNameLine || nameCol != lastNameCol {
			transitions = append(transitions, Transition{Line: line, Col: col, NameLine: nameLine, NameCol: nameCol, Named: named})
			lastNameLine, lastNameCol, lastNamed, haveLast = nameLine, nameCol, named, true
		}
	}
	return transitions
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
