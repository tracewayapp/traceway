package notifications

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/tracewayapp/traceway/backend/v2/app/models"
)

// PageOpener opens an on-call page for a rule that targets an escalation
// channel, reporting whether a new page was opened (false = deduped into one
// already unresolved). It is implemented by the oncall package and registered
// from cmd/run.go; the indirection exists because notifications cannot import
// oncall (oncall imports this package for Message and the adapters).
type PageOpener func(ctx context.Context, channelConfig json.RawMessage, rule *models.NotificationRuleWithChannel, msg Message) (opened bool, err error)

var pageOpener PageOpener

func RegisterPageOpener(opener PageOpener) {
	pageOpener = opener
}

// Grouped metrics change their firing state and escalation page in one transaction.
type MetricSeriesPager func(*sql.Tx, json.RawMessage, *models.NotificationRuleWithChannel, Message) (bool, error)

var metricSeriesPager MetricSeriesPager
var wakeMetricPager func()

func RegisterMetricSeriesPager(pager MetricSeriesPager, wake func()) {
	metricSeriesPager, wakeMetricPager = pager, wake
}
