package services

import (
	"sort"

	"github.com/tracewayapp/traceway/backend/app/models"
)

type FunctionStat struct {
	Name    string  `json:"name"`
	Flat    int64   `json:"flat"`
	Cum     int64   `json:"cum"`
	FlatPct float64 `json:"flatPct"`
	CumPct  float64 `json:"cumPct"`
}

func TopFunctions(stacks []models.ProfileStackValue, limit int) []FunctionStat {
	flat := map[string]int64{}
	cum := map[string]int64{}
	var total int64

	for _, s := range stacks {
		total += s.Value
		if len(s.Stack) > 0 {
			flat[s.Stack[len(s.Stack)-1]] += s.Value
		}
		seen := map[string]struct{}{}
		for _, frame := range s.Stack {
			if _, ok := seen[frame]; ok {
				continue
			}
			seen[frame] = struct{}{}
			cum[frame] += s.Value
		}
	}

	names := map[string]struct{}{}
	for n := range flat {
		names[n] = struct{}{}
	}
	for n := range cum {
		names[n] = struct{}{}
	}

	out := make([]FunctionStat, 0, len(names))
	for n := range names {
		stat := FunctionStat{Name: n, Flat: flat[n], Cum: cum[n]}
		if total > 0 {
			stat.FlatPct = float64(stat.Flat) / float64(total) * 100
			stat.CumPct = float64(stat.Cum) / float64(total) * 100
		}
		out = append(out, stat)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Flat != out[j].Flat {
			return out[i].Flat > out[j].Flat
		}
		if out[i].Cum != out[j].Cum {
			return out[i].Cum > out[j].Cum
		}
		return out[i].Name < out[j].Name
	})

	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
