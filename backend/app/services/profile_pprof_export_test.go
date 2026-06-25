package services

import (
	"bytes"
	"testing"

	"github.com/google/pprof/profile"
	"github.com/tracewayapp/traceway/backend/app/models"
)

func TestBuildPprof_RoundTrips(t *testing.T) {
	stacks := []models.ProfileStackValue{
		{Stack: []string{"main.main", "net/http.serve", "checkout.Handle", "db.Query"}, Value: 300},
		{Stack: []string{"main.main", "net/http.serve", "cart.Add", "cache.Get"}, Value: 100},
	}

	raw, err := BuildPprof("cpu", "nanoseconds", stacks)
	if err != nil {
		t.Fatalf("BuildPprof: %v", err)
	}

	p, err := profile.Parse(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("re-parse exported pprof: %v", err)
	}

	if len(p.SampleType) != 1 || p.SampleType[0].Type != "cpu" || p.SampleType[0].Unit != "nanoseconds" {
		t.Fatalf("sample type = %+v, want cpu/nanoseconds", p.SampleType)
	}
	if len(p.Sample) != 2 {
		t.Fatalf("samples = %d, want 2", len(p.Sample))
	}

	var total int64
	for _, s := range p.Sample {
		total += s.Value[0]
		leaf := s.Location[0].Line[0].Function.Name
		if leaf != "db.Query" && leaf != "cache.Get" {
			t.Errorf("leaf (first location) = %q, want a leaf frame", leaf)
		}
	}
	if total != 400 {
		t.Errorf("total value = %d, want 400", total)
	}
}

func TestBuildPprof_PreservesArbitraryTypeAndUnit(t *testing.T) {
	raw, err := BuildPprof("lock", "nanoseconds", []models.ProfileStackValue{{Stack: []string{"a"}, Value: 1}})
	if err != nil {
		t.Fatalf("BuildPprof: %v", err)
	}
	p, err := profile.Parse(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if p.SampleType[0].Type != "lock" || p.SampleType[0].Unit != "nanoseconds" {
		t.Errorf("sample type = %+v, want lock/nanoseconds (arbitrary type carried through)", p.SampleType[0])
	}
}

func TestBuildPprof_EmptyTypeFallsBack(t *testing.T) {
	raw, err := BuildPprof("", "", []models.ProfileStackValue{{Stack: []string{"a"}, Value: 1}})
	if err != nil {
		t.Fatalf("BuildPprof: %v", err)
	}
	p, err := profile.Parse(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if p.SampleType[0].Type != "samples" || p.SampleType[0].Unit != "count" {
		t.Errorf("fallback sample type = %+v, want samples/count", p.SampleType[0])
	}
}
