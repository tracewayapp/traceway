package services

import (
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/backend/app/models"
)

func TestEveryTemplateRenders(t *testing.T) {
	payload := &models.NotificationEmail{
		Exception: &models.EmailException{
			ProjectName: "Acme <Prod>",
			ErrorType:   "*fmt.wrapError",
			ExceptionId: "01930f8c",
			Hash:        "9c7bc73da37328f2",
			OccurredAt:  "2026-08-24 09:41:12 UTC",
			AppVersion:  "1.9.19",
			ServerName:  "api-blue-3",
			TraceLabel:  "Endpoint",
			TraceName:   "GET /api/users/:id",
			Attributes:  []models.EmailField{{Label: "user_id", Value: "8412"}},
			StackTrace:  "err := <nil>\n\tat main.go:1",
		},
		Check:   &models.EmailCheck{ProjectName: "Acme", CheckName: "Checkout API", ConsecutiveFailures: 3, LastError: "i/o timeout"},
		Alert:   &models.EmailAlert{ProjectName: "Acme", Headline: "The error rate has reached 12.4%.", Observed: "12.4%", Threshold: "5.0%", WindowMins: 15},
		Flagged: &models.EmailFlagged{ProjectName: "Acme", ConversationId: "conv_1", UserId: "8412", Terms: []string{"password"}},
		Page:    &models.EmailPage{BodyText: "Something broke", RuleName: "Critical errors", EventCount: 4},
		Test:    &models.EmailTest{Target: "notification channel"},
	}

	templates := []string{
		models.EmailTemplateNewError,
		models.EmailTemplateErrorRegression,
		models.EmailTemplateCheckDown,
		models.EmailTemplateCheckRecovered,
		models.EmailTemplateAlert,
		models.EmailTemplateAiFlagged,
		models.EmailTemplatePage,
		models.EmailTemplateTest,
	}

	for _, name := range templates {
		html, err := RenderEmail(Email{
			Template:   name,
			Title:      "Something happened",
			Badge:      "CRITICAL",
			BadgeColor: EmailColorCritical,
			URL:        "https://traceway.example.com/issues/abc",
			Footer:     "You are receiving this because a rule fired.",
			Data:       payload,
		})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, want := range []string{"<!DOCTYPE html>", "Something happened", EmailColorCritical, "traceway-mark.png", "Traceway &middot; Error tracking"} {
			if !strings.Contains(html, want) {
				t.Errorf("%s: rendered HTML missing %q", name, want)
			}
		}
	}
}

func TestTemplatesEscapeContent(t *testing.T) {
	html, err := RenderEmail(Email{
		Template: models.EmailTemplateNewError,
		Title:    "[Project <X>] New error",
		URL:      "https://traceway.example.com/issues/abc",
		Data: &models.NotificationEmail{Exception: &models.EmailException{
			ProjectName: "Acme <Prod>",
			ErrorType:   "*fmt.wrapError",
			StackTrace:  "err := <nil>",
		}},
	})
	if err != nil {
		t.Fatalf("RenderEmail failed: %v", err)
	}
	for _, want := range []string{"[Project &lt;X&gt;] New error", "Acme &lt;Prod&gt;", "err := &lt;nil&gt;", "View Issue"} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered HTML missing %q", want)
		}
	}
	if strings.Contains(html, "<nil>") {
		t.Error("stack trace was not escaped")
	}
}

func TestRenderEmailRejectsUnknownTemplate(t *testing.T) {
	if _, err := RenderEmail(Email{Template: "nope"}); err == nil {
		t.Error("expected an error for an unknown template")
	}
}

