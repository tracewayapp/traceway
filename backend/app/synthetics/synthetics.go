// Package synthetics runs scheduled synthetic checks (http, tcp, browser)
// from a durable queue in the main DB. The scheduler enqueues due runs, the
// in-process executor claims and probes, and results land in the telemetry
// check_results table. Modeled on the notification outbox: guarded status
// transitions, stale-claim reclaim, advisory-locked ticks. Unlike the outbox
// there is no retry/backoff — a failed probe IS a result — and terminal queue
// rows are deleted; expired queued runs are recorded as missed, never
// executed late.
package synthetics

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/models"
)

const (
	schedulerAdvisoryLockId = 824737004 // outbox: 824737003, escalator: 824737002, backfill: 824737001

	// staleClaimAge must exceed the largest per-check timeout (browser 120s)
	// plus result-processing headroom.
	staleClaimAge = 5 * time.Minute

	// maxRunExpiry caps how stale a queued probe may execute; per-check
	// expiry is min(interval, maxRunExpiry).
	maxRunExpiry = 5 * time.Minute

	scheduleBatchSize = 200
	claimBatchSize    = 50
	expiredBatchSize  = 500

	BrowserModeOff      = "off"
	BrowserModeEmbedded = "embedded"
	BrowserModeRemote   = "remote"
)

// Browser execution limits; validation clamps to these.
const (
	MinIntervalSeconds       = 30
	DefaultTimeoutSeconds    = 30
	MaxTimeoutSeconds        = 120
	MaxBrowserTimeoutSeconds = 120
)

func pollInterval() time.Duration {
	return config.PollSeconds(config.Config.SyntheticsPollSeconds, 15)
}

// BrowserMode returns the validated SYNTHETICS_BROWSER_MODE, defaulting to off.
func BrowserMode() string {
	switch config.Config.SyntheticsBrowserMode {
	case BrowserModeEmbedded:
		return BrowserModeEmbedded
	case BrowserModeRemote:
		return BrowserModeRemote
	default:
		return BrowserModeOff
	}
}

func AllowPrivateTargets() bool {
	// Multi-tenant cloud must never let a check probe the platform's own
	// network (metadata endpoints, databases, localhost); the guard is forced
	// on there regardless of environment.
	if config.Config.CloudMode == "true" {
		return false
	}
	// Default allow: probing LAN services is a core self-hosted use case.
	return config.Config.SyntheticsAllowPrivateTargets != "false"
}

func httpConcurrency() int {
	return parsePositiveInt(config.Config.SyntheticsHTTPConcurrency, 8)
}

func browserConcurrency() int {
	return parsePositiveInt(config.Config.SyntheticsBrowserConcurrency, 2)
}

func parsePositiveInt(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}

// instanceName identifies this process in check_runs.claimed_by.
var instanceName = func() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "traceway"
	}
	return fmt.Sprintf("%s-%d", host, os.Getpid())
}()

// wakeCh nudges the executor loop; a full channel means a wake-up is already
// scheduled.
var wakeCh = make(chan struct{}, 1)

// Wake nudges the executor loops (and any waiting remote-runner long-poll) to
// look for claimable runs without waiting for the next tick. Call after the
// enqueueing transaction commits.
func Wake() {
	select {
	case wakeCh <- struct{}{}:
	default:
	}
	select {
	case browserExecWakeCh <- struct{}{}:
	default:
	}
	wakeBrowser()
}

// browserWakeCh nudges remote-runner long-polls when browser runs are queued.
var browserWakeCh = make(chan struct{}, 1)

func wakeBrowser() {
	select {
	case browserWakeCh <- struct{}{}:
	default:
	}
}

// BrowserWakeChannel is drained by the runner long-poll endpoint so a freshly
// queued browser run answers a waiting runner immediately.
func BrowserWakeChannel() <-chan struct{} {
	return browserWakeCh
}

// StateTransition describes a check crossing up<->down, delivered to the
// notify hook AFTER the state transaction commits (the hook opens its own
// transactions; calling it with one held would deadlock the single-connection
// SQLite main DB).
type StateTransition struct {
	Check    models.SyntheticCheck
	From     string
	To       string
	ErrorMsg string
	At       time.Time
}

// NotifierFunc is implemented by the notifications package and registered in
// cmd/run.go; the indirection avoids an import cycle.
type NotifierFunc func(transition StateTransition)

var notifier NotifierFunc

func RegisterNotifier(fn NotifierFunc) {
	notifier = fn
}

// RunExpiry computes when a queued run stops being worth executing.
func RunExpiry(scheduledFor time.Time, intervalSeconds int) time.Time {
	expiry := time.Duration(intervalSeconds) * time.Second
	if expiry > maxRunExpiry {
		expiry = maxRunExpiry
	}
	return scheduledFor.Add(expiry)
}

// EffectiveTimeout clamps a check's timeout to sane bounds.
func EffectiveTimeout(check *models.SyntheticCheck) time.Duration {
	seconds := check.TimeoutSeconds
	if seconds < 1 {
		seconds = DefaultTimeoutSeconds
	}
	if seconds > MaxTimeoutSeconds {
		seconds = MaxTimeoutSeconds
	}
	return time.Duration(seconds) * time.Second
}
