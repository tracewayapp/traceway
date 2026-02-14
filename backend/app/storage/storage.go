package storage

import (
	"context"
	"fmt"
	"os"
)

type Storage interface {
	Write(ctx context.Context, key string, data []byte) error
	Read(ctx context.Context, key string) ([]byte, error)
}

var Store Storage

func Init() error {
	storageType := os.Getenv("STORAGE_TYPE")
	if storageType == "" {
		storageType = "local"
	}

	switch storageType {
	case "local":
		path := os.Getenv("STORAGE_PATH")
		if path == "" {
			path = "./storage"
		}
		s, err := NewLocalStorage(path)
		if err != nil {
			return fmt.Errorf("failed to create local storage: %w", err)
		}
		Store = s
	case "s3":
		bucket := os.Getenv("S3_BUCKET")
		if bucket == "" {
			return fmt.Errorf("S3_BUCKET is required when STORAGE_TYPE=s3")
		}
		region := os.Getenv("S3_REGION")
		if region == "" {
			return fmt.Errorf("S3_REGION is required when STORAGE_TYPE=s3")
		}
		s, err := NewS3Storage(bucket, region, os.Getenv("S3_ACCESS_KEY"), os.Getenv("S3_SECRET_KEY"), os.Getenv("S3_ENDPOINT"))
		if err != nil {
			return fmt.Errorf("failed to create S3 storage: %w", err)
		}
		Store = s
	default:
		return fmt.Errorf("unknown STORAGE_TYPE: %s", storageType)
	}

	return nil
}
