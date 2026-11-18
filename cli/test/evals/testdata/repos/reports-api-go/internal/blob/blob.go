// Package blob stores export files in the object store and signs download
// links. Credentials are short-lived and fetched from the node's credential
// endpoint.
package blob

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"example.com/reports-api/internal/telemetry"
)

// Client is an object store client for one bucket.
type Client struct {
	endpoint string // https://storage.example
	bucket   string
	credsURL string
	http     *http.Client
	meta     *http.Client

	mu    sync.Mutex
	creds credentials
}

type credentials struct {
	KeyID   string    `json:"keyId"`
	Secret  string    `json:"secret"`
	Expires time.Time `json:"expires"`
}

// New returns a Client.
func New(endpoint, bucket, credsURL string) *Client {
	return &Client{endpoint: endpoint, bucket: bucket, credsURL: credsURL,
		http: telemetry.HTTPClient(), meta: &http.Client{Timeout: 2 * time.Second}}
}

// Put uploads data under key.
func (c *Client) Put(ctx context.Context, key, contentType string, data []byte) error {
	cr, err := c.credentials(ctx)
	if err != nil {
		return fmt.Errorf("storage credentials: %w", err)
	}
	sum := sha256.Sum256(data)
	u := fmt.Sprintf("%s/%s/%s", c.endpoint, c.bucket, key)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Content-Sha256", hex.EncodeToString(sum[:]))
	req.Header.Set("Authorization", "HMAC "+cr.KeyID+":"+sign(cr.Secret, http.MethodPut, key, hex.EncodeToString(sum[:])))
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("PUT %s: %s", key, resp.Status)
	}
	return nil
}

// SignedURL returns a download link for key valid for ttl.
func (c *Client) SignedURL(key string, ttl time.Duration) (string, error) {
	c.mu.Lock()
	cr := c.creds
	c.mu.Unlock()
	if cr.KeyID == "" {
		return "", fmt.Errorf("no storage credentials loaded")
	}
	exp := strconv.FormatInt(time.Now().Add(ttl).Unix(), 10)
	q := url.Values{"key": {cr.KeyID}, "expires": {exp}, "sig": {sign(cr.Secret, http.MethodGet, key, exp)}}
	return fmt.Sprintf("%s/%s/%s?%s", c.endpoint, c.bucket, key, q.Encode()), nil
}

func (c *Client) credentials(ctx context.Context) (credentials, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Until(c.creds.Expires) > time.Minute {
		return c.creds, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.credsURL, nil)
	if err != nil {
		return credentials{}, err
	}
	resp, err := c.meta.Do(req)
	if err != nil {
		return credentials{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return credentials{}, fmt.Errorf("credential endpoint: %s", resp.Status)
	}
	var cr credentials
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return credentials{}, err
	}
	c.creds = cr
	return cr, nil
}

func sign(secret string, parts ...string) string {
	m := hmac.New(sha256.New, []byte(secret))
	for _, p := range parts {
		m.Write([]byte(p))
		m.Write([]byte{'\n'})
	}
	return hex.EncodeToString(m.Sum(nil))
}
