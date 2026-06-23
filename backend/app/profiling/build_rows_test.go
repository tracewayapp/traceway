package profiling

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
)

func TestBuildRows_PerTypeProfileRows(t *testing.T) {
	projectId := uuid.MustParse("00000000-0000-0000-0000-000000000007")
	start := time.Unix(1_700_000_000, 0).UTC()
	d := Decoded{
		Meta: Meta{
			ServiceName: "checkout",
			Start:       start,
			End:         start.Add(30 * time.Second),
			ServerName:  "pod-a",
			AppVersion:  "1.2.3",
		},
		Stacks: []Stack{
			{Hash: 0xAA, Frames: []string{"main.main", "main.work"}},
		},
		Samples: []Sample{
			{Type: TypeHeapAllocSpace, StackHash: 0xAA, Value: 4096},
			{Type: TypeHeapInuseSpace, StackHash: 0xAA, Value: 2048},
		},
	}

	stacks, samples, profiles := BuildRows(d, projectId)

	if len(stacks) != 1 {
		t.Fatalf("expected 1 stack, got %d", len(stacks))
	}
	if stacks[0].ProjectId != projectId || stacks[0].ServiceName != "checkout" || stacks[0].StackHash != 0xAA {
		t.Errorf("stack identity wrong: %+v", stacks[0])
	}
	if !stacks[0].LastSeen.Equal(d.Meta.End) {
		t.Errorf("stack LastSeen = %v, want profile End %v", stacks[0].LastSeen, d.Meta.End)
	}

	if len(profiles) != 2 {
		t.Fatalf("expected one profile row per type (alloc + inuse), got %d", len(profiles))
	}
	byType := map[string]models.Profile{}
	for _, p := range profiles {
		byType[p.ProfileType] = p
		if p.Id == uuid.Nil {
			t.Errorf("profile %q has nil id", p.ProfileType)
		}
		if p.ProjectId != projectId || p.ServiceName != "checkout" {
			t.Errorf("profile identity wrong: %+v", p)
		}
		if p.ServerName != "pod-a" || p.AppVersion != "1.2.3" {
			t.Errorf("profile provenance not carried: %+v", p)
		}
		if !p.RecordedAt.Equal(start) {
			t.Errorf("profile RecordedAt = %v, want %v", p.RecordedAt, start)
		}
		if p.Duration != 30*time.Second {
			t.Errorf("profile Duration = %v, want 30s", p.Duration)
		}
		if p.SampleCount != 1 {
			t.Errorf("profile %q SampleCount = %d, want 1", p.ProfileType, p.SampleCount)
		}
	}
	if alloc, ok := byType[TypeHeapAllocSpace]; !ok || alloc.TotalValue != 4096 {
		t.Errorf("alloc profile TotalValue = %d, want 4096", alloc.TotalValue)
	}
	if inuse, ok := byType[TypeHeapInuseSpace]; !ok || inuse.TotalValue != 2048 {
		t.Errorf("inuse profile TotalValue = %d, want 2048", inuse.TotalValue)
	}
	if byType[TypeHeapAllocSpace].Id == byType[TypeHeapInuseSpace].Id {
		t.Error("per-type profiles must have distinct ids")
	}

	if len(samples) != 2 {
		t.Fatalf("expected 2 samples, got %d", len(samples))
	}
	for _, s := range samples {
		want := byType[s.Type]
		if s.ProfileId != want.Id {
			t.Errorf("sample type %q ProfileId = %v, want matching profile id %v", s.Type, s.ProfileId, want.Id)
		}
		if s.ProjectId != projectId || s.ServiceName != "checkout" || s.StackHash != 0xAA {
			t.Errorf("sample identity wrong: %+v", s)
		}
		if !s.Start.Equal(start) || !s.End.Equal(start.Add(30*time.Second)) {
			t.Errorf("sample window wrong: %+v", s)
		}
		if s.ServerName != "pod-a" || s.AppVersion != "1.2.3" {
			t.Errorf("sample provenance not carried: %+v", s)
		}
	}
}

func TestBuildRows_CPUSingleType(t *testing.T) {
	projectId := uuid.New()
	start := time.Unix(1_700_000_500, 0).UTC()
	d := Decoded{
		Meta:   Meta{ServiceName: "api", Start: start, End: start.Add(10 * time.Second)},
		Stacks: []Stack{{Hash: 1, Frames: []string{"main.main"}}, {Hash: 2, Frames: []string{"main.main", "main.idle"}}},
		Samples: []Sample{
			{Type: TypeCPUNanos, StackHash: 1, Value: 300},
			{Type: TypeCPUNanos, StackHash: 2, Value: 100},
		},
	}

	_, samples, profiles := BuildRows(d, projectId)

	if len(profiles) != 1 {
		t.Fatalf("expected 1 cpu profile, got %d", len(profiles))
	}
	p := profiles[0]
	if p.ProfileType != TypeCPUNanos {
		t.Errorf("ProfileType = %q, want %q", p.ProfileType, TypeCPUNanos)
	}
	if p.TotalValue != 400 {
		t.Errorf("TotalValue = %d, want 400 (300+100)", p.TotalValue)
	}
	if p.SampleCount != 2 {
		t.Errorf("SampleCount = %d, want 2", p.SampleCount)
	}
	for _, s := range samples {
		if s.ProfileId != p.Id {
			t.Errorf("sample ProfileId = %v, want %v", s.ProfileId, p.Id)
		}
	}
}
