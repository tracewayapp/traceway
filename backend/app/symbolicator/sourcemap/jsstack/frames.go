package jsstack

import (
	"strconv"
	"strings"
)

type Frame struct {
	Function string
	URL      string
	Line     uint32
	Col      uint32
}

func ParseFrames(trace string) []Frame {
	lines := strings.Split(trace, "\n")
	parse := detectJsFrameParser(lines)
	if parse == nil {
		return nil
	}
	var frames []Frame
	for _, line := range lines {
		f, ok := parse(line)
		if !ok || f.loc == "" {
			continue
		}
		url, lineNum, col, ok := splitJsLoc(f.loc)
		if !ok {
			continue
		}
		frames = append(frames, Frame{Function: f.fn, URL: url, Line: lineNum, Col: col})
	}
	return frames
}

func splitJsLoc(loc string) (string, uint32, uint32, bool) {
	last := strings.LastIndex(loc, ":")
	if last == -1 {
		return "", 0, 0, false
	}
	second := strings.LastIndex(loc[:last], ":")
	if second == -1 {
		return "", 0, 0, false
	}
	line, err1 := strconv.ParseUint(loc[second+1:last], 10, 32)
	col, err2 := strconv.ParseUint(loc[last+1:], 10, 32)
	if err1 != nil || err2 != nil {
		return "", 0, 0, false
	}
	return loc[:second], uint32(line), uint32(col), true
}
