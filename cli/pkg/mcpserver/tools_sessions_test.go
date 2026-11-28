package mcpserver

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

const sessionID = "00000000-0000-0000-0000-000000000005"

func sessionsBackend(t *testing.T, listBody, detailBody *map[string]any) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/sessions":
			_ = json.NewDecoder(r.Body).Decode(listBody)
			_, _ = w.Write([]byte(`{"data":[{"id":"` + sessionID + `","startedAt":"2026-06-23T14:30:00Z","duration":1000,"attributes":{"userId":"u_42"},"hasRecording":true}],"pagination":{"page":1,"pageSize":20,"total":1,"totalPages":1}}`))
		case "/api/sessions/" + sessionID:
			_ = json.NewDecoder(r.Body).Decode(detailBody)
			_, _ = w.Write([]byte(`{"session":{"id":"` + sessionID + `","startedAt":"2026-06-23T14:30:00Z"},"exceptions":[]}`))
		default:
			http.NotFound(w, r)
		}
	})
}

func TestListSessions(t *testing.T) {
	var listBody, detailBody map[string]any
	cs := connect(t, sessionsBackend(t, &listBody, &detailBody), "p-1")

	res := callTool(t, cs, "list_sessions", map[string]any{"since": "24h", "search": "u_42"})
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", resultText(t, res))
	}
	if listBody["search"] != "u_42" || listBody["orderBy"] != "started_at" || listBody["sortDirection"] != "desc" || listBody["fromDate"] == nil {
		t.Errorf("body = %v", listBody)
	}
	if !strings.Contains(resultText(t, res), sessionID) || !strings.Contains(resultText(t, res), "startedAt") {
		t.Errorf("result = %s", resultText(t, res))
	}

	bad := callTool(t, cs, "list_sessions", map[string]any{"order_by": "user"})
	if !bad.IsError || !strings.Contains(resultText(t, bad), "invalid_argument") {
		t.Errorf("an unknown order_by should be refused: %s", resultText(t, bad))
	}
}

func TestGetSessionLooksUpTheStartWhenOmitted(t *testing.T) {
	var listBody, detailBody map[string]any
	cs := connect(t, sessionsBackend(t, &listBody, &detailBody), "p-1")

	res := callTool(t, cs, "get_session", map[string]any{"id": sessionID})
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", resultText(t, res))
	}
	if listBody["search"] != sessionID {
		t.Errorf("lookup body = %v", listBody)
	}
	if detailBody["startedAt"] != "2026-06-23T14:30:00Z" {
		t.Errorf("detail startedAt = %v", detailBody["startedAt"])
	}

	listBody = nil
	res = callTool(t, cs, "get_session", map[string]any{"id": sessionID, "started_at": "2026-06-23T14:00:00Z"})
	if res.IsError || listBody != nil || detailBody["startedAt"] != "2026-06-23T14:00:00Z" {
		t.Errorf("a given started_at skips the lookup: %s, list body %v, detail %v", resultText(t, res), listBody, detailBody)
	}

	missing := callTool(t, cs, "get_session", map[string]any{"id": "00000000-0000-0000-0000-000000000009"})
	if !missing.IsError || !strings.Contains(resultText(t, missing), "started_at") {
		t.Errorf("an unknown session should name started_at: %s", resultText(t, missing))
	}
}
