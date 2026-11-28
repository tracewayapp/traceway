package otelcontrollers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
)

func boolKV(key string, val bool) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: key, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_BoolValue{BoolValue: val}}}
}

// appIssueFixture splits a sample of clientcontrollers/testdata/app_issues into what an SDK sends: the message on the
// first line, the frames below it.
func appIssueFixture(t *testing.T, name string) (message, stacktrace string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "clientcontrollers", "testdata", "app_issues", name))
	if err != nil {
		t.Fatal(err)
	}
	message, stacktrace, _ = strings.Cut(strings.TrimRight(string(b), "\n"), "\n")
	return message, stacktrace
}

// appSDKLogs is one export of an app SDK: its resource, and exception events reported at the next launch with the
// time and session of the run that died.
func appSDKLogs(language, scope string, records ...[]*commonpb.KeyValue) *collogspb.ExportLogsServiceRequest {
	at := uint64(time.Date(2026, 11, 26, 21, 4, 0, 0, time.UTC).UnixNano())
	var logs []*logspb.LogRecord
	for _, attrs := range records {
		logs = append(logs, &logspb.LogRecord{TimeUnixNano: at, SeverityNumber: logspb.SeverityNumber_SEVERITY_NUMBER_FATAL,
			EventName: "exception", Attributes: attrs})
	}
	return &collogspb.ExportLogsServiceRequest{ResourceLogs: []*logspb.ResourceLogs{{
		Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{
			strKV("service.name", "shop"), strKV("service.version", "4.2.0"), strKV("telemetry.sdk.language", language),
			strKV("os.name", "Android"), strKV("device.model.identifier", "Pixel 7"),
		}},
		ScopeLogs: []*logspb.ScopeLogs{{Scope: &commonpb.InstrumentationScope{Name: scope, Version: "2.3.0"}, LogRecords: logs}},
	}}}
}

func exceptionRecord(typ, message, stacktrace string, extra ...*commonpb.KeyValue) []*commonpb.KeyValue {
	attrs := []*commonpb.KeyValue{strKV("exception.type", typ), strKV("exception.stacktrace", stacktrace),
		strKV("session.id", "0195f0c2-6a1e-7c3b-9f1d-2a4b6c8d0e1f")}
	if message != "" {
		attrs = append(attrs, strKV("exception.message", message))
	}
	return append(attrs, extra...)
}

// The Android SDK reports a tombstone as NativeCrash, an ANR as ApplicationNotResponding and a
// hang the process did not survive as AppHang with app.hang.fatal, each with the screen it happened on.
func TestConvertLogs_AndroidExitReasonsBecomeIssues(t *testing.T) {
	projectId := uuid.MustParse("00000000-0000-0000-0000-0000000000e1")
	setFakeStore(t, nil)
	project := tokenProject(projectId)
	project.Framework = "android"
	screen := []*commonpb.KeyValue{strKV("app.screen.name", "/CheckoutActivity"), strKV("app.screen.route", "/checkout"),
		strKV("app.screen.previous", "/CartActivity"), intKV("app.screen.duration_ms", 8400), strKV("viewport", "411x914")}

	crashMessage, crashFrames := appIssueFixture(t, "native_crash_pixel7.txt")
	otherMessage, otherFrames := appIssueFixture(t, "native_crash_galaxy.txt")
	anrMessage, anrFrames := appIssueFixture(t, "anr_blocked_main.txt")
	_, hangFrames := appIssueFixture(t, "anr_blocked_main_again.txt")
	req := appSDKLogs("android", "traceway-android",
		exceptionRecord("NativeCrash", crashMessage, crashFrames, screen...),
		exceptionRecord("NativeCrash", otherMessage, otherFrames),
		exceptionRecord("ApplicationNotResponding", anrMessage, anrFrames, screen...),
		exceptionRecord("AppHang", "Main thread blocked for 7.8 s", hangFrames, append([]*commonpb.KeyValue{boolKV("app.hang.fatal", true)}, screen...)...),
	)
	converted := convertLogs(context.Background(), project, projectId, "", req)
	if len(converted.Exceptions) != 4 || len(converted.Records) != 4 {
		t.Fatalf("got %d issues from %d records, want 4 and 4", len(converted.Exceptions), len(converted.Records))
	}
	crash, otherDevice, anr, hang := converted.Exceptions[0], converted.Exceptions[1], converted.Exceptions[2], converted.Exceptions[3]

	titles := map[string]string{
		"NativeCrash: SIGSEGV (SEGV_MAPERR), fault addr 0x0000000000000000": crash.StackTrace,
		"ApplicationNotResponding: " + anrMessage:                           anr.StackTrace,
		"AppHang: Main thread blocked for 7.8 s":                            hang.StackTrace,
	}
	for title, trace := range titles {
		if first, _, _ := strings.Cut(trace, "\n"); first != title {
			t.Errorf("issue title %q, want %q", first, title)
		}
	}
	if !strings.Contains(crash.StackTrace, "(pay::Card::charge(int)+52)") {
		t.Errorf("native frames were changed on the way in:\n%s", crash.StackTrace)
	}
	if crash.ExceptionHash != otherDevice.ExceptionHash {
		t.Error("one native crash on two devices became two issues")
	}
	if anr.ExceptionHash == hang.ExceptionHash {
		t.Error("an ANR and a hang at the same frames are one issue")
	}
	for _, issue := range []struct {
		name  string
		attrs map[string]string
	}{{"crash", crash.Attributes}, {"anr", anr.Attributes}, {"hang", hang.Attributes}} {
		if issue.attrs["app.screen.name"] != "/CheckoutActivity" || issue.attrs["app.screen.route"] != "/checkout" ||
			issue.attrs["app.screen.previous"] != "/CartActivity" || issue.attrs["app.screen.duration_ms"] != "8400" ||
			issue.attrs["viewport"] != "411x914" {
			t.Errorf("%s lost its screen context: %v", issue.name, issue.attrs)
		}
	}
	if hang.Attributes["app.hang.fatal"] != "true" || crash.Attributes["app.hang.fatal"] != "" {
		t.Errorf("app.hang.fatal: hang %q, crash %q", hang.Attributes["app.hang.fatal"], crash.Attributes["app.hang.fatal"])
	}
	for _, issue := range converted.Exceptions {
		if issue.IsMessage || issue.SessionId == nil || issue.SessionId.String() != "0195f0c2-6a1e-7c3b-9f1d-2a4b6c8d0e1f" {
			t.Errorf("issue %q: message %v, session %v", issue.ExceptionHash, issue.IsMessage, issue.SessionId)
		}
		if got := issue.RecordedAt; !got.Equal(time.Date(2026, 11, 26, 21, 4, 0, 0, time.UTC)) {
			t.Errorf("an exit reported at the next launch is placed at %v, not when the process died", got)
		}
	}
}

