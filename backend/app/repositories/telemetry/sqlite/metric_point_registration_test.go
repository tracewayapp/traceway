//go:build !telemetry_ch && !telemetry_duckdb

package sqlite

import (
	"reflect"
	"testing"

	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
)

// DiscoverTagValues used to register its row model on every call. lit's registry is a plain
// map, so a dashboard reading tag values while another request queried crashed the server
// with "concurrent map read and map write". The model must be registered at startup.
func TestTagValueRowIsRegisteredAtStartup(t *testing.T) {
	models.Init(lit.SQLite)
	if _, err := lit.GetFieldMap(reflect.TypeOf(tagValueRow{})); err != nil {
		t.Fatalf("tagValueRow is not registered by models.Init: %v", err)
	}
}
