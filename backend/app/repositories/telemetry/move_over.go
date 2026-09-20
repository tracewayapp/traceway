package telemetry

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

// MoveOverOptions tunes RunMoveOver.
type MoveOverOptions struct {
	// PageSize bounds the rows held in memory at once. Zero means the default.
	PageSize int
	// Oldest, when set, is the last day moved. History before it stays in the old tables.
	Oldest time.Time
	Log    func(format string, args ...any)
}

const (
	defaultPageSize = 20000
	moveOverDay     = 24 * time.Hour
	// An owner is recorded near its spans and exceptions, not at the same instant: an endpoint at its start, an old
	// OTel task at its end.
	ownerWindow = moveOverDay
	// A moved task is recorded at its start, which can be days before the end the old row carried.
	movedWindow = 7 * moveOverDay
)

// Entities before what hangs off them, so a day is usable as soon as its endpoints are in.
var tableOrder = []string{"endpoints", "tasks", "ai_traces", "exception_stack_traces", "spans"}

type mover struct {
	MoveOverOptions
	legacy interface {
		FindEndpoints(ctx context.Context, from, to time.Time, limit int, after *uuid.UUID) ([]models.Endpoint, error)
		FindTasks(ctx context.Context, from, to time.Time, limit int, after *uuid.UUID) ([]models.Task, error)
		FindAiTraces(ctx context.Context, from, to time.Time, limit int, after *uuid.UUID) ([]models.AiTrace, error)
		FindExceptions(ctx context.Context, from, to time.Time, limit int, after *uuid.UUID) ([]models.ExceptionStackTrace, error)
		FindSpans(ctx context.Context, from, to time.Time, limit int, after *uuid.UUID) ([]shared.LegacySpan, error)
		FindOwners(ctx context.Context, ids []uuid.UUID, from, to time.Time) ([]models.Endpoint, []models.Task, []models.AiTrace, error)
		FindBounds(ctx context.Context, table string) (oldest, newest time.Time, found bool, err error)
		FindMovedIds(ctx context.Context, table string, ids []uuid.UUID, from, to time.Time) (map[uuid.UUID]bool, error)
		FindMovedSpans(ctx context.Context, traceIds []string, from, to time.Time) (map[string]bool, error)
	}
	progress interface {
		FindProgress(ctx context.Context) ([]transactional.MoveOverDay, error)
		SaveProgress(ctx context.Context, day transactional.MoveOverDay) error
	}
}

// RunMoveOver copies history from the tables V2 replaced into the V2 tables. It runs inside the backend, because the
// embedded DuckDB file cannot be opened by a second process. It goes newest day first, so recent data is back first,
// one day of one table at a time, and records each finished day so it can stop and resume. It returns when history is
// exhausted, the context ends, or a read or a write fails. Running it again picks up where it stopped.
func RunMoveOver(ctx context.Context, options MoveOverOptions) error {
	m := &mover{MoveOverOptions: options, legacy: LegacyRepository, progress: moveOverProgress{}}
	return m.run(ctx)
}

