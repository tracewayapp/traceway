package write

import (
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

// Some write cases need telemetry that lines up with objects the backend
// stamps with the wall clock: a page opens when its first error is
// ingested, a monitor incident starts and ends at real probe times. Those
// seeds move the scenario's "now" a few minutes into the future, so the
// story (errors, then the page, then the fix) can continue after ingest,
// and then wait until that future has passed before the agent starts.

// wrClock returns the wall clock, or the scenario's fixed now in a dry sink
// (the leakage lint seeds without a backend).
func wrClock(s *seed.Scenario) time.Time {
	if s.Sink.Dry() {
		return s.Now
	}
	return time.Now().UTC()
}

// wrEndTelemetryAt makes the generated traffic run until end (the DSL stops
// one minute before the scenario's now).
func wrEndTelemetryAt(s *seed.Scenario, end time.Time) {
	s.Now = end.Add(time.Minute)
}

// wrSleepUntil waits for the wall clock to reach t (no-op in a dry sink).
func wrSleepUntil(s *seed.Scenario, t time.Time) {
	if s.Sink.Dry() {
		return
	}
	if d := time.Until(t); d > 0 {
		time.Sleep(d)
	}
}

// wrStubURL is the switchable monitor target ({{env.stub}}).
func wrStubURL(s *seed.Scenario) string {
	if u, ok := s.Sink.(interface{ StubURL() string }); ok && u.StubURL() != "" {
		return u.StubURL()
	}
	return "http://127.0.0.1:9"
}
