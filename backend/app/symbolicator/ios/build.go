package ios

import (
	"debug/dwarf"
	"encoding/binary"
	"path/filepath"
	"sort"
)

const maxInlineFrames = 64

func BuildFlat(dsymBytes []byte, uuid, arch string) ([]byte, error) {
	data, textVMAddr, found, err := dwarfForSlice(dsymBytes, uuid, arch)
	if err != nil {
		return nil, err
	}
	return flatten(data, textVMAddr, found).marshal(), nil
}

type flatBuilder struct {
	textVMAddr uint64
	uuid       string

	entries []flatEntry
	frames  []flatFrame
	files   []string
	fns     []string
}

type flatEntry struct {
	pc, hi     uint64
	frameStart uint32
	frameCount uint32
}

type flatFrame struct {
	fileIdx int32
	line    int32
	col     int32
	fnIdx   int32
}

type funcRange struct {
	low, high uint64
	off       dwarf.Offset
}

type lineRow struct {
	addr, end uint64
	file      string
	line, col int
}

func flatten(data *dwarf.Data, textVMAddr uint64, uuid string) *flatBuilder {
	f := &flatBuilder{textVMAddr: textVMAddr, uuid: uuid}
	files := newInterner()
	fns := newInterner()

	rdr := data.Reader()
	for {
		cu, err := rdr.Next()
		if err != nil || cu == nil {
			break
		}
		if cu.Tag != dwarf.TagCompileUnit {
			rdr.SkipChildren()
			continue
		}

		lr, lrErr := data.LineReader(cu)

		topFuncs, bounds := collectFuncsAndBounds(data, rdr)
		if lrErr != nil || lr == nil || len(topFuncs) == 0 {
			continue
		}
		sort.Slice(topFuncs, func(i, j int) bool { return topFuncs[i].low < topFuncs[j].low })
		lineFiles := lr.Files()

		var raw []dwarf.LineEntry
		for {
			var le dwarf.LineEntry
			if err := lr.Next(&le); err != nil {
				break
			}
			raw = append(raw, le)
		}
		var rows []lineRow
		for i := 0; i+1 < len(raw); i++ {
			le := raw[i]
			if le.EndSequence {
				continue
			}
			end := raw[i+1].Address
			if end <= le.Address {
				continue
			}
			file := ""
			if le.File != nil {
				file = le.File.Name
			}
			rows = append(rows, lineRow{addr: le.Address, end: end, file: file, line: le.Line, col: le.Column})
			bounds = append(bounds, le.Address, end)
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].addr < rows[j].addr })
		bounds = sortUnique(bounds)

		var lastFrames []SymFrame
		for bi := 0; bi+1 < len(bounds); bi++ {
			a, b := bounds[bi], bounds[bi+1]
			fn := findFunc(topFuncs, a)
			if fn == nil {
				lastFrames = nil
				continue
			}
			var chain []*dwarf.Entry
			cr := data.Reader()
			cr.Seek(fn.off)
			descend(data, cr, a, &chain)
			if len(chain) == 0 {
				lastFrames = nil
				continue
			}
			lf, ll, lc := rowAt(rows, a)
			sfs := framesFromChain(data, chain, lineFiles, lf, ll, lc)

			if lastFrames != nil && len(f.entries) > 0 && f.entries[len(f.entries)-1].hi == a && sameFrames(sfs, lastFrames) {
				f.entries[len(f.entries)-1].hi = b
				continue
			}
			start := uint32(len(f.frames))
			for _, sf := range sfs {
				f.frames = append(f.frames, flatFrame{
					fileIdx: files.intern(normalizeFilePath(sf.File)),
					line:    int32(sf.Line),
					col:     int32(sf.Col),
					fnIdx:   fns.intern(sf.Function),
				})
			}
			f.entries = append(f.entries, flatEntry{pc: a, hi: b, frameStart: start, frameCount: uint32(len(sfs))})
			lastFrames = sfs
		}
	}

	f.files = files.list
	f.fns = fns.list
	sort.Slice(f.entries, func(i, j int) bool { return f.entries[i].pc < f.entries[j].pc })
	return f
}

