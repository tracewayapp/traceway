package clientmetrics

import (
	"hash/fnv"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
)

// Other replaces a dimension value that is too long, outside its closed set, or past
// the cap of distinct values its dimension may have per project per day.
const Other = "other"

// DimensionCountry is set by the server from the request, never taken from the client.
const DimensionCountry = "country"

type dimensionRule struct {
	// cap is the number of distinct values a project may use per day.
	cap int
	// values is a closed set; when set, cap is not used.
	values []string
	path   bool
}

// The fixed dimension set. Never a session, user, device or instance id, and never a
// full URL: each key is bounded, so the number of series is.
var (
	resourceDimensions = map[string]dimensionRule{
		"service.version":             {cap: 50},
		"deployment.environment.name": {cap: 10},
		"os.name":                     {cap: 10},
		"os.major":                    {cap: 30},
		"browser.name":                {cap: 20},
		"browser.major":               {cap: 60},
		"browser.mobile":              {values: []string{"true", "false"}},
		"device.form_factor":          {values: []string{"phone", "tablet", "desktop"}},
		"device.class":                {values: []string{"low", "mid", "high"}},
		"server_name":                 {cap: 20},
	}
	pointDimensions = map[string]dimensionRule{
		"url.path":     {cap: 200, path: true},
		"screen":       {cap: 200},
		"network.type": {cap: 8},
	}
	// metricDimensions are accepted only on the metrics that list them.
	metricDimensions = map[string]dimensionRule{
		"host":   {cap: 50},
		"target": {cap: 200},
		"script": {cap: 100},
	}
	// foldOrder is what a project over its series budget loses first.
	foldOrder = []string{"target", "script", "url.path", "screen", "host"}
)

const (
	defaultSeriesBudget = 5000
	// maxPendingEntries bounds what waits for the state store while it is down.
	maxPendingEntries = 4096

	scopeDimension   = "dim:"
	scopeMetric      = "metric"
	scopeMetricKey   = "metric_key:"
	scopeGlobalKey   = "global_key"
	persistentDay    = 0
	secondsPerDay    = 86400
	countryCodeBytes = 2
)

// GuardEntry is one thing the guard admitted for a project: a dimension value of a
// day, or a registration that outlives the day (Day 0). Entries are what instances
// share, so the caps hold for a deployment and not per process, and survive a restart.
type GuardEntry struct {
	ProjectId uuid.UUID
	Day       int64
	Scope     string
	Value     string
}

type projectState struct {
	day    int64
	values map[string]map[string]struct{}
	series map[string]map[uint64]struct{}
	// custom metric names, their dimension keys and the global dimension keys are kept
	// across days: they are a registration, not a daily budget.
	custom     map[string][]string
	globalKeys []string
}

type guard struct {
	seriesBudget int
	shards       [32]guardShard
}

type guardShard struct {
	mu       sync.Mutex
	projects map[uuid.UUID]*projectState
	pending  []GuardEntry
}

func newGuard(seriesBudget int) *guard {
	if seriesBudget <= 0 {
		seriesBudget = defaultSeriesBudget
	}
	g := &guard{seriesBudget: seriesBudget}
	for i := range g.shards {
		g.shards[i].projects = make(map[uuid.UUID]*projectState)
	}
	return g
}

func (g *guard) shard(projectId uuid.UUID) *guardShard {
	return &g.shards[int(projectId[0])%len(g.shards)]
}

func (s *guardShard) state(projectId uuid.UUID, day int64) *projectState {
	state := s.projects[projectId]
	if state == nil {
		state = &projectState{custom: make(map[string][]string)}
		s.projects[projectId] = state
	}
	if state.day != day || state.values == nil {
		state.day = day
		state.values = make(map[string]map[string]struct{})
		state.series = make(map[string]map[uint64]struct{})
	}
	return state
}

func (s *guardShard) remember(projectId uuid.UUID, day int64, scope, value string) {
	if len(s.pending) < maxPendingEntries {
		s.pending = append(s.pending, GuardEntry{ProjectId: projectId, Day: day, Scope: scope, Value: value})
	}
}

