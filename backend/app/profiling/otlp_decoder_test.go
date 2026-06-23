package profiling

import (
	"math"
	"testing"
	"time"

	"github.com/google/pprof/profile"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	colprofilespb "go.opentelemetry.io/proto/otlp/collector/profiles/v1development"
	profilespb "go.opentelemetry.io/proto/otlp/profiles/v1development"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/proto"
)

type otlpBuilder struct {
	serviceName string
	strings     []string
	functions   []*profilespb.Function
	locations   []*profilespb.Location
	stacks      []*profilespb.Stack
	profiles    []*profilespb.Profile
}

func newOTLPBuilder() *otlpBuilder {
	return &otlpBuilder{
		strings:   []string{""},
		functions: []*profilespb.Function{{}},
		locations: []*profilespb.Location{{}},
		stacks:    []*profilespb.Stack{{}},
	}
}

func (b *otlpBuilder) str(s string) int32 {
	for i, existing := range b.strings {
		if existing == s {
			return int32(i)
		}
	}
	b.strings = append(b.strings, s)
	return int32(len(b.strings) - 1)
}

func (b *otlpBuilder) fn(name string) int32 {
	idx := int32(len(b.functions))
	b.functions = append(b.functions, &profilespb.Function{
		NameStrindex:       b.str(name),
		SystemNameStrindex: b.str(name),
	})
	return idx
}

func (b *otlpBuilder) loc(fnIdx int32) int32 {
	idx := int32(len(b.locations))
	b.locations = append(b.locations, &profilespb.Location{
		Lines: []*profilespb.Line{{FunctionIndex: fnIdx}},
	})
	return idx
}

func (b *otlpBuilder) frameStack(rootFirst ...string) int32 {
	var locsLeafFirst []int32
	for i := len(rootFirst) - 1; i >= 0; i-- {
		locsLeafFirst = append(locsLeafFirst, b.loc(b.fn(rootFirst[i])))
	}
	idx := int32(len(b.stacks))
	b.stacks = append(b.stacks, &profilespb.Stack{LocationIndices: locsLeafFirst})
	return idx
}

func (b *otlpBuilder) inlinedStack(rootFirst ...string) int32 {
	var lines []*profilespb.Line
	for i := len(rootFirst) - 1; i >= 0; i-- {
		lines = append(lines, &profilespb.Line{FunctionIndex: b.fn(rootFirst[i])})
	}
	locIdx := int32(len(b.locations))
	b.locations = append(b.locations, &profilespb.Location{Lines: lines})
	idx := int32(len(b.stacks))
	b.stacks = append(b.stacks, &profilespb.Stack{LocationIndices: []int32{locIdx}})
	return idx
}

func (b *otlpBuilder) profile(typeName, unit string, timeNanos, durNanos uint64, samples ...*profilespb.Sample) {
	b.profiles = append(b.profiles, &profilespb.Profile{
		SampleType:   &profilespb.ValueType{TypeStrindex: b.str(typeName), UnitStrindex: b.str(unit)},
		TimeUnixNano: timeNanos,
		DurationNano: durNanos,
		Samples:      samples,
	})
}

func (b *otlpBuilder) build(t *testing.T) []byte {
	t.Helper()
	var res *resourcepb.Resource
	if b.serviceName != "" {
		res = &resourcepb.Resource{Attributes: []*commonpb.KeyValue{{
			Key:   "service.name",
			Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: b.serviceName}},
		}}}
	}
	req := &colprofilespb.ExportProfilesServiceRequest{
		Dictionary: &profilespb.ProfilesDictionary{
			StringTable:   b.strings,
			FunctionTable: b.functions,
			LocationTable: b.locations,
			StackTable:    b.stacks,
		},
		ResourceProfiles: []*profilespb.ResourceProfiles{{
			Resource: res,
			ScopeProfiles: []*profilespb.ScopeProfiles{{
				Profiles: b.profiles,
			}},
		}},
	}
	data, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal export request: %v", err)
	}
	return data
}

func otlpSample(stackIdx int32, value int64) *profilespb.Sample {
	return &profilespb.Sample{StackIndex: stackIdx, Values: []int64{value}}
}

func decodeOTLPAll(t *testing.T, payload []byte) []Decoded {
	t.Helper()
	out, err := OTLPDecoder{}.Decode(testIngest, payload)
	if err != nil {
		t.Fatalf("OTLPDecoder.Decode error: %v", err)
	}
	return out
}

