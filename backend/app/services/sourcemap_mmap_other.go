//go:build !unix

package services

import "os"

func mmapFile(path string) ([]byte, func(), error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	return data, func() {}, nil
}
