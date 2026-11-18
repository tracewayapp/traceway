package env

import (
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
)

// Stub is a local HTTP target whose health the harness switches, so monitors
// can go down and recover on cue.
type Stub struct {
	URL     string
	healthy atomic.Bool
	hits    atomic.Int64
	srv     *http.Server
	ln      net.Listener
}

// StartStub starts a stub on a free loopback port.
func StartStub(healthy bool) (*Stub, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	s := &Stub{URL: "http://" + ln.Addr().String(), ln: ln}
	s.healthy.Store(healthy)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		s.hits.Add(1)
		if s.healthy.Load() {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"status":"ok"}`)
			return
		}
		http.Error(w, `{"status":"unavailable"}`, http.StatusServiceUnavailable)
	})
	s.srv = &http.Server{Handler: mux}
	go func() { _ = s.srv.Serve(ln) }()
	return s, nil
}

// SetHealthy switches the stub's answers.
func (s *Stub) SetHealthy(v bool) { s.healthy.Store(v) }

// Healthy reports the current state.
func (s *Stub) Healthy() bool { return s.healthy.Load() }

// Hits counts requests served.
func (s *Stub) Hits() int64 { return s.hits.Load() }

// Close stops the stub.
func (s *Stub) Close() { _ = s.srv.Close() }
