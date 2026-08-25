package notifications

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/models"
)

func alertEmail(alert models.EmailAlert) *models.NotificationEmail {
	return &models.NotificationEmail{Template: models.EmailTemplateAlert, Alert: &alert}
}

func buildErrorRateMessage(rate float64, threshold float64, window int, projectName string) Message {
	severity := SeverityWarning
	if rate >= threshold*2 {
		severity = SeverityCritical
	}
	headline := fmt.Sprintf("The error rate has reached %.1f%% over the last %d minutes (threshold: %.1f%%).", rate, window, threshold)
	return Message{
		Subject:  fmt.Sprintf("[%s] Error rate %.1f%% exceeds %.1f%%", projectName, rate, threshold),
		Body:     headline,
		Severity: severity,
		URL:      "/issues?preset=1h",
		Email: alertEmail(models.EmailAlert{
			ProjectName: projectName,
			Headline:    headline,
			Observed:    fmt.Sprintf("%.1f%% of requests", rate),
			Threshold:   fmt.Sprintf("%.1f%%", threshold),
			WindowMins:  window,
		}),
	}
}

func endpointTimeRangeURL(now time.Time) string {
	from := now.Add(-1 * time.Minute).UTC().Format(time.RFC3339)
	to := now.Add(2 * time.Minute).UTC().Format(time.RFC3339)
	return fmt.Sprintf("/endpoints?from=%s&to=%s", from, to)
}

func buildEndpointLatencyMessage(percentile string, latencyMs float64, thresholdMs float64, endpoint string, window int, projectName string) Message {
	headline := fmt.Sprintf("The %s latency for %s has reached %.0fms over the last %d minutes (threshold: %.0fms).", percentile, endpoint, latencyMs, window, thresholdMs)
	return Message{
		Subject:    fmt.Sprintf("[%s] %s latency %.0fms on %s", projectName, percentile, latencyMs, endpoint),
		Body:       headline,
		Severity:   SeverityWarning,
		URL:        endpointTimeRangeURL(time.Now()),
		DedupToken: endpoint,
		Email: alertEmail(models.EmailAlert{
			ProjectName: projectName,
			Headline:    headline,
			ScopeLabel:  "Endpoint",
			Scope:       endpoint,
			Observed:    fmt.Sprintf("%s %.0fms", percentile, latencyMs),
			Threshold:   fmt.Sprintf("%.0fms", thresholdMs),
			WindowMins:  window,
		}),
	}
}

func buildApdexDropMessage(apdex float64, threshold float64, total int64, window int, projectName string) Message {
	severity := SeverityWarning
	if apdex < 0.5 {
		severity = SeverityCritical
	}
	headline := fmt.Sprintf("The Apdex score has dropped to %.2f across %d requests over the last %d minutes (threshold: %.2f).", apdex, total, window, threshold)
	return Message{
		Subject:  fmt.Sprintf("[%s] Apdex dropped to %.2f (threshold: %.2f)", projectName, apdex, threshold),
		Body:     headline,
		Severity: severity,
		URL:      endpointTimeRangeURL(time.Now()),
		Email: alertEmail(models.EmailAlert{
			ProjectName: projectName,
			Headline:    headline,
			Observed:    fmt.Sprintf("%.2f across %d requests", apdex, total),
			Threshold:   fmt.Sprintf("%.2f", threshold),
			WindowMins:  window,
		}),
	}
}

func buildMetricThresholdMessage(metricName string, value float64, operator string, threshold float64, aggregation string, window int, projectName string) Message {
	severity := SeverityWarning
	diff := value - threshold
	if diff < 0 {
		diff = -diff
	}
	if diff > threshold*0.2 {
		severity = SeverityCritical
	}
	if aggregation == "" {
		aggregation = "avg"
	}
	headline := fmt.Sprintf("The metric %s has a %s of %.2f over the last %d minutes which violates the threshold %s %.2f.", metricName, aggregation, value, window, operator, threshold)
	return Message{
		Subject:    fmt.Sprintf("[%s] Metric %s is %.2f (threshold: %s %.2f)", projectName, metricName, value, operator, threshold),
		Body:       headline,
		Severity:   severity,
		URL:        "/metrics?preset=1h",
		DedupToken: metricName,
		Email: alertEmail(models.EmailAlert{
			ProjectName: projectName,
			Headline:    headline,
			ScopeLabel:  "Metric",
			Scope:       metricName,
			Observed:    fmt.Sprintf("%s %.2f", aggregation, value),
			Threshold:   fmt.Sprintf("%s %.2f", operator, threshold),
			WindowMins:  window,
		}),
	}
}

