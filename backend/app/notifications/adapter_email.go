package notifications

import (
	"context"
	"fmt"
	"strings"

	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/services"
)

type EmailAdapter struct {
	Recipients []string `json:"recipients"`
}

func (a *EmailAdapter) Type() string { return "email" }

func (a *EmailAdapter) Validate() error {
	if len(a.Recipients) == 0 {
		return fmt.Errorf("at least one recipient is required")
	}
	if len(a.Recipients) > 10 {
		return fmt.Errorf("maximum 10 recipients allowed")
	}
	for _, r := range a.Recipients {
		if !strings.Contains(r, "@") {
			return fmt.Errorf("invalid email address: %s", r)
		}
	}
	return nil
}

func (a *EmailAdapter) Send(ctx context.Context, msg Message) error {
	if msg.Email == nil {
		msg.Email = &models.NotificationEmail{Template: models.EmailTemplateAlert, Alert: &models.EmailAlert{Headline: msg.Body}}
	}
	return services.SendEmail(ctx, services.NotificationEmail(msg, a.Recipients))
}
