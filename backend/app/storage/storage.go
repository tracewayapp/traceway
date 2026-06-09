package storage

import (
	"context"
	"errors"
	"fmt"
	"github.com/tracewayapp/traceway/backend/app/config"
)

var ErrNotFound = errors.New("storage: object not found")

type Storage interface {
	Write(ctx context.Context, key string, data []byte) error
	Read(ctx context.Context, key string) ([]byte, error)
}

var Store Storage

func Init() error {
	cfg := config.Config

	storageType := cfg.StorageType
	if storageType == "" {
		storageType = "local"
	}

	switch storageType {
	case "local":
		path := cfg.StoragePath
		if path == "" {
			path = "./storage"
		}
		s, err := NewLocalStorage(path)
		if err != nil {
			return fmt.Errorf("failed to create local storage: %w", err)
		}
		Store = s
	case "s3":
		bucket := cfg.S3Bucket
		if bucket == "" {
			return fmt.Errorf("S3_BUCKET is required when STORAGE_TYPE=s3")
		}
		region := cfg.S3Region
		if region == "" {
			return fmt.Errorf("S3_REGION is required when STORAGE_TYPE=s3")
		}
		s, err := NewS3Storage(bucket, region, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Endpoint)
		if err != nil {
			return fmt.Errorf("failed to create S3 storage: %w", err)
		}
		Store = s
	default:
		return fmt.Errorf("unknown STORAGE_TYPE: %s", storageType)
	}

	return nil
}