func buildNoDataMessage(dataType string, silenceMinutes int, projectName string) Message {
	headline := fmt.Sprintf("No %s data has been received for the last %d minutes.", dataType, silenceMinutes)
	return Message{
		Subject:    fmt.Sprintf("[%s] No %s data for %d minutes", projectName, dataType, silenceMinutes),
		Body:       headline,
		Severity:   SeverityCritical,
		URL:        "/",
		DedupToken: dataType,
		Email: alertEmail(models.EmailAlert{
			ProjectName: projectName,
			Headline:    headline,
			ScopeLabel:  "Signal",
			Scope:       dataType,
			Observed:    fmt.Sprintf("silent for %d minutes", silenceMinutes),
		}),
	}
}

func buildErrorCountMessage(count int64, threshold int64, window int, projectName string) Message {
	severity := SeverityWarning
	if count >= threshold*5 {
		severity = SeverityCritical
	}
	headline := fmt.Sprintf("There have been %d errors in the last %d minutes (threshold: %d).", count, window, threshold)
	return Message{
		Subject:  fmt.Sprintf("[%s] %d errors in last %d minutes", projectName, count, window),
		Body:     headline,
		Severity: severity,
		URL:      "/issues?preset=1h",
		Email: alertEmail(models.EmailAlert{
			ProjectName: projectName,
			Headline:    headline,
			Observed:    fmt.Sprintf("%d errors", count),
			Threshold:   fmt.Sprintf("%d errors", threshold),
			WindowMins:  window,
		}),
	}
}

func buildTaskDurationMessage(taskName string, p95Ms float64, thresholdMs float64, window int, projectName string) Message {
	headline := fmt.Sprintf("The task %s P95 duration is %.0fms over the last %d minutes (threshold: %.0fms).", taskName, p95Ms, window, thresholdMs)
	return Message{
		Subject:    fmt.Sprintf("[%s] Task %s P95 %.0fms exceeds %.0fms", projectName, taskName, p95Ms, thresholdMs),
		Body:       headline,
		Severity:   SeverityWarning,
		URL:        "/tasks?preset=1h",
		DedupToken: taskName,
		Email: alertEmail(models.EmailAlert{
			ProjectName: projectName,
			Headline:    headline,
			ScopeLabel:  "Task",
			Scope:       taskName,
			Observed:    fmt.Sprintf("P95 %.0fms", p95Ms),
			Threshold:   fmt.Sprintf("%.0fms", thresholdMs),
			WindowMins:  window,
		}),
	}
}

func buildTaskFailureRateMessage(taskName string, rate float64, threshold float64, failed int64, total int64, window int, projectName string) Message {
	severity := SeverityWarning
	if rate >= threshold*2 {
		severity = SeverityCritical
	}
	headline := fmt.Sprintf("The task %s failed %d of %d executions (%.1f%%) over the last %d minutes (threshold: %.1f%%).", taskName, failed, total, rate, window, threshold)
	return Message{
		Subject:    fmt.Sprintf("[%s] Task %s failure rate %.1f%% exceeds %.1f%%", projectName, taskName, rate, threshold),
		Body:       headline,
		Severity:   severity,
		URL:        "/tasks?preset=1h",
		DedupToken: taskName,
		Email: alertEmail(models.EmailAlert{
			ProjectName: projectName,
			Headline:    headline,
			ScopeLabel:  "Task",
			Scope:       taskName,
			Observed:    fmt.Sprintf("%d of %d failed (%.1f%%)", failed, total, rate),
			Threshold:   fmt.Sprintf("%.1f%%", threshold),
			WindowMins:  window,
		}),
	}
}

