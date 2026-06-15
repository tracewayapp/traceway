package dart

import (
	"bytes"
	"debug/dwarf"
	"debug/elf"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
)

const maxInlineFrames = 64

type SymFrame struct {
	Function string
	File     string
	Line     int
	Col      int
}

func (s SymFrame) Location() string {
	if s.Line <= 0 {
		return s.File
	}
	if s.Col <= 0 {
		return fmt.Sprintf("%s:%d", s.File, s.Line)
	}
	return fmt.Sprintf("%s:%d:%d", s.File, s.Line, s.Col)
}

func dwarfFromELF(elfBytes []byte) (*dwarf.Data, map[string]uint64, string, error) {
	f, err := elf.NewFile(bytes.NewReader(elfBytes))
	if err != nil {
		return nil, nil, "", fmt.Errorf("not an ELF file: %w", err)
	}
	data, err := f.DWARF()
	if err != nil {
		return nil, nil, "", fmt.Errorf("reading DWARF: %w", err)
	}
	syms, err := f.Symbols()
	if err != nil {
		return nil, nil, "", fmt.Errorf("reading symbols: %w", err)
	}
	base := map[string]uint64{}
	for _, s := range syms {
		switch s.Name {
		case isolateInstructionsSymbol:
			base["isolate"] = s.Value
		case vmInstructionsSymbol:
			base["vm"] = s.Value
		}
	}
	return data, base, readBuildID(f), nil
}

func ReadBuildID(elfBytes []byte) (string, error) {
	_, _, buildID, err := dwarfFromELF(elfBytes)
	return buildID, err
}

const (
	isolateInstructionsSymbol = "_kDartIsolateSnapshotInstructions"
	vmInstructionsSymbol      = "_kDartVmSnapshotInstructions"
)

func InstructionSymbol(section string) string {
	if section == "vm" {
		return vmInstructionsSymbol
	}
	return isolateInstructionsSymbol
}

