//go:build windows

package workspace

// The harness runs on Linux (embedded mode needs bubblewrap); on Windows the
// mirror is not locked, which only matters for two concurrent attempts on
// one repository.
func lockFile(string) (func(), error) {
	return func() {}, nil
}
