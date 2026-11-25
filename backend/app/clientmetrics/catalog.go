// Package clientmetrics aggregates performance metrics of browser and mobile clients
// in memory. A client metric is a distribution over a population, never a time series
// per client: the stored unit is a histogram per (project, metric, dimensions, minute),
// so storage grows with the number of dimension combinations and not with the number
// of users. A request only touches memory; a flush every few seconds writes one batch
// per table.
//
// The package knows nothing about HTTP or storage. It is fed by the OTLP metrics
// endpoint and by the performance events of older SDKs, and writes through Sink.
package clientmetrics

import "regexp"

type Shape int

const (
	// Distribution is a value per observation, stored as a histogram.
	Distribution Shape = iota
	// Counter counts events. A ratio is two counters, <name> and <name>.total.
	Counter
)

// Metric is one entry of the catalog. A client cannot send a metric that is not
// here or registered as one of the project's custom metrics, and cannot attach a
// dimension that is not listed, so a public project token cannot create series.
type Metric struct {
	Name  string
	Unit  string
	Shape Shape
	// Max clamps an observation: an LCP of three hours is a suspended tab, not a
	// measurement.
	Max float64
	// Dims are the per-metric dimensions and their closed value sets. A nil set
	// accepts any value up to the per-project cap of that dimension.
	Dims map[string][]string
	// Good and Poor are the thresholds of a rated distribution; zero when unrated.
	Good, Poor float64
	// PerUser marks a metric the device counts once per day: it describes a person, not
	// a page, so it takes no page dimension.
	PerUser bool
	// EventsPerCost is how many events of a counter cost one place of an address's
	// per-minute budget; zero means one. Frames come at the display's rate, so a client
	// pays for a second of them, not for each one.
	EventsPerCost float64
}

var (
	ratings         = []string{"good", "needs-improvement", "poor"}
	navigationTypes = []string{"navigate", "reload", "back-forward", "back-forward-cache", "prerender", "restore"}
	webVitalDims    = map[string][]string{"rating": ratings, "navigation_type": navigationTypes}
	hostDims        = map[string][]string{"host": nil}
	viewDims        = map[string][]string{"rating": ratings, "view_type": {"load", "soft"}}
)

const (
	minute = 60_000
	hour   = 60 * minute
	// framesPerSecond is what a second of frames costs a client's budget.
	framesPerSecond = 60
)