func flattenStacks(ds []Decoded) map[uint64][]string {
	out := map[uint64][]string{}
	for _, d := range ds {
		for _, s := range d.Stacks {
			out[s.Hash] = s.Frames
		}
	}
	return out
}

func flattenSamples(ds []Decoded) map[sampleKey]int64 {
	out := map[sampleKey]int64{}
	for _, d := range ds {
		for _, s := range d.Samples {
			out[sampleKey{typ: s.Type, stackHash: s.StackHash}] += s.Value
		}
	}
	return out
}

func TestOTLPDecoder_CPU_ExplodesDistinctStacks(t *testing.T) {
	b := newOTLPBuilder()
	b.serviceName = "checkout"
	sWork := b.frameStack("main.main", "main.work")
	sIdle := b.frameStack("main.main", "main.idle")
	b.profile("cpu", "nanoseconds", 1_700_000_000_000_000_000, 5_000_000_000,
		otlpSample(sWork, 300), otlpSample(sIdle, 100))

	out := decodeOTLPAll(t, b.build(t))
	stacks := flattenStacks(out)
	samples := flattenSamples(out)

	if len(stacks) != 2 {
		t.Fatalf("expected 2 unique stacks, got %d", len(stacks))
	}
	if len(samples) != 2 {
		t.Fatalf("expected 2 cpu samples, got %d", len(samples))
	}

	wantWork := HashFrames([]string{"main.main", "main.work"})
	wantIdle := HashFrames([]string{"main.main", "main.idle"})
	if samples[sampleKey{TypeCPUNanos, wantWork}] != 300 {
		t.Errorf("main->work cpu = %d, want 300", samples[sampleKey{TypeCPUNanos, wantWork}])
	}
	if samples[sampleKey{TypeCPUNanos, wantIdle}] != 100 {
		t.Errorf("main->idle cpu = %d, want 100", samples[sampleKey{TypeCPUNanos, wantIdle}])
	}
	if got := stacks[wantWork]; len(got) != 2 || got[0] != "main.main" || got[1] != "main.work" {
		t.Errorf("frames = %v, want [main.main main.work] (root-first)", got)
	}
}

func TestOTLPDecoder_DedupesIdenticalStacks(t *testing.T) {
	b := newOTLPBuilder()
	b.serviceName = "checkout"
	s := b.frameStack("main.main", "main.work")
	b.profile("cpu", "nanoseconds", 1_700_000_000_000_000_000, 1_000_000_000,
		otlpSample(s, 200), otlpSample(s, 50))

	out := decodeOTLPAll(t, b.build(t))
	stacks := flattenStacks(out)
	samples := flattenSamples(out)

	if len(stacks) != 1 {
		t.Errorf("expected 1 unique stack, got %d", len(stacks))
	}
	if len(samples) != 1 {
		t.Fatalf("expected 1 deduped sample, got %d", len(samples))
	}
	want := HashFrames([]string{"main.main", "main.work"})
	if samples[sampleKey{TypeCPUNanos, want}] != 250 {
		t.Errorf("summed value = %d, want 250", samples[sampleKey{TypeCPUNanos, want}])
	}
}

func TestOTLPDecoder_Heap_KeepsSpaceTypesOnly(t *testing.T) {
	b := newOTLPBuilder()
	b.serviceName = "checkout"
	s := b.frameStack("main.main", "main.allocate")
	b.profile("inuse_space", "bytes", 1_700_000_000_000_000_000, 0, otlpSample(s, 2048))
	b.profile("alloc_space", "bytes", 1_700_000_000_000_000_000, 0, otlpSample(s, 4096))
	b.profile("inuse_objects", "count", 1_700_000_000_000_000_000, 0, otlpSample(s, 2))

	out := decodeOTLPAll(t, b.build(t))
	samples := flattenSamples(out)

	want := HashFrames([]string{"main.main", "main.allocate"})
	if samples[sampleKey{TypeHeapInuseSpace, want}] != 2048 {
		t.Errorf("inuse_space = %d, want 2048", samples[sampleKey{TypeHeapInuseSpace, want}])
	}
	if samples[sampleKey{TypeHeapAllocSpace, want}] != 4096 {
		t.Errorf("alloc_space = %d, want 4096", samples[sampleKey{TypeHeapAllocSpace, want}])
	}
	for k := range samples {
		if k.typ != TypeHeapInuseSpace && k.typ != TypeHeapAllocSpace {
			t.Errorf("unexpected sample type emitted: %q", k.typ)
		}
	}
}

