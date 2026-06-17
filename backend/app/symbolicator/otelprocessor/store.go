package otelprocessor

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"cloud.google.com/go/storage"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/tracewayapp/traceway/backend/app/symbolicator/dart"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/ios"
)

const (
	fileStoreKey = "file_store"
	s3StoreKey   = "s3_store"
	gcsStoreKey  = "gcs_store"
)

var errObjectNotFound = errors.New("object not found")
var sourceMappingURLRe = regexp.MustCompile(`//[#@]\s*sourceMappingURL\s*=\s*(\S+)`)

type objectStore interface {
	fetch(ctx context.Context, key string) ([]byte, error)
}

type artifactStore struct {
	store  objectStore
	prefix string
}

func newStore(cfg *Config) (*artifactStore, error) {
	switch cfg.SourceMapStoreKey {
	case fileStoreKey:
		return &artifactStore{store: &fileStore{root: cfg.LocalSourceMaps.Path}}, nil
	case s3StoreKey:
		awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(cfg.S3SourceMaps.Region))
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config: %w", err)
		}
		return &artifactStore{
			store:  &s3Store{client: s3.NewFromConfig(awsCfg), bucket: cfg.S3SourceMaps.Bucket},
			prefix: cfg.S3SourceMaps.Prefix,
		}, nil
	case gcsStoreKey:
		client, err := storage.NewClient(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to create GCS client: %w", err)
		}
		return &artifactStore{
			store:  &gcsStore{bucket: client.Bucket(cfg.GCSSourceMaps.Bucket)},
			prefix: cfg.GCSSourceMaps.Prefix,
		}, nil
	}
	return nil, fmt.Errorf("unknown source_map_store %q", cfg.SourceMapStoreKey)
}

func (a *artifactStore) getSourceAndMap(ctx context.Context, frameURL, buildUUID string) ([]byte, []byte, error) {
	u, err := url.Parse(frameURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse frame url %q: %w", frameURL, err)
	}
	base := path.Base(u.Path)
	if base == "." || base == "/" || base == "" {
		return nil, nil, fmt.Errorf("frame url %q has no file name", frameURL)
	}

	sourceKey := a.key(buildUUID, base)
	var source []byte
	if u.RawQuery != "" {
		source, err = a.store.fetch(ctx, sourceKey+"?"+u.RawQuery)
	}
	if source == nil || err != nil {
		if source, err = a.store.fetch(ctx, sourceKey); err != nil {
			return nil, nil, fmt.Errorf("failed to find source file %q: %w", sourceKey, err)
		}
	}

	mapRef := findSourceMappingURL(source)
	if mapRef == "" {
		return nil, nil, fmt.Errorf("source file %q has no sourceMappingURL comment", sourceKey)
	}

	if data, ok := decodeDataURI(mapRef); ok {
		return source, data, nil
	}

	if refURL, err := url.Parse(mapRef); err == nil && refURL.IsAbs() {
		mapRef = path.Base(refURL.Path)
	}
	mapKey := path.Join(path.Dir(sourceKey), mapRef)
	sourceMap, err := a.store.fetch(ctx, mapKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find source map %q: %w", mapKey, err)
	}
	return source, sourceMap, nil
}

func (a *artifactStore) dartSymbolsKey(buildID, arch string) string {
	base := dart.NormalizeDebugID(buildID) + "-" + dart.NormalizeArch(arch) + ".symbols"
	return a.key("", base)
}

func (a *artifactStore) getDartSymbols(ctx context.Context, buildID, arch string) ([]byte, error) {
	key := a.dartSymbolsKey(buildID, arch)
	data, err := a.store.fetch(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to find dart symbols %q: %w", key, err)
	}
	return data, nil
}

func (a *artifactStore) iosSymbolsKey(uuid string) string {
	return a.key("", ios.NormalizeUUID(uuid)+".dsym")
}

func (a *artifactStore) getIOSDsym(ctx context.Context, uuid string) ([]byte, error) {
	key := a.iosSymbolsKey(uuid)
	data, err := a.store.fetch(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to find ios symbols %q: %w", key, err)
	}
	return data, nil
}

func (a *artifactStore) key(buildUUID, base string) string {
	parts := make([]string, 0, 3)
	if a.prefix != "" {
		parts = append(parts, a.prefix)
	}
	if buildUUID != "" {
		parts = append(parts, buildUUID)
	}
	parts = append(parts, base)
	return path.Join(parts...)
}

func findSourceMappingURL(source []byte) string {
	tail := source
	if len(tail) > 16384 {
		tail = tail[len(tail)-16384:]
	}
	matches := sourceMappingURLRe.FindAllSubmatch(tail, -1)
	if len(matches) == 0 {
		return ""
	}
	return string(matches[len(matches)-1][1])
}

func decodeDataURI(ref string) ([]byte, bool) {
	if !strings.HasPrefix(ref, "data:") {
		return nil, false
	}
	idx := strings.Index(ref, "base64,")
	if idx == -1 {
		return nil, false
	}
	data, err := base64.StdEncoding.DecodeString(ref[idx+len("base64,"):])
	if err != nil {
		return nil, false
	}
	return data, true
}

type fileStore struct {
	root string
}

func (f *fileStore) fetch(_ context.Context, key string) ([]byte, error) {
	rel := filepath.FromSlash(key)
	if !filepath.IsLocal(rel) {
		return nil, fmt.Errorf("invalid key %q", key)
	}
	data, err := os.ReadFile(filepath.Join(f.root, rel))
	if errors.Is(err, os.ErrNotExist) {
		return nil, errObjectNotFound
	}
	return data, err
}

type s3Store struct {
	client *s3.Client
	bucket string
}

func (s *s3Store) fetch(ctx context.Context, key string) ([]byte, error) {
	key = strings.TrimPrefix(key, "/")
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key})
	if err != nil {
		var noKey *types.NoSuchKey
		if errors.As(err, &noKey) {
			return nil, errObjectNotFound
		}
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

type gcsStore struct {
	bucket *storage.BucketHandle
}

func (g *gcsStore) fetch(ctx context.Context, key string) ([]byte, error) {
	key = strings.TrimPrefix(key, "/")
	r, err := g.bucket.Object(key).NewReader(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return nil, errObjectNotFound
		}
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}
