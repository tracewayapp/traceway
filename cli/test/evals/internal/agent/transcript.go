package agent

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// Transcript appends one JSON event per line: the opening messages, every
// model response (with usage, provider and generation id), every tool call
// (id, op, argv/args, exit, ok, truncated, output) and the stop reason.
type Transcript struct {
	mu sync.Mutex
	f  *os.File
}

// OpenTranscript creates path.
func OpenTranscript(path string) (*Transcript, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	return &Transcript{f: f}, nil
}

// Event writes one event.
func (t *Transcript) Event(kind string, fields map[string]any) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	fields["type"] = kind
	fields["at"] = time.Now().UTC().Format(time.RFC3339Nano)
	raw, err := json.Marshal(fields)
	if err != nil {
		return
	}
	t.f.Write(append(raw, '\n'))
}

// Close flushes and closes the file.
func (t *Transcript) Close() error {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.f.Close()
}