func TestOTLPDecoder_UnsupportedTypeYieldsNoSamples(t *testing.T) {
	b := newOTLPBuilder()
	b.serviceName = "checkout"
	s := b.frameStack("main.main")
	b.profile("goroutine", "count", 1_700_000_000_000_000_000, 0, otlpSample(s, 7))

	out := decodeOTLPAll(t, b.build(t))
	if got := len(flattenSamples(out)); got != 0 {
		t.Errorf("expected 0 samples for unsupported type, got %d", got)
	}
}

func TestOTLPDecoder_TimestampsOnlySampleCountsTimestamps(t *testing.T) {
	b := newOTLPBuilder()
	b.serviceName = "checkout"
	s := b.frameStack("main.main", "main.work")
	b.profile("cpu", "nanoseconds", 1_700_000_000_000_000_000, 1_000_000_000,
		&profilespb.Sample{StackIndex: s, TimestampsUnixNano: []uint64{1, 2, 3}})

	out := decodeOTLPAll(t, b.build(t))
	samples := flattenSamples(out)
	want := HashFrames([]string{"main.main", "main.work"})
	if samples[sampleKey{TypeCPUNanos, want}] != 3 {
		t.Errorf("value = %d, want 3 (one per timestamp when Values is empty)", samples[sampleKey{TypeCPUNanos, want}])
	}
}

func TestOTLPDecoder_HugeDurationDoesNotGoNegative(t *testing.T) {
	b := newOTLPBuilder()
	b.serviceName = "checkout"
	start := uint64(1_700_000_000_000_000_000)
	s := b.frameStack("main.main")
	b.profile("cpu", "nanoseconds", start, math.MaxUint64, otlpSample(s, 100))

	out := decodeOTLPAll(t, b.build(t))
	if len(out) != 1 {
		t.Fatalf("expected 1 decoded profile, got %d", len(out))
	}
	if out[0].Meta.End.Before(out[0].Meta.Start) {
		t.Errorf("end %v is before start %v on overflowing duration", out[0].Meta.End, out[0].Meta.Start)
	}
}

func TestOTLPDecoder_InlinedFramesExpand(t *testing.T) {
	b := newOTLPBuilder()
	b.serviceName = "checkout"
	s := b.inlinedStack("fmt.Printf", "runtime.memmove")
	b.profile("cpu", "nanoseconds", 1_700_000_000_000_000_000, 1_000_000_000, otlpSample(s, 100))

	out := decodeOTLPAll(t, b.build(t))
	stacks := flattenStacks(out)
	want := HashFrames([]string{"fmt.Printf", "runtime.memmove"})
	got, ok := stacks[want]
	if !ok || len(got) != 2 || got[0] != "fmt.Printf" || got[1] != "runtime.memmove" {
		t.Errorf("frames = %v, want [fmt.Printf runtime.memmove] (root-first within location)", got)
	}
}

func TestOTLPDecoder_Metadata(t *testing.T) {
	b := newOTLPBuilder()
	b.serviceName = "orders"
	start := uint64(1_700_000_000_000_000_000)
	dur := uint64(30_000_000_000)
	s := b.frameStack("main.main")
	b.profile("cpu", "nanoseconds", start, dur, otlpSample(s, 100))

	out := decodeOTLPAll(t, b.build(t))
	if len(out) != 1 {
		t.Fatalf("expected 1 decoded profile, got %d", len(out))
	}
	meta := out[0].Meta
	if meta.ServiceName != "orders" {
		t.Errorf("service name = %q, want orders (resource wins over default)", meta.ServiceName)
	}
	if meta.ServerName != "pod-abc" || meta.AppVersion != "1.2.3" {
		t.Errorf("server/version not carried from ingest context: %+v", meta)
	}
	wantStart := time.Unix(0, int64(start)).UTC()
	if !meta.Start.Equal(wantStart) {
		t.Errorf("start = %v, want %v", meta.Start.UTC(), wantStart)
	}
	if !meta.End.Equal(wantStart.Add(30 * time.Second)) {
		t.Errorf("end = %v, want start+30s", meta.End.UTC())
	}
}

