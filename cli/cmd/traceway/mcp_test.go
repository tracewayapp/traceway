package main

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestIsClientDisconnect(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"plain EOF", io.EOF, true},
		{"wrapped EOF", fmt.Errorf("reading: %w", io.EOF), true},
		{"connection closed wrap", fmt.Errorf("%w: calling \"ping\": %v", mcp.ErrConnectionClosed, io.EOF), true},
		{"jsonrpc2 shutdown shape", errors.New("server is closing: EOF"), true},
		{"unexpected EOF", io.ErrUnexpectedEOF, false},
		{"wrapped unexpected EOF", fmt.Errorf("reading: %w", io.ErrUnexpectedEOF), false},
		{"stringified transport EOF", errors.New("read tcp 1.2.3.4:1->5.6.7.8:2: EOF"), false},
		{"unrelated error", errors.New("boom"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isClientDisconnect(tc.err); got != tc.want {
				t.Errorf("isClientDisconnect(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