func collectFuncsAndBounds(d *dwarf.Data, rdr *dwarf.Reader) ([]funcRange, []uint64) {
	var funcs []funcRange
	var bounds []uint64
	depth := 0
	for {
		e, err := rdr.Next()
		if err != nil || e == nil {
			return funcs, bounds
		}
		if e.Tag == 0 {
			depth--
			if depth < 0 {
				return funcs, bounds
			}
			continue
		}
		if e.Tag == dwarf.TagSubprogram || e.Tag == dwarf.TagInlinedSubroutine {
			if rs, rerr := d.Ranges(e); rerr == nil {
				for _, rg := range rs {
					if rg[1] > rg[0] {
						bounds = append(bounds, rg[0], rg[1])
						if e.Tag == dwarf.TagSubprogram && depth == 0 {
							funcs = append(funcs, funcRange{low: rg[0], high: rg[1], off: e.Offset})
						}
					}
				}
			}
		}
		if e.Children {
			depth++
		}
	}
}

func findFunc(funcs []funcRange, addr uint64) *funcRange {
	i := sort.Search(len(funcs), func(i int) bool { return funcs[i].low > addr }) - 1
	if i >= 0 && addr >= funcs[i].low && addr < funcs[i].high {
		return &funcs[i]
	}
	return nil
}

func rowAt(rows []lineRow, addr uint64) (string, int, int) {
	i := sort.Search(len(rows), func(i int) bool { return rows[i].addr > addr }) - 1
	if i >= 0 && addr >= rows[i].addr && addr < rows[i].end {
		return rows[i].file, rows[i].line, rows[i].col
	}
	return "", 0, 0
}

func sortUnique(xs []uint64) []uint64 {
	if len(xs) < 2 {
		return xs
	}
	sort.Slice(xs, func(i, j int) bool { return xs[i] < xs[j] })
	out := xs[:1]
	for _, x := range xs[1:] {
		if x != out[len(out)-1] {
			out = append(out, x)
		}
	}
	return out
}

func sameFrames(a, b []SymFrame) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func framesFromChain(d *dwarf.Data, chain []*dwarf.Entry, files []*dwarf.LineFile, leafFile string, leafLine, leafCol int) []SymFrame {
	if len(chain) > maxInlineFrames {
		chain = chain[len(chain)-maxInlineFrames:]
	}
	out := make([]SymFrame, len(chain))
	for i := len(chain) - 1; i >= 0; i-- {
		pos := len(chain) - 1 - i
		sf := SymFrame{Function: nameOf(d, chain[i])}
		if i == len(chain)-1 {
			sf.File, sf.Line, sf.Col = leafFile, leafLine, leafCol
		} else {
			inner := chain[i+1]
			if v, ok := inner.Val(dwarf.AttrCallFile).(int64); ok && int(v) < len(files) && files[v] != nil {
				sf.File = files[v].Name
			}
			if v, ok := inner.Val(dwarf.AttrCallLine).(int64); ok {
				sf.Line = int(v)
			}
			if v, ok := inner.Val(dwarf.AttrCallColumn).(int64); ok {
				sf.Col = int(v)
			}
		}
		out[pos] = sf
	}
	return out
}

func descend(d *dwarf.Data, r *dwarf.Reader, pc uint64, chain *[]*dwarf.Entry) {
	for {
		e, err := r.Next()
		if err != nil || e == nil || e.Tag == 0 {
			return
		}
		scope := e.Tag == dwarf.TagSubprogram || e.Tag == dwarf.TagInlinedSubroutine
		block := e.Tag == dwarf.TagLexDwarfBlock
		if (scope || block) && rangesContain(d, e, pc) {
			if scope {
				*chain = append(*chain, e)
			}
			if e.Children {
				descend(d, r, pc, chain)
			}
			return
		}
		if e.Children {
			r.SkipChildren()
		}
	}
}