func buildThroughputDropMessage(dropPercent float64, current int64, expected float64, window int, projectName string) Message {
	severity := SeverityWarning
	if dropPercent >= 80 {
		severity = SeverityCritical
	}
	headline := fmt.Sprintf("Request throughput has dropped by %.0f%%: %d requests in the last %d minutes vs an expected %.0f based on the baseline window.", dropPercent, current, window, expected)
	return Message{
		Subject:  fmt.Sprintf("[%s] Throughput dropped %.0f%% vs baseline", projectName, dropPercent),
		Body:     headline,
		Severity: severity,
		URL:      endpointTimeRangeURL(time.Now()),
		Email: alertEmail(models.EmailAlert{
			ProjectName: projectName,
			Headline:    headline,
			Observed:    fmt.Sprintf("%d requests", current),
			Threshold:   fmt.Sprintf("%.0f expected", expected),
			WindowMins:  window,
		}),
	}
}

func buildEndpointErrorRateMessage(endpoint string, rate float64, threshold float64, projectName string) Message {
	severity := SeverityWarning
	if rate >= threshold*2 {
		severity = SeverityCritical
	}
	headline := fmt.Sprintf("The endpoint %s has an error rate of %.1f%% (threshold: %.1f%%).", endpoint, rate, threshold)
	return Message{
		Subject:    fmt.Sprintf("[%s] %s error rate %.1f%%", projectName, endpoint, rate),
		Body:       headline,
		Severity:   severity,
		URL:        endpointTimeRangeURL(time.Now()),
		DedupToken: endpoint,
		Email: alertEmail(models.EmailAlert{
			ProjectName: projectName,
			Headline:    headline,
			ScopeLabel:  "Endpoint",
			Scope:       endpoint,
			Observed:    fmt.Sprintf("%.1f%% of requests", rate),
			Threshold:   fmt.Sprintf("%.1f%%", threshold),
		}),
	}
}

func buildImpactScoreMessage(level string, severity Severity, endpoint string, score float64, reason string, projectName string) Message {
	headline := fmt.Sprintf("The endpoint %s has become %s impact (impact score: %.2f). Reason: %s", endpoint, level, score, reason)
	if level == "critical" {
		headline = fmt.Sprintf("The endpoint %s has become critical (impact score: %.2f). Reason: %s", endpoint, score, reason)
	}
	return Message{
		Subject:    fmt.Sprintf("[%s] Endpoint %s impact became %s", projectName, endpoint, level),
		Body:       headline,
		Severity:   severity,
		URL:        endpointTimeRangeURL(time.Now()),
		Endpoint:   endpoint,
		DedupToken: endpoint,
		Email: alertEmail(models.EmailAlert{
			ProjectName: projectName,
			Headline:    headline,
			ScopeLabel:  "Endpoint",
			Scope:       endpoint,
			Observed:    fmt.Sprintf("impact score %.2f", score),
			Threshold:   reason,
		}),
	}
}

func buildImpactScoreCriticalMessage(endpoint string, score float64, reason string, projectName string) Message {
	return buildImpactScoreMessage("critical", SeverityCritical, endpoint, score, reason, projectName)
}

func buildImpactScoreHighMessage(endpoint string, score float64, reason string, projectName string) Message {
	return buildImpactScoreMessage("high", SeverityWarning, endpoint, score, reason, projectName)
}

func buildImpactScoreMediumMessage(endpoint string, score float64, reason string, projectName string) Message {
	return buildImpactScoreMessage("medium", SeverityInfo, endpoint, score, reason, projectName)
}

func formatCostForMessage(cost float64) string {
	if cost < 0.01 {
		return fmt.Sprintf("$%.4f", cost)
	}
	return fmt.Sprintf("$%.2f", cost)
}