func (m *mover) run(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if m.PageSize <= 0 {
		m.PageSize = defaultPageSize
	}
	if m.Log == nil {
		m.Log = func(format string, args ...any) { log.Printf("[tracewaybackend] move-over: "+format, args...) }
	}
	saved, err := m.progress.FindProgress(ctx)
	if err != nil {
		return fmt.Errorf("move-over progress: %w", err)
	}
	state := map[string]string{}
	for _, entry := range saved {
		state[entry.Table+"|"+entry.Day] = entry.State
	}

	type span struct{ oldest, newest time.Time }
	bounds := map[string]span{}
	var oldest, newest time.Time
	for _, table := range tableOrder {
		low, high, found, err := m.legacy.FindBounds(ctx, table)
		if err != nil {
			return fmt.Errorf("move-over bounds of %s: %w", table, err)
		}
		if !found {
			continue
		}
		low, high = low.UTC().Truncate(moveOverDay), high.UTC().Truncate(moveOverDay)
		bounds[table] = span{low, high}
		if oldest.IsZero() || low.Before(oldest) {
			oldest = low
		}
		if high.After(newest) {
			newest = high
		}
	}
	if len(bounds) == 0 {
		m.Log("the old tables are empty, nothing to move")
		return nil
	}
	if limit := m.Oldest.UTC().Truncate(moveOverDay); !m.Oldest.IsZero() && limit.After(oldest) {
		oldest = limit
	}

	for current := newest; !current.Before(oldest); current = current.Add(-moveOverDay) {
		for _, table := range tableOrder {
			held, known := bounds[table]
			label := current.Format(time.DateOnly)
			if !known || current.Before(held.oldest) || current.After(held.newest) || state[table+"|"+label] == transactional.MoveOverDone {
				continue
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			resumed := state[table+"|"+label] == transactional.MoveOverStarted
			if err := m.progress.SaveProgress(ctx, transactional.MoveOverDay{Table: table, Day: label, State: transactional.MoveOverStarted}); err != nil {
				return fmt.Errorf("move-over progress of %s %s: %w", table, label, err)
			}
			began := time.Now()
			rows, err := m.moveDay(ctx, table, current, resumed)
			if err != nil {
				return fmt.Errorf("move-over of %s %s: %w", table, label, err)
			}
			if err := m.progress.SaveProgress(ctx, transactional.MoveOverDay{Table: table, Day: label, State: transactional.MoveOverDone, Rows: rows}); err != nil {
				return fmt.Errorf("move-over progress of %s %s: %w", table, label, err)
			}
			m.Log("%s %s: %d rows in %s", label, table, rows, time.Since(began).Round(time.Millisecond))
		}
	}
	m.Log("done, every day from %s back to %s is in the V2 tables", newest.Format(time.DateOnly), oldest.Format(time.DateOnly))
	return nil
}

// moveDay walks one day in slices narrow enough to hold in memory. A slice that comes back full is halved and read
// again, so no read sorts a day. Only a single second that is still too full is paged, in id order.
func (m *mover) moveDay(ctx context.Context, table string, start time.Time, resumed bool) (int64, error) {
	var total int64
	width, end := time.Hour, start.Add(moveOverDay)
	for cursor := start; cursor.Before(end); {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		to := cursor.Add(width)
		if to.After(end) {
			to = end
		}
		var moved int64
		if to.Sub(cursor) > time.Second {
			var full bool
			var err error
			if moved, full, _, err = m.move(ctx, table, cursor, to, nil, resumed); err != nil {
				return total, err
			}
			if full {
				width = max(to.Sub(cursor)/2, time.Second).Truncate(time.Second)
				continue
			}
		} else {
			for after, full := uuid.Nil, true; full; {
				page, more, last, err := m.move(ctx, table, cursor, to, &after, resumed)
				if err != nil {
					return total, err
				}
				moved, full, after = moved+page, more, last
			}
		}
		total, cursor = total+moved, to
		// A sparse stretch widens the slice again, or one busy second would leave the rest of the day read second by second.
		if moved*2 <= int64(m.PageSize) {
			width = min(width*2, moveOverDay)
		}
	}
	return total, nil
}

// move reads one page and writes it. Without after the read is unordered, and a page that hit the limit is not written:
// the caller narrows the slice and reads again. With after the read continues in id order behind that id.
func (m *mover) move(ctx context.Context, table string, from, to time.Time, after *uuid.UUID, resumed bool) (int64, bool, uuid.UUID, error) {
	var last uuid.UUID
	var count int
	var write func() error
	switch table {
	case "endpoints":
		rows, err := m.legacy.FindEndpoints(ctx, from, to, m.PageSize, after)
		if err != nil {
			return 0, false, last, err
		}
		if count = len(rows); count > 0 {
			last = rows[count-1].Id
		}
		write = func() error { return m.writeEndpoints(ctx, rows, from, to, resumed) }
	case "tasks":
		rows, err := m.legacy.FindTasks(ctx, from, to, m.PageSize, after)
		if err != nil {
			return 0, false, last, err
		}
		if count = len(rows); count > 0 {
			last = rows[count-1].Id
		}
		write = func() error { return m.writeTasks(ctx, rows, from, to, resumed) }
	case "ai_traces":
		rows, err := m.legacy.FindAiTraces(ctx, from, to, m.PageSize, after)
		if err != nil {
			return 0, false, last, err
		}
		if count = len(rows); count > 0 {
			last = rows[count-1].Id
		}
		write = func() error { return m.writeAiTraces(ctx, rows, from, to, resumed) }
	case "exception_stack_traces":
		rows, err := m.legacy.FindExceptions(ctx, from, to, m.PageSize, after)
		if err != nil {
			return 0, false, last, err
		}
		if count = len(rows); count > 0 {
			last = rows[count-1].Id
		}
		write = func() error { return m.writeExceptions(ctx, rows, from, to, resumed) }
	case "spans":
		rows, err := m.legacy.FindSpans(ctx, from, to, m.PageSize, after)
		if err != nil {
			return 0, false, last, err
		}
		if count = len(rows); count > 0 {
			last = rows[count-1].Id
		}
		write = func() error { return m.writeSpans(ctx, rows, from, to, resumed) }
	default:
		return 0, false, last, fmt.Errorf("unknown table %q", table)
	}
	full := count >= m.PageSize
	if count == 0 || (full && after == nil) {
		return 0, full, last, nil
	}
	return int64(count), full, last, write()
}

func (m *mover) movedIds(ctx context.Context, table string, ids []uuid.UUID, from, to time.Time, resumed bool) (map[uuid.UUID]bool, error) {
	if !resumed || len(ids) == 0 {
		return nil, nil
	}
	return m.legacy.FindMovedIds(ctx, shared.LegacyTables[table], ids, from.Add(-movedWindow), to.Add(moveOverDay))
}

// insertSpans writes the spans of a page. On a resumed day it leaves out the ones the interrupted run already wrote.
func (m *mover) insertSpans(ctx context.Context, spans []models.Span, from, to time.Time, resumed bool) error {
	if resumed && len(spans) > 0 {
		seen := map[string]bool{}
		var traces []string
		for _, span := range spans {
			if !seen[span.TraceId] {
				seen[span.TraceId] = true
				traces = append(traces, span.TraceId)
			}
		}
		moved, err := m.legacy.FindMovedSpans(ctx, traces, from.Add(-movedWindow), to.Add(moveOverDay))
		if err != nil {
			return err
		}
		kept := spans[:0]
		for _, span := range spans {
			if !moved[shared.MovedSpanKey(span.ProjectId, span.TraceId, span.SpanId)] {
				kept = append(kept, span)
			}
		}
		spans = kept
	}
	if len(spans) == 0 {
		return nil
	}
	return SpanRepository.InsertAsync(ctx, spans)
}

func (m *mover) writeEndpoints(ctx context.Context, rows []models.Endpoint, from, to time.Time, resumed bool) error {
	ids := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		ids[i] = row.Id
	}
	moved, err := m.movedIds(ctx, "endpoints", ids, from, to, resumed)
	if err != nil {
		return err
	}
	var entities []models.Endpoint
	var spans []models.Span
	for _, row := range rows {
		if moved[row.Id] {
			continue
		}
		entity, span := mapEndpoint(row)
		entities, spans = append(entities, entity), append(spans, span)
	}
	if err := m.insertSpans(ctx, spans, from, to, resumed); err != nil || len(entities) == 0 {
		return err
	}
	return EndpointRepository.InsertAsync(ctx, entities)
}

