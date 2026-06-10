//go:build !unix

package twcache

import (
	"errors"
	"os"
)

func mmapFile(path string) ([]byte, func(), error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	return data, func() {}, nil
}

func DiskCapacityBytes(dir string) (int64, error) {
	return 0, errors.New("disk capacity detection is not supported on this platform")
}
