package profiling

import (
	"fmt"
	"time"

	colprofilespb "go.opentelemetry.io/proto/otlp/collector/profiles/v1development"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	profilespb "go.opentelemetry.io/proto/otlp/profiles/v1development"
	"google.golang.org/protobuf/proto"
)

type OTLPDecoder struct{}

func (OTLPDecoder) Decode(ctx IngestContext, payload []byte) ([]Decoded, error) {
	req := &colprofilespb.ExportProfilesServiceRequest{}
	if err := proto.Unmarshal(payload, req); err != nil {
		return nil, fmt.Errorf("profiling: unmarshal otlp profiles: %w", err)
	}
	if req.Dictionary == nil {
		return nil, nil
	}
	resolver := newStackResolver(req.Dictionary)

	var out []Decoded
	for _, rp := range req.ResourceProfiles {
		serviceName := ctx.DefaultServiceName
		if rp.Resource != nil {
			if sn := resourceServiceName(rp.Resource.Attributes); sn != "" {
				serviceName = sn
			}
		}
		for _, sp := range rp.ScopeProfiles {
			for _, p := range sp.Profiles {
				if decoded, ok := decodeProfile(ctx, serviceName, resolver, p); ok {
					out = append(out, decoded)
				}
			}
		}
	}
	return out, nil
}

func decodeProfile(ctx IngestContext, serviceName string, resolver *stackResolver, p *profilespb.Profile) (Decoded, bool) {
	if p.SampleType == nil {
		return Decoded{}, false
	}
	internalType, ok := keptSampleTypes[resolver.stringAt(p.SampleType.TypeStrindex)]
	if !ok {
		return Decoded{}, false
	}

	start := ctx.ReceivedAt
	if p.TimeUnixNano > 0 {
		start = time.Unix(0, int64(p.TimeUnixNano)).UTC()
	}
	end := start
	if dur := int64(p.DurationNano); dur > 0 {
		end = start.Add(time.Duration(dur))
	}

	values := make(map[dedupKey]*dedupValue)
	stacks := make(map[uint64][]string)
	for _, s := range p.Samples {
		rs := resolver.lookup(s.StackIndex)
		if len(rs.frames) == 0 {
			continue
		}
		var v int64
		if len(s.Values) == 0 {
			v = int64(len(s.TimestampsUnixNano))
		} else {
			for _, val := range s.Values {
				v += val
			}
		}
		if v == 0 {
			continue
		}
		labels := resolver.attributes(s.AttributeIndices, ctx.LabelAllowlist)
		key := dedupKey{typ: internalType, stackHash: rs.hash, labelHash: labelFingerprint(labels)}
		agg := values[key]
		if agg == nil {
			agg = &dedupValue{labels: labels}
			values[key] = agg
		}
		agg.value += v
		stacks[rs.hash] = rs.frames
	}

	decoded := Decoded{Meta: Meta{
		ServiceName: serviceName,
		ServerName:  ctx.ServerName,
		AppVersion:  ctx.AppVersion,
		Start:       start,
		End:         end,
	}}
	decoded.Stacks = make([]Stack, 0, len(stacks))
	decoded.Samples = make([]Sample, 0, len(values))
	for hash, frames := range stacks {
		decoded.Stacks = append(decoded.Stacks, Stack{Hash: hash, Frames: frames})
	}
	for key, agg := range values {
		decoded.Samples = append(decoded.Samples, Sample{Type: key.typ, StackHash: key.stackHash, Value: agg.value, Labels: agg.labels})
	}
	return decoded, true
}

func resourceServiceName(attrs []*commonpb.KeyValue) string {
	for _, kv := range attrs {
		if kv.Key == "service.name" && kv.Value != nil {
			if sv, ok := kv.Value.Value.(*commonpb.AnyValue_StringValue); ok {
				return sv.StringValue
			}
		}
	}
	return ""
}

type resolvedStack struct {
	frames []string
	hash   uint64
}

type stackResolver struct {
	dict  *profilespb.ProfilesDictionary
	cache map[int32]resolvedStack
}

func newStackResolver(dict *profilespb.ProfilesDictionary) *stackResolver {
	return &stackResolver{dict: dict, cache: make(map[int32]resolvedStack)}
}

func (r *stackResolver) attributes(indices []int32, allow map[string]struct{}) map[string]string {
	if len(indices) == 0 || len(allow) == 0 {
		return nil
	}
	var out map[string]string
	for _, idx := range indices {
		if idx < 0 || int(idx) >= len(r.dict.AttributeTable) {
			continue
		}
		kv := r.dict.AttributeTable[idx]
		if kv == nil || kv.Value == nil {
			continue
		}
		key := r.stringAt(kv.KeyStrindex)
		if key == "" {
			continue
		}
		if _, ok := allow[key]; !ok {
			continue
		}
		sv, ok := kv.Value.Value.(*commonpb.AnyValue_StringValue)
		if !ok {
			continue
		}
		if out == nil {
			out = make(map[string]string)
		}
		out[key] = sv.StringValue
	}
	return out
}

func (r *stackResolver) stringAt(i int32) string {
	if i < 0 || int(i) >= len(r.dict.StringTable) {
		return ""
	}
	return r.dict.StringTable[i]
}

func (r *stackResolver) lookup(stackIndex int32) resolvedStack {
	if cached, ok := r.cache[stackIndex]; ok {
		return cached
	}
	frames := r.resolve(stackIndex)
	rs := resolvedStack{frames: frames}
	if len(frames) > 0 {
		rs.hash = HashFrames(frames)
	}
	r.cache[stackIndex] = rs
	return rs
}

func (r *stackResolver) resolve(stackIndex int32) []string {
	if stackIndex <= 0 || int(stackIndex) >= len(r.dict.StackTable) {
		return nil
	}
	stack := r.dict.StackTable[stackIndex]
	if stack == nil {
		return nil
	}
	var leafToRoot []string
	for _, locIdx := range stack.LocationIndices {
		if locIdx <= 0 || int(locIdx) >= len(r.dict.LocationTable) {
			continue
		}
		loc := r.dict.LocationTable[locIdx]
		if loc == nil {
			continue
		}
		for _, line := range loc.Lines {
			if name := r.functionName(line.FunctionIndex); name != "" {
				leafToRoot = append(leafToRoot, name)
			}
		}
	}
	reverseInPlace(leafToRoot)
	return leafToRoot
}

func (r *stackResolver) functionName(funcIndex int32) string {
	if funcIndex <= 0 || int(funcIndex) >= len(r.dict.FunctionTable) {
		return ""
	}
	fn := r.dict.FunctionTable[funcIndex]
	if fn == nil {
		return ""
	}
	return r.stringAt(fn.NameStrindex)
}
