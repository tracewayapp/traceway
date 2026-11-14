package otelcontrollers

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/controllers/clientcontrollers"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
)

const (
	eventException      = "exception"
	eventMessage        = "traceway.message"
	eventSessionStart   = "session.start"
	eventSessionEnd     = "session.end"
	eventSessionUpdate  = "traceway.session.update"
	attrEventName       = "event.name"
	attrSessionStarted  = "traceway.session.started_at"
	sessionAttrsPrefix  = "traceway.session."
	logExceptionIdSpace = "log"
)

type convertedLogs struct {
	Records    []models.LogRecord
	Exceptions []models.ExceptionStackTrace
	Sessions   []models.Session
}

// convertLogs stores every record and additionally turns the events browser SDKs emit into the rows the dashboard
// reads: exception and traceway.message events become issues, and session lifecycle events upsert the session.
func convertLogs(ctx context.Context, existingProject *models.Project, projectId uuid.UUID, clientIP string, req *collogspb.ExportLogsServiceRequest) convertedLogs {
	var result convertedLogs
	clientSide := existingProject != nil && clientcontrollers.IsFrontendFramework(existingProject.Framework)
	ingestedAt := time.Now().UTC()
	sessions := map[uuid.UUID]int{}

	for _, rl := range req.ResourceLogs {
		resource := rl.GetResource().GetAttributes()
		resAttrs := extractAttributes(resource)
		shared.NormalizeStoredAttributes(resAttrs)
		serviceName := getStringAttribute(resource, "service.name")
		appVersion := getStringAttribute(resource, "service.version")
		language := getStringAttribute(resource, "telemetry.sdk.language")
		resourceSchemaUrl := rl.GetSchemaUrl()
		clientResource := resource
		if !clientSide {
			clientResource = nil
		}

		for _, sl := range rl.ScopeLogs {
			scope := sl.GetScope()
			scopeName := scope.GetName()
			scopeVersion := scope.GetVersion()
			scopeAttrs := extractAttributes(scope.GetAttributes())
			shared.NormalizeStoredAttributes(scopeAttrs)
			scopeSchemaUrl := sl.GetSchemaUrl()

			for _, lr := range sl.LogRecords {
				record := toLogRecord(projectId, lr, serviceName, resourceSchemaUrl, resAttrs, scopeSchemaUrl, scopeName, scopeVersion, scopeAttrs)
				if record.LogAttributes == nil {
					record.LogAttributes = map[string]string{}
				}
				// Producers on the older events convention name the event only in the event.name attribute.
				eventName := lr.GetEventName()
				if eventName == "" {
					eventName = record.LogAttributes[attrEventName]
				} else if record.LogAttributes[attrEventName] == "" {
					record.LogAttributes[attrEventName] = eventName
				}
				debugIds := debugIdsFrom(lr.Attributes, resource)
				shared.NormalizeStoredAttributes(record.LogAttributes)
				if stackTrace := record.LogAttributes["exception.stacktrace"]; stackTrace != "" {
					record.LogAttributes["exception.stacktrace"] = otelSymbolicateJs(existingProject, projectId, ctx, stackTrace, language, scopeName, debugIds)
				}

				issue := logIssue{
					ctx: ctx, project: existingProject,
					record: record, lr: lr, resource: resource, clientResource: clientResource,
					serviceName: serviceName, appVersion: appVersion, language: language, ingestedAt: ingestedAt,
				}
				switch eventName {
				case eventException:
					result.Exceptions = append(result.Exceptions, issue.exception(projectId))
				case eventMessage:
					if msg, ok := issue.message(projectId); ok {
						result.Exceptions = append(result.Exceptions, msg)
					}
				case eventSessionStart, eventSessionEnd, eventSessionUpdate:
					if session, ok := sessionFromLog(eventName, lr, resource, record, clientIP, ingestedAt); ok {
						session.ProjectId = projectId
						result.Sessions = mergeSession(result.Sessions, sessions, session)
					}
				}
				result.Records = append(result.Records, record)
			}
		}
	}

	return result
}

type logIssue struct {
	ctx            context.Context
	project        *models.Project
	record         models.LogRecord
	lr             *logspb.LogRecord
	resource       []*commonpb.KeyValue
	clientResource []*commonpb.KeyValue
	serviceName    string
	appVersion     string
	language       string
	ingestedAt     time.Time
}

// exception builds the issue row from an exception event. The record's stack trace was symbolicated in place
// already, and the header passes through symbolication, so formatting it afterwards matches the span path.
func (l logIssue) exception(projectId uuid.UUID) models.ExceptionStackTrace {
	attrs := l.record.LogAttributes
	stackTrace := formatExceptionStackTrace(attrs["exception.type"], attrs["exception.message"], attrs["exception.stacktrace"])
	// Dart symbolication reads the error line together with the frames, so it runs on the formatted trace.
	stackTrace = otelSymbolicateDart(l.project, projectId, l.ctx, stackTrace, l.language)
	return l.row(projectId, stackTrace, clientcontrollers.ComputeExceptionHash(stackTrace, false), false)
}

