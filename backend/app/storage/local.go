package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type localStorage struct {
	basePath string
}

var ErrInvalidKey = errors.New("storage: key escapes base path")

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

func (l *localStorage) resolve(key string) (string, error) {
	rel := filepath.FromSlash(key)
	if !filepath.IsLocal(rel) {
		return "", fmt.Errorf("%w: %q", ErrInvalidKey, key)
	}
	return filepath.Join(l.basePath, rel), nil
}

func (l *localStorage) Write(_ context.Context, key string, data []byte) error {
	fullPath, err := l.resolve(key)
	if err != nil {
		return err
	}
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", fullPath, err)
	}
	return nil
}

func (l *localStorage) Delete(_ context.Context, key string) error {
	fullPath, err := l.resolve(key)
	if err != nil {
		return err
	}
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file %s: %w", fullPath, err)
	}
	return nil
}

func (l *localStorage) Read(_ context.Context, key string) ([]byte, error) {
	fullPath, err := l.resolve(key)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to read file %s: %w", fullPath, err)
	}
	return data, nil
}
