package symbolicator

type StackTraceFrame struct {
	File string
	Line uint32
	Col  uint32
}

type TwSymbolicResolver interface {
	ResolveStackTraceLine(file string, line, col uint32) (StackTraceFrame, bool)
}

func ResolveStackTraceLine(twSymbolic TwSymbolicResolver, frame StackTraceFrame) StackTraceFrame {
	if frame.Line == 0 || frame.Col == 0 || twSymbolic == nil {
		return frame
	}
	sstf, ok := twSymbolic.ResolveStackTraceLine(frame.File, frame.Line-1, frame.Col-1)
	if !ok {
		return frame
	}
	return sstf
}