func (m *mover) writeTasks(ctx context.Context, rows []models.Task, from, to time.Time, resumed bool) error {
	ids := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		ids[i] = row.Id
	}
	moved, err := m.movedIds(ctx, "tasks", ids, from, to, resumed)
	if err != nil {
		return err
	}
	var entities []models.Task
	var spans []models.Span
	for _, row := range rows {
		if moved[row.Id] {
			continue
		}
		entity, span := mapTask(row)
		entities, spans = append(entities, entity), append(spans, span)
	}
	if err := m.insertSpans(ctx, spans, from, to, resumed); err != nil || len(entities) == 0 {
		return err
	}
	return TaskRepository.InsertAsync(ctx, entities)
}

func (m *mover) writeAiTraces(ctx context.Context, rows []models.AiTrace, from, to time.Time, resumed bool) error {
	ids := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		ids[i] = row.Id
	}
	moved, err := m.movedIds(ctx, "ai_traces", ids, from, to, resumed)
	if err != nil {
		return err
	}
	var entities []models.AiTrace
	var spans []models.Span
	for _, row := range rows {
		if moved[row.Id] {
			continue
		}
		entity, span := mapAiTrace(row)
		entities, spans = append(entities, entity), append(spans, span)
	}
	if err := m.insertSpans(ctx, spans, from, to, resumed); err != nil || len(entities) == 0 {
		return err
	}
	return AiTraceRepository.InsertAsync(ctx, entities)
}

