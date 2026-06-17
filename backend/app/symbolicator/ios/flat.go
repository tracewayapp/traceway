package ios

import (
	"encoding/binary"
	"fmt"
	"sort"
)

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

var twMagic = [4]byte{'T', 'W', 'I', 'O'}

const (
	twVersion     = 1
	flatEntrySize = 24
	flatFrameSize = 16
	dwHeaderSize  = 48
)

type flatLayout struct {
	base         uint64
	entryCount   int
	framesOff    int
	frameCount   int
	fileTableOff int
	filesCount   int
	fnTableOff   int
	fnsCount     int
}

func readFlatLayout(data []byte) (flatLayout, bool) {
	var l flatLayout
	if len(data) < dwHeaderSize {
		return l, false
	}
	le := binary.LittleEndian
	if data[0] != twMagic[0] || data[1] != twMagic[1] || data[2] != twMagic[2] || data[3] != twMagic[3] {
		return l, false
	}
	if le.Uint32(data[4:]) != twVersion {
		return l, false
	}
	l.base = le.Uint64(data[8:])
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

func LookupFlat(data []byte, off uint64) []SymFrame {
	l, ok := readFlatLayout(data)
	if !ok {
		return nil
	}
	pc := l.base + off

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
