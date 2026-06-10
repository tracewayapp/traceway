package symbolicator

import (
	"encoding/binary"
	"errors"
	"runtime"
	"unsafe"
)

const twVersion = 1
const twHeaderSize = 24
const twTokenSize = 24

var twMagic = [4]byte{'T', 'W', 'S', 'M'}

var ErrInvalidTW = errors.New("symbolicator: invalid tw data")

var hostLittleEndian = func() bool {
	var x uint16 = 1
	return *(*byte)(unsafe.Pointer(&x)) == 1
}()

func (r *Resolver) MarshalTW() []byte {
	defer runtime.KeepAlive(r)
	fileOffsets, fileBlob := encodeStringTable(r.files)
	fnOffsets, fnBlob := encodeStringTable(r.fns)

	size := twHeaderSize +
		len(r.tokens)*twTokenSize +
		len(fileOffsets)*4 + len(fileBlob) +
		len(fnOffsets)*4 + len(fnBlob)

	out := make([]byte, 0, size)
	out = append(out, twMagic[:]...)
	out = binary.LittleEndian.AppendUint32(out, twVersion)
	out = binary.LittleEndian.AppendUint32(out, uint32(len(r.tokens)))
	out = binary.LittleEndian.AppendUint32(out, uint32(len(r.files)))
	out = binary.LittleEndian.AppendUint32(out, uint32(len(r.fns)))
	out = binary.LittleEndian.AppendUint32(out, 0)

	for i := range r.tokens {
		t := &r.tokens[i]
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

func OpenTW(data []byte) (*Resolver, error) {
	if len(data) < twHeaderSize || [4]byte(data[:4]) != twMagic {
		return nil, ErrInvalidTW
	}
	if binary.LittleEndian.Uint32(data[4:]) != twVersion {
		return nil, ErrInvalidTW
	}
	tokenCount := int(binary.LittleEndian.Uint32(data[8:]))
	filesCount := int(binary.LittleEndian.Uint32(data[12:]))
	fnsCount := int(binary.LittleEndian.Uint32(data[16:]))

	pos := uint64(twHeaderSize)
	tokensOff := pos
	pos += uint64(tokenCount) * twTokenSize
	if pos > uint64(len(data)) {
		return nil, ErrInvalidTW
	}

	files, pos, err := decodeStringTable(data, pos, filesCount)
	if err != nil {
		return nil, err
	}
	fns, pos, err := decodeStringTable(data, pos, fnsCount)
	if err != nil {
		return nil, err
	}
	if pos != uint64(len(data)) {
		return nil, ErrInvalidTW
	}

	r := &Resolver{files: files, fns: fns}
	if tokenCount > 0 {
		tokenBytes := data[tokensOff : tokensOff+uint64(tokenCount)*twTokenSize]
		if hostLittleEndian &&
			unsafe.Sizeof(resolvedToken{}) == twTokenSize &&
			uintptr(unsafe.Pointer(&tokenBytes[0]))%unsafe.Alignof(resolvedToken{}) == 0 {
			r.tokens = unsafe.Slice((*resolvedToken)(unsafe.Pointer(&tokenBytes[0])), tokenCount)
		} else {
			r.tokens = decodeTokens(tokenBytes, tokenCount)
		}
	}
	for i := range r.tokens {
		t := &r.tokens[i]
		if int(t.fileIdx) >= len(files) || int(t.fnIdx) >= len(fns) {
			return nil, ErrInvalidTW
		}
	}
	return r, nil
}

func decodeTokens(b []byte, count int) []resolvedToken {
	tokens := make([]resolvedToken, count)
	for i := range tokens {
		rec := b[i*twTokenSize:]
		tokens[i] = resolvedToken{
			genLine: binary.LittleEndian.Uint32(rec),
			genCol:  binary.LittleEndian.Uint32(rec[4:]),
			srcLine: binary.LittleEndian.Uint32(rec[8:]),
			srcCol:  binary.LittleEndian.Uint32(rec[12:]),
			fileIdx: int32(binary.LittleEndian.Uint32(rec[16:])),
			fnIdx:   int32(binary.LittleEndian.Uint32(rec[20:])),
		}
	}
	return tokens
}

func decodeStringTable(data []byte, pos uint64, count int) ([]string, uint64, error) {
	offsetsEnd := pos + uint64(count+1)*4
	if count < 0 || offsetsEnd > uint64(len(data)) {
		return nil, 0, ErrInvalidTW
	}
	offsets := make([]uint32, count+1)
	for i := range offsets {
		offsets[i] = binary.LittleEndian.Uint32(data[pos+uint64(i)*4:])
		if i > 0 && offsets[i-1] > offsets[i] {
			return nil, 0, ErrInvalidTW
		}
	}
	blobStart := offsetsEnd
	blobEnd := blobStart + uint64(offsets[count])
	if blobEnd > uint64(len(data)) {
		return nil, 0, ErrInvalidTW
	}
	strs := make([]string, count)
	for i := range count {
		strs[i] = string(data[blobStart+uint64(offsets[i]) : blobStart+uint64(offsets[i+1])])
	}
	return strs, blobEnd, nil
}
