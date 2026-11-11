package controllers

import (
	"testing"

	"github.com/tracewayapp/traceway/backend/app/models"
)

func TestUniqueSegmentFilesKeepsOneRowPerFile(t *testing.T) {
	segments := []models.SessionRecording{
		{SegmentIndex: 0, FilePath: "recordings/p/sessions/s/0.json"},
		{SegmentIndex: 1, FilePath: "recordings/p/sessions/s/1.json"},
		{SegmentIndex: 1, FilePath: "recordings/p/sessions/s/1.json"},
		{SegmentIndex: 2, FilePath: "recordings/p/sessions/s/2.json"},
	}
	got := uniqueSegmentFiles(segments)
	if len(got) != 3 || got[0].SegmentIndex != 0 || got[1].SegmentIndex != 1 || got[2].SegmentIndex != 2 {
		t.Fatalf("unexpected segments: %+v", got)
	}
}