// takePending hands over what was admitted since the last call.
func (g *guard) takePending() []GuardEntry {
	var out []GuardEntry
	for i := range g.shards {
		shard := &g.shards[i]
		shard.mu.Lock()
		out = append(out, shard.pending...)
		shard.pending = nil
		shard.mu.Unlock()
	}
	return out
}

// merge adds what other instances, or this one before a restart, admitted. A value
// that arrives this way is kept even past its cap: the cap was applied where it was
// admitted, and refusing it here would split one series in two.
func (g *guard) merge(entries []GuardEntry, now time.Time) {
	day := now.Unix() / secondsPerDay
	for _, entry := range entries {
		if entry.Day != persistentDay && entry.Day != day {
			continue
		}
		shard := g.shard(entry.ProjectId)
		shard.mu.Lock()
		state := shard.state(entry.ProjectId, day)
		switch {
		case entry.Scope == scopeMetric:
			if _, ok := state.custom[entry.Value]; !ok {
				state.custom[entry.Value] = nil
			}
		case entry.Scope == scopeGlobalKey:
			if !contains(state.globalKeys, entry.Value) {
				state.globalKeys = append(state.globalKeys, entry.Value)
			}
		case strings.HasPrefix(entry.Scope, scopeMetricKey):
			name := strings.TrimPrefix(entry.Scope, scopeMetricKey)
			if !contains(state.custom[name], entry.Value) {
				state.custom[name] = append(state.custom[name], entry.Value)
			}
		case strings.HasPrefix(entry.Scope, scopeDimension):
			scope := strings.TrimPrefix(entry.Scope, scopeDimension)
			seen := state.values[scope]
			if seen == nil {
				seen = make(map[string]struct{})
				state.values[scope] = seen
			}
			seen[entry.Value] = struct{}{}
		}
		shard.mu.Unlock()
	}
}

// SeriesSet is the series a metric already has in storage for the day.
type SeriesSet = shared.MetricSeriesSet