func (l logIssue) message(projectId uuid.UUID) (models.ExceptionStackTrace, bool) {
	body := l.record.Body
	if body == "" {
		body = l.record.LogAttributes["exception.message"]
	}
	if body == "" {
		return models.ExceptionStackTrace{}, false
	}
	return l.row(projectId, body, clientcontrollers.ComputeExceptionHash(body, true), true), true
}

func (l logIssue) row(projectId uuid.UUID, stackTrace, hash string, isMessage bool) models.ExceptionStackTrace {
	ts := l.lr.TimeUnixNano
	if ts == 0 {
		ts = l.lr.ObservedTimeUnixNano
	}
	recordedAt, _, _ := shared.OtelStorageTimes(ts, l.ingestedAt)
	attrs := exceptionAttributes(l.clientResource, l.record.LogAttributes)
	delete(attrs, attrEventName)
	if isJsLanguage(l.language) || isAndroidLanguage(l.language) {
		attrs["telemetry.sdk.language"] = l.language
	}
	id := exceptionIdFrom(l.lr.Attributes, func() uuid.UUID {
		// A retried export carries the same record, so it must map to the same row.
		return uuid.NewSHA1(projectId, []byte(fmt.Sprintf("%s|%d|%s|%s|%s", logExceptionIdSpace, ts, l.record.TraceId, l.record.SpanId, hash)))
	})
	return models.ExceptionStackTrace{
		Id:            id,
		ProjectId:     projectId,
		TraceId:       l.record.TraceId,
		SpanId:        l.record.SpanId,
		ExceptionHash: hash,
		StackTrace:    stackTrace,
		RecordedAt:    recordedAt,
		Attributes:    attrs,
		AppVersion:    l.appVersion,
		ServerName:    l.serviceName,
		IsMessage:     isMessage,
		SessionId:     sessionIdFrom(l.lr.Attributes, l.resource),
	}
}

// sessionFromLog turns a lifecycle event into a session upsert. Every event must name the session's start: on
// ClickHouse started_at is part of the sorting key, so an update with a different start would create a second
// session instead of replacing the first.
func sessionFromLog(eventName string, lr *logspb.LogRecord, resource []*commonpb.KeyValue, record models.LogRecord, clientIP string, ingestedAt time.Time) (models.Session, bool) {
	id := sessionIdFrom(lr.Attributes, resource)
	if id == nil {
		return models.Session{}, false
	}
	ts := lr.TimeUnixNano
	if ts == 0 {
		ts = lr.ObservedTimeUnixNano
	}
	eventTime, _, _ := shared.OtelStorageTimes(ts, ingestedAt)

	startedAt, ok := sessionStartedAt(lr.Attributes)
	if !ok {
		if eventName != eventSessionStart {
			return models.Session{}, false
		}
		startedAt = eventTime
	}

	attrs := clientResourceAttributes(resource)
	for key, value := range record.LogAttributes {
		attrs[key] = value
	}
	for key := range attrs {
		if strings.HasPrefix(key, sessionAttrsPrefix) {
			delete(attrs, key)
		}
	}
	delete(attrs, attrSessionId)
	delete(attrs, attrEventName)
	if clientIP != "" {
		attrs["client.ip"] = clientIP
	}

	if record.ScopeName == "@tracewayapp/frontend" && record.ScopeVersion != "" {
		attrs["traceway.sdk.name"] = record.ScopeName
		attrs["traceway.sdk.version"] = record.ScopeVersion
	}

	session := models.Session{
		Id:         *id,
		StartedAt:  startedAt,
		ClientIP:   clientIP,
		Attributes: attrs,
		AppVersion: getStringAttribute(resource, "service.version"),
		ServerName: getStringAttribute(resource, "service.name"),
	}
	if eventName == eventSessionEnd {
		endedAt := eventTime
		session.EndedAt = &endedAt
		session.Duration = max(endedAt.Sub(startedAt).Nanoseconds(), 0)
	}
	return session, true
}

func sessionStartedAt(attrs []*commonpb.KeyValue) (time.Time, bool) {
	if ms, ok := getIntAttribute(attrs, attrSessionStarted); ok && ms > 0 {
		return time.UnixMilli(ms).UTC(), true
	}
	raw := getStringAttribute(attrs, attrSessionStarted)
	if raw == "" {
		return time.Time{}, false
	}
	if ms, err := strconv.ParseInt(raw, 10, 64); err == nil && ms > 0 {
		return time.UnixMilli(ms).UTC(), true
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t.UTC(), true
	}
	return time.Time{}, false
}

