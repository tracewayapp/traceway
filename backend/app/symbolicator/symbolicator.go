package symbolicator

import (
	"runtime"
	"sort"

	"github.com/tracewayapp/traceway/backend/app/symbolicator/scopes"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/sourcemap"
)

type StackTraceFrame struct {
	File string
	Line uint32
	Col  uint32
	Fn   string
}

type resolvedToken struct {
	genLine, genCol uint32
	srcLine, srcCol uint32
	fileIdx         int32
	fnIdx           int32
}

type Resolver struct {
	tokens []resolvedToken
	files  []string
	fns    []string
}

func NewResolver(sourceMap, bundle []byte) (*Resolver, error) {
	m, err := sourcemap.Parse(sourceMap)
	if err != nil {
		return nil, err
	}

	var transitions []scopes.Transition
	if len(bundle) > 0 {
		if ts, err := scopes.Parse(bundle); err == nil {
			transitions = ts
		}
	}

	files := newInterner()
	fns := newInterner()

	ti := -1
	var curFnIdx int32 = -1
	curResolved := false

	tokens := make([]resolvedToken, len(m.Tokens))
	for i := range m.Tokens {
		t := m.Tokens[i]

		// Tokens are sorted by generated position, so the floor transition
		// pointer only moves forward across the whole map (linear merge).
		for ti+1 < len(transitions) {
			next := &transitions[ti+1]
			if t.GenLine < next.Line || (t.GenLine == next.Line && t.GenCol < next.Col) {
				break
			}
			ti++
			curResolved = false
		}

		rt := resolvedToken{genLine: t.GenLine, genCol: t.GenCol, fileIdx: -1, fnIdx: -1}
		if t.SrcID != sourcemap.NoID && int(t.SrcID) < len(m.Sources) {
			rt.fileIdx = files.intern(m.Sources[t.SrcID])
			rt.srcLine = t.SrcLine + 1
			rt.srcCol = t.SrcCol + 1
			if ti >= 0 && transitions[ti].Named {
				// Resolve the enclosing function's name once per transition,
				// not once per token.
				if !curResolved {
					curFnIdx = -1
					tr := transitions[ti]
					if nt := m.FloorToken(tr.NameLine, tr.NameCol); nt != nil && nt.NameID != sourcemap.NoID {
						curFnIdx = fns.intern(m.Names[nt.NameID])
					}
					curResolved = true
				}
				rt.fnIdx = curFnIdx
			}
		}
		tokens[i] = rt
	}

	return &Resolver{tokens: tokens, files: files.list, fns: fns.list}, nil
}

func (r *Resolver) Lookup(genLine, genCol uint32) (StackTraceFrame, bool) {
	defer runtime.KeepAlive(r)
	toks := r.tokens
	idx := sort.Search(len(toks), func(i int) bool {
		return toks[i].genLine > genLine || (toks[i].genLine == genLine && toks[i].genCol > genCol)
	})
	if idx == 0 {
		return StackTraceFrame{}, false
	}
	idx--
	for idx > 0 && toks[idx-1].genLine == toks[idx].genLine && toks[idx-1].genCol == toks[idx].genCol {
		idx--
	}

	t := toks[idx]
	if t.genLine < genLine || t.fileIdx < 0 {
		return StackTraceFrame{}, false
	}
	frame := StackTraceFrame{File: r.files[t.fileIdx], Line: t.srcLine, Col: t.srcCol}
	if t.fnIdx >= 0 {
		frame.Fn = r.fns[t.fnIdx]
	}
	return frame, true
}

// ApproxSize estimates the resolver's retained heap footprint, for cache
// accounting. The flat token table dominates; the bundle and source map are
// not retained.
func (r *Resolver) ApproxSize() int64 {
	n := int64(len(r.tokens)) * 24
	for _, s := range r.files {
		n += int64(len(s)) + 16
	}
	for _, s := range r.fns {
		n += int64(len(s)) + 16
	}
	return n
}

type interner struct {
	list []string
	idx  map[string]int32
}

func newInterner() *interner {
	return &interner{idx: make(map[string]int32)}
}

func (in *interner) intern(s string) int32 {
	if i, ok := in.idx[s]; ok {
		return i
	}
	i := int32(len(in.list))
	in.list = append(in.list, s)
	in.idx[s] = i
	return i
}
