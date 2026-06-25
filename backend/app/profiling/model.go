package profiling

import (
	"hash/fnv"
	"sort"
	"strings"
	"time"
)

const (
	TypeCPUNanos       = "go:profile_cpu:nanoseconds"
	TypeHeapInuseSpace = "go:profile_heap:inuse_space"
	TypeHeapAllocSpace = "go:profile_heap:alloc_space"
)

var keptSampleTypes = map[string]string{
	"cpu":         TypeCPUNanos,
	"inuse_space": TypeHeapInuseSpace,
	"alloc_space": TypeHeapAllocSpace,
}

const LabelEndpoint = "endpoint"

const MaxLabelValuesPerKey = 1000

func NewLabelAllowSet(additional []string) map[string]struct{} {
	allow := map[string]struct{}{LabelEndpoint: {}}
	for _, k := range additional {
		k = strings.TrimSpace(k)
		if k != "" {
			allow[k] = struct{}{}
		}
	}
	return allow
}

func LabelAllowKeys(additional []string) []string {
	allow := NewLabelAllowSet(additional)
	keys := make([]string, 0, len(allow))
	for k := range allow {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func IsGauge(profileType string) bool {
	return profileType == TypeHeapInuseSpace
}

type Stack struct {
	Hash   uint64
	Frames []string
}

type Sample struct {
	Type      string
	StackHash uint64
	Value     int64
	Labels    map[string]string
}

type Meta struct {
	ServiceName string
	Start       time.Time
	End         time.Time
	ServerName  string
	AppVersion  string
	Attributes  map[string]string
	TraceId     *string
	SpanId      *string
}

type Decoded struct {
	Meta    Meta
	Stacks  []Stack
	Samples []Sample
}

func HashFrames(frames []string) uint64 {
	h := fnv.New64a()
	for _, f := range frames {
		_, _ = h.Write([]byte(f))
		_, _ = h.Write([]byte{0})
	}
	return h.Sum64()
}

func allowlistedLabels(raw map[string][]string, allow map[string]struct{}) map[string]string {
	var out map[string]string
	for k, v := range raw {
		if _, ok := allow[k]; !ok {
			continue
		}
		if len(v) == 0 {
			continue
		}
		if out == nil {
			out = make(map[string]string)
		}
		out[k] = v[0]
	}
	return out
}

func labelFingerprint(labels map[string]string) uint64 {
	if len(labels) == 0 {
		return 0
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := fnv.New64a()
	for _, k := range keys {
		_, _ = h.Write([]byte(k))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(labels[k]))
		_, _ = h.Write([]byte{0})
	}
	return h.Sum64()
}

func reverseInPlace(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
