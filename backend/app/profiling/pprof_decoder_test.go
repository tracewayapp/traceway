package profiling

import (
	"bytes"
	"testing"
	"time"

	"github.com/google/pprof/profile"
	"github.com/google/uuid"
)

var testIngest = IngestContext{
	ProjectId:          uuid.MustParse("00000000-0000-0000-0000-000000000001"),
	DefaultServiceName: "checkout",
	ServerName:         "pod-abc",
	AppVersion:         "1.2.3",
	ReceivedAt:         time.Unix(1_700_000_000, 0).UTC(),
	LabelAllowlist:     NewLabelAllowSet(nil),
}

func fn(id uint64, name, file string) *profile.Function {
	return &profile.Function{ID: id, Name: name, SystemName: name, Filename: file}
}

func loc(id uint64, lines ...profile.Line) *profile.Location {
	return &profile.Location{ID: id, Line: lines}
}

func writeProfile(t *testing.T, p *profile.Profile) []byte {
	t.Helper()
	if err := p.CheckValid(); err != nil {
		t.Fatalf("invalid synthetic profile: %v", err)
	}
	var buf bytes.Buffer
	if err := p.Write(&buf); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	return buf.Bytes()
}

func cpuProfile(t *testing.T, timeNanos, durationNanos int64, samples []*profile.Sample, funcs []*profile.Function, locs []*profile.Location) []byte {
	p := &profile.Profile{
		SampleType: []*profile.ValueType{
			{Type: "samples", Unit: "count"},
			{Type: "cpu", Unit: "nanoseconds"},
		},
		PeriodType:    &profile.ValueType{Type: "cpu", Unit: "nanoseconds"},
		Period:        10_000_000,
		TimeNanos:     timeNanos,
		DurationNanos: durationNanos,
		Sample:        samples,
		Function:      funcs,
		Location:      locs,
	}
	return writeProfile(t, p)
}

func decodeOne(t *testing.T, payload []byte) Decoded {
	t.Helper()
	out, err := PprofDecoder{}.Decode(testIngest, payload)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 decoded profile, got %d", len(out))
	}
	return out[0]
}

func sampleByType(d Decoded, typ string) []Sample {
	var out []Sample
	for _, s := range d.Samples {
		if s.Type == typ {
			out = append(out, s)
		}
	}
	return out
}

func stackByHash(d Decoded, h uint64) (Stack, bool) {
	for _, s := range d.Stacks {
		if s.Hash == h {
			return s, true
		}
	}
	return Stack{}, false
}

func TestPprofDecoder_CPU_ExplodesDistinctStacks(t *testing.T) {
	main := fn(1, "main.main", "main.go")
	work := fn(2, "main.work", "work.go")
	idle := fn(3, "main.idle", "idle.go")

	lMain := loc(1, profile.Line{Function: main, Line: 10})
	lWork := loc(2, profile.Line{Function: work, Line: 20})
	lIdle := loc(3, profile.Line{Function: idle, Line: 30})

	samples := []*profile.Sample{
		{Location: []*profile.Location{lWork, lMain}, Value: []int64{1, 300}},
		{Location: []*profile.Location{lIdle, lMain}, Value: []int64{1, 100}},
	}
	payload := cpuProfile(t, 1_700_000_000_000_000_000, 5_000_000_000, samples,
		[]*profile.Function{main, work, idle}, []*profile.Location{lMain, lWork, lIdle})

	d := decodeOne(t, payload)

	cpu := sampleByType(d, TypeCPUNanos)
	if len(cpu) != 2 {
		t.Fatalf("expected 2 cpu samples, got %d", len(cpu))
	}
	if len(d.Stacks) != 2 {
		t.Fatalf("expected 2 unique stacks, got %d", len(d.Stacks))
	}

	wantWork := HashFrames([]string{"main.main", "main.work"})
	wantIdle := HashFrames([]string{"main.main", "main.idle"})
	gotValues := map[uint64]int64{}
	for _, s := range cpu {
		gotValues[s.StackHash] = s.Value
	}
	if gotValues[wantWork] != 300 {
		t.Errorf("main->work cpu value = %d, want 300", gotValues[wantWork])
	}
	if gotValues[wantIdle] != 100 {
		t.Errorf("main->idle cpu value = %d, want 100", gotValues[wantIdle])
	}

	st, ok := stackByHash(d, wantWork)
	if !ok {
		t.Fatalf("stack for main->work not present")
	}
	if got := st.Frames; len(got) != 2 || got[0] != "main.main" || got[1] != "main.work" {
		t.Errorf("frames = %v, want [main.main main.work] (root-first)", got)
	}
}

