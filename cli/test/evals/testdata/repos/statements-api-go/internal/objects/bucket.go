// Package objects stores statement PDFs in S3.
package objects

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"
)

// Bucket is an S3 bucket reached through presigned requests from the
// signing sidecar (see deploy/).
type Bucket struct {
	Endpoint string // https://<bucket>.s3.<region>.amazonaws.com
	Signer   func(method, key string, ttl time.Duration) (string, error)
	Client   *http.Client
}

// Put uploads body under key.
func (b *Bucket) Put(ctx context.Context, key string, body []byte) error {
	url, err := b.Signer(http.MethodPut, key, 5*time.Minute)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/pdf")
	res, err := b.Client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("put %s: %s", key, res.Status)
	}
	return nil
}

// PresignGet returns a download URL for key.
func (b *Bucket) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	return b.Signer(http.MethodGet, key, ttl)
}

// SidecarSigner asks the signing sidecar for a presigned URL.
func SidecarSigner(base string) func(method, key string, ttl time.Duration) (string, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	return func(method, key string, ttl time.Duration) (string, error) {
		res, err := client.Get(fmt.Sprintf("%s/sign?method=%s&key=%s&ttl=%d", base, method, key, int(ttl.Seconds())))
		if err != nil {
			return "", err
		}
		defer res.Body.Close()
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(res.Body); err != nil {
			return "", err
		}
		return buf.String(), nil
	}
}
