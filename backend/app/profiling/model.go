package profiling

import (
	"hash/fnv"
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

func reverseInPlace(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
