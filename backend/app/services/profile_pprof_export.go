package services

import (
	"bytes"

	"github.com/google/pprof/profile"
	"github.com/tracewayapp/traceway/backend/app/models"
)

func BuildPprof(name, unit string, stacks []models.ProfileStackValue) ([]byte, error) {
	if name == "" {
		name = "samples"
	}
	if unit == "" {
		unit = "count"
	}

	p := &profile.Profile{
		SampleType: []*profile.ValueType{{Type: name, Unit: unit}},
	}

	funcs := map[string]*profile.Function{}
	locs := map[string]*profile.Location{}
	var nextID uint64 = 1

	locFor := func(frame string) *profile.Location {
		if l, ok := locs[frame]; ok {
			return l
		}
		f, ok := funcs[frame]
		if !ok {
			f = &profile.Function{ID: nextID, Name: frame, SystemName: frame}
			nextID++
			funcs[frame] = f
			p.Function = append(p.Function, f)
		}
		l := &profile.Location{ID: nextID, Line: []profile.Line{{Function: f}}}
		nextID++
		locs[frame] = l
		p.Location = append(p.Location, l)
		return l
	}

	for _, s := range stacks {
		if len(s.Stack) == 0 {
			continue
		}
		leafFirst := make([]*profile.Location, 0, len(s.Stack))
		for i := len(s.Stack) - 1; i >= 0; i-- {
			leafFirst = append(leafFirst, locFor(s.Stack[i]))
		}
		p.Sample = append(p.Sample, &profile.Sample{Location: leafFirst, Value: []int64{s.Value}})
	}

	if err := p.CheckValid(); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := p.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