// The iOS SDK reports a MetricKit watchdog termination and a hang in its wire format; with no dSYM uploaded the
// server renders the frames as image+offset, which group the same as the raw frames.
func TestConvertLogs_IosWatchdogAndHangIssues(t *testing.T) {
	projectId := uuid.MustParse("00000000-0000-0000-0000-0000000000e2")
	setFakeStore(t, nil)
	project := tokenProject(projectId)
	project.Framework = "ios"
	record := func(typ, fixture string) []*commonpb.KeyValue {
		message, frames := appIssueFixture(t, fixture)
		header := typ + ": " + message
		return exceptionRecord(typ, message, header+"\n"+frames)
	}
	req := appSDKLogs("swift", "traceway-ios",
		record("AppHang", "ios_hang_sample_1.txt"),
		record("AppHang", "ios_hang_sample_2.txt"),
		record("WatchdogTermination", "ios_watchdog_metrickit.txt"),
	)
	converted := convertLogs(context.Background(), project, projectId, "", req)
	if len(converted.Exceptions) != 3 {
		t.Fatalf("got %d issues, want 3", len(converted.Exceptions))
	}
	first, second, watchdog := converted.Exceptions[0], converted.Exceptions[1], converted.Exceptions[2]
	if first.ExceptionHash != second.ExceptionHash {
		t.Errorf("two samples of one hang are two issues:\n%s\n\n%s", first.StackTrace, second.StackTrace)
	}
	if !strings.HasPrefix(first.StackTrace, "AppHang: Main thread blocked for 3.2 s\n") {
		t.Errorf("hang title: %q", first.StackTrace)
	}
	if !strings.HasPrefix(watchdog.StackTrace, "WatchdogTermination: Watchdog terminated the app (0x8badf00d)") ||
		!strings.Contains(watchdog.StackTrace, "ShopApp+0x55e10") {
		t.Errorf("watchdog trace:\n%s", watchdog.StackTrace)
	}
}

// A browser issue keeps the screen context the browser SDK stamps, as an app issue does.
func TestConvertLogs_BrowserExceptionKeepsScreenContext(t *testing.T) {
	projectId := uuid.MustParse("00000000-0000-0000-0000-0000000000e3")
	project := tokenProject(projectId)
	project.Framework = "react"
	req := logsRequest("webjs", "@tracewayapp/frontend",
		strKV("event.name", "exception"), strKV("exception.type", "TypeError"),
		strKV("exception.message", "Cannot read properties of undefined (reading 'total')"),
		strKV("app.screen.name", "/orders/:id"), strKV("app.screen.route", "/orders/:id"),
		strKV("app.screen.previous", "/orders"), intKV("app.screen.duration_ms", 1520), strKV("viewport", "1440x900"),
	)
	converted := convertLogs(context.Background(), project, projectId, "", req)
	if len(converted.Exceptions) != 1 {
		t.Fatalf("got %d issues", len(converted.Exceptions))
	}
	attrs := converted.Exceptions[0].Attributes
	for key, want := range map[string]string{"app.screen.name": "/orders/:id", "app.screen.route": "/orders/:id",
		"app.screen.previous": "/orders", "app.screen.duration_ms": "1520", "viewport": "1440x900"} {
		if attrs[key] != want {
			t.Errorf("%s = %q, want %q", key, attrs[key], want)
		}
	}
}
