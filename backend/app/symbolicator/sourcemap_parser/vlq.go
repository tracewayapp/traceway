package sourcemap_parser

import "fmt"

const vlqAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

var vlqLookup [128]int8

func init() {
	for i := range vlqLookup {
		vlqLookup[i] = -1
	}
	for i := 0; i < len(vlqAlphabet); i++ {
		vlqLookup[vlqAlphabet[i]] = int8(i)
	}
}

func decodeVLQ(seg string, dst []int64) ([]int64, error) {
	var cur uint64
	var shift uint
	for i := 0; i < len(seg); i++ {
		c := seg[i]
		if c >= 128 || vlqLookup[c] < 0 {
			return dst, fmt.Errorf("vlq: invalid character %q in segment %q", c, seg)
		}
		d := uint64(vlqLookup[c])
		if shift > 58 {
			return dst, fmt.Errorf("vlq: segment %q overflows", seg)
		}
		cur |= (d & 31) << shift
		if d&32 != 0 {
			shift += 5
		} else {
			val := int64(cur >> 1)
			if cur&1 != 0 {
				val = -val
			}
			dst = append(dst, val)
			cur = 0
			shift = 0
		}
	}
	if shift != 0 {
		return dst, fmt.Errorf("vlq: truncated segment %q", seg)
	}
	return dst, nil
}
