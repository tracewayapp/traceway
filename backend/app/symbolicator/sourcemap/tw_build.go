package sourcemap

import (
	"github.com/tracewayapp/traceway/backend/app/symbolicator/sourcemap/scopes"
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

type builder struct {
	tokens []resolvedToken
	files  []string
	fns    []string
}

func BuildTW(sourceMap, bundle []byte) ([]byte, error) {
	b, err := newBuilder(sourceMap, bundle)
	if err != nil {
		return nil, err
	}
	return b.marshal(), nil
}

func newBuilder(sourceMap, bundle []byte) (*builder, error) {
	m, err := Parse(sourceMap)
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

		for ti+1 < len(transitions) {
			next := &transitions[ti+1]
			if t.GenLine < next.Line || (t.GenLine == next.Line && t.GenCol < next.Col) {
				break
			}
			ti++
			curResolved = false
		}

		rt := resolvedToken{genLine: t.GenLine, genCol: t.GenCol, fileIdx: -1, fnIdx: -1}
		if t.SrcID != NoID && int(t.SrcID) < len(m.Sources) {
			rt.fileIdx = files.intern(m.Sources[t.SrcID])
			rt.srcLine = t.SrcLine + 1
			rt.srcCol = t.SrcCol + 1
			if ti >= 0 && transitions[ti].Named {

				if !curResolved {
					curFnIdx = -1
					tr := transitions[ti]
					if nt := m.FloorToken(tr.NameLine, tr.NameCol); nt != nil && nt.NameID != NoID {
						curFnIdx = fns.intern(m.Names[nt.NameID])
					}
					curResolved = true
				}
				rt.fnIdx = curFnIdx
			}
		}
		tokens[i] = rt
	}

	return &builder{tokens: tokens, files: files.list, fns: fns.list}, nil
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
