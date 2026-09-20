//go:build telemetry_ch

package clickhouse

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/chdb"
	"github.com/tracewayapp/traceway/backend/app/models"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func partitionRows(days, perDay int) []clickhouseOtelRow {
	rows := make([]clickhouseOtelRow, 0, days*perDay)
	for i := range perDay {
		for day := days; day > 0; day-- {
			rows = append(rows, clickhouseOtelRow{values: []any{day, i}, day: int64(day)})
		}
	}
	return rows
}

func TestPartitionedInsertsRespectThePartitionLimit(t *testing.T) {
	if inserts, overflow := partitionedInserts(partitionRows(clickhouseMaxPartitionsPerInsert, 3)); len(inserts) != 1 || len(overflow) != 0 || len(inserts[0]) != 300 {
		t.Fatalf("a request within the limit must stay one insert: %d inserts, %d overflow", len(inserts), len(overflow))
	}
	inserts, overflow := partitionedInserts(partitionRows(130, 3))
	if len(inserts) != 2 || len(overflow) != 0 {
		t.Fatalf("130 days need two inserts: %d inserts, %d overflow", len(inserts), len(overflow))
	}
	stored, seen := 0, map[int64]int{}
	for index, insert := range inserts {
		days := map[int64]bool{}
		for _, row := range insert {
			days[row.day] = true
		}
		if len(days) > clickhouseMaxPartitionsPerInsert {
			t.Fatalf("insert %d touches %d partitions", index, len(days))
		}
		for day := range days {
			seen[day]++
		}
		stored += len(insert)
	}
	if stored != 390 {
		t.Fatalf("rows lost while splitting: %d", stored)
	}
	for day, inserts := range seen {
		if inserts != 1 {
			t.Fatalf("day %d was split across %d inserts", day, inserts)
		}
	}
	limit := clickhouseMaxPartitionsPerInsert * clickhouseMaxPartitionedInserts
	inserts, overflow = partitionedInserts(partitionRows(limit, 1))
	if len(inserts) != clickhouseMaxPartitionedInserts || len(overflow) != 0 {
		t.Fatalf("the bound itself must fit: %d inserts, %d overflow", len(inserts), len(overflow))
	}
	inserts, overflow = partitionedInserts(partitionRows(limit+50, 2))
	if len(inserts) != clickhouseMaxPartitionedInserts || len(overflow) != 100 {
		t.Fatalf("one request must not cause unbounded inserts: %d inserts, %d overflow", len(inserts), len(overflow))
	}
}

func TestOtelSpanInsertSpanningManyDays(t *testing.T) {
	ctx, conn := scratchDatabase(t)
	previous := chdb.Conn
	chdb.Conn = conn
	defer func() { chdb.Conn = previous }()
	createSpansTable(t, ctx, conn)

	const days = 130
	project, first := uuid.New(), time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	spans := make([]models.OtelSpan, days)
	for day := range spans {
		start := uint64(first.AddDate(0, 0, day).UnixNano())
		source := &tracepb.Span{TraceId: make([]byte, 16), SpanId: []byte{0, 0, 0, 0, 0, 0, 1, byte(day + 1)}, Name: "buffered", StartTimeUnixNano: start, EndTimeUnixNano: start + 1000}
		source.TraceId[15] = byte(day + 1)
		spans[day] = models.OtelSpan{OTLP: source, Span: models.Span{ProjectId: project, TraceId: fmt.Sprintf("%032x", day+1), SpanId: fmt.Sprintf("%016x", 256+day+1), Name: "buffered"}}
	}
	rejected, err := OtelSpanRepository.InsertAsync(ctx, spans)
	if err != nil || rejected != 0 {
		t.Fatalf("a request spanning %d days must be stored: rejected=%d err=%v", days, rejected, err)
	}
	var rows, partitions uint64
	if err := conn.QueryRow(ctx, "SELECT count(), uniqExact(toYYYYMMDD(recorded_at)) FROM spans_v2 WHERE project_id = ?", project).Scan(&rows, &partitions); err != nil {
		t.Fatal(err)
	}
	if rows != days || partitions != days {
		t.Fatalf("stored %d rows in %d daily partitions, want %d in %d", rows, partitions, days, days)
	}
}
