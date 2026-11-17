package services

import (
	"bytes"
	"context"
	"log"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/tracewayapp/traceway/backend/v2/app/config"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
	sdklog "go.opentelemetry.io/otel/sdk/log"
)

type logRecorder struct {
	mu     sync.Mutex
	bodies []string
}

func (r *logRecorder) Export(_ context.Context, records []sdklog.Record) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, record := range records {
		r.bodies = append(r.bodies, record.Body().AsString())
	}
	return nil
}

func (r *logRecorder) Shutdown(context.Context) error   { return nil }
func (r *logRecorder) ForceFlush(context.Context) error { return nil }

// With SMTP off the email is written to the log so the operator can still
// follow the link. Members of the self-monitoring project's organization can
// read its logs, so the reset and invitation links must stay on stdout:
// anyone could otherwise request a reset for the owner and read the link.
func TestLoggedEmailsAreNeverExported(t *testing.T) {
	exported := &logRecorder{}
	t.Cleanup(traceway.Install(traceway.Providers{
		LoggerProvider: sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(exported))),
	}))
	prev := config.Config
	config.Init(&config.Cfg{SMTPEnabled: "false", AppBaseURL: "https://traceway.example.com"})
	t.Cleanup(func() { config.Init(prev) })
	var stdout bytes.Buffer
	log.SetOutput(&stdout)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	if err := EmailService.SendPasswordReset("owner@example.com", "reset-secret"); err != nil {
		t.Fatal(err)
	}
	if err := EmailService.SendInvitation("new@example.com", "Dana", "Acme", "invite-secret"); err != nil {
		t.Fatal(err)
	}

	if len(exported.bodies) == 0 {
		t.Fatal("the delivery lines carry no secret and must still be exported")
	}
	for _, body := range exported.bodies {
		if strings.Contains(body, "reset-secret") || strings.Contains(body, "invite-secret") {
			t.Fatalf("an email link reached self-monitoring: %q", body)
		}
	}
	for _, link := range []string{"/reset-password?token=reset-secret", "/accept-invitation?token=invite-secret"} {
		if !strings.Contains(stdout.String(), link) {
			t.Fatalf("the operator must still find %s on stdout", link)
		}
	}
}
