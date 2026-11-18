package notifications

import (
	"testing"

	"github.com/tracewayapp/traceway/backend/v2/app/models"
)

func TestPushoverPriority(t *testing.T) {
	emergency := &PushoverAdapter{UserKey: "u", AppToken: "a", Priority: 2, Retry: 60, Expire: 3600, Callback: "https://example.com/ack"}
	down := buildCheckDownMessage(&models.SyntheticCheck{Id: 7, Name: "api health"}, "timeout", "api")
	recovered := buildCheckRecoveredMessage(&models.SyntheticCheck{Id: 7, Name: "api health"}, "api")

	alert := emergency.form(down)
	if got := alert.Get("priority"); got != "2" {
		t.Errorf("alert priority = %s, want 2", got)
	}
	for _, key := range []string{"retry", "expire", "callback"} {
		if !alert.Has(key) {
			t.Errorf("alert is missing %s", key)
		}
	}

	recovery := emergency.form(recovered)
	if got := recovery.Get("priority"); got != "0" {
		t.Errorf("recovery priority = %s, want 0", got)
	}
	for _, key := range []string{"retry", "expire", "callback"} {
		if recovery.Has(key) {
			t.Errorf("recovery sets %s = %s, want it unset", key, recovery.Get(key))
		}
	}
}