func (m *mover) owners(ctx context.Context, ids map[uuid.UUID]bool, from, to time.Time) (ownerIndex, error) {
	found := ownerIndex{}
	if len(ids) == 0 {
		return found, nil
	}
	list := make([]uuid.UUID, 0, len(ids))
	for id := range ids {
		list = append(list, id)
	}
	endpoints, tasks, aiTraces, err := m.legacy.FindOwners(ctx, list, from.Add(-ownerWindow), to.Add(ownerWindow))
	if err != nil {
		return nil, err
	}
	for _, row := range endpoints {
		entity, _ := mapEndpoint(row)
		found.add(row.ProjectId, row.Id, owner{entity.TraceId, entity.SpanId, entity.LinkedTraceId, entity.ServerName, row.RecordedAt})
	}
	for _, row := range tasks {
		entity, _ := mapTask(row)
		found.add(row.ProjectId, row.Id, owner{entity.TraceId, entity.SpanId, entity.LinkedTraceId, entity.ServerName, row.RecordedAt})
	}
	for _, row := range aiTraces {
		entity, _ := mapAiTrace(row)
		found.add(row.ProjectId, row.Id, owner{entity.TraceId, entity.SpanId, entity.LinkedTraceId, entity.ServerName, row.RecordedAt})
	}
	return found, nil
}

func (m *mover) writeExceptions(ctx context.Context, rows []models.ExceptionStackTrace, from, to time.Time, resumed bool) error {
	ids, ownerIds := make([]uuid.UUID, len(rows)), map[uuid.UUID]bool{}
	for i, row := range rows {
		ids[i] = row.Id
		if id, err := uuid.Parse(row.TraceId); err == nil {
			ownerIds[id] = true
		}
	}
	moved, err := m.movedIds(ctx, "exception_stack_traces", ids, from, to, resumed)
	if err != nil {
		return err
	}
	owners, err := m.owners(ctx, ownerIds, from, to)
	if err != nil {
		return err
	}
	var exceptions []models.ExceptionStackTrace
	for _, row := range rows {
		if !moved[row.Id] {
			exceptions = append(exceptions, mapException(row, owners))
		}
	}
	if len(exceptions) == 0 {
		return nil
	}
	return ExceptionStackTraceRepository.InsertAsync(ctx, exceptions)
}

