package main

import (
	"io/fs"
	"path"
	"slices"
	"testing"

	"github.com/tracewayapp/traceway/cli/pkg/mcpserver/knowledge"
)

func TestLearnTopicsCoverEveryChunk(t *testing.T) {
	files, err := fs.Glob(knowledge.FS, "learn/*.md")
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, f := range files {
		if name := path.Base(f); name != "overview.md" {
			want = append(want, name)
		}
	}
	got := slices.Sorted(slices.Values(learnTopics))
	if !slices.Equal(got, want) {
		t.Errorf("learnTopics = %v, want every learn chunk except overview.md: %v", got, want)
	}
}
