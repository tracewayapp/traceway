// Package scenarios holds one Go seed function per case, registered by the
// name a case's `seed:` field uses (perf/db-n-plus-1, ...). The per-suite
// subpackages register themselves; import scenarios/all to load every one.
package scenarios

import (
	"fmt"
	"sort"
	"sync"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

// Func seeds one scenario.
type Func func(s *seed.Scenario) error

var (
	mu       sync.RWMutex
	registry = map[string]Func{}
)

// Register adds a seed function. Duplicate names panic at init.
func Register(name string, f Func) {
	mu.Lock()
	defer mu.Unlock()
	if _, dup := registry[name]; dup {
		panic("scenarios: duplicate seed " + name)
	}
	registry[name] = f
}

// Get returns a registered seed function.
func Get(name string) (Func, error) {
	mu.RLock()
	defer mu.RUnlock()
	f, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("no seed function registered as %q", name)
	}
	return f, nil
}

// Names lists every registered seed.
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Run seeds s with the named function, converting panics from the DSL (which
// panics on API errors to keep scenario code flat) into errors.
func Run(name string, s *seed.Scenario) (err error) {
	f, err := Get(name)
	if err != nil {
		return err
	}
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = fmt.Errorf("seed %s: %w", name, e)
				return
			}
			err = fmt.Errorf("seed %s: %v", name, r)
		}
	}()
	if err := f(s); err != nil {
		return fmt.Errorf("seed %s: %w", name, err)
	}
	return s.Flush()
}