func (m *mover) writeSpans(ctx context.Context, rows []shared.LegacySpan, from, to time.Time, resumed bool) error {
	ownerIds := map[uuid.UUID]bool{}
	for _, row := range rows {
		ownerIds[row.OwnerId] = true
	}
	owners, err := m.owners(ctx, ownerIds, from, to)
	if err != nil {
		return err
	}
	spans := make([]models.Span, len(rows))
	for i, row := range rows {
		spans[i] = mapSpan(row, owners)
	}
	return m.insertSpans(ctx, spans, from, to, resumed)
}

type ownerKey struct {
	projectId uuid.UUID
	id        uuid.UUID
}

type owner struct {
	traceId, spanId, linkedTraceId, serverName string
	recordedAt                                 time.Time
}

// ownerIndex holds every old row an id answers to. An OTel id was a span id, which repeats across traces, and a retried
// export stored the same row twice, so the owner of a span or an exception is the one recorded nearest to it.
type ownerIndex map[ownerKey][]owner

func (index ownerIndex) add(projectId, id uuid.UUID, found owner) {
	key := ownerKey{projectId, id}
	index[key] = append(index[key], found)
}

func (index ownerIndex) nearest(projectId, id uuid.UUID, at time.Time) (owner, bool) {
	var best owner
	candidates := index[ownerKey{projectId, id}]
	for i, candidate := range candidates {
		if i == 0 || candidate.recordedAt.Sub(at).Abs() < best.recordedAt.Sub(at).Abs() {
			best = candidate
		}
	}
	return best, len(candidates) > 0
}

const (
	identityPrefix   = "traceway.otel."
	traceIdAttribute = "traceway.otel.trace_id"
	zeroPadding      = "0000000000000000"

	spanKindInternal = 1
	spanKindServer   = 2
	spanKindClient   = 3
	spanKindConsumer = 5
	spanStatusError  = 2
)

func hexId(id uuid.UUID) string { return shared.NormalizeTraceId(id.String()) }

func validTraceId(id string) bool {
	return len(id) == 32 && strings.Trim(id, "0123456789abcdef") == ""
}

// spanHex undoes the old storage of an OTel span id, which was kept as a UUID with eight zero bytes in front. A native
// id is a real UUID and stays 32 characters.
func spanHex(id string) string {
	id = shared.NormalizeTraceId(id)
	if len(id) == 32 && strings.HasPrefix(id, zeroPadding) {
		return id[16:]
	}
	return id
}

// traceIds picks the trace a row belongs to: the OTel trace id the old ingest kept in the attributes, else the old
// distributed trace id, else the row's own id. When the first two differ, the distributed id was the browser's.
func traceIds(attributes map[string]string, distributed string, id uuid.UUID) (traceId, linkedTraceId string) {
	distributed = shared.NormalizeTraceId(distributed)
	if !validTraceId(distributed) {
		distributed = ""
	}
	if source := shared.NormalizeTraceId(attributes[traceIdAttribute]); validTraceId(source) {
		if distributed != source {
			linkedTraceId = distributed
		}
		return source, linkedTraceId
	}
	if distributed != "" {
		return distributed, ""
	}
	return hexId(id), ""
}

func withoutIdentity(attributes map[string]string) map[string]string {
	for key := range attributes {
		if strings.HasPrefix(key, identityPrefix) {
			delete(attributes, key)
		}
	}
	return attributes
}

func mapEndpoint(row models.Endpoint) (models.Endpoint, models.Span) {
	row.TraceId, row.LinkedTraceId = traceIds(row.Attributes, row.TraceId, row.Id)
	if row.SpanId = spanHex(row.SpanId); row.SpanId == "" {
		row.SpanId = hexId(row.Id)
	}
	row.ParentSpanId, row.Attributes = "", withoutIdentity(row.Attributes)
	span := models.Span{ProjectId: row.ProjectId, TraceId: row.TraceId, SpanId: row.SpanId, Name: row.Endpoint, StartTime: row.RecordedAt,
		RecordedAt: row.RecordedAt, Duration: row.Duration, SpanKind: spanKindServer, ServiceName: row.ServerName, Attributes: row.Attributes}
	if row.StatusCode >= 500 {
		span.StatusCode = spanStatusError
	}
	return row, span
}

