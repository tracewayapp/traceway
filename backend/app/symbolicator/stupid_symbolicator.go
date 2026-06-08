package symbolicator

type TwSymbolicResolverStupid struct {
	Sourcemap string
	Bundle    string
}

func (resolver *TwSymbolicResolverStupid) ResolveStackTraceLine(file string, line, col uint32) (StackTraceFrame, bool) {
	// parse sourcemap
	// parse bundle
	// resolve the line

}
