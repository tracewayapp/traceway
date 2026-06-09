package symbolicator

type StackTraceFrame struct {
	File string
	Line uint32
	Col  uint32
	Fn   string
}

type SourceLocation struct {
	File string
	Line uint32
	Col  uint32
	Name string
}

type SourceMap interface {
	Source(genLine, genCol uint32) (SourceLocation, bool)
}

type SourceMapParser interface {
	Parse(sourceMap []byte) (SourceMap, error)
}

type FunctionScopes interface {
	EnclosingFunction(genLine, genCol uint32) (nameLine, nameCol uint32, ok bool)
}

type BundleParser interface {
	Parse(bundle []byte) (FunctionScopes, error)
}

type Resolver struct {
	sourceMap SourceMap
	scopes    FunctionScopes
}

func NewResolver(smParser SourceMapParser, bundleParser BundleParser, sourceMap, bundle []byte) (*Resolver, error) {
	sm, err := smParser.Parse(sourceMap)
	if err != nil {
		return nil, err
	}

	var scopes FunctionScopes
	if bundleParser != nil && len(bundle) > 0 {
		if fs, err := bundleParser.Parse(bundle); err == nil {
			scopes = fs
		}
	}
	return &Resolver{sourceMap: sm, scopes: scopes}, nil
}

func (r *Resolver) Lookup(genLine, genCol uint32) (StackTraceFrame, bool) {
	loc, ok := r.sourceMap.Source(genLine, genCol)
	if !ok {
		return StackTraceFrame{}, false
	}

	frame := StackTraceFrame{File: loc.File, Line: loc.Line, Col: loc.Col}
	if r.scopes != nil {
		if nameLine, nameCol, ok := r.scopes.EnclosingFunction(genLine, genCol); ok {
			if name, ok := r.sourceMap.Source(nameLine, nameCol); ok && name.Name != "" {
				frame.Fn = name.Name
			}
		}
	}
	return frame, true
}

func ResolveStackTraceLine(r *Resolver, frame StackTraceFrame) StackTraceFrame {
	if frame.Line == 0 || frame.Col == 0 || r == nil {
		return frame
	}
	resolved, ok := r.Lookup(frame.Line-1, frame.Col-1)
	if !ok {
		return frame
	}
	return resolved
}