func mapTask(row models.Task) (models.Task, models.Span) {
	row.TraceId, row.LinkedTraceId = traceIds(row.Attributes, row.TraceId, row.Id)
	kind := int32(spanKindConsumer)
	if row.SpanId = spanHex(row.SpanId); row.SpanId == "" {
		row.SpanId, kind = hexId(row.Id), spanKindInternal
	} else {
		// The old ingest recorded an OTel task at its end. V2 records everything at its span's start.
		row.RecordedAt = row.RecordedAt.Add(-row.Duration)
	}
	row.ParentSpanId, row.Attributes = "", withoutIdentity(row.Attributes)
	return row, models.Span{ProjectId: row.ProjectId, TraceId: row.TraceId, SpanId: row.SpanId, Name: row.TaskName, StartTime: row.RecordedAt,
		RecordedAt: row.RecordedAt, Duration: row.Duration, SpanKind: kind, ServiceName: row.ServerName, Attributes: row.Attributes}
}

func mapAiTrace(row models.AiTrace) (models.AiTrace, models.Span) {
	row.TraceId, row.LinkedTraceId = traceIds(row.Attributes, row.TraceId, row.Id)
	// The old table kept no span id. A call below the root was stored under its span id. The root was stored under the
	// trace id, which serves as its span id here: it only has to be the same on the row and on the span.
	row.SpanId, row.ParentSpanId, row.Attributes = spanHex(row.Id.String()), "", withoutIdentity(row.Attributes)
	return row, models.Span{ProjectId: row.ProjectId, TraceId: row.TraceId, SpanId: row.SpanId, Name: row.TraceName, StartTime: row.RecordedAt,
		RecordedAt: row.RecordedAt, Duration: row.Duration, SpanKind: spanKindClient, ServiceName: row.ServerName, Attributes: row.Attributes}
}

// mapException puts an exception on its owner's span. The old row named the owner, not the span it happened on, and
// the owner's span is where a detail page and the issue page look first.
func mapException(row models.ExceptionStackTrace, owners ownerIndex) models.ExceptionStackTrace {
	distributed := shared.NormalizeTraceId(row.LinkedTraceId)
	if !validTraceId(distributed) {
		distributed = ""
	}
	ownerId, err := uuid.Parse(row.TraceId)
	row.Attributes, row.SpanId, row.LinkedTraceId = withoutIdentity(row.Attributes), "", ""
	switch found, known := owners.nearest(row.ProjectId, ownerId, row.RecordedAt); {
	case err != nil:
		row.TraceId, row.TraceType = distributed, ""
	case known:
		row.TraceId, row.SpanId, row.LinkedTraceId = found.traceId, found.spanId, found.linkedTraceId
	default:
		row.TraceId, row.TraceType = hexId(ownerId), ""
	}
	if row.LinkedTraceId == "" && distributed != row.TraceId {
		row.LinkedTraceId = distributed
	}
	return row
}

func mapSpan(row shared.LegacySpan, owners ownerIndex) models.Span {
	span := models.Span{ProjectId: row.ProjectId, TraceId: hexId(row.OwnerId), SpanId: spanHex(row.Id.String()), ParentSpanId: spanHex(row.ParentSpanId),
		Name: row.Name, StartTime: row.StartTime, RecordedAt: row.RecordedAt, Duration: row.Duration, Attributes: withoutIdentity(row.Attributes)}
	found, known := owners.nearest(row.ProjectId, row.OwnerId, row.RecordedAt)
	if known {
		span.TraceId, span.ServiceName = found.traceId, found.serverName
	}
	if span.ParentSpanId == "" {
		// Only a native span was stored without a parent. It hangs under its run, whose span id is the run's own id.
		if span.ParentSpanId = hexId(row.OwnerId); known {
			span.ParentSpanId = found.spanId
		}
	}
	return span
}
