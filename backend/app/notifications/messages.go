package notifications

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

func buildErrorRateMessage(rate float64, threshold float64, window int, projectName string) Message {
	severity := SeverityWarning
	if rate >= threshold*2 {
		severity = SeverityCritical
	}
	return Message{
		Subject:  fmt.Sprintf("[%s] Error rate %.1f%% exceeds %.1f%%", projectName, rate, threshold),
		Body:     fmt.Sprintf("The error rate has reached %.1f%% over the last %d minutes (threshold: %.1f%%).", rate, window, threshold),
		Severity: severity,
		URL:      "/issues?preset=1h",
	}
}

func endpointTimeRangeURL(now time.Time) string {
	from := now.Add(-1 * time.Minute).UTC().Format(time.RFC3339)
	to := now.Add(2 * time.Minute).UTC().Format(time.RFC3339)
	return fmt.Sprintf("/endpoints?from=%s&to=%s", from, to)
}

func buildEndpointLatencyMessage(percentile string, latencyMs float64, thresholdMs float64, endpoint string, projectName string) Message {
	return Message{
		Subject:  fmt.Sprintf("[%s] %s latency %.0fms on %s", projectName, percentile, latencyMs, endpoint),
		Body:     fmt.Sprintf("The %s latency for %s has reached %.0fms (threshold: %.0fms).", percentile, endpoint, latencyMs, thresholdMs),
		Severity: SeverityWarning,
		URL:      endpointTimeRangeURL(time.Now()),
	}
}

func buildApdexDropMessage(apdex float64, threshold float64, projectName string) Message {
	severity := SeverityWarning
	if apdex < 0.5 {
		severity = SeverityCritical
	}
	return Message{
		Subject:  fmt.Sprintf("[%s] Apdex dropped to %.2f (threshold: %.2f)", projectName, apdex, threshold),
		Body:     fmt.Sprintf("The Apdex score has dropped to %.2f (threshold: %.2f).", apdex, threshold),
		Severity: severity,
		URL:      endpointTimeRangeURL(time.Now()),
	}
}

func buildMetricThresholdMessage(metricName string, value float64, operator string, threshold float64, projectName string) Message {
	severity := SeverityWarning
	diff := value - threshold
	if diff < 0 {
		diff = -diff
	}
	if diff > threshold*0.2 {
		severity = SeverityCritical
	}
	return Message{
		Subject:  fmt.Sprintf("[%s] Metric %s is %.2f (threshold: %s %.2f)", projectName, metricName, value, operator, threshold),
		Body:     fmt.Sprintf("The metric %s has a value of %.2f which violates the threshold %s %.2f.", metricName, value, operator, threshold),
		Severity: severity,
		URL:      "/metrics?preset=1h",
	}
}

func buildNoDataMessage(dataType string, silenceMinutes int, projectName string) Message {
	return Message{
		Subject:  fmt.Sprintf("[%s] No %s data for %d minutes", projectName, dataType, silenceMinutes),
		Body:     fmt.Sprintf("No %s data has been received for the last %d minutes.", dataType, silenceMinutes),
		Severity: SeverityCritical,
		URL:      "/",
	}
}

func buildErrorCountMessage(count int64, threshold int64, window int, projectName string) Message {
	severity := SeverityWarning
	if count >= threshold*5 {
		severity = SeverityCritical
	}
	return Message{
		Subject:  fmt.Sprintf("[%s] %d errors in last %d minutes", projectName, count, window),
		Body:     fmt.Sprintf("There have been %d errors in the last %d minutes (threshold: %d).", count, window, threshold),
		Severity: severity,
		URL:      "/issues?preset=1h",
	}
}

func buildTaskDurationMessage(taskName string, p95Ms float64, thresholdMs float64, projectName string) Message {
	return Message{
		Subject:  fmt.Sprintf("[%s] Task %s P95 %.0fms exceeds %.0fms", projectName, taskName, p95Ms, thresholdMs),
		Body:     fmt.Sprintf("The task %s P95 duration is %.0fms (threshold: %.0fms).", taskName, p95Ms, thresholdMs),
		Severity: SeverityWarning,
		URL:      "/tasks?preset=1h",
	}
}

func buildThroughputDropMessage(dropPercent float64, projectName string) Message {
	severity := SeverityWarning
	if dropPercent >= 80 {
		severity = SeverityCritical
	}
	return Message{
		Subject:  fmt.Sprintf("[%s] Throughput dropped %.0f%% vs baseline", projectName, dropPercent),
		Body:     fmt.Sprintf("Request throughput has dropped by %.0f%% compared to the baseline window.", dropPercent),
		Severity: severity,
		URL:      endpointTimeRangeURL(time.Now()),
	}
}

func buildEndpointErrorRateMessage(endpoint string, rate float64, threshold float64, projectName string) Message {
	severity := SeverityWarning
	if rate >= threshold*2 {
		severity = SeverityCritical
	}
	return Message{
		Subject:  fmt.Sprintf("[%s] %s error rate %.1f%%", projectName, endpoint, rate),
		Body:     fmt.Sprintf("The endpoint %s has an error rate of %.1f%% (threshold: %.1f%%).", endpoint, rate, threshold),
		Severity: severity,
		URL:      endpointTimeRangeURL(time.Now()),
	}
}