func rangesContain(d *dwarf.Data, e *dwarf.Entry, pc uint64) bool {
	rs, err := d.Ranges(e)
	if err != nil {
		return false
	}
	for _, r := range rs {
		if pc >= r[0] && pc < r[1] {
			return true
		}
	}
	return false
}

func nameOf(d *dwarf.Data, e *dwarf.Entry) string {
	if n, ok := e.Val(dwarf.AttrName).(string); ok && n != "" {
		return n
	}
	for _, attr := range []dwarf.Attr{dwarf.AttrAbstractOrigin, dwarf.AttrSpecification} {
		if off, ok := e.Val(attr).(dwarf.Offset); ok {
			r := d.Reader()
			r.Seek(off)
			if ref, err := r.Next(); err == nil && ref != nil {
				if n := nameOf(d, ref); n != "" {
					return n
				}
			}
		}
	}
	return "<unknown>"
}

func normalizeFilePath(path string) string {
	if path == "" {
		return path
	}
	return filepath.Base(path)
}

type interner struct {
	list []string
	idx  map[string]int32
}

func newInterner() *interner { return &interner{idx: make(map[string]int32)} }

func (in *interner) intern(s string) int32 {
	if i, ok := in.idx[s]; ok {
		return i
	}
	i := int32(len(in.list))
	in.list = append(in.list, s)
	in.idx[s] = i
	return i
}

func (f *flatBuilder) marshal() []byte {
	le := binary.LittleEndian
	fileOffsets, fileBlob := encodeStringTable(f.files)
	fnOffsets, fnBlob := encodeStringTable(f.fns)

	size := dwHeaderSize +
		len(f.entries)*flatEntrySize +
		len(f.frames)*flatFrameSize +
		len(fileOffsets)*4 + len(fileBlob) +
		len(fnOffsets)*4 + len(fnBlob) +
		len(f.uuid)

	out := make([]byte, 0, size)
	out = append(out, twMagic[:]...)
	out = le.AppendUint32(out, twVersion)
	out = le.AppendUint64(out, f.textVMAddr)
	out = le.AppendUint64(out, 0)
	out = le.AppendUint32(out, uint32(len(f.entries)))
	out = le.AppendUint32(out, uint32(len(f.frames)))
	out = le.AppendUint32(out, uint32(len(f.files)))
	out = le.AppendUint32(out, uint32(len(f.fns)))
	out = le.AppendUint32(out, uint32(len(f.uuid)))
	out = le.AppendUint32(out, 0)
	for _, e := range f.entries {
		out = le.AppendUint64(out, e.pc)
		out = le.AppendUint64(out, e.hi)
		out = le.AppendUint32(out, e.frameStart)
		out = le.AppendUint32(out, e.frameCount)
	}
	for _, fr := range f.frames {
		out = le.AppendUint32(out, uint32(fr.fileIdx))
		out = le.AppendUint32(out, uint32(fr.line))
		out = le.AppendUint32(out, uint32(fr.col))
		out = le.AppendUint32(out, uint32(fr.fnIdx))
	}
	for _, off := range fileOffsets {
		out = le.AppendUint32(out, off)
	}
	out = append(out, fileBlob...)
	for _, off := range fnOffsets {
		out = le.AppendUint32(out, off)
	}
	out = append(out, fnBlob...)
	out = append(out, f.uuid...)
	return out
}

func encodeStringTable(strs []string) ([]uint32, []byte) {
	offsets := make([]uint32, len(strs)+1)
	var blob []byte
	for i, s := range strs {
		offsets[i] = uint32(len(blob))
		blob = append(blob, s...)
	}
	offsets[len(strs)] = uint32(len(blob))
	return offsets, blob
}
