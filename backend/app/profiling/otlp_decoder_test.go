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
	attributes  []*profilespb.KeyValueAndUnit
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

func (b *otlpBuilder) attr(key, value string) int32 {
	idx := int32(len(b.attributes))
	b.attributes = append(b.attributes, &profilespb.KeyValueAndUnit{
		KeyStrindex: b.str(key),
		Value:       &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: value}},
	})
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
			StringTable:    b.strings,
			FunctionTable:  b.functions,
			LocationTable:  b.locations,
			StackTable:     b.stacks,
			AttributeTable: b.attributes,
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

func otlpSampleAttr(stackIdx int32, value int64, attrIdxs ...int32) *profilespb.Sample {
	return &profilespb.Sample{StackIndex: stackIdx, Values: []int64{value}, AttributeIndices: attrIdxs}
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
	if samples[sampleKey{"cpu", wantWork}] != 300 {
		t.Errorf("main->work cpu = %d, want 300", samples[sampleKey{"cpu", wantWork}])
	}
	if samples[sampleKey{"cpu", wantIdle}] != 100 {
		t.Errorf("main->idle cpu = %d, want 100", samples[sampleKey{"cpu", wantIdle}])
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
	if samples[sampleKey{"cpu", want}] != 250 {
		t.Errorf("summed value = %d, want 250", samples[sampleKey{"cpu", want}])
	}
}

func TestOTLPDecoder_Heap_KeepsObjectAndSpaceTypes(t *testing.T) {
	b := newOTLPBuilder()
	b.serviceName = "checkout"
	s := b.frameStack("main.main", "main.allocate")
	b.profile("inuse_space", "bytes", 1_700_000_000_000_000_000, 0, otlpSample(s, 2048))
	b.profile("alloc_space", "bytes", 1_700_000_000_000_000_000, 0, otlpSample(s, 4096))
	b.profile("inuse_objects", "count", 1_700_000_000_000_000_000, 0, otlpSample(s, 2))
	b.profile("alloc_objects", "count", 1_700_000_000_000_000_000, 0, otlpSample(s, 5))

	out := decodeOTLPAll(t, b.build(t))
	samples := flattenSamples(out)

	want := HashFrames([]string{"main.main", "main.allocate"})
	expected := map[string]int64{
		"inuse_space":   2048,
		"alloc_space":   4096,
		"inuse_objects": 2,
		"alloc_objects": 5,
	}
	for typ, wantValue := range expected {
		if samples[sampleKey{typ, want}] != wantValue {
			t.Errorf("%s = %d, want %d", typ, samples[sampleKey{typ, want}], wantValue)
		}
	}
	if len(samples) != len(expected) {
		t.Errorf("emitted %d sample types, want %d", len(samples), len(expected))
	}
}

func TestOTLPDecoder_GoroutineMutexBlock(t *testing.T) {
	cases := []struct {
		sampleTyp string
		unit      string
		internal  string
	}{
		{"goroutine", "count", "goroutine"},
		{"mutex", "nanoseconds", "mutex"},
		{"block", "nanoseconds", "block"},
	}
	for _, tc := range cases {
		t.Run(tc.sampleTyp, func(t *testing.T) {
			b := newOTLPBuilder()
			b.serviceName = "checkout"
			s := b.frameStack("main.main", "sync.(*Mutex).Lock")
			b.profile(tc.sampleTyp, tc.unit, 1_700_000_000_000_000_000, 0, otlpSample(s, 42))

			out := decodeOTLPAll(t, b.build(t))
			samples := flattenSamples(out)
			want := HashFrames([]string{"main.main", "sync.(*Mutex).Lock"})
			if samples[sampleKey{tc.internal, want}] != 42 {
				t.Errorf("%s = %d, want 42", tc.internal, samples[sampleKey{tc.internal, want}])
			}
		})
	}
}

func TestOTLPDecoder_KeepsArbitraryCrossLanguageType(t *testing.T) {
	b := newOTLPBuilder()
	b.serviceName = "checkout"
	s := b.frameStack("main.main", "sync.(*Mutex).Lock")
	b.profile("lock", "nanoseconds", 1_700_000_000_000_000_000, 0, otlpSample(s, 42))

	out := decodeOTLPAll(t, b.build(t))
	var got []Sample
	for _, d := range out {
		got = append(got, d.Samples...)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 sample for arbitrary type, got %d", len(got))
	}
	if got[0].Type != "lock" || got[0].Unit != "nanoseconds" || got[0].Value != 42 {
		t.Errorf("sample = %+v, want type=lock unit=nanoseconds value=42", got[0])
	}
	if got[0].IsGauge {
		t.Errorf("lock should not be a gauge by name heuristic")
	}
}

func TestOTLPDecoder_GaugeFromNameForInuse(t *testing.T) {
	b := newOTLPBuilder()
	b.serviceName = "checkout"
	s := b.frameStack("main.main", "main.alloc")
	b.profile("inuse_space", "bytes", 1_700_000_000_000_000_000, 0, otlpSample(s, 2048))
	b.profile("goroutine", "count", 1_700_000_000_000_000_000, 0, otlpSample(s, 7))

	out := decodeOTLPAll(t, b.build(t))
	byType := map[string]Sample{}
	for _, d := range out {
		for _, smp := range d.Samples {
			byType[smp.Type] = smp
		}
	}
	if !byType["inuse_space"].IsGauge {
		t.Errorf("inuse_space should be a gauge (name heuristic)")
	}
	if !byType["goroutine"].IsGauge {
		t.Errorf("goroutine should be a gauge (name heuristic)")
	}
	if byType["inuse_space"].Unit != "bytes" {
		t.Errorf("inuse_space unit = %q, want bytes", byType["inuse_space"].Unit)
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
	if samples[sampleKey{"cpu", want}] != 3 {
		t.Errorf("value = %d, want 3 (one per timestamp when Values is empty)", samples[sampleKey{"cpu", want}])
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

	pprofSmpl := cpuOnly(flattenSamples([]Decoded{fromPprof}))
	otlpSmpl := cpuOnly(flattenSamples(fromOTLP))
	if len(pprofSmpl) != len(otlpSmpl) {
		t.Fatalf("cpu sample count differs: pprof=%d otlp=%d", len(pprofSmpl), len(otlpSmpl))
	}
	for k, v := range pprofSmpl {
		if otlpSmpl[k] != v {
			t.Errorf("sample %+v: pprof=%d otlp=%d", k, v, otlpSmpl[k])
		}
	}
}

func cpuOnly(m map[sampleKey]int64) map[sampleKey]int64 {
	out := map[sampleKey]int64{}
	for k, v := range m {
		if k.typ == "cpu" {
			out[k] = v
		}
	}
	return out
}

func TestOTLPDecoder_ExtractsAllowlistedLabels(t *testing.T) {
	b := newOTLPBuilder()
	b.serviceName = "checkout"
	s := b.frameStack("main.main", "main.work")
	epCheckout := b.attr("endpoint", "/checkout")
	epCart := b.attr("endpoint", "/cart")
	tenant := b.attr("tenant", "acme")
	reqID := b.attr("request_id", "abc-123")
	b.profile("cpu", "nanoseconds", 1_700_000_000_000_000_000, 1_000_000_000,
		otlpSampleAttr(s, 300, epCheckout, tenant, reqID),
		otlpSampleAttr(s, 100, epCart),
		otlpSampleAttr(s, 50, epCheckout, tenant, reqID),
	)

	out, err := OTLPDecoder{}.Decode(IngestContext{
		DefaultServiceName: "checkout",
		ReceivedAt:         testIngest.ReceivedAt,
		LabelAllowlist:     NewLabelAllowSet([]string{"tenant"}),
	}, b.build(t))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	var samples []Sample
	for _, d := range out {
		samples = append(samples, d.Samples...)
	}
	if len(samples) != 2 {
		t.Fatalf("expected 2 label-separated samples, got %d", len(samples))
	}
	byEndpoint := map[string]Sample{}
	for _, smp := range samples {
		byEndpoint[smp.Labels[LabelEndpoint]] = smp
	}
	checkout := byEndpoint["/checkout"]
	if checkout.Value != 350 {
		t.Errorf("/checkout value = %d, want 350 (same stack+labels summed)", checkout.Value)
	}
	if checkout.Labels["tenant"] != "acme" {
		t.Errorf("tenant label = %q, want acme (additional allowlisted key kept)", checkout.Labels["tenant"])
	}
	if _, ok := checkout.Labels["request_id"]; ok {
		t.Errorf("non-allowlisted request_id must be dropped, got %v", checkout.Labels)
	}
	if byEndpoint["/cart"].Value != 100 {
		t.Errorf("/cart value = %d, want 100", byEndpoint["/cart"].Value)
	}
	if got := len(flattenStacks(out)); got != 1 {
		t.Errorf("unique stacks = %d, want 1 (label-separated samples still share the stack row)", got)
	}
}

func TestOTLPDecoder_DropsNonStringAttributes(t *testing.T) {
	b := newOTLPBuilder()
	b.serviceName = "checkout"
	s := b.frameStack("main.main", "main.work")
	ep := b.attr("endpoint", "/checkout")
	numIdx := int32(len(b.attributes))
	b.attributes = append(b.attributes, &profilespb.KeyValueAndUnit{
		KeyStrindex: b.str("request_count"),
		Value:       &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: 42}},
	})
	b.profile("cpu", "nanoseconds", 1_700_000_000_000_000_000, 1_000_000_000,
		otlpSampleAttr(s, 100, ep, numIdx),
	)

	out, err := OTLPDecoder{}.Decode(IngestContext{
		DefaultServiceName: "checkout",
		ReceivedAt:         testIngest.ReceivedAt,
		LabelAllowlist:     NewLabelAllowSet([]string{"request_count"}),
	}, b.build(t))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	var labels map[string]string
	for _, d := range out {
		for _, smp := range d.Samples {
			labels = smp.Labels
		}
	}
	if labels[LabelEndpoint] != "/checkout" {
		t.Errorf("labels = %v, want endpoint /checkout", labels)
	}
	if _, ok := labels["request_count"]; ok {
		t.Errorf("non-string IntValue attribute must be dropped even when allowlisted, got %v", labels)
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
