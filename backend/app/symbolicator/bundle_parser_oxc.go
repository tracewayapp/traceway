//go:build oxc && cgo

package symbolicator

/*
#cgo CFLAGS: -I${SRCDIR}/oxc-shim/include
#cgo LDFLAGS: ${SRCDIR}/oxc-shim/target/release/liboxc_shim.a
#cgo linux LDFLAGS: -lm -ldl -lpthread
#include <oxc_shim.h>
*/
import "C"

import (
	"fmt"
	"unsafe"
)

func init() {
	bundleParsers["oxc"] = parseFunctionScopesOxc
}

var oxcParseErrors = map[int32]string{
	1: "bad arguments",
	2: "source is not valid utf-8",
	3: "parse failed",
	4: "parser panicked",
}

func parseFunctionScopesOxc(bundle []byte) (*functionScopes, error) {
	if len(bundle) == 0 {
		return scopesFromRaw("", nil), nil
	}

	var out *C.uint32_t
	var outLen C.size_t
	rc := int32(C.oxc_parse_scopes(
		(*C.char)(unsafe.Pointer(&bundle[0])),
		C.size_t(len(bundle)),
		&out,
		&outLen,
	))
	if rc != 0 {
		msg, ok := oxcParseErrors[rc]
		if !ok {
			msg = fmt.Sprintf("unknown error code %d", rc)
		}
		return nil, fmt.Errorf("oxc: %s", msg)
	}
	defer C.oxc_free_scopes(out, outLen)

	n := int(outLen)
	raw := make([]rawScope, 0, n/3)
	if n > 0 {
		vals := unsafe.Slice((*uint32)(unsafe.Pointer(out)), n)
		for i := 0; i+2 < n; i += 3 {
			raw = append(raw, rawScope{start: vals[i], end: vals[i+1], namePos: vals[i+2]})
		}
	}
	return scopesFromRaw(string(bundle), raw), nil
}
