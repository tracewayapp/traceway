package services

import (
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/backend/v2/app/models"
)

// A day-long rule used to read "last 1440 minutes", an SLO's "last 43200 minutes".
func TestAlertEmailNamesTheWindowInUnitsAPersonReads(t *testing.T) {
	cases := map[int]string{5: "last 5 minutes", 60: "last hour", 1440: "last 24 hours", 43200: "last 30 days"}
	for minutes, want := range cases {
		html, err := RenderEmail(Email{
			Template: models.EmailTemplateAlert,
			Title:    "Alert",
			Data:     &models.NotificationEmail{Alert: &models.EmailAlert{ProjectName: "Acme", Headline: "h", WindowMins: minutes}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(html, want) {
			t.Errorf("window of %d minutes: rendered email lacks %q", minutes, want)
		}
	}
}