func TestPprofDecoder_DedupesIdenticalStacks(t *testing.T) {
	main := fn(1, "main.main", "main.go")
	work := fn(2, "main.work", "work.go")
	lMain := loc(1, profile.Line{Function: main, Line: 10})
	lWork := loc(2, profile.Line{Function: work, Line: 20})

	samples := []*profile.Sample{
		{Location: []*profile.Location{lWork, lMain}, Value: []int64{1, 200}},
		{Location: []*profile.Location{lWork, lMain}, Value: []int64{1, 50}},
	}
	payload := cpuProfile(t, 1_700_000_000_000_000_000, 1_000_000_000, samples,
		[]*profile.Function{main, work}, []*profile.Location{lMain, lWork})

	d := decodeOne(t, payload)

	cpu := sampleByType(d, TypeCPUNanos)
	if len(cpu) != 1 {
		t.Fatalf("expected 1 deduped cpu sample, got %d", len(cpu))
	}
	if cpu[0].Value != 250 {
		t.Errorf("summed value = %d, want 250", cpu[0].Value)
	}
	if len(d.Stacks) != 1 {
		t.Errorf("expected 1 unique stack, got %d", len(d.Stacks))
	}
}

func TestPprofDecoder_Heap_KeepsSpaceTypesOnly(t *testing.T) {
	main := fn(1, "main.main", "main.go")
	alloc := fn(2, "main.allocate", "alloc.go")
	lMain := loc(1, profile.Line{Function: main, Line: 10})
	lAlloc := loc(2, profile.Line{Function: alloc, Line: 20})

	p := &profile.Profile{
		SampleType: []*profile.ValueType{
			{Type: "alloc_objects", Unit: "count"},
			{Type: "alloc_space", Unit: "bytes"},
			{Type: "inuse_objects", Unit: "count"},
			{Type: "inuse_space", Unit: "bytes"},
		},
		PeriodType: &profile.ValueType{Type: "space", Unit: "bytes"},
		Period:     524288,
		TimeNanos:  1_700_000_000_000_000_000,
		Sample: []*profile.Sample{
			{Location: []*profile.Location{lAlloc, lMain}, Value: []int64{5, 4096, 2, 2048}},
		},
		Function: []*profile.Function{main, alloc},
		Location: []*profile.Location{lMain, lAlloc},
	}
	d := decodeOne(t, writeProfile(t, p))

	inuse := sampleByType(d, TypeHeapInuseSpace)
	allocS := sampleByType(d, TypeHeapAllocSpace)
	if len(inuse) != 1 || inuse[0].Value != 2048 {
		t.Errorf("inuse_space = %+v, want one sample value 2048", inuse)
	}
	if len(allocS) != 1 || allocS[0].Value != 4096 {
		t.Errorf("alloc_space = %+v, want one sample value 4096", allocS)
	}
	for _, s := range d.Samples {
		if s.Type != TypeHeapInuseSpace && s.Type != TypeHeapAllocSpace {
			t.Errorf("unexpected sample type emitted: %q", s.Type)
		}
	}
}

