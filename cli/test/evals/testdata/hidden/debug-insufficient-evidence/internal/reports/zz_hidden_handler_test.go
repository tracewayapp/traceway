package reports

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/reports-api/internal/export"
)

type hiddenExporter struct{ err error }

func (h hiddenExporter) Export(context.Context, int64, string) (string, error) {
	if h.err != nil {
		return "", h.err
	}
	return "https://storage.example/x", nil
}

func TestHiddenHandlerStillAnswers500WithoutDetails(t *testing.T) {
	err := fmt.Errorf("%w: upload: storage credentials: 503 Service Unavailable", export.ErrExportFailed)
	req := httptest.NewRequest(http.MethodPost, "/api/reports/5/export", strings.NewReader(`{"format":"csv"}`))
	req.SetPathValue("id", "5")
	rec := httptest.NewRecorder()
	NewHandler(hiddenExporter{err: err}).Export(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("a failed export must answer 500, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "503") || strings.Contains(rec.Body.String(), "credentials") {
		t.Fatalf("internal causes must not reach the client: %s", rec.Body.String())
	}
}