func TestTransactionalTemplatesRender(t *testing.T) {
	invitation, err := RenderEmail(Email{
		Template: "invitation",
		Title:    "Join Acme on Traceway",
		URL:      "https://traceway.example.com/accept-invitation?token=t",
		Data:     invitationData{InviterName: "Dana", OrgName: "Acme <Inc>"},
	})
	if err != nil {
		t.Fatalf("invitation failed: %v", err)
	}
	for _, want := range []string{"Dana", "Acme &lt;Inc&gt;", "Accept Invitation", `href="https://traceway.example.com/accept-invitation?token=t"`} {
		if !strings.Contains(invitation, want) {
			t.Errorf("invitation missing %q", want)
		}
	}

	reset, err := RenderEmail(Email{Template: "password_reset", Title: "Reset your password", URL: "https://traceway.example.com/reset-password?token=t"})
	if err != nil {
		t.Fatalf("password reset failed: %v", err)
	}
	for _, want := range []string{"Reset Password", `href="https://traceway.example.com/reset-password?token=t"`} {
		if !strings.Contains(reset, want) {
			t.Errorf("password reset missing %q", want)
		}
	}
}

func TestNotificationEmailFillsChrome(t *testing.T) {
	msg := models.NotificationMessage{
		Subject:  "[Acme] New error: *fmt.wrapError",
		Body:     "A new error has been detected.",
		Severity: models.NotificationSeverityCritical,
		RuleType: "new_error",
		RuleName: "Critical errors",
		URL:      "/issues/abc",
		Email:    &models.NotificationEmail{Template: models.EmailTemplateNewError, Exception: &models.EmailException{}},
	}

	email := NotificationEmail(msg, []string{"dev@example.com"})

	if email.Subject != "[CRITICAL] "+msg.Subject {
		t.Errorf("subject = %q, want the severity prefix", email.Subject)
	}
	if email.Title != msg.Subject {
		t.Errorf("title = %q, want the unprefixed subject", email.Title)
	}
	if email.Badge != "CRITICAL" || email.BadgeColor != EmailColorCritical {
		t.Errorf("badge = %q/%q, want CRITICAL", email.Badge, email.BadgeColor)
	}
	if email.URL != emailBaseURL()+"/issues/abc" {
		t.Errorf("url = %q, want the relative link absolutized", email.URL)
	}
	if !strings.Contains(email.Footer, `"Critical errors"`) {
		t.Errorf("footer = %q, want it to name the rule", email.Footer)
	}
	if email.Text != msg.Body {
		t.Error("plaintext alternative must stay the message body")
	}
}

func TestNotificationEmailKeepsAbsoluteURLsAndSkipsTestFooter(t *testing.T) {
	msg := models.NotificationMessage{
		Severity: models.NotificationSeverityInfo,
		RuleType: "test",
		RuleName: "Test",
		URL:      "https://traceway.example.com/ack/twk_1",
		Email:    &models.NotificationEmail{Template: models.EmailTemplateTest, Test: &models.EmailTest{Target: "notification channel"}},
	}

	email := NotificationEmail(msg, []string{"dev@example.com"})

	if email.URL != msg.URL {
		t.Errorf("url = %q, want the already absolute link untouched", email.URL)
	}
	if email.Footer != "" {
		t.Errorf("footer = %q, want none: the test template says it is a test", email.Footer)
	}
}

func TestPageTemplateSwitchesCopyOnEscalation(t *testing.T) {
	render := func(level int) string {
		t.Helper()
		html, err := RenderEmail(Email{
			Template: models.EmailTemplatePage,
			Title:    "[Page] Something broke",
			URL:      "https://traceway.example.com/ack/twk_1",
			Data:     &models.NotificationEmail{Page: &models.EmailPage{BodyText: "Something broke", EscalationLevel: level}},
		})
		if err != nil {
			t.Fatalf("level %d: %v", level, err)
		}
		return html
	}

	if first := render(0); !strings.Contains(first, "You are on call and a page needs your attention.") {
		t.Error("first delivery should read as a fresh page")
	}
	escalated := render(2)
	if !strings.Contains(escalated, "escalated to level 2") {
		t.Error("escalated delivery should name the level it reached, matching the subject prefix")
	}
	if strings.Contains(escalated, "You are on call and a page needs your attention. Acknowledge") {
		t.Error("escalated delivery should not also show the first-delivery copy")
	}
}