type flatBuilder struct {
	isolateBase uint64
	vmBase      uint64
	buildID     string

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

const (
	flatEntrySize = 24
	flatFrameSize = 16
	dwHeaderSize  = 48
)

func BuildFlat(elfBytes []byte) ([]byte, error) {
	data, base, buildID, err := dwarfFromELF(elfBytes)
	if err != nil {
		return nil, err
	}
	return flatten(data, base, buildID).marshal(), nil
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

func flatten(data *dwarf.Data, base map[string]uint64, buildID string) *flatBuilder {
	f := &flatBuilder{
		isolateBase: base["isolate"],
		vmBase:      base["vm"],
		buildID:     buildID,
	}
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
					fileIdx: files.intern(sf.File),
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

func readBuildID(f *elf.File) string {
	sec := f.Section(".note.gnu.build-id")
	if sec == nil {
		return ""
	}
	data, err := sec.Data()
	if err != nil || len(data) < 12 {
		return ""
	}
	nameSize := f.ByteOrder.Uint32(data[0:4])
	descSize := f.ByteOrder.Uint32(data[4:8])
	off := 12 + ((nameSize + 3) &^ 3)
	if int(off+descSize) > len(data) {
		return ""
	}
	return fmt.Sprintf("%x", data[off:off+descSize])
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

var dwMagic = [4]byte{'T', 'W', 'D', 'F'}

const dwVersion = 1

var ErrInvalidFlat = errors.New("dart: invalid flat data")

func (f *flatBuilder) marshal() []byte {
	le := binary.LittleEndian
	fileOffsets, fileBlob := encodeStringTable(f.files)
	fnOffsets, fnBlob := encodeStringTable(f.fns)

	size := dwHeaderSize +
		len(f.entries)*flatEntrySize +
		len(f.frames)*flatFrameSize +
		len(fileOffsets)*4 + len(fileBlob) +
		len(fnOffsets)*4 + len(fnBlob) +
		len(f.buildID)

	out := make([]byte, 0, size)
	out = append(out, dwMagic[:]...)
	out = le.AppendUint32(out, dwVersion)
	out = le.AppendUint64(out, f.isolateBase)
	out = le.AppendUint64(out, f.vmBase)
	out = le.AppendUint32(out, uint32(len(f.entries)))
	out = le.AppendUint32(out, uint32(len(f.frames)))
	out = le.AppendUint32(out, uint32(len(f.files)))
	out = le.AppendUint32(out, uint32(len(f.fns)))
	out = le.AppendUint32(out, uint32(len(f.buildID)))
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
	out = append(out, f.buildID...)
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

type flatLayout struct {
	isolateBase, vmBase uint64
	entryCount          int
	framesOff           int
	frameCount          int
	fileTableOff        int
	filesCount          int
	fnTableOff          int
	fnsCount            int
}

func readFlatLayout(data []byte) (flatLayout, bool) {
	var l flatLayout
	if len(data) < dwHeaderSize {
		return l, false
	}
	le := binary.LittleEndian
	if data[0] != dwMagic[0] || data[1] != dwMagic[1] || data[2] != dwMagic[2] || data[3] != dwMagic[3] {
		return l, false
	}
	if le.Uint32(data[4:]) != dwVersion {
		return l, false
	}
	l.isolateBase = le.Uint64(data[8:])
	l.vmBase = le.Uint64(data[16:])
	entryCount := int64(le.Uint32(data[24:]))
	frameCount := int64(le.Uint32(data[28:]))
	l.filesCount = int(le.Uint32(data[32:]))
	l.fnsCount = int(le.Uint32(data[36:]))
	n := int64(len(data))

	entriesOff := int64(dwHeaderSize)
	framesOff := entriesOff + entryCount*flatEntrySize
	fileTableOff := framesOff + frameCount*flatFrameSize
	if entryCount < 0 || frameCount < 0 || l.filesCount < 0 || l.fnsCount < 0 ||
		framesOff < entriesOff || fileTableOff < framesOff {
		return l, false
	}
	fileBlobOff := fileTableOff + int64(l.filesCount+1)*4
	if fileBlobOff > n {
		return l, false
	}
	fileBlobLen := int64(le.Uint32(data[fileTableOff+int64(l.filesCount)*4:]))
	fnTableOff := fileBlobOff + fileBlobLen
	fnBlobOff := fnTableOff + int64(l.fnsCount+1)*4
	if fnTableOff < fileBlobOff || fnBlobOff > n {
		return l, false
	}
	fnBlobLen := int64(le.Uint32(data[fnTableOff+int64(l.fnsCount)*4:]))
	if fnBlobOff+fnBlobLen > n {
		return l, false
	}

	l.entryCount = int(entryCount)
	l.frameCount = int(frameCount)
	l.framesOff = int(framesOff)
	l.fileTableOff = int(fileTableOff)
	l.fnTableOff = int(fnTableOff)
	return l, true
}

func ValidFlat(data []byte) bool {
	_, ok := readFlatLayout(data)
	return ok
}

func LookupFlat(data []byte, frame StackFrame) []SymFrame {
	l, ok := readFlatLayout(data)
	if !ok {
		return nil
	}
	base := l.isolateBase
	if frame.Section == "vm" {
		base = l.vmBase
	}
	if base == 0 {
		return nil
	}
	pc := base + frame.Offset

	le := binary.LittleEndian
	entryAt := func(i int) (pcLo, hi uint64, frameStart, frameCount uint32) {
		rec := data[dwHeaderSize+i*flatEntrySize:]
		return le.Uint64(rec), le.Uint64(rec[8:]), le.Uint32(rec[16:]), le.Uint32(rec[20:])
	}

	i := sort.Search(l.entryCount, func(i int) bool {
		pcLo, _, _, _ := entryAt(i)
		return pcLo > pc
	}) - 1
	if i < 0 {
		return nil
	}
	ePc, eHi, frameStart, frameCount := entryAt(i)
	if pc < ePc || pc >= eHi {
		return nil
	}

	if uint64(frameStart)+uint64(frameCount) > uint64(l.frameCount) {
		return nil
	}
	out := make([]SymFrame, 0, frameCount)
	for k := uint32(0); k < frameCount; k++ {
		rec := data[l.framesOff+int(frameStart+k)*flatFrameSize:]
		fileIdx := int32(le.Uint32(rec))
		line := int32(le.Uint32(rec[4:]))
		col := int32(le.Uint32(rec[8:]))
		fnIdx := int32(le.Uint32(rec[12:]))
		file, _ := stringAt(data, l.fileTableOff, l.filesCount, fileIdx)
		fn, _ := stringAt(data, l.fnTableOff, l.fnsCount, fnIdx)
		out = append(out, SymFrame{Function: fn, File: file, Line: int(line), Col: int(col)})
	}
	return out
}

func stringAt(data []byte, tableOff, count int, idx int32) (string, bool) {
	if idx < 0 || int(idx) >= count {
		return "", false
	}
	le := binary.LittleEndian
	blobStart := tableOff + (count+1)*4
	o0 := int(le.Uint32(data[tableOff+int(idx)*4:]))
	o1 := int(le.Uint32(data[tableOff+(int(idx)+1)*4:]))
	if o0 > o1 || blobStart+o1 > len(data) {
		return "", false
	}
	return string(data[blobStart+o0 : blobStart+o1]), true
}