var catalog = buildCatalog([]Metric{
	{Name: "browser.web_vital.lcp", Unit: "ms", Max: 2 * minute, Dims: webVitalDims, Good: 2500, Poor: 4000},
	{Name: "browser.web_vital.inp", Unit: "ms", Max: minute, Dims: webVitalDims, Good: 200, Poor: 500},
	{Name: "browser.web_vital.cls", Unit: "", Max: 10, Dims: webVitalDims, Good: 0.1, Poor: 0.25},
	{Name: "browser.web_vital.fcp", Unit: "ms", Max: 2 * minute, Dims: webVitalDims, Good: 1800, Poor: 3000},
	{Name: "browser.web_vital.ttfb", Unit: "ms", Max: 2 * minute, Dims: webVitalDims, Good: 800, Poor: 1800},
	{Name: "browser.route_timing.duration", Unit: "ms", Max: minute, Dims: map[string][]string{
		"outcome": {"settled", "timeout", "interrupted"}, "type": {"push", "replace", "traverse"}}},
	{Name: "browser.long_task.duration", Unit: "ms", Max: minute, Dims: map[string][]string{"type": {"long-animation-frame", "longtask"}}},
	{Name: "browser.long_task.blocking_time", Unit: "ms", Max: 10 * minute, Good: 200, Poor: 600},
	{Name: "browser.page_load.duration", Unit: "ms", Max: 5 * minute},
	{Name: "browser.page_load.dom_content_loaded", Unit: "ms", Max: 5 * minute},
	{Name: "browser.page.transfer_size", Unit: "By", Max: 1 << 30},
	{Name: "browser.request.duration", Unit: "ms", Max: 5 * minute, Dims: hostDims},
	{Name: "browser.request.failed", Shape: Counter, Dims: hostDims},
	{Name: "browser.request.failed.total", Shape: Counter, Dims: hostDims},
	{Name: "browser.js_error", Shape: Counter},
	{Name: "browser.rage_click", Shape: Counter},
	{Name: "browser.page_view", Shape: Counter},
	{Name: "browser.session", Shape: Counter},
	{Name: "browser.view.inp", Unit: "ms", Max: minute, Dims: viewDims, Good: 200, Poor: 500},
	{Name: "browser.view.cls", Unit: "", Max: 10, Dims: viewDims, Good: 0.1, Poor: 0.25},
	// phase is INP's largest part (input delay, processing or presentation): a slow interaction
	// waiting on another task needs a different fix than a slow handler or a slow render.
	{Name: "browser.web_vital.attribution", Shape: Counter, Dims: map[string][]string{
		"vital": {"lcp", "inp", "cls"}, "rating": {"needs-improvement", "poor"}, "target": nil,
		"phase": {"input_delay", "processing", "presentation"}}},
	{Name: "browser.long_task.script_time", Unit: "ms", Max: minute, Dims: map[string][]string{"script": nil}},
	{Name: "browser.user", Shape: Counter, PerUser: true},
	{Name: "browser.user.affected", Shape: Counter, PerUser: true, Dims: map[string][]string{
		"issue": {"js_error", "poor_lcp", "poor_inp", "poor_cls", "rage_click", "failed_request"}}},

	{Name: "app.start.duration", Unit: "ms", Max: 2 * minute, Dims: map[string][]string{"start_type": {"cold", "warm", "hot"}}},
	{Name: "app.screen.ttid", Unit: "ms", Max: 2 * minute},
	{Name: "app.screen.ttfd", Unit: "ms", Max: 2 * minute},
	{Name: "app.frames.slow", Shape: Counter, EventsPerCost: framesPerSecond},
	{Name: "app.frames.slow.total", Shape: Counter, EventsPerCost: framesPerSecond},
	{Name: "app.frames.frozen", Shape: Counter, EventsPerCost: framesPerSecond},
	{Name: "app.frames.frozen.total", Shape: Counter, EventsPerCost: framesPerSecond},
	{Name: "app.hang", Shape: Counter},
	{Name: "app.hang.duration", Unit: "ms", Max: 10 * minute},
	{Name: "app.anr", Shape: Counter},
	{Name: "app.crash", Shape: Counter},
	{Name: "app.oom", Shape: Counter},
	{Name: "app.session", Shape: Counter},
	{Name: "app.user", Shape: Counter, PerUser: true},
	{Name: "app.user.affected", Shape: Counter, PerUser: true, Dims: map[string][]string{
		"issue": {"crash", "anr", "hang", "oom", "slow_start", "frozen_frames", "failed_request"}}},
	{Name: "app.memory.peak", Unit: "By", Max: 64 << 30},
	{Name: "app.cpu.time", Unit: "ms", Max: 24 * hour},
	{Name: "app.energy", Unit: "", Max: 1e9},
	{Name: "app.request.duration", Unit: "ms", Max: 5 * minute, Dims: hostDims},
	{Name: "app.request.bytes", Unit: "By", Max: 1 << 32, Dims: hostDims},
	{Name: "app.request.failed", Shape: Counter, Dims: hostDims},
	{Name: "app.request.failed.total", Shape: Counter, Dims: hostDims},
	{Name: "app.size", Unit: "By", Max: 64 << 30},
	{Name: "app.disk.used", Unit: "By", Max: 1 << 40},
})

func buildCatalog(metrics []Metric) map[string]*Metric {
	out := make(map[string]*Metric, len(metrics))
	for i := range metrics {
		out[metrics[i].Name] = &metrics[i]
	}
	return out
}

// Lookup returns the catalog entry of a metric name.
func Lookup(name string) (*Metric, bool) {
	m, ok := catalog[name]
	return m, ok
}

// Catalog lists every catalog metric, for the registry and the docs.
func Catalog() []Metric {
	out := make([]Metric, 0, len(catalog))
	for _, m := range catalog {
		out = append(out, *m)
	}
	return out
}

const (
	// MaxCustomMetrics, MaxCustomDimensions and the per-dimension value cap bound
	// what `traceway.metric()` and `traceway.count()` can create per project.
	MaxCustomMetrics    = 50
	MaxCustomDimensions = 2
	customDimensionCap  = 50
	customValueMax      = 1e12
	// MaxGlobalDimensions is the number of `custom.<key>` dimensions an app may put on
	// every metric it sends (a plan, an experiment arm).
	MaxGlobalDimensions   = 2
	globalDimensionPrefix = "custom."
)

var (
	customName      = regexp.MustCompile(`^[a-z][a-z0-9_.]{0,62}$`)
	reservedPrefix  = regexp.MustCompile(`^(browser|app|traceway|custom)\.`)
	globalDimension = regexp.MustCompile(`^custom\.[a-z][a-z0-9_]{0,31}$`)
)

func validCustomName(name string) bool {
	return customName.MatchString(name) && !reservedPrefix.MatchString(name)
}
