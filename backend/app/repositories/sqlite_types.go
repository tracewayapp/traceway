//go:build !pgch

package repositories

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/models"
)

// SQLiteTime handles scanning time.Time from SQLite DATETIME text columns
// and serializing time.Time for SQLite inserts.
type SQLiteTime struct {
	time.Time
}

func NewSQLiteTime(t time.Time) SQLiteTime {
	return SQLiteTime{Time: t}
}

func (t *SQLiteTime) Scan(src interface{}) error {
	switch v := src.(type) {
	case nil:
		t.Time = time.Time{}
		return nil
	case string:
		return t.parseString(v)
	case []byte:
		return t.parseString(string(v))
	case time.Time:
		t.Time = v
		return nil
	}
	return fmt.Errorf("SQLiteTime.Scan: unsupported type %T", src)
}

func (t *SQLiteTime) parseString(s string) error {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05Z"} {
		if parsed, err := time.Parse(layout, s); err == nil {
			t.Time = parsed
			return nil
		}
	}
	return fmt.Errorf("SQLiteTime: cannot parse %q", s)
}

func (t SQLiteTime) Value() (driver.Value, error) {
	return t.Time.UTC().Format("2006-01-02T15:04:05.000000000Z07:00"), nil
}

// SQLiteJSONMap handles scanning map[string]string from SQLite TEXT JSON columns
// and serializing map[string]string for SQLite inserts.
type SQLiteJSONMap map[string]string

func NewSQLiteJSONMap(m map[string]string) SQLiteJSONMap {
	if m == nil {
		return nil
	}
	return SQLiteJSONMap(m)
}

func (m *SQLiteJSONMap) Scan(src interface{}) error {
	if src == nil {
		*m = nil
		return nil
	}
	var data []byte
	switch v := src.(type) {
	case string:
		if v == "" || v == "{}" {
			*m = nil
			return nil
		}
		data = []byte(v)
	case []byte:
		if len(v) == 0 {
			*m = nil
			return nil
		}
		data = v
	default:
		return fmt.Errorf("SQLiteJSONMap.Scan: unsupported type %T", src)
	}
	return json.Unmarshal(data, m)
}

func (m SQLiteJSONMap) Value() (driver.Value, error) {
	if m == nil {
		return "{}", nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "{}", err
	}
	return string(b), nil
}

// timeSeriesResult is a generic result type for all time-bucketed aggregation queries.
// Queries should alias the bucket as "bucket" and the value as "agg_value".
type timeSeriesResult struct {
	Bucket string  `lit:"bucket"`
	Value  float64 `lit:"agg_value"`
}

func timeSeriesResultsToPoints(results []*timeSeriesResult) []models.TimeSeriesPoint {
	points := make([]models.TimeSeriesPoint, 0, len(results))
	for _, r := range results {
		t, _ := time.Parse("2006-01-02 15:04:05", r.Bucket)
		points = append(points, models.TimeSeriesPoint{Timestamp: t, Value: r.Value})
	}
	return points
}

// groupedTimeSeriesResult is for time-bucketed queries with a group-by key.
type groupedTimeSeriesResult struct {
	Bucket   string  `lit:"bucket"`
	GroupKey *string `lit:"group_key"`
	Value    float64 `lit:"agg_value"`
}

// filePathResult is for single-column file_path queries.
type filePathResult struct {
	FilePath string `lit:"file_path"`
}

func init() {
	models.ExtensionModelRegistrations = append(models.ExtensionModelRegistrations, func(driver lit.Driver) {
		lit.RegisterModel[timeSeriesResult](driver)
		lit.RegisterModel[groupedTimeSeriesResult](driver)
		lit.RegisterModel[filePathResult](driver)
	})
}