func TestPprofDecoder_InlinedFramesExpand(t *testing.T) {
	main := fn(1, "main.main", "main.go")
	printf := fn(2, "fmt.Printf", "print.go")
	memcpy := fn(3, "runtime.memmove", "memmove.go")

	lMain := loc(1, profile.Line{Function: main, Line: 10})
	lInlined := loc(2,
		profile.Line{Function: memcpy, Line: 5},
		profile.Line{Function: printf, Line: 99},
	)
	samples := []*profile.Sample{
		{Location: []*profile.Location{lInlined, lMain}, Value: []int64{1, 100}},
	}
	payload := cpuProfile(t, 1_700_000_000_000_000_000, 1_000_000_000, samples,
		[]*profile.Function{main, printf, memcpy}, []*profile.Location{lMain, lInlined})

	d := decodeOne(t, payload)
	if len(d.Stacks) != 1 {
		t.Fatalf("expected 1 stack, got %d", len(d.Stacks))
	}
	want := []string{"main.main", "fmt.Printf", "runtime.memmove"}
	got := d.Stacks[0].Frames
	if len(got) != len(want) {
		t.Fatalf("frames = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("frames = %v, want %v", got, want)
		}
	}
}

func TestPprofDecoder_Metadata(t *testing.T) {
	main := fn(1, "main.main", "main.go")
	lMain := loc(1, profile.Line{Function: main, Line: 10})
	start := int64(1_700_000_000_000_000_000)
	dur := int64(30_000_000_000)
	samples := []*profile.Sample{{Location: []*profile.Location{lMain}, Value: []int64{1, 100}}}
	payload := cpuProfile(t, start, dur, samples, []*profile.Function{main}, []*profile.Location{lMain})

	d := decodeOne(t, payload)
	if d.Meta.ServiceName != "checkout" {
		t.Errorf("service name = %q, want checkout", d.Meta.ServiceName)
	}
	if d.Meta.ServerName != "pod-abc" || d.Meta.AppVersion != "1.2.3" {
		t.Errorf("server/version not carried from ingest context: %+v", d.Meta)
	}
	wantStart := time.Unix(0, start).UTC()
	if !d.Meta.Start.Equal(wantStart) {
		t.Errorf("start = %v, want %v", d.Meta.Start.UTC(), wantStart)
	}
	if !d.Meta.End.Equal(wantStart.Add(30 * time.Second)) {
		t.Errorf("end = %v, want start+30s", d.Meta.End.UTC())
	}
}

func TestPprofDecoder_DropsZeroValueSamples(t *testing.T) {
	main := fn(1, "main.main", "main.go")
	live := fn(2, "main.live", "live.go")
	freed := fn(3, "main.freed", "freed.go")
	lMain := loc(1, profile.Line{Function: main, Line: 10})
	lLive := loc(2, profile.Line{Function: live, Line: 20})
	lFreed := loc(3, profile.Line{Function: freed, Line: 30})

	p := &profile.Profile{
		SampleType: []*profile.ValueType{
			{Type: "alloc_objects", Unit: "count"},
			{Type: "alloc_space", Unit: "bytes"},
			{Type: "inuse_objects", Unit: "count"},
			{Type: "inuse_space", Unit: "bytes"},
		},
		PeriodType: &profile.ValueType{Type: "space", Unit: "bytes"},
		Period:     524288,
		TimeNanos:  1_700_000_000_000_000_000,
		Sample: []*profile.Sample{
			{Location: []*profile.Location{lLive, lMain}, Value: []int64{10, 8192, 5, 4096}},
			{Location: []*profile.Location{lFreed, lMain}, Value: []int64{10, 8192, 0, 0}},
		},
		Function: []*profile.Function{main, live, freed},
		Location: []*profile.Location{lMain, lLive, lFreed},
	}
	d := decodeOne(t, writeProfile(t, p))

	inuse := sampleByType(d, TypeHeapInuseSpace)
	if len(inuse) != 1 {
		t.Fatalf("expected 1 inuse_space sample (zero dropped), got %d", len(inuse))
	}
	wantLive := HashFrames([]string{"main.main", "main.live"})
	if inuse[0].StackHash != wantLive || inuse[0].Value != 4096 {
		t.Errorf("inuse sample = %+v, want live stack value 4096", inuse[0])
	}
	if got := len(sampleByType(d, TypeHeapAllocSpace)); got != 2 {
		t.Errorf("alloc_space samples = %d, want 2", got)
	}
}

