package android

import (
	"encoding/binary"
	"sort"
	"strings"
)

var r8Magic = [4]byte{'T', 'W', 'R', '8'}

const (
	r8Version    = 1
	r8HeaderSize = 20
	r8ClassSize  = 20
	r8MemberSize = 36

	r8FlagHasObf      = 1 << 0
	r8FlagHasOrig     = 1 << 1
	r8FlagSynthesized = 1 << 2
)

func BuildR8Flat(mappingText string) []byte {
	return ParseMapping(mappingText).marshalR8()
}

type r8ClassRec struct {
	obfIdx, origIdx, fileIdx int32
	memberStart, memberCount uint32
}

type r8MemberRec struct {
	obfMethodIdx, origIdx, methodIdx, fileIdx int32
	obfStart, obfEnd, origStart, origEnd      int32
	flags                                     uint32
}

func (m *Mapping) marshalR8() []byte {
	strs := newInterner()
	var classes []r8ClassRec
	var members []r8MemberRec

	obfNames := make([]string, 0, len(m.classOrig))
	for obf := range m.classOrig {
		obfNames = append(obfNames, obf)
	}
	sort.Strings(obfNames)

	for _, obf := range obfNames {
		orig := m.classOrig[obf]

		type flatMember struct {
			obfMethod string
			e         methodMapping
		}
		var flat []flatMember
		for obfMethod, entries := range m.methods[obf] {
			for _, e := range entries {
				flat = append(flat, flatMember{obfMethod, e})
			}
		}
		sort.SliceStable(flat, func(i, j int) bool { return flat[i].obfMethod < flat[j].obfMethod })

		start := uint32(len(members))
		for _, fm := range flat {
			e := fm.e
			var flags uint32
			if e.hasObf {
				flags |= r8FlagHasObf
			}
			if e.hasOrig {
				flags |= r8FlagHasOrig
			}
			if e.synthesized {
				flags |= r8FlagSynthesized
			}
			members = append(members, r8MemberRec{
				obfMethodIdx: strs.intern(fm.obfMethod),
				origIdx:      strs.intern(e.origClass),
				methodIdx:    strs.intern(e.method),
				fileIdx:      strs.intern(m.fileFor(e.origClass, "")),
				obfStart:     int32(e.obfStart),
				obfEnd:       int32(e.obfEnd),
				origStart:    int32(e.origStart),
				origEnd:      int32(e.origEnd),
				flags:        flags,
			})
		}
		classes = append(classes, r8ClassRec{
			obfIdx:      strs.intern(obf),
			origIdx:     strs.intern(orig),
			fileIdx:     strs.intern(m.fileFor(orig, "")),
			memberStart: start,
			memberCount: uint32(len(members)) - start,
		})
	}

	le := binary.LittleEndian
	strOffsets, strBlob := encodeStringTable(strs.list)

	size := r8HeaderSize + len(classes)*r8ClassSize + len(members)*r8MemberSize + len(strOffsets)*4 + len(strBlob)
	out := make([]byte, 0, size)
	out = append(out, r8Magic[:]...)
	out = le.AppendUint32(out, r8Version)
	out = le.AppendUint32(out, uint32(len(classes)))
	out = le.AppendUint32(out, uint32(len(members)))
	out = le.AppendUint32(out, uint32(len(strs.list)))
	for _, c := range classes {
		out = le.AppendUint32(out, uint32(c.obfIdx))
		out = le.AppendUint32(out, uint32(c.origIdx))
		out = le.AppendUint32(out, uint32(c.fileIdx))
		out = le.AppendUint32(out, c.memberStart)
		out = le.AppendUint32(out, c.memberCount)
	}
	for _, mr := range members {
		out = le.AppendUint32(out, uint32(mr.obfMethodIdx))
		out = le.AppendUint32(out, uint32(mr.origIdx))
		out = le.AppendUint32(out, uint32(mr.methodIdx))
		out = le.AppendUint32(out, uint32(mr.fileIdx))
		out = le.AppendUint32(out, uint32(mr.obfStart))
		out = le.AppendUint32(out, uint32(mr.obfEnd))
		out = le.AppendUint32(out, uint32(mr.origStart))
		out = le.AppendUint32(out, uint32(mr.origEnd))
		out = le.AppendUint32(out, mr.flags)
	}
	for _, off := range strOffsets {
		out = le.AppendUint32(out, off)
	}
	out = append(out, strBlob...)
	return out
}

type r8Layout struct {
	classCount  int
	memberOff   int
	memberCount int
	strTableOff int
	strCount    int
}

func readR8Layout(data []byte) (r8Layout, bool) {
	var l r8Layout
	if len(data) < r8HeaderSize {
		return l, false
	}
	le := binary.LittleEndian
	if data[0] != r8Magic[0] || data[1] != r8Magic[1] || data[2] != r8Magic[2] || data[3] != r8Magic[3] {
		return l, false
	}
	if le.Uint32(data[4:]) != r8Version {
		return l, false
	}
	classCount := int64(le.Uint32(data[8:]))
	memberCount := int64(le.Uint32(data[12:]))
	strCount := int64(le.Uint32(data[16:]))
	n := int64(len(data))

	classOff := int64(r8HeaderSize)
	memberOff := classOff + classCount*r8ClassSize
	strTableOff := memberOff + memberCount*r8MemberSize
	if classCount < 0 || memberCount < 0 || strCount < 0 || memberOff < classOff || strTableOff < memberOff {
		return l, false
	}
	strBlobOff := strTableOff + (strCount+1)*4
	if strBlobOff > n {
		return l, false
	}
	blobLen := int64(le.Uint32(data[strTableOff+strCount*4:]))
	if strBlobOff+blobLen > n {
		return l, false
	}

	l.classCount = int(classCount)
	l.memberOff = int(memberOff)
	l.memberCount = int(memberCount)
	l.strTableOff = int(strTableOff)
	l.strCount = int(strCount)
	return l, true
}

