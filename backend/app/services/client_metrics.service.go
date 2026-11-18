package services

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/clientmetrics"
	"github.com/tracewayapp/traceway/backend/v2/app/config"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/geoip"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
)

// repositorySink writes a flush of the client metrics aggregator: one batch per
// table, whatever number of requests fed it.
type repositorySink struct{}

func (repositorySink) WriteHistograms(ctx context.Context, cells []shared.MetricHistogramCell) error {
	return telemetry.MetricHistogramRepository.InsertCells(ctx, cells)
}

func (repositorySink) WritePoints(ctx context.Context, points []models.MetricPoint) error {
	if len(points) == 0 {
		return nil
	}
	return telemetry.MetricPointRepository.InsertAsync(ctx, points)
}

func (repositorySink) WriteExemplars(ctx context.Context, exemplars []shared.MetricExemplar) error {
	return telemetry.MetricHistogramRepository.InsertExemplars(ctx, exemplars)
}

// guardStore keeps what the guard admitted in the main database, so every instance
// applies one set of caps and a restart does not start them again.
type guardStore struct{}

func (guardStore) SaveGuardEntries(ctx context.Context, entries []clientmetrics.GuardEntry) error {
	_, err := db.ExecuteTransactionContext(ctx, func(tx *sql.Tx) (struct{}, error) {
		return struct{}{}, transactional.ClientMetricGuardRepository.Insert(tx, entries, time.Now())
	})
	return err
}

func (guardStore) LoadGuardEntries(ctx context.Context, day int64, since time.Time) ([]clientmetrics.GuardEntry, error) {
	return db.ExecuteTransactionContext(ctx, func(tx *sql.Tx) ([]clientmetrics.GuardEntry, error) {
		return transactional.ClientMetricGuardRepository.ListSince(tx, day, since)
	})
}

func (guardStore) PruneGuardEntries(ctx context.Context, beforeDay int64) error {
	_, err := db.ExecuteTransactionContext(ctx, func(tx *sql.Tx) (struct{}, error) {
		return struct{}{}, transactional.ClientMetricGuardRepository.PruneBefore(tx, beforeDay)
	})
	return err
}

func envInt(name string) int {
	n, err := strconv.Atoi(os.Getenv(name))
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

var lastClientMetricsError atomic.Int64

// Built on first use, not at package init: godotenv.Load() runs inside run().
var clientMetricsPipeline = sync.OnceValue(func() *clientmetrics.Pipeline {
	return clientmetrics.New(repositorySink{}, clientmetrics.Config{
		FlushInterval: time.Duration(envInt("CLIENT_METRICS_FLUSH_SECONDS")) * time.Second,
		MaxCells:      envInt("CLIENT_METRICS_MAX_CELLS"),
		DedupMB:       envInt("CLIENT_METRICS_DEDUP_MB"),
		Instances:     clientMetricsInstances,
		Store:         guardStore{},
		HotSeries:     telemetry.MetricHistogramRepository.HotSeries,
		OnError: func(err error) {
			now := time.Now().Unix()
			if last := lastClientMetricsError.Load(); now-last >= 60 && lastClientMetricsError.CompareAndSwap(last, now) {
				traceway.CaptureException(fmt.Errorf("client metrics flush failed: %w", err))
			}
		},
		OnMetrics: registerClientMetrics,
	})
})

func registerClientMetrics(ctx context.Context, projectId uuid.UUID, metrics []clientmetrics.SeenMetric) {
	entries := make([]transactional.MetricRegistrationEntry, 0, len(metrics))
	for _, metric := range metrics {
		metricType := models.MetricTypeCounter
		if metric.Histogram {
			metricType = models.MetricTypeHistogram
		}
		entries = append(entries, transactional.MetricRegistrationEntry{Name: metric.Name, Unit: metric.Unit, MetricType: metricType})
	}
	AutoRegisterMetricsWithUnits(context.WithoutCancel(ctx), projectId, entries)
}

// clientMetricsInstances is set by the binary once it knows how many instances share
// the traffic; nil means one.
var clientMetricsInstances func() int

// SetClientMetricsInstances tells the aggregator how to count the live instances.
// Call it before the first request.
func SetClientMetricsInstances(count func() int) {
	clientMetricsInstances = count
}

var countryResolver = sync.OnceValue(func() *geoip.Resolver {
	resolver, err := geoip.New(os.Getenv("GEOIP_COUNTRY_HEADER"), os.Getenv("GEOIP_DB_PATH"))
	if err != nil {
		config.Logf("WARNING: country lookup is off: %v", err)
		traceway.CaptureException(fmt.Errorf("country lookup is off: %w", err))
		resolver, _ = geoip.New(os.Getenv("GEOIP_COUNTRY_HEADER"), "")
	}
	return resolver
})

// ClientOf describes the sender of an ingest request to the client metrics
// aggregator: its address for the per-address cap, and its country as a dimension.
func ClientOf(header func(string) string, ip string) clientmetrics.Client {
	return clientmetrics.Client{IP: ip, Country: countryResolver().Country(header, ip)}
}

// ClientMetrics is the aggregator every client metric goes through.
func ClientMetrics() *clientmetrics.Pipeline {
	return clientMetricsPipeline()
}

func StartClientMetrics(ctx context.Context) {
	ClientMetrics().Start(ctx)
}
