package profiling

import (
	"bytes"
	"fmt"
	"time"

	"github.com/google/pprof/profile"
)

type PprofDecoder struct{}

type sampleKey struct {
	typ       string
	stackHash uint64
}

func (PprofDecoder) Decode(ctx IngestContext, payload []byte) ([]Decoded, error) {
	p, err := profile.Parse(bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("profiling: parse pprof: %w", err)
	}

	type keptType struct {
		index int
		typ   string
	}
	var kept []keptType
	for i, st := range p.SampleType {
		if internal, ok := keptSampleTypes[st.Type]; ok {
			kept = append(kept, keptType{index: i, typ: internal})
		}
	}

	meta := Meta{
		ServiceName: ctx.DefaultServiceName,
		ServerName:  ctx.ServerName,
		AppVersion:  ctx.AppVersion,
		Start:       profileStart(p, ctx.ReceivedAt),
	}
	meta.End = meta.Start
	if p.DurationNanos > 0 {
		meta.End = meta.Start.Add(time.Duration(p.DurationNanos))
	}

	values := make(map[sampleKey]int64)
	stacks := make(map[uint64][]string)

	for _, s := range p.Sample {
		frames := rootFirstFrames(s)
		if len(frames) == 0 {
			continue
		}
		hash := HashFrames(frames)
		for _, k := range kept {
			if k.index >= len(s.Value) || s.Value[k.index] == 0 {
				continue
			}
			values[sampleKey{typ: k.typ, stackHash: hash}] += s.Value[k.index]
			stacks[hash] = frames
		}
	}

	decoded := Decoded{Meta: meta}
	for hash, frames := range stacks {
		decoded.Stacks = append(decoded.Stacks, Stack{Hash: hash, Frames: frames})
	}
	for key, v := range values {
		decoded.Samples = append(decoded.Samples, Sample{
			Type:      key.typ,
			StackHash: key.stackHash,
			Value:     v,
		})
	}

	return []Decoded{decoded}, nil
}

func profileStart(p *profile.Profile, fallback time.Time) time.Time {
	if p.TimeNanos > 0 {
		return time.Unix(0, p.TimeNanos).UTC()
	}
	return fallback
}

func rootFirstFrames(s *profile.Sample) []string {
	var leafToRoot []string
	for _, loc := range s.Location {
		for _, line := range loc.Line {
			if line.Function == nil {
				continue
			}
			leafToRoot = append(leafToRoot, line.Function.Name)
		}
	}
	reverseInPlace(leafToRoot)
	return leafToRoot
}
