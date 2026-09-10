package agent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tracewayapp/traceway/cli/pkg/access"
	"github.com/tracewayapp/traceway/cli/pkg/access/traceway"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

// TelemetryClient opens an API client the way the agent process would: with
// a run token, against this instance. Wired from cmd/run.go with an
// in-process loopback so building a pack never leaves the process.
type TelemetryClient func(runToken string) *client.Client

var telemetryClient TelemetryClient

func SetTelemetryClient(fn TelemetryClient) {
	telemetryClient = fn
}

const (
	exceptionOccurrencePage = 5
	exceptionTraceNodeLimit = 20
)

// TracewayExceptionContext builds the evidence for a traceway_exception
// subject through the CLI's access layer, so the agent's own tools and the
// pack read the same records.
type TracewayExceptionContext struct{}

func (TracewayExceptionContext) Kind() string { return "traceway_exception" }

func (TracewayExceptionContext) Build(ctx context.Context, subject Subject, runToken string) (ContextSection, error) {
	if telemetryClient == nil {
		return ContextSection{}, errors.New("agent: no telemetry client wired")
	}
	source := traceway.New(traceway.DefaultName, telemetryClient(runToken))
	projectId := subject.ProjectId.String()

	detail, err := source.GetException(ctx, access.ExceptionLookup{ProjectID: projectId, Hash: subject.Ref, Page: access.PageRequest{Page: 1, PageSize: exceptionOccurrencePage}})
	if err != nil {
		return ContextSection{}, err
	}
	if detail.Group == nil {
		return ContextSection{}, access.ErrNotFound
	}

	section := ContextSection{
		Title: "Exception " + subject.Ref,
		Summary: fmt.Sprintf("Exception group %s: %d occurrences, first seen %s, last seen %s. The stack trace, the latest occurrences with their attributes, and the distributed trace of the latest occurrence follow as data.",
			subject.Ref, detail.Group.Count, detail.Group.FirstSeen.UTC().Format(time.RFC3339), detail.Group.LastSeen.UTC().Format(time.RFC3339)),
		Data: map[string]any{
			"stackTrace":  detail.Group.StackTrace,
			"occurrences": detail.Occurrences,
		},
		Links: []Link{{Provider: traceway.Provider, Kind: "issue", ExternalRef: subject.Ref, URL: "/issues/" + subject.Ref}},
	}

	if len(detail.Occurrences) == 0 {
		return section, nil
	}
	latest := detail.Occurrences[0]
	if latest.DistributedTraceId == nil {
		return section, nil
	}
	trace, err := source.GetTrace(ctx, latest.DistributedTraceId.String(), latest.RecordedAt)
	if err != nil {
		if errors.Is(err, access.ErrNotFound) {
			return section, nil
		}
		return ContextSection{}, err
	}
	if len(trace.Nodes) > exceptionTraceNodeLimit {
		trace.Nodes = trace.Nodes[:exceptionTraceNodeLimit]
	}
	section.Data["distributedTrace"] = trace
	return section, nil
}
