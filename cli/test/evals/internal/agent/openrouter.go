// Package agent drives a model through the eval: an OpenRouter Chat
// Completions client (the same code path for every model and the judge), the
// tool-calling loop with budgets and the cost ledger, prompt caching,
// transcripts, and the fake model that plays oracle trajectories.
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/guard"
)

// OpenRouterURL is the only remote endpoint the harness talks to.
const OpenRouterURL = "https://openrouter.ai/api/v1"

// Message is one Chat Completions message.
type Message struct {
	Role       string     `json:"role"`
	Content    any        `json:"content,omitempty"` // string or []Part
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	// ReasoningDetails must be sent back unchanged on the assistant message
	// so reasoning models keep their chain across tool calls.
	ReasoningDetails json.RawMessage `json:"reasoning_details,omitempty"`
}

// Part is one content part.
type Part struct {
	Type         string        `json:"type"`
	Text         string        `json:"text"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

// CacheControl marks an Anthropic prompt-cache breakpoint.
type CacheControl struct {
	Type string `json:"type"`
}

// ToolCall is an assistant tool call.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall is the called function and its JSON arguments.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolDef is a function tool definition.
type ToolDef struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

// FunctionDef describes a function.
type FunctionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
}

// ProviderPrefs is OpenRouter's provider routing.
type ProviderPrefs struct {
	Order             []string `json:"order,omitempty"`
	AllowFallbacks    *bool    `json:"allow_fallbacks,omitempty"`
	RequireParameters bool     `json:"require_parameters,omitempty"`
}

// Request is a Chat Completions request.
type Request struct {
	Model          string         `json:"model"`
	Messages       []Message      `json:"messages"`
	Tools          []ToolDef      `json:"tools,omitempty"`
	ToolChoice     any            `json:"tool_choice,omitempty"`
	MaxTokens      int            `json:"max_tokens,omitempty"`
	Temperature    *float64       `json:"temperature,omitempty"`
	Provider       *ProviderPrefs `json:"provider,omitempty"`
	Reasoning      map[string]any `json:"reasoning,omitempty"`
	ResponseFormat map[string]any `json:"response_format,omitempty"`
	Usage          map[string]any `json:"usage,omitempty"`
	ParallelTools  *bool          `json:"parallel_tool_calls,omitempty"`
}

// Response is a Chat Completions response.
type Response struct {
	ID       string `json:"id"`
	Model    string `json:"model"`
	Provider string `json:"provider"`
	Choices  []struct {
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens        int     `json:"prompt_tokens"`
		CompletionTokens    int     `json:"completion_tokens"`
		Cost                float64 `json:"cost"`
		PromptTokensDetails struct {
			CachedTokens     int `json:"cached_tokens"`
			CacheWriteTokens int `json:"cache_write_tokens"`
		} `json:"prompt_tokens_details"`
		CompletionTokensDetails struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"completion_tokens_details"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Code    any    `json:"code"`
	} `json:"error,omitempty"`
}

// Chatter sends one Chat Completions request.
type Chatter interface {
	Chat(ctx context.Context, req *Request) (*Response, error)
}

// Client talks Chat Completions to OpenRouter (or to the fake model on
// loopback). A non-loopback base URL is refused unless guard.Live() says a
// paid run was authorized, which is never the case under go test.
type Client struct {
	BaseURL string
	Key     string
	HTTP    *http.Client
	// Retries on 408/429/5xx/timeouts (default 5).
	Retries int
}

// ErrNotAuthorized is returned when a paid call was not unlocked.
var ErrNotAuthorized = errors.New("paid model calls are locked: only `twevals run|resume` unlock them after the manual-run guard passes (never under go test)")

// NewOpenRouter returns the real client. It fails unless the guard unlocked
// paid calls.
func NewOpenRouter(key string) (*Client, error) {
	if !guard.Live() {
		return nil, ErrNotAuthorized
	}
	if key == "" {
		return nil, errors.New("OPENROUTER_API_KEY is not set")
	}
	return &Client{BaseURL: OpenRouterURL, Key: key, HTTP: &http.Client{Timeout: 5 * time.Minute}}, nil
}

func isLoopback(base string) bool {
	u, err := url.Parse(base)
	if err != nil {
		return false
	}
	host := u.Hostname()
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// RetryableError marks an error worth retrying.
type RetryableError struct{ Err error }

func (e *RetryableError) Error() string { return e.Err.Error() }
func (e *RetryableError) Unwrap() error { return e.Err }

// Chat sends a request with retries and exponential backoff.
func (c *Client) Chat(ctx context.Context, req *Request) (*Response, error) {
	if !isLoopback(c.BaseURL) && !guard.Live() {
		return nil, ErrNotAuthorized
	}
	retries := c.Retries
	if retries == 0 {
		retries = 5
	}
	var last error
	for attempt := 0; attempt <= retries; attempt++ {
		if attempt > 0 {
			d := time.Duration(1<<uint(attempt-1)) * 2 * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(d):
			}
		}
		resp, err := c.once(ctx, req)
		if err == nil {
			return resp, nil
		}
		last = err
		var re *RetryableError
		if !errors.As(err, &re) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("after %d retries: %w", retries, last)
}

func (c *Client) once(ctx context.Context, req *Request) (*Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSuffix(c.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	hreq.Header.Set("Content-Type", "application/json")
	if c.Key != "" {
		hreq.Header.Set("Authorization", "Bearer "+c.Key)
	}
	hreq.Header.Set("HTTP-Referer", "https://github.com/tracewayapp/traceway")
	hreq.Header.Set("X-Title", "traceway-evals")
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 5 * time.Minute}
	}
	resp, err := hc.Do(hreq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, &RetryableError{err}
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &RetryableError{err}
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode >= 500 {
		return nil, &RetryableError{fmt.Errorf("openrouter HTTP %d: %s", resp.StatusCode, clip(raw))}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openrouter HTTP %d: %s", resp.StatusCode, clip(raw))
	}
	var out Response
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, &RetryableError{fmt.Errorf("decode response: %w (%s)", err, clip(raw))}
	}
	if out.Error != nil {
		code := fmt.Sprint(out.Error.Code)
		if code == "429" || strings.HasPrefix(code, "5") {
			return nil, &RetryableError{fmt.Errorf("openrouter error %s: %s", code, out.Error.Message)}
		}
		return nil, fmt.Errorf("openrouter error %s: %s", code, out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return nil, &RetryableError{fmt.Errorf("openrouter returned no choices: %s", clip(raw))}
	}
	return &out, nil
}

func clip(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 500 {
		return s[:500] + "..."
	}
	return s
}
