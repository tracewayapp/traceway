package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type localStorage struct {
	basePath string
}

func NewLocalStorage(basePath string) (*localStorage, error) {
	abs, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
	}
	if err := os.MkdirAll(abs, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}
	return &localStorage{basePath: abs}, nil
}

func (l *localStorage) Write(_ context.Context, key string, data []byte) error {
	fullPath := filepath.Join(l.basePath, key)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", fullPath, err)
	}
	return nil
}

func (l *localStorage) Read(_ context.Context, key string) ([]byte, error) {
	fullPath := filepath.Join(l.basePath, key)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", key)
		}
		return nil, fmt.Errorf("failed to read file %s: %w", fullPath, err)
	}
	return data, nil
}
