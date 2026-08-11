package notifications

import (
	"encoding/json"

	"github.com/tracewayapp/traceway/backend/app/models"
)

// PageOpener opens an on-call page for a rule that targets an escalation
// channel, reporting whether a new page was opened (false = deduped into one
// already unresolved). It is implemented by the oncall package and registered
// from cmd/run.go; the indirection exists because notifications cannot import
// oncall (oncall imports this package for Message and the adapters).
type PageOpener func(channelConfig json.RawMessage, rule *models.NotificationRuleWithChannel, msg Message) (opened bool, err error)

var pageOpener PageOpener

func RegisterPageOpener(opener PageOpener) {
	pageOpener = opener
}
