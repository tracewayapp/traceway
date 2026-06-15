package sourcemap

import (
	"encoding/binary"
	"sort"
)

const twVersion = 1
const twHeaderSize = 24
const twTokenSize = 24

var twMagic = [4]byte{'T', 'W', 'S', 'M'}

func (b *builder) marshal() []byte {
	fileOffsets, fileBlob := encodeStringTable(b.files)
	fnOffsets, fnBlob := encodeStringTable(b.fns)

	size := twHeaderSize +
		len(b.tokens)*twTokenSize +
		len(fileOffsets)*4 + len(fileBlob) +
		len(fnOffsets)*4 + len(fnBlob)

	out := make([]byte, 0, size)
	out = append(out, twMagic[:]...)
	out = binary.LittleEndian.AppendUint32(out, twVersion)
	out = binary.LittleEndian.AppendUint32(out, uint32(len(b.tokens)))
	out = binary.LittleEndian.AppendUint32(out, uint32(len(b.files)))
	out = binary.LittleEndian.AppendUint32(out, uint32(len(b.fns)))
	out = binary.LittleEndian.AppendUint32(out, 0)

	for i := range b.tokens {
		t := &b.tokens[i]
		out = binary.LittleEndian.AppendUint32(out, t.genLine)
		out = binary.LittleEndian.AppendUint32(out, t.genCol)
		out = binary.LittleEndian.AppendUint32(out, t.srcLine)
		out = binary.LittleEndian.AppendUint32(out, t.srcCol)
		out = binary.LittleEndian.AppendUint32(out, uint32(t.fileIdx))
		out = binary.LittleEndian.AppendUint32(out, uint32(t.fnIdx))
	}

	for _, off := range fileOffsets {
		out = binary.LittleEndian.AppendUint32(out, off)
	}
	out = append(out, fileBlob...)
	for _, off := range fnOffsets {
		out = binary.LittleEndian.AppendUint32(out, off)
	}
	out = append(out, fnBlob...)
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

type twLayout struct {
	tokenCount   int
	fileTableOff int
	filesCount   int
	fnTableOff   int
	fnsCount     int
}

func readTWLayout(data []byte) (twLayout, bool) {
	var l twLayout
	if len(data) < twHeaderSize {
		return l, false
	}
	if [4]byte(data[:4]) != twMagic {
		return l, false
	}
	le := binary.LittleEndian
	if le.Uint32(data[4:]) != twVersion {
		return l, false
	}
	tokenCount := int64(le.Uint32(data[8:]))
	l.filesCount = int(le.Uint32(data[12:]))
	l.fnsCount = int(le.Uint32(data[16:]))
	n := int64(len(data))

	fileTableOff := int64(twHeaderSize) + tokenCount*twTokenSize
	if tokenCount < 0 || l.filesCount < 0 || l.fnsCount < 0 || fileTableOff < int64(twHeaderSize) {
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

	l.tokenCount = int(tokenCount)
	l.fileTableOff = int(fileTableOff)
	l.fnTableOff = int(fnTableOff)
	return l, true
}

func ValidTW(data []byte) bool {
	_, ok := readTWLayout(data)
	return ok
}

func LookupTW(data []byte, genLine, genCol uint32) (StackTraceFrame, bool) {
	l, ok := readTWLayout(data)
	if !ok {
		return StackTraceFrame{}, false
	}
	le := binary.LittleEndian
	tokAt := func(i int) (gl, gc uint32) {
		rec := data[twHeaderSize+i*twTokenSize:]
		return le.Uint32(rec), le.Uint32(rec[4:])
	}

	idx := sort.Search(l.tokenCount, func(i int) bool {
		gl, gc := tokAt(i)
		return gl > genLine || (gl == genLine && gc > genCol)
	})
	if idx == 0 {
		return StackTraceFrame{}, false
	}
	idx--
	for idx > 0 {
		gl, gc := tokAt(idx)
		pl, pc := tokAt(idx - 1)
		if pl != gl || pc != gc {
			break
		}
		idx--
	}

	rec := data[twHeaderSize+idx*twTokenSize:]
	tGenLine := le.Uint32(rec)
	srcLine := le.Uint32(rec[8:])
	srcCol := le.Uint32(rec[12:])
	fileIdx := int32(le.Uint32(rec[16:]))
	fnIdx := int32(le.Uint32(rec[20:]))
	if tGenLine < genLine || fileIdx < 0 {
		return StackTraceFrame{}, false
	}
	file, fok := stringAt(data, l.fileTableOff, l.filesCount, fileIdx)
	if !fok {
		return StackTraceFrame{}, false
	}
	frame := StackTraceFrame{File: file, Line: srcLine, Col: srcCol}
	if fnIdx >= 0 {
		if fn, ok := stringAt(data, l.fnTableOff, l.fnsCount, fnIdx); ok {
			frame.Fn = fn
		}
	}
	return frame, true
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