func buildAiTraceCostMessage(traceName string, cost float64, threshold float64, projectName string) Message {
	severity := SeverityWarning
	if cost >= threshold*3 {
		severity = SeverityCritical
	}
	headline := fmt.Sprintf("The AI trace %q cost %s, exceeding the threshold of %s.", traceName, formatCostForMessage(cost), formatCostForMessage(threshold))
	return Message{
		Subject:    fmt.Sprintf("[%s] AI trace %s cost %s exceeds %s", projectName, traceName, formatCostForMessage(cost), formatCostForMessage(threshold)),
		Body:       headline,
		Severity:   severity,
		URL:        "/ai-traces?preset=1h",
		DedupToken: traceName,
		Email: alertEmail(models.EmailAlert{
			ProjectName: projectName,
			Headline:    headline,
			ScopeLabel:  "AI trace",
			Scope:       traceName,
			Observed:    formatCostForMessage(cost),
			Threshold:   formatCostForMessage(threshold),
		}),
	}
}

func buildAiConversationCostMessage(conversationId string, cost, threshold float64, projectName string) Message {
	severity := SeverityWarning
	if cost >= threshold*3 {
		severity = SeverityCritical
	}
	headline := fmt.Sprintf("The AI conversation %q has cost %s over the last 24 hours, exceeding the threshold of %s.", conversationId, formatCostForMessage(cost), formatCostForMessage(threshold))
	return Message{
		Subject:    fmt.Sprintf("[%s] AI conversation cost %s exceeds %s", projectName, formatCostForMessage(cost), formatCostForMessage(threshold)),
		Body:       headline,
		Severity:   severity,
		URL:        "/ai-traces/conversations?preset=24h",
		DedupToken: conversationId,
		Email: alertEmail(models.EmailAlert{
			ProjectName: projectName,
			Headline:    headline,
			ScopeLabel:  "Conversation",
			Scope:       conversationId,
			Observed:    formatCostForMessage(cost) + " in 24 hours",
			Threshold:   formatCostForMessage(threshold),
		}),
	}
}

func buildAiFlaggedContentMessage(conversationId, userId string, terms []string, projectName string) Message {
	termList := strings.Join(terms, ", ")
	body := fmt.Sprintf("An AI conversation matched flagged terms: %s.", termList)
	if conversationId != "" {
		body = fmt.Sprintf("AI conversation %q matched flagged terms: %s.", conversationId, termList)
	}
	if userId != "" {
		body += fmt.Sprintf(" User: %s.", userId)
	}
	return Message{
		Subject:  fmt.Sprintf("[%s] AI conversation flagged: %s", projectName, termList),
		Body:     body,
		Severity: SeverityWarning,
		URL:      "/ai-traces/conversations?preset=24h&flagged=1",
		Email: &models.NotificationEmail{
			Template: models.EmailTemplateAiFlagged,
			Flagged: &models.EmailFlagged{
				ProjectName:    projectName,
				ConversationId: conversationId,
				UserId:         userId,
				Terms:          terms,
			},
		},
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
		Subject:    fmt.Sprintf("[%s] New error: %s", projectName, details.ErrorType),
		Body:       buildExceptionBody("A new error has been detected: "+details.ErrorType, details),
		Severity:   SeverityCritical,
		URL:        fmt.Sprintf("/issues/%s", details.Hash),
		Endpoint:   details.endpointForMessage(),
		DedupToken: details.Hash,
		Email:      exceptionEmail(models.EmailTemplateNewError, details, projectName),
	}
}

func buildErrorRegressionMessage(details ExceptionDetails, projectName string) Message {
	return Message{
		Subject:    fmt.Sprintf("[%s] Resolved error reappeared: %s", projectName, details.ErrorType),
		Body:       buildExceptionBody("A previously resolved error has reappeared: "+details.ErrorType, details),
		Severity:   SeverityCritical,
		URL:        fmt.Sprintf("/issues/%s", details.Hash),
		Endpoint:   details.endpointForMessage(),
		DedupToken: details.Hash,
		Email:      exceptionEmail(models.EmailTemplateErrorRegression, details, projectName),
	}
}