func TestOTLPDecoder_NoServiceNameFallsBackToDefault(t *testing.T) {
	b := newOTLPBuilder()
	s := b.frameStack("main.main")
	b.profile("cpu", "nanoseconds", 1_700_000_000_000_000_000, 0, otlpSample(s, 100))

	out := decodeOTLPAll(t, b.build(t))
	if len(out) != 1 {
		t.Fatalf("expected 1 decoded profile, got %d", len(out))
	}
	if out[0].Meta.ServiceName != testIngest.DefaultServiceName {
		t.Errorf("service name = %q, want fallback %q", out[0].Meta.ServiceName, testIngest.DefaultServiceName)
	}
}

func TestOTLPDecoder_NoTimestampFallsBackToReceivedAt(t *testing.T) {
	b := newOTLPBuilder()
	b.serviceName = "checkout"
	s := b.frameStack("main.main")
	b.profile("cpu", "nanoseconds", 0, 0, otlpSample(s, 100))

	out := decodeOTLPAll(t, b.build(t))
	if len(out) != 1 {
		t.Fatalf("expected 1 decoded profile, got %d", len(out))
	}
	if !out[0].Meta.Start.Equal(testIngest.ReceivedAt) {
		t.Errorf("start = %v, want fallback to ReceivedAt %v", out[0].Meta.Start, testIngest.ReceivedAt)
	}
}

func TestOTLPDecoder_RejectsGarbage(t *testing.T) {
	if _, err := (OTLPDecoder{}).Decode(testIngest, []byte("not a valid export request")); err == nil {
		t.Fatalf("expected error decoding garbage, got nil")
	}
}

func TestDecoderParity_PprofVsOTLP(t *testing.T) {
	main := fn(1, "main.main", "main.go")
	work := fn(2, "main.work", "work.go")
	idle := fn(3, "main.idle", "idle.go")
	lMain := loc(1, profile.Line{Function: main, Line: 10})
	lWork := loc(2, profile.Line{Function: work, Line: 20})
	lIdle := loc(3, profile.Line{Function: idle, Line: 30})
	pprofSamples := []*profile.Sample{
		{Location: []*profile.Location{lWork, lMain}, Value: []int64{1, 300}},
		{Location: []*profile.Location{lIdle, lMain}, Value: []int64{1, 100}},
	}
	pprofBytes := cpuProfile(t, 1_700_000_000_000_000_000, 5_000_000_000, pprofSamples,
		[]*profile.Function{main, work, idle}, []*profile.Location{lMain, lWork, lIdle})

	b := newOTLPBuilder()
	b.serviceName = "checkout"
	sWork := b.frameStack("main.main", "main.work")
	sIdle := b.frameStack("main.main", "main.idle")
	b.profile("cpu", "nanoseconds", 1_700_000_000_000_000_000, 5_000_000_000,
		otlpSample(sWork, 300), otlpSample(sIdle, 100))

	fromPprof := decodeOne(t, pprofBytes)
	fromOTLP := decodeOTLPAll(t, b.build(t))

	pprofStacks := flattenStacks([]Decoded{fromPprof})
	otlpStacks := flattenStacks(fromOTLP)
	if !stacksEqual(pprofStacks, otlpStacks) {
		t.Errorf("stack rows differ:\npprof=%v\notlp=%v", pprofStacks, otlpStacks)
	}

	pprofSmpl := flattenSamples([]Decoded{fromPprof})
	otlpSmpl := flattenSamples(fromOTLP)
	if len(pprofSmpl) != len(otlpSmpl) {
		t.Fatalf("sample count differs: pprof=%d otlp=%d", len(pprofSmpl), len(otlpSmpl))
	}
	for k, v := range pprofSmpl {
		if otlpSmpl[k] != v {
			t.Errorf("sample %+v: pprof=%d otlp=%d", k, v, otlpSmpl[k])
		}
	}
}

func stacksEqual(a, b map[uint64][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for h, fa := range a {
		fb, ok := b[h]
		if !ok || len(fa) != len(fb) {
			return false
		}
		for i := range fa {
			if fa[i] != fb[i] {
				return false
			}
		}
	}
	return true
}
