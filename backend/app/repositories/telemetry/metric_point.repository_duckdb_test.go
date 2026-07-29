//go:build telemetry_duckdb

package telemetry

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/duckdb/duckdb-go/v2"
	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
)

func setupDuckDB(t *testing.T) {
	t.Helper()
	connector, err := duckdb.NewConnector("", nil)
	if err != nil {
		t.Fatalf("NewConnector: %v", err)
	}
	telemetry := sql.OpenDB(connector)
	if _, err := telemetry.Exec(`CREATE TABLE metric_points (
		project_id UUID NOT NULL,
		name VARCHAR NOT NULL DEFAULT '',
		value DOUBLE NOT NULL DEFAULT 0,
		tags VARCHAR NOT NULL DEFAULT '{}',
		recorded_at TIMESTAMP NOT NULL,
		server_name VARCHAR NOT NULL DEFAULT ''
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	db.DuckDBConnector = connector
	db.TelemetryDB = telemetry
	db.Driver = lit.SQLite
	models.Init(lit.SQLite)

	t.Cleanup(func() { telemetry.Close() })
}

func TestDuckDBMetricPointsRoundTrip(t *testing.T) {
	setupDuckDB(t)
	ctx := context.Background()
	repo := MetricPointRepository

	project := uuid.New()
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	points := []models.MetricPoint{
		{ProjectId: project, Name: "cpu.used_pcnt", Value: 10, Tags: map[string]string{"server_name": "a"}, RecordedAt: base},
		{ProjectId: project, Name: "cpu.used_pcnt", Value: 30, Tags: map[string]string{"server_name": "a"}, RecordedAt: base.Add(10 * time.Second)},
		{ProjectId: project, Name: "cpu.used_pcnt", Value: 50, Tags: map[string]string{"server_name": "b"}, RecordedAt: base.Add(20 * time.Second)},
	}

	if err := repo.InsertAsync(ctx, points); err != nil {
		t.Fatalf("InsertAsync (appender): %v", err)
	}

	from := base.Add(-time.Hour)
	to := base.Add(time.Hour)

	avg, err := repo.GetAverageBetween(ctx, project, "cpu.used_pcnt", from, to)
	if err != nil {
		t.Fatalf("GetAverageBetween: %v", err)
	}
	if avg != 30 {
		t.Fatalf("avg = %v, want 30", avg)
	}

	// time_bucket over a 1-minute window collapses all three points into one bucket.
	series, err := repo.QueryTimeSeries(ctx, project, "cpu.used_pcnt", from, to, 1, "avg", nil, "")
	if err != nil {
		t.Fatalf("QueryTimeSeries: %v", err)
	}
	if len(series["__all__"]) != 1 || series["__all__"][0].Value != 30 {
		t.Fatalf("QueryTimeSeries __all__ = %+v, want one bucket value 30", series["__all__"])
	}

	// json_extract_string group-by splits the same window by server tag.
	grouped, err := repo.QueryTimeSeries(ctx, project, "cpu.used_pcnt", from, to, 1, "avg", nil, "server_name")
	if err != nil {
		t.Fatalf("QueryTimeSeries grouped: %v", err)
	}
	if len(grouped["a"]) != 1 || grouped["a"][0].Value != 20 {
		t.Fatalf("grouped[a] = %+v, want value 20", grouped["a"])
	}
	if len(grouped["b"]) != 1 || grouped["b"][0].Value != 50 {
		t.Fatalf("grouped[b] = %+v, want value 50", grouped["b"])
	}

	servers, err := repo.GetDistinctServers(ctx, project, from, to)
	if err != nil {
		t.Fatalf("GetDistinctServers: %v", err)
	}
	if len(servers) != 2 || servers[0] != "a" || servers[1] != "b" {
		t.Fatalf("servers = %v, want [a b]", servers)
	}

	metrics, err := repo.DiscoverMetrics(ctx, project, from, to)
	if err != nil {
		t.Fatalf("DiscoverMetrics: %v", err)
	}
	if len(metrics) != 1 || metrics[0].Name != "cpu.used_pcnt" {
		t.Fatalf("DiscoverMetrics = %+v, want one cpu.used_pcnt", metrics)
	}
	if len(metrics[0].TagKeys) != 1 || metrics[0].TagKeys[0] != "server_name" {
		t.Fatalf("DiscoverMetrics tag keys = %v, want [server_name]", metrics[0].TagKeys)
	}

	values, err := repo.DiscoverTagValues(ctx, project, "cpu.used_pcnt", "server_name", from, to)
	if err != nil {
		t.Fatalf("DiscoverTagValues: %v", err)
	}
	if len(values) != 2 || values[0] != "a" || values[1] != "b" {
		t.Fatalf("DiscoverTagValues = %v, want [a b]", values)
	}
}