// mergeSession keeps one upsert per session within a request: the latest snapshot wins and an end, once seen, stays.
func mergeSession(sessions []models.Session, index map[uuid.UUID]int, next models.Session) []models.Session {
	i, ok := index[next.Id]
	if !ok {
		index[next.Id] = len(sessions)
		return append(sessions, next)
	}
	prev := sessions[i]
	if next.EndedAt == nil && prev.EndedAt != nil {
		next.EndedAt = prev.EndedAt
		next.Duration = prev.Duration
	}
	sessions[i] = next
	return sessions
}

func toLogRecord(
	projectId uuid.UUID,
	lr *logspb.LogRecord,
	serviceName string,
	resourceSchemaUrl string,
	resourceAttrs map[string]string,
	scopeSchemaUrl string,
	scopeName string,
	scopeVersion string,
	scopeAttrs map[string]string,
) models.LogRecord {
	ts := lr.TimeUnixNano
	if ts == 0 {
		ts = lr.ObservedTimeUnixNano
	}

	severityNumber := uint8(lr.SeverityNumber)
	severityText := strings.ToUpper(lr.SeverityText)
	if severityText == "" {
		severityText = severityTextFromNumber(severityNumber)
	}

	var traceIDHex, spanIDHex string
	if validOtelID(lr.TraceId, 16) {
		traceIDHex = hex.EncodeToString(lr.TraceId)
	}
	if validOtelID(lr.SpanId, 8) {
		spanIDHex = hex.EncodeToString(lr.SpanId)
	}

	return models.LogRecord{
		Id:                 uuid.New(),
		ProjectId:          projectId,
		Timestamp:          shared.OtelNanosToTime(ts),
		TraceId:            traceIDHex,
		SpanId:             spanIDHex,
		TraceFlags:         uint8(lr.Flags),
		SeverityText:       severityText,
		SeverityNumber:     severityNumber,
		ServiceName:        serviceName,
		Body:               anyValueToString(lr.Body),
		ResourceSchemaUrl:  resourceSchemaUrl,
		ResourceAttributes: resourceAttrs,
		ScopeSchemaUrl:     scopeSchemaUrl,
		ScopeName:          scopeName,
		ScopeVersion:       scopeVersion,
		ScopeAttributes:    scopeAttrs,
		LogAttributes:      extractAttributes(lr.Attributes),
	}
}

func severityTextFromNumber(n uint8) string {
	switch {
	case n == 0:
		return ""
	case n <= 4:
		return "TRACE"
	case n <= 8:
		return "DEBUG"
	case n <= 12:
		return "INFO"
	case n <= 16:
		return "WARN"
	case n <= 20:
		return "ERROR"
	default:
		return "FATAL"
	}
}

func anyValueToString(v *commonpb.AnyValue) string {
	if v == nil || v.Value == nil {
		return ""
	}
	switch val := v.Value.(type) {
	case *commonpb.AnyValue_StringValue:
		return val.StringValue
	case *commonpb.AnyValue_IntValue:
		return strconv.FormatInt(val.IntValue, 10)
	case *commonpb.AnyValue_DoubleValue:
		return strconv.FormatFloat(val.DoubleValue, 'g', -1, 64)
	case *commonpb.AnyValue_BoolValue:
		return strconv.FormatBool(val.BoolValue)
	case *commonpb.AnyValue_BytesValue:
		return string(val.BytesValue)
	case *commonpb.AnyValue_ArrayValue:
		items := make([]interface{}, 0, len(val.ArrayValue.Values))
		for _, item := range val.ArrayValue.Values {
			items = append(items, anyValueToJSON(item))
		}
		if b, err := json.Marshal(items); err == nil {
			return string(b)
		}
		return ""
	case *commonpb.AnyValue_KvlistValue:
		m := make(map[string]interface{}, len(val.KvlistValue.Values))
		for _, kv := range val.KvlistValue.Values {
			m[kv.Key] = anyValueToJSON(kv.Value)
		}
		if b, err := json.Marshal(m); err == nil {
			return string(b)
		}
		return ""
	}
	return ""
}

func anyValueToJSON(v *commonpb.AnyValue) interface{} {
	if v == nil || v.Value == nil {
		return nil
	}
	switch val := v.Value.(type) {
	case *commonpb.AnyValue_StringValue:
		return val.StringValue
	case *commonpb.AnyValue_IntValue:
		return val.IntValue
	case *commonpb.AnyValue_DoubleValue:
		return val.DoubleValue
	case *commonpb.AnyValue_BoolValue:
		return val.BoolValue
	case *commonpb.AnyValue_BytesValue:
		return val.BytesValue
	case *commonpb.AnyValue_ArrayValue:
		out := make([]interface{}, 0, len(val.ArrayValue.Values))
		for _, item := range val.ArrayValue.Values {
			out = append(out, anyValueToJSON(item))
		}
		return out
	case *commonpb.AnyValue_KvlistValue:
		out := make(map[string]interface{}, len(val.KvlistValue.Values))
		for _, kv := range val.KvlistValue.Values {
			out[kv.Key] = anyValueToJSON(kv.Value)
		}
		return out
	}
	return nil
}
