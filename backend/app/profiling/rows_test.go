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

func TestBuildRows_PerTypeProfileRowsCarryProvenance(t *testing.T) {
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

	stacks, samples, profiles := BuildRows(projectId, []Decoded{d})

	if len(stacks) != 1 || stacks[0].StackHash != 0xAA {
		t.Fatalf("stacks wrong: %+v", stacks)
	}

	if len(profiles) != 2 {
		t.Fatalf("expected one profile row per type (alloc + inuse), got %d", len(profiles))
	}
	byType := map[string]models.Profile{}
	for _, p := range profiles {
		byType[p.ProfileType] = p
		if p.Id == uuid.Nil || p.ProjectId != projectId || p.ServiceName != "checkout" {
			t.Errorf("profile identity wrong: %+v", p)
		}
		if p.ServerName != "pod-a" || p.AppVersion != "1.2.3" {
			t.Errorf("profile provenance not carried: %+v", p)
		}
		if p.SampleCount != 1 {
			t.Errorf("profile %q SampleCount = %d, want 1", p.ProfileType, p.SampleCount)
		}
	}
	if byType[TypeHeapAllocSpace].TotalValue != 4096 || byType[TypeHeapInuseSpace].TotalValue != 2048 {
		t.Errorf("totals wrong: alloc=%d inuse=%d", byType[TypeHeapAllocSpace].TotalValue, byType[TypeHeapInuseSpace].TotalValue)
	}
	if byType[TypeHeapAllocSpace].Id == byType[TypeHeapInuseSpace].Id {
		t.Error("per-type profiles must have distinct ids")
	}

	for _, s := range samples {
		if s.ProfileId != byType[s.Type].Id {
			t.Errorf("sample type %q ProfileId = %v, want %v", s.Type, s.ProfileId, byType[s.Type].Id)
		}
		if s.ServerName != "pod-a" || s.AppVersion != "1.2.3" {
			t.Errorf("sample provenance not carried: %+v", s)
		}
	}
}

func TestBuildRows_MergesAcrossDecodedBatch(t *testing.T) {
	projectId := uuid.New()
	start := time.Unix(1_700_000_500, 0).UTC()
	d1 := Decoded{
		Meta:    Meta{ServiceName: "api", Start: start, End: start.Add(10 * time.Second)},
		Stacks:  []Stack{{Hash: 1, Frames: []string{"main.main"}}},
		Samples: []Sample{{Type: TypeCPUNanos, StackHash: 1, Value: 300}},
	}
	d2 := Decoded{
		Meta:    Meta{ServiceName: "api", Start: start, End: start.Add(10 * time.Second)},
		Stacks:  []Stack{{Hash: 2, Frames: []string{"main.main", "main.idle"}}},
		Samples: []Sample{{Type: TypeCPUNanos, StackHash: 2, Value: 100}},
	}

	stacks, samples, profiles := BuildRows(projectId, []Decoded{d1, d2})

	if len(stacks) != 2 {
		t.Fatalf("stacks = %d, want 2 across the batch", len(stacks))
	}
	if len(samples) != 2 {
		t.Fatalf("samples = %d, want 2 across the batch", len(samples))
	}
	if len(profiles) != 2 {
		t.Fatalf("profiles = %d, want 2 (one per decoded, each its own profile id)", len(profiles))
	}
	if profiles[0].Id == profiles[1].Id {
		t.Error("profiles from distinct decoded inputs must have distinct ids")
	}
}