func TestPprofDecoder_UnsupportedTypesYieldNoSamples(t *testing.T) {
	main := fn(1, "main.main", "main.go")
	lMain := loc(1, profile.Line{Function: main, Line: 10})
	p := &profile.Profile{
		SampleType: []*profile.ValueType{{Type: "goroutine", Unit: "count"}},
		PeriodType: &profile.ValueType{Type: "goroutine", Unit: "count"},
		Period:     1,
		TimeNanos:  1_700_000_000_000_000_000,
		Sample:     []*profile.Sample{{Location: []*profile.Location{lMain}, Value: []int64{7}}},
		Function:   []*profile.Function{main},
		Location:   []*profile.Location{lMain},
	}
	out, err := PprofDecoder{}.Decode(testIngest, writeProfile(t, p))
	if err != nil {
		t.Fatalf("unexpected error on unsupported profile: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 decoded profile, got %d", len(out))
	}
	if len(out[0].Samples) != 0 {
		t.Errorf("expected 0 samples for unsupported types, got %d", len(out[0].Samples))
	}
}

func TestPprofDecoder_ZeroDurationEndEqualsStart(t *testing.T) {
	main := fn(1, "main.main", "main.go")
	lMain := loc(1, profile.Line{Function: main, Line: 10})
	start := int64(1_700_000_000_000_000_000)
	p := &profile.Profile{
		SampleType: []*profile.ValueType{{Type: "inuse_space", Unit: "bytes"}},
		PeriodType: &profile.ValueType{Type: "space", Unit: "bytes"},
		Period:     524288,
		TimeNanos:  start,
		Sample:     []*profile.Sample{{Location: []*profile.Location{lMain}, Value: []int64{4096}}},
		Function:   []*profile.Function{main},
		Location:   []*profile.Location{lMain},
	}
	d := decodeOne(t, writeProfile(t, p))
	if !d.Meta.End.Equal(d.Meta.Start) {
		t.Errorf("End=%v Start=%v; want equal for zero-duration profile", d.Meta.End, d.Meta.Start)
	}
}

func TestPprofDecoder_NoTimestampFallsBackToReceivedAt(t *testing.T) {
	main := fn(1, "main.main", "main.go")
	lMain := loc(1, profile.Line{Function: main, Line: 10})
	samples := []*profile.Sample{{Location: []*profile.Location{lMain}, Value: []int64{1, 100}}}
	payload := cpuProfile(t, 0, 0, samples, []*profile.Function{main}, []*profile.Location{lMain})

	d := decodeOne(t, payload)
	if !d.Meta.Start.Equal(testIngest.ReceivedAt) {
		t.Errorf("Start=%v, want fallback to ReceivedAt=%v", d.Meta.Start, testIngest.ReceivedAt)
	}
}

func TestPprofDecoder_RejectsGarbage(t *testing.T) {
	if _, err := (PprofDecoder{}).Decode(testIngest, []byte("not a pprof profile")); err == nil {
		t.Fatalf("expected error decoding garbage, got nil")
	}
}

func TestHashFrames(t *testing.T) {
	a := HashFrames([]string{"main.main", "main.work"})
	b := HashFrames([]string{"main.main", "main.work"})
	c := HashFrames([]string{"main.work", "main.main"})
	if a != b {
		t.Errorf("identical frame sequences hashed differently")
	}
	if a == c {
		t.Errorf("frame order must affect the hash")
	}
}
