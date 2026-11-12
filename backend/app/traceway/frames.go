package traceway

import (
	"reflect"
	"runtime"
	"strings"
	"sync"
)

const ginNextFunction = "github.com/gin-gonic/gin.(*Context).Next"

// Traceway hashes an exception's frames, file:line included, so a wrapper
// frame of the tracing itself would regroup every issue whenever otelgin is
// upgraded or a wrapper is refactored. The wrappers are left out of every
// stack captured here instead, and the stack reads as if the request or task
// had not been traced.
var hiddenFrames struct {
	sync.RWMutex
	functions []string
	packages  []string
}

func init() {
	HideFrames(RunTask)
}

// HideFrames leaves each function, the closures it defines, and the
// gin.(*Context).Next it called out of every stack this package captures.
func HideFrames(fns ...any) {
	hiddenFrames.Lock()
	defer hiddenFrames.Unlock()
	for _, fn := range fns {
		hiddenFrames.functions = append(hiddenFrames.functions, functionName(fn))
	}
}

// HidePackageFrames is HideFrames for every function of the package fn
// belongs to.
func HidePackageFrames(fn any) {
	name := functionName(fn)
	pkgEnd := strings.LastIndexByte(name, '/') + 1
	pkgEnd += strings.IndexByte(name[pkgEnd:], '.') + 1
	hiddenFrames.Lock()
	defer hiddenFrames.Unlock()
	hiddenFrames.packages = append(hiddenFrames.packages, name[:pkgEnd])
}

func isHiddenFrame(function string) bool {
	hiddenFrames.RLock()
	defer hiddenFrames.RUnlock()
	for _, name := range hiddenFrames.functions {
		if function == name || strings.HasPrefix(function, name+".") {
			return true
		}
	}
	for _, pkg := range hiddenFrames.packages {
		if strings.HasPrefix(function, pkg) {
			return true
		}
	}
	return false
}

func functionName(fn any) string {
	return runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
}
