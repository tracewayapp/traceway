package traceway

import (
	"testing"

	"github.com/tracewayapp/traceway/cli/internal/apifixture"
	"github.com/tracewayapp/traceway/cli/pkg/access"
	"github.com/tracewayapp/traceway/cli/pkg/access/conformance"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

func TestConformance(t *testing.T) {
	fx := apifixture.Known
	api := apifixture.New(t)
	source := New("traceway", client.New(api.URL, client.WithJWT("token")))
	conformance.RunSuite(t, source, conformance.Fixtures{
		ProjectID:     fx.ProjectID,
		Window:        access.Window{From: fx.From, To: fx.To},
		ExceptionHash: fx.ExceptionHash,
		OccurrenceID:  fx.OccurrenceID.String(),
		RequestID:     fx.RequestID.String(),
		EndpointName:  fx.EndpointName,
		MetricName:    fx.MetricName,
		TaskID:        fx.TaskID.String(),
		AITraceID:     fx.AITraceID.String(),
		SessionID:     fx.SessionID.String(),
		TraceID:       fx.TraceID.String(),
		At:            fx.At,
		UnknownHash:   fx.UnknownHash,
		UnknownID:     fx.UnknownID.String(),
	})
}

func TestRegisteredFactoryNeedsURLAndToken(t *testing.T) {
	if _, err := access.Open(access.Config{Name: "other", Provider: Provider, Settings: map[string]string{"url": "https://x"}}); err == nil {
		t.Fatal("factory accepted a config without a token")
	}
	source, err := access.Open(access.Config{Name: "other", Provider: Provider, Domains: []access.Domain{access.DomainLogs}, Settings: map[string]string{"url": "https://x", "token": "t"}})
	if err != nil {
		t.Fatal(err)
	}
	if source.Name() != "other" || source.Provider() != Provider {
		t.Fatalf("opened %s/%s", source.Name(), source.Provider())
	}
	if got := access.ImplementedDomains(source); len(got) != len(access.Domains) {
		t.Fatalf("implemented domains = %v", got)
	}
}