func buildImpactScoreCriticalMessage(endpoint string, score float64, reason string, projectName string) Message {
	return Message{
		Subject:  fmt.Sprintf("[%s] Endpoint %s impact became critical", projectName, endpoint),
		Body:     fmt.Sprintf("The endpoint %s has become critical (impact score: %.2f). Reason: %s", endpoint, score, reason),
		Severity: SeverityCritical,
		URL:      endpointTimeRangeURL(time.Now()),
		Endpoint: endpoint,
	}
}

func buildImpactScoreHighMessage(endpoint string, score float64, reason string, projectName string) Message {
	return Message{
		Subject:  fmt.Sprintf("[%s] Endpoint %s impact became high", projectName, endpoint),
		Body:     fmt.Sprintf("The endpoint %s has become high impact (impact score: %.2f). Reason: %s", endpoint, score, reason),
		Severity: SeverityWarning,
		URL:      endpointTimeRangeURL(time.Now()),
		Endpoint: endpoint,
	}
}

func buildImpactScoreMediumMessage(endpoint string, score float64, reason string, projectName string) Message {
	return Message{
		Subject:  fmt.Sprintf("[%s] Endpoint %s impact became medium", projectName, endpoint),
		Body:     fmt.Sprintf("The endpoint %s has become medium impact (impact score: %.2f). Reason: %s", endpoint, score, reason),
		Severity: SeverityInfo,
		URL:      endpointTimeRangeURL(time.Now()),
		Endpoint: endpoint,
	}
}

func formatCostForMessage(cost float64) string {
	if cost < 0.01 {
		return fmt.Sprintf("$%.4f", cost)
	}
	return fmt.Sprintf("$%.2f", cost)
}

func buildAiTraceCostMessage(traceName string, cost float64, threshold float64, window int, projectName string) Message {
	severity := SeverityWarning
	if cost >= threshold*3 {
		severity = SeverityCritical
	}
	return Message{
		Subject:  fmt.Sprintf("[%s] AI trace %s cost %s exceeds %s", projectName, traceName, formatCostForMessage(cost), formatCostForMessage(threshold)),
		Body:     fmt.Sprintf("The AI trace \"%s\" cost %s in the last %d minutes (threshold: %s).", traceName, formatCostForMessage(cost), window, formatCostForMessage(threshold)),
		Severity: severity,
		URL:      "/ai-traces?preset=1h",
	}
}

type ExceptionDetails struct {
	Id         string
	Hash       string
	ErrorType  string
	StackTrace string
	Attributes map[string]string
	AppVersion string
	ServerName string
	RecordedAt time.Time
	TraceType  string
	TraceName  string
}

func (d ExceptionDetails) endpointForMessage() string {
	if d.TraceType == "endpoint" {
		return d.TraceName
	}
	return ""
}

func buildNewErrorMessage(details ExceptionDetails, projectName string) Message {
	return Message{
		Subject:  fmt.Sprintf("[%s] New error: %s", projectName, details.ErrorType),
		Body:     buildExceptionBody("A new error has been detected: "+details.ErrorType, details),
		Severity: SeverityCritical,
		URL:      fmt.Sprintf("/issues/%s", details.Hash),
		Endpoint: details.endpointForMessage(),
	}
}

func buildErrorRegressionMessage(details ExceptionDetails, projectName string) Message {
	return Message{
		Subject:  fmt.Sprintf("[%s] Resolved error reappeared: %s", projectName, details.ErrorType),
		Body:     buildExceptionBody("A previously resolved error has reappeared: "+details.ErrorType, details),
		Severity: SeverityCritical,
		URL:      fmt.Sprintf("/issues/%s", details.Hash),
		Endpoint: details.endpointForMessage(),
	}
}

func buildExceptionBody(headline string, d ExceptionDetails) string {
	var b strings.Builder
	b.WriteString(headline)
	b.WriteString("\n")

	if d.Id != "" {
		fmt.Fprintf(&b, "\nException ID: %s", d.Id)
	}
	if d.Hash != "" {
		fmt.Fprintf(&b, "\nHash: %s", d.Hash)
	}
	if !d.RecordedAt.IsZero() {
		fmt.Fprintf(&b, "\nOccurred at: %s", d.RecordedAt.UTC().Format("2006-01-02 15:04:05 UTC"))
	}
	if d.AppVersion != "" {
		fmt.Fprintf(&b, "\nApp version: %s", d.AppVersion)
	}
	if d.ServerName != "" {
		fmt.Fprintf(&b, "\nServer: %s", d.ServerName)
	}
	if d.TraceName != "" {
		switch d.TraceType {
		case "task":
			fmt.Fprintf(&b, "\nTask: %s", d.TraceName)
		case "ai_trace":
			fmt.Fprintf(&b, "\nAI trace: %s", d.TraceName)
		default:
			fmt.Fprintf(&b, "\nEndpoint: %s", d.TraceName)
		}
	}

	if d.StackTrace != "" {
		fmt.Fprintf(&b, "\n\nStack trace:\n%s", d.StackTrace)
	}

	if len(d.Attributes) > 0 {
		b.WriteString("\n\nAttributes:")
		keys := make([]string, 0, len(d.Attributes))
		for k := range d.Attributes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "\n  %s: %s", k, d.Attributes[k])
		}
	}

	if d.Hash != "" {
		fmt.Fprintf(&b, "\n\nView details: /issues/%s", d.Hash)
	}

	return b.String()
}
