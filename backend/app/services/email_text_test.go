package services

import (
	"testing"

	"github.com/tracewayapp/traceway/backend/v2/app/models"
)

func TestNotificationEmailPlaintextCarriesTheLinkAndFooter(t *testing.T) {
	msg := models.NotificationMessage{
		Subject:  "Error rate high",
		Body:     "The error rate has reached 12.0% over the last 5 minutes.\n",
		URL:      "https://traceway.example.com/issues?preset=1h",
		RuleName: "Error rate",
		RuleType: "error_rate",
		Severity: models.NotificationSeverityWarning,
		Email:    &models.NotificationEmail{Template: models.EmailTemplateAlert, Alert: &models.EmailAlert{}},
	}
	email := NotificationEmail(msg, []string{"ops@example.com"})
	want := "The error rate has reached 12.0% over the last 5 minutes.\n\nhttps://traceway.example.com/issues?preset=1h\n\nYou are receiving this because the notification rule \"Error rate\" fired."
	if email.Text != want {
		t.Fatalf("plaintext:\n got %q\nwant %q", email.Text, want)
	}

	msg.Body = "Page opened, acknowledge at https://traceway.example.com/issues?preset=1h"
	msg.RuleName = ""
	email = NotificationEmail(msg, []string{"ops@example.com"})
	if email.Text != msg.Body {
		t.Fatalf("a link already in the body was repeated: %q", email.Text)
	}
}
