package controllers

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func twentyOneLabelKeys() []string {
	keys := make([]string, 21)
	for i := range keys {
		keys[i] = fmt.Sprintf("k%d", i)
	}
	return keys
}

func TestCleanProfileLabelAllowlist(t *testing.T) {
	tests := []struct {
		name    string
		in      []string
		want    []string
		wantErr bool
	}{
		{name: "valid keys incl dotted/colon/hyphen", in: []string{"tenant", "http.route", "region-1", "team:core"}, want: []string{"tenant", "http.route", "region-1", "team:core"}},
		{name: "drops endpoint case-insensitively", in: []string{"endpoint", "Endpoint", "ENDPOINT", "tenant"}, want: []string{"tenant"}},
		{name: "dedups preserving order", in: []string{"tenant", "tenant", "region"}, want: []string{"tenant", "region"}},
		{name: "skips empty and whitespace", in: []string{"", "   ", "tenant"}, want: []string{"tenant"}},
		{name: "all-endpoint yields empty", in: []string{"endpoint", "  "}, want: []string{}},
		{name: "rejects spaces", in: []string{"has space"}, wantErr: true},
		{name: "rejects slash", in: []string{"a/b"}, wantErr: true},
		{name: "rejects at-sign", in: []string{"user@host"}, wantErr: true},
		{name: "rejects more than 20 keys", in: twentyOneLabelKeys(), wantErr: true},
		{name: "rejects over-long key", in: []string{strings.Repeat("a", 101)}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, errMsg := cleanProfileLabelAllowlist(tt.in)
			if tt.wantErr {
				if errMsg == "" {
					t.Fatalf("expected validation error, got cleaned=%v", got)
				}
				return
			}
			if errMsg != "" {
				t.Fatalf("unexpected error: %s", errMsg)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("cleaned = %v, want %v", got, tt.want)
			}
		})
	}
}