// mergeSeries adds the stored series of the metrics that are near their budget, so
// the budget counts what every instance wrote and a restart does not start it again.
func (g *guard) mergeSeries(sets []SeriesSet, now time.Time) {
	day := now.Unix() / secondsPerDay
	for _, set := range sets {
		shard := g.shard(set.ProjectId)
		shard.mu.Lock()
		state := shard.state(set.ProjectId, day)
		series := state.series[set.Name]
		if series == nil {
			series = make(map[uint64]struct{}, len(set.Dims))
			state.series[set.Name] = series
		}
		for _, dims := range set.Dims {
			series[hashDims(canonicalDims(dims))] = struct{}{}
		}
		shard.mu.Unlock()
	}
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// admit applies one rule to one value. It reports whether the value was replaced and
// whether it is new to the project today.
func (state *projectState) admit(scope string, rule dimensionRule, value string) (admitted string, replaced, fresh bool) {
	if value == "" {
		return "", false, false
	}
	if rule.path {
		value = NormalizePath(value)
	}
	if len(value) > MaxDimensionLength {
		return Other, true, false
	}
	if rule.values != nil {
		if contains(rule.values, value) {
			return value, false, false
		}
		return Other, true, false
	}
	seen := state.values[scope]
	if seen == nil {
		seen = make(map[string]struct{})
		state.values[scope] = seen
	}
	if _, ok := seen[value]; ok {
		return value, false, false
	}
	if len(seen) >= rule.cap {
		return Other, true, false
	}
	seen[value] = struct{}{}
	return value, false, true
}

func canonicalDims(dims map[string]string) string {
	keys := make([]string, 0, len(dims))
	for key := range dims {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, key := range keys {
		if i > 0 {
			b.WriteByte(0x1f)
		}
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(dims[key])
	}
	return b.String()
}

func hashDims(canonical string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(canonical))
	return h.Sum64()
}

type guardResult struct {
	dims map[string]string
	// folded counts the values replaced by `other`.
	folded int
	// reason is set when the observation is refused.
	reason string
}

func validCountry(code string) bool {
	if len(code) != countryCodeBytes {
		return false
	}
	return code[0] >= 'A' && code[0] <= 'Z' && code[1] >= 'A' && code[1] <= 'Z'
}

// resolve decides whether a metric name may be recorded for this project and
// reduces its attributes to the bounded dimension set. metric is nil for a custom
// metric.
func (g *guard) resolve(projectId uuid.UUID, name string, metric *Metric, resource, attrs map[string]string, country string, now time.Time) guardResult {
	shard := g.shard(projectId)
	shard.mu.Lock()
	defer shard.mu.Unlock()
	day := now.Unix() / secondsPerDay
	state := shard.state(projectId, day)

	var customKeys []string
	if metric == nil {
		keys, known := state.custom[name]
		if !known {
			if !validCustomName(name) || len(state.custom) >= MaxCustomMetrics {
				return guardResult{reason: ReasonUnknownMetric}
			}
			state.custom[name] = nil
			shard.remember(projectId, persistentDay, scopeMetric, name)
		}
		customKeys = keys
	}

	result := guardResult{dims: make(map[string]string, 8)}
	set := func(key, scope string, rule dimensionRule, value string) {
		admitted, replaced, fresh := state.admit(scope, rule, value)
		if admitted == "" {
			return
		}
		if replaced {
			result.folded++
		}
		if fresh {
			shard.remember(projectId, day, scopeDimension+scope, admitted)
		}
		result.dims[key] = admitted
	}
	for key, rule := range resourceDimensions {
		if value, ok := resource[key]; ok {
			set(key, key, rule, value)
		}
	}
	if validCountry(country) {
		result.dims[DimensionCountry] = country
	}
	for _, key := range sortedKeys(resource) {
		if !strings.HasPrefix(key, globalDimensionPrefix) || !globalDimension.MatchString(key) {
			continue
		}
		if !contains(state.globalKeys, key) {
			if len(state.globalKeys) >= MaxGlobalDimensions {
				continue
			}
			state.globalKeys = append(state.globalKeys, key)
			shard.remember(projectId, persistentDay, scopeGlobalKey, key)
		}
		set(key, key, dimensionRule{cap: customDimensionCap}, resource[key])
	}
	for _, key := range sortedKeys(attrs) {
		value := attrs[key]
		if metric != nil {
			if allowed, ok := metric.Dims[key]; ok {
				rule, shared := pointDimensions[key]
				if !shared {
					rule = metricDimensions[key]
				}
				if allowed != nil {
					rule = dimensionRule{values: allowed}
				}
				set(key, key, rule, value)
				continue
			}
			if rule, ok := pointDimensions[key]; ok && !metric.PerUser {
				set(key, key, rule, value)
			}
			continue
		}
		// What the server or the resource decides is not the metric's to restate: country is resolved from
		// the request, never taken from the client.
		if _, owned := resourceDimensions[key]; owned || key == DimensionCountry || strings.HasPrefix(key, globalDimensionPrefix) {
			continue
		}
		if !contains(customKeys, key) {
			if len(customKeys) >= MaxCustomDimensions || len(key) > MaxDimensionLength {
				continue
			}
			customKeys = append(customKeys, key)
			state.custom[name] = customKeys
			shard.remember(projectId, persistentDay, scopeMetricKey+name, key)
		}
		set(key, name+"/"+key, dimensionRule{cap: customDimensionCap}, value)
	}

	series := state.series[name]
	if series == nil {
		series = make(map[uint64]struct{})
		state.series[name] = series
	}
	used := len(series)
	hash := hashDims(canonicalDims(result.dims))
	if _, ok := series[hash]; !ok && used >= g.seriesBudget {
		for _, key := range foldOrder {
			if value, present := result.dims[key]; !present || value == Other {
				continue
			}
			result.dims[key] = Other
			result.folded++
			hash = hashDims(canonicalDims(result.dims))
			if _, known := series[hash]; known {
				break
			}
		}
		// Folded series are few by construction (the dimensions left are capped one
		// by one), so they get room of their own; past twice the budget nothing new
		// is admitted.
		if _, known := series[hash]; !known && used >= 2*g.seriesBudget {
			return guardResult{reason: ReasonCardinality}
		}
	}
	series[hash] = struct{}{}
	return result
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