func exceptionEmail(template string, d ExceptionDetails, projectName string) *models.NotificationEmail {
	payload := &models.EmailException{
		ProjectName: projectName,
		ErrorType:   d.ErrorType,
		ExceptionId: d.Id,
		Hash:        d.Hash,
		AppVersion:  d.AppVersion,
		ServerName:  d.ServerName,
		TraceLabel:  "Endpoint",
		TraceName:   d.TraceName,
		StackTrace:  d.StackTrace,
	}
	if !d.RecordedAt.IsZero() {
		payload.OccurredAt = d.RecordedAt.UTC().Format("2006-01-02 15:04:05 UTC")
	}
	switch d.TraceType {
	case "task":
		payload.TraceLabel = "Task"
	case "ai_trace":
		payload.TraceLabel = "AI trace"
	}
	for _, key := range sortedKeys(d.Attributes) {
		payload.Attributes = append(payload.Attributes, models.EmailField{Label: key, Value: d.Attributes[key]})
	}
	return &models.NotificationEmail{Template: template, Exception: payload}
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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
		for _, k := range sortedKeys(d.Attributes) {
			fmt.Fprintf(&b, "\n  %s: %s", k, d.Attributes[k])
		}
	}

	if d.Hash != "" {
		fmt.Fprintf(&b, "\n\nView details: /issues/%s", d.Hash)
	}

	return b.String()
}

func buildCheckDownMessage(check *models.SyntheticCheck, errorMsg string, projectName string) Message {
	body := fmt.Sprintf("The synthetic check %q is failing (%d consecutive failures).", check.Name, check.ConsecutiveFailures)
	if errorMsg != "" {
		body += "\n\nLast error: " + errorMsg
	}
	return Message{
		Subject:    fmt.Sprintf("[%s] Check %q is down", projectName, check.Name),
		Body:       body,
		Severity:   SeverityCritical,
		URL:        fmt.Sprintf("/monitors/%d", check.Id),
		DedupToken: fmt.Sprintf("check:%d", check.Id),
		Email: &models.NotificationEmail{
			Template: models.EmailTemplateCheckDown,
			Check: &models.EmailCheck{
				ProjectName:         projectName,
				CheckName:           check.Name,
				ConsecutiveFailures: check.ConsecutiveFailures,
				LastError:           errorMsg,
			},
		},
	}
}

func buildCheckRecoveredMessage(check *models.SyntheticCheck, projectName string) Message {
	return Message{
		Subject:    fmt.Sprintf("[%s] Check %q recovered", projectName, check.Name),
		Body:       fmt.Sprintf("The synthetic check %q is passing again.", check.Name),
		Severity:   SeverityInfo,
		URL:        fmt.Sprintf("/monitors/%d", check.Id),
		DedupToken: fmt.Sprintf("check:%d", check.Id),
		Email: &models.NotificationEmail{
			Template: models.EmailTemplateCheckRecovered,
			Check: &models.EmailCheck{
				ProjectName: projectName,
				CheckName:   check.Name,
			},
		},
	}
}

func TestChannelMessage() Message {
	return Message{
		Subject:  "Traceway Test Notification",
		Body:     "This is a test notification from Traceway. If you received this, your notification channel is configured correctly.",
		Severity: SeverityInfo,
		RuleType: "test",
		RuleName: "Test",
		Email: &models.NotificationEmail{
			Template: models.EmailTemplateTest,
			Test:     &models.EmailTest{Target: "notification channel"},
		},
	}
}

func TestContactMethodMessage() Message {
	return Message{
		Subject:  "Traceway on-call test",
		Body:     "This is a test of your on-call contact method. If you received this, pages will reach you here.",
		Severity: SeverityInfo,
		RuleType: "test",
		RuleName: "Test",
		Email: &models.NotificationEmail{
			Template: models.EmailTemplateTest,
			Test:     &models.EmailTest{Target: "on-call contact method"},
		},
	}
}
