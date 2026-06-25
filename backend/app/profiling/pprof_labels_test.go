package profiling

import (
	"testing"

	"github.com/google/pprof/profile"
	"github.com/google/uuid"
)

func labeledCPU(t *testing.T, samples []*profile.Sample, funcs []*profile.Function, locs []*profile.Location) []byte {
	return cpuProfile(t, 1_700_000_000_000_000_000, 1_000_000_000, samples, funcs, locs)
}

func TestPprofDecoder_KeepsOnlyAllowlistedLabels(t *testing.T) {
	main := fn(1, "main.main", "main.go")
	work := fn(2, "main.work", "work.go")
	lMain := loc(1, profile.Line{Function: main, Line: 10})
	lWork := loc(2, profile.Line{Function: work, Line: 20})

	samples := []*profile.Sample{
		{
			Location: []*profile.Location{lWork, lMain},
			Value:    []int64{1, 100},
			Label:    map[string][]string{"endpoint": {"/checkout"}, "request_id": {"abc-123"}},
		},
	}
	d := decodeOne(t, labeledCPU(t, samples, []*profile.Function{main, work}, []*profile.Location{lMain, lWork}))

	cpu := sampleByType(d, "cpu")
	if len(cpu) != 1 {
		t.Fatalf("expected 1 sample, got %d", len(cpu))
	}
	got := cpu[0].Labels
	if len(got) != 1 || got[LabelEndpoint] != "/checkout" {
		t.Errorf("labels = %v, want only {endpoint:/checkout} (high-cardinality request_id dropped)", got)
	}
}

func TestPprofDecoder_HonorsAdditionalAllowlistedKeys(t *testing.T) {
	main := fn(1, "main.main", "main.go")
	lMain := loc(1, profile.Line{Function: main, Line: 10})
	samples := []*profile.Sample{{
		Location: []*profile.Location{lMain},
		Value:    []int64{1, 100},
		Label:    map[string][]string{"endpoint": {"/x"}, "tenant": {"acme"}, "request_id": {"abc"}},
	}}
	out, err := PprofDecoder{}.Decode(IngestContext{
		DefaultServiceName: "checkout",
		ReceivedAt:         testIngest.ReceivedAt,
		LabelAllowlist:     NewLabelAllowSet([]string{"tenant"}),
	}, labeledCPU(t, samples, []*profile.Function{main}, []*profile.Location{lMain}))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	cpu := sampleByType(out[0], "cpu")
	if len(cpu) != 1 {
		t.Fatalf("expected 1 sample, got %d", len(cpu))
	}
	got := cpu[0].Labels
	if got[LabelEndpoint] != "/x" || got["tenant"] != "acme" {
		t.Errorf("labels = %v, want {endpoint:/x, tenant:acme} (built-in + additional allowlisted)", got)
	}
	if _, ok := got["request_id"]; ok {
		t.Errorf("non-allowlisted request_id must be dropped, got %v", got)
	}
}

func TestPprofDecoder_LabelsSeparateTheDedup(t *testing.T) {
	main := fn(1, "main.main", "main.go")
	work := fn(2, "main.work", "work.go")
	lMain := loc(1, profile.Line{Function: main, Line: 10})
	lWork := loc(2, profile.Line{Function: work, Line: 20})

	mk := func(endpoint string, v int64) *profile.Sample {
		return &profile.Sample{
			Location: []*profile.Location{lWork, lMain},
			Value:    []int64{1, v},
			Label:    map[string][]string{"endpoint": {endpoint}},
		}
	}
	samples := []*profile.Sample{mk("/a", 100), mk("/b", 50), mk("/a", 25)}
	d := decodeOne(t, labeledCPU(t, samples, []*profile.Function{main, work}, []*profile.Location{lMain, lWork}))

	cpu := sampleByType(d, "cpu")
	if len(cpu) != 2 {
		t.Fatalf("expected 2 samples (one per endpoint), got %d", len(cpu))
	}
	byEndpoint := map[string]int64{}
	for _, s := range cpu {
		byEndpoint[s.Labels[LabelEndpoint]] = s.Value
	}
	if byEndpoint["/a"] != 125 {
		t.Errorf("/a value = %d, want 125 (summed within the same endpoint)", byEndpoint["/a"])
	}
	if byEndpoint["/b"] != 50 {
		t.Errorf("/b value = %d, want 50", byEndpoint["/b"])
	}
	if len(d.Stacks) != 1 {
		t.Errorf("unique stacks = %d, want 1 (same stack, different labels still shares the stack row)", len(d.Stacks))
	}
}

func TestPprofDecoder_NoLabelsStaysEmpty(t *testing.T) {
	main := fn(1, "main.main", "main.go")
	lMain := loc(1, profile.Line{Function: main, Line: 10})
	samples := []*profile.Sample{{Location: []*profile.Location{lMain}, Value: []int64{1, 100}}}
	d := decodeOne(t, labeledCPU(t, samples, []*profile.Function{main}, []*profile.Location{lMain}))

	cpu := sampleByType(d, "cpu")
	if len(cpu) != 1 {
		t.Fatalf("expected 1 sample, got %d", len(cpu))
	}
	if len(cpu[0].Labels) != 0 {
		t.Errorf("labels = %v, want empty for a sample with no pprof labels", cpu[0].Labels)
	}
}

func TestBuildRows_CarriesSampleLabels(t *testing.T) {
	decoded := []Decoded{{
		Meta:    Meta{ServiceName: "svc"},
		Stacks:  []Stack{{Hash: 1, Frames: []string{"main.main"}}},
		Samples: []Sample{{Type: "cpu", StackHash: 1, Value: 10, Labels: map[string]string{LabelEndpoint: "/x"}}},
	}}
	_, samples, _ := BuildRows(uuid.New(), decoded)
	if len(samples) != 1 {
		t.Fatalf("expected 1 ProfileSample, got %d", len(samples))
	}
	if samples[0].Labels[LabelEndpoint] != "/x" {
		t.Errorf("ProfileSample.Labels = %v, want {endpoint:/x}", samples[0].Labels)
	}
}

func TestPprofDecoder_DropsNumericLabels(t *testing.T) {
	main := fn(1, "main.main", "main.go")
	lMain := loc(1, profile.Line{Function: main, Line: 10})
	samples := []*profile.Sample{{
		Location: []*profile.Location{lMain},
		Value:    []int64{1, 100},
		Label:    map[string][]string{"endpoint": {"/x"}},
		NumLabel: map[string][]int64{"request_count": {42}},
	}}
	out, err := PprofDecoder{}.Decode(IngestContext{
		DefaultServiceName: "checkout",
		ReceivedAt:         testIngest.ReceivedAt,
		LabelAllowlist:     NewLabelAllowSet([]string{"request_count"}),
	}, labeledCPU(t, samples, []*profile.Function{main}, []*profile.Location{lMain}))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	cpu := sampleByType(out[0], "cpu")
	if len(cpu) != 1 {
		t.Fatalf("expected 1 sample, got %d", len(cpu))
	}
	got := cpu[0].Labels
	if got[LabelEndpoint] != "/x" {
		t.Errorf("labels = %v, want endpoint /x", got)
	}
	if _, ok := got["request_count"]; ok {
		t.Errorf("numeric NumLabel must be dropped even when allowlisted, got %v", got)
	}
}
