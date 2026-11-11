//go:build !unix

package monitoring

import "time"

func ReadProcessCPUTime() (time.Duration, bool) {
	return 0, false
}
