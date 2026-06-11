package otelcontrollers

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/google/uuid"
	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"

	"github.com/tracewayapp/traceway/backend/app/models"
)

func convertLogs(existingProject *models.Project, ctx context.Context, projectId uuid.UUID, req *collogspb.ExportLogsServiceRequest) []models.LogRecord {
	var records []models.LogRecord

	for _, rl := range req.ResourceLogs {
		resAttrs := extractAttributes(rl.GetResource().GetAttributes())
		serviceName := getStringAttribute(rl.GetResource().GetAttributes(), "service.name")
		language := getStringAttribute(rl.GetResource().GetAttributes(), "telemetry.sdk.language")
		resourceSchemaUrl := rl.GetSchemaUrl()

		for _, sl := range rl.ScopeLogs {
			scope := sl.GetScope()
			scopeName := scope.GetName()
			scopeVersion := scope.GetVersion()
			scopeAttrs := extractAttributes(scope.GetAttributes())
			scopeSchemaUrl := sl.GetSchemaUrl()

			for _, lr := range sl.LogRecords {
				record := toLogRecord(projectId, lr, serviceName, resourceSchemaUrl, resAttrs, scopeSchemaUrl, scopeName, scopeVersion, scopeAttrs)
				if stackTrace := record.LogAttributes["exception.stacktrace"]; stackTrace != "" {
					record.LogAttributes["exception.stacktrace"] = otelSymbolicateJs(existingProject, projectId, ctx, stackTrace, language, scopeName)
				}
				records = append(records, record)
			}
		}
	}

	return records
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

	// JSON-OTLP trace_id/span_id arrive as 24/12 base64 bytes — round-trip through the UUID helpers.
	var traceIDHex, spanIDHex string
	if u := otelTraceIDToUUID(lr.TraceId); u != uuid.Nil {
		traceIDHex = hex.EncodeToString(u[:])
	}
	if u := otelSpanIDToUUID(lr.SpanId); u != uuid.Nil {
		spanIDHex = spanUUIDToHex(u)
	}

	return models.LogRecord{
		Id:                 uuid.New(),
		ProjectId:          projectId,
		Timestamp:          nanoToTime(ts),
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