func ValidR8Flat(data []byte) bool {
	_, ok := readR8Layout(data)
	return ok
}

func RetraceFlat(data []byte, text string) string {
	r, ok := newR8Reader(data)
	if !ok {
		return text
	}
	return retraceWith(r, text)
}

type r8Reader struct {
	data []byte
	l    r8Layout
}

func newR8Reader(data []byte) (*r8Reader, bool) {
	l, ok := readR8Layout(data)
	if !ok {
		return nil, false
	}
	return &r8Reader{data: data, l: l}, true
}

func (r *r8Reader) str(idx int32) string {
	s, _ := stringAt(r.data, r.l.strTableOff, r.l.strCount, idx)
	return s
}

func (r *r8Reader) classObfIdx(i int) int32 {
	return int32(binary.LittleEndian.Uint32(r.data[r8HeaderSize+i*r8ClassSize:]))
}

func (r *r8Reader) classRec(i int) (origIdx, fileIdx int32, memberStart, memberCount uint32) {
	rec := r.data[r8HeaderSize+i*r8ClassSize:]
	le := binary.LittleEndian
	return int32(le.Uint32(rec[4:])), int32(le.Uint32(rec[8:])), le.Uint32(rec[12:]), le.Uint32(rec[16:])
}

func (r *r8Reader) findClass(obf string) (int, bool) {
	lo, hi := 0, r.l.classCount
	for lo < hi {
		mid := (lo + hi) / 2
		switch strings.Compare(obf, r.str(r.classObfIdx(mid))) {
		case 0:
			return mid, true
		case -1:
			hi = mid
		default:
			lo = mid + 1
		}
	}
	return 0, false
}

func (r *r8Reader) classInfo(obfClass string) (string, string, bool) {
	i, ok := r.findClass(obfClass)
	if !ok {
		return "", "", false
	}
	origIdx, fileIdx, _, _ := r.classRec(i)
	return r.str(origIdx), r.str(fileIdx), true
}

func (r *r8Reader) memberObfMethodIdx(i int) int32 {
	return int32(binary.LittleEndian.Uint32(r.data[r.l.memberOff+i*r8MemberSize:]))
}

func (r *r8Reader) memberRec(i int) r8MemberRec {
	rec := r.data[r.l.memberOff+i*r8MemberSize:]
	le := binary.LittleEndian
	return r8MemberRec{
		obfMethodIdx: int32(le.Uint32(rec)),
		origIdx:      int32(le.Uint32(rec[4:])),
		methodIdx:    int32(le.Uint32(rec[8:])),
		fileIdx:      int32(le.Uint32(rec[12:])),
		obfStart:     int32(le.Uint32(rec[16:])),
		obfEnd:       int32(le.Uint32(rec[20:])),
		origStart:    int32(le.Uint32(rec[24:])),
		origEnd:      int32(le.Uint32(rec[28:])),
		flags:        le.Uint32(rec[32:]),
	}
}

func (r *r8Reader) matched(obfClass, obfMethod string, obfLine int) []memberFrame {
	ci, ok := r.findClass(obfClass)
	if !ok {
		return nil
	}
	_, _, memberStart, memberCount := r.classRec(ci)
	start, end := int(memberStart), int(memberStart)+int(memberCount)
	if end > r.l.memberCount {
		return nil
	}

	lo, hi := start, end
	for lo < hi {
		mid := (lo + hi) / 2
		if strings.Compare(r.str(r.memberObfMethodIdx(mid)), obfMethod) < 0 {
			lo = mid + 1
		} else {
			hi = mid
		}
	}

	var out []memberFrame
	for i := lo; i < end; i++ {
		mr := r.memberRec(i)
		if r.str(mr.obfMethodIdx) != obfMethod {
			break
		}
		if mr.flags&r8FlagSynthesized != 0 {
			continue
		}
		if mr.flags&r8FlagHasObf != 0 && !(obfLine >= int(mr.obfStart) && obfLine <= int(mr.obfEnd)) {
			continue
		}
		out = append(out, memberFrame{
			cls:    r.str(mr.origIdx),
			method: r.str(mr.methodIdx),
			file:   r.str(mr.fileIdx),
			line:   computeLineFlat(mr, obfLine),
		})
	}
	return out
}

func computeLineFlat(mr r8MemberRec, obfLine int) int {
	if mr.flags&r8FlagHasOrig == 0 {
		return obfLine
	}
	if mr.flags&r8FlagHasObf != 0 && mr.obfEnd > mr.obfStart && mr.origEnd > mr.origStart {
		return int(mr.origStart) + (obfLine - int(mr.obfStart))
	}
	return int(mr.origStart)
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
