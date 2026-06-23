package profiling

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
)

func profileByType(profiles []models.Profile, typ string) (models.Profile, bool) {
	for _, p := range profiles {
		if p.ProfileType == typ {
			return p, true
		}
	}
	return models.Profile{}, false
}

func TestBuildRows_SingleType(t *testing.T) {
	projectId := uuid.New()
	start := time.Unix(1_700_000_000, 0).UTC()
	h1 := HashFrames([]string{"main.main", "main.work"})
	h2 := HashFrames([]string{"main.main", "main.idle"})

	d := Decoded{
		Meta: Meta{
			ServiceName: "checkout", ServerName: "pod-a", AppVersion: "1.2.3",
			Start: start, End: start.Add(30 * time.Second),
		},
		Stacks: []Stack{
			{Hash: h1, Frames: []string{"main.main", "main.work"}},
			{Hash: h2, Frames: []string{"main.main", "main.idle"}},
		},
		Samples: []Sample{
			{Type: TypeCPUNanos, StackHash: h1, Value: 300},
			{Type: TypeCPUNanos, StackHash: h2, Value: 100},
		},
	}

	stacks, samples, profiles := BuildRows(projectId, []Decoded{d})

	if len(stacks) != 2 {
		t.Fatalf("stacks = %d, want 2", len(stacks))
	}
	for _, s := range stacks {
		if s.ProjectId != projectId || s.ServiceName != "checkout" {
			t.Errorf("stack project/service wrong: %+v", s)
		}
		if !s.LastSeen.Equal(d.Meta.End) {
			t.Errorf("stack LastSeen = %v, want %v", s.LastSeen, d.Meta.End)
		}
	}

	if len(profiles) != 1 {
		t.Fatalf("profiles = %d, want 1", len(profiles))
	}
	p := profiles[0]
	if p.ProfileType != TypeCPUNanos || p.SampleCount != 2 || p.TotalValue != 400 {
		t.Errorf("profile = (%s, count=%d, total=%d), want (cpu, 2, 400)", p.ProfileType, p.SampleCount, p.TotalValue)
	}
	if p.ServiceName != "checkout" || p.ServerName != "pod-a" || p.AppVersion != "1.2.3" {
		t.Errorf("profile metadata not carried: %+v", p)
	}
	if !p.RecordedAt.Equal(start) || p.Duration != 30*time.Second {
		t.Errorf("profile time = (%v, %v), want (%v, 30s)", p.RecordedAt, p.Duration, start)
	}
	if p.Id == uuid.Nil || p.ProjectId != projectId {
		t.Errorf("profile id/project wrong: %+v", p)
	}

	if len(samples) != 2 {
		t.Fatalf("samples = %d, want 2", len(samples))
	}
	for _, s := range samples {
		if s.ProfileId != p.Id {
			t.Errorf("sample ProfileId = %v, want %v", s.ProfileId, p.Id)
		}
		if s.Type != TypeCPUNanos || s.ProjectId != projectId || s.ServiceName != "checkout" {
			t.Errorf("sample fields wrong: %+v", s)
		}
		if !s.Start.Equal(start) || !s.End.Equal(d.Meta.End) {
			t.Errorf("sample time = (%v, %v), want (%v, %v)", s.Start, s.End, start, d.Meta.End)
		}
	}
}

func TestBuildRows_MultiTypeSplitsProfiles(t *testing.T) {
	projectId := uuid.New()
	start := time.Unix(1_700_000_000, 0).UTC()
	h1 := HashFrames([]string{"main.main", "main.work"})

	d := Decoded{
		Meta:   Meta{ServiceName: "checkout", Start: start, End: start},
		Stacks: []Stack{{Hash: h1, Frames: []string{"main.main", "main.work"}}},
		Samples: []Sample{
			{Type: TypeCPUNanos, StackHash: h1, Value: 300},
			{Type: TypeHeapInuseSpace, StackHash: h1, Value: 2048},
		},
	}

	_, samples, profiles := BuildRows(projectId, []Decoded{d})

	if len(profiles) != 2 {
		t.Fatalf("profiles = %d, want 2 (one per type)", len(profiles))
	}
	cpu, ok := profileByType(profiles, TypeCPUNanos)
	if !ok {
		t.Fatalf("no cpu profile row")
	}
	heap, ok := profileByType(profiles, TypeHeapInuseSpace)
	if !ok {
		t.Fatalf("no heap profile row")
	}
	if cpu.Id == heap.Id {
		t.Errorf("expected distinct ProfileIds per type, both = %v", cpu.Id)
	}
	if cpu.TotalValue != 300 || heap.TotalValue != 2048 {
		t.Errorf("totals = (cpu=%d, heap=%d), want (300, 2048)", cpu.TotalValue, heap.TotalValue)
	}

	for _, s := range samples {
		switch s.Type {
		case TypeCPUNanos:
			if s.ProfileId != cpu.Id {
				t.Errorf("cpu sample linked to %v, want %v", s.ProfileId, cpu.Id)
			}
		case TypeHeapInuseSpace:
			if s.ProfileId != heap.Id {
				t.Errorf("heap sample linked to %v, want %v", s.ProfileId, heap.Id)
			}
		default:
			t.Errorf("unexpected sample type %q", s.Type)
		}
	}
}
