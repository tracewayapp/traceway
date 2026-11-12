package clientcontrollers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/middleware"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/models/clientmodels"
	"github.com/tracewayapp/traceway/backend/v2/app/recordings"
)

func postRecordings(t *testing.T, project *models.Project, body string) (*httptest.ResponseRecorder, []recordings.Job) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	var jobs []recordings.Job
	prev := enqueueRecording
	enqueueRecording = func(j recordings.Job) { jobs = append(jobs, j) }
	t.Cleanup(func() { enqueueRecording = prev })

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/recordings", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(middleware.ProjectIdContextKey, project.Id)
	c.Set(middleware.ProjectContextKey, project)
	RecordingsController.Ingest(c)
	return recorder, jobs
}

func TestRecordingsIngest_LinksClipToClientExceptionId(t *testing.T) {
	project := &models.Project{Id: uuid.New(), Framework: "react"}
	exceptionId := uuid.New()
	sessionId := uuid.New()
	body := `{"segments":[
		{"exceptionId":"` + exceptionId.String() + `","sessionId":"` + sessionId.String() + `","segmentIndex":3,"events":[{"type":2}]},
		{"sessionId":"` + sessionId.String() + `","segmentIndex":4,"events":[{"type":3}]},
		{"exceptionId":"` + exceptionId.String() + `","events":[{"type":2}]},
		{"exceptionId":"not-a-uuid","events":[{"type":2}]},
		{"sessionId":"` + sessionId.String() + `","segmentIndex":5,"events":[]}
	]}`

	recorder, jobs := postRecordings(t, project, body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", recorder.Code, recorder.Body.String())
	}
	if len(jobs) != 3 {
		t.Fatalf("got %d jobs, want 3 (unlinked and empty segments are dropped): %+v", len(jobs), jobs)
	}
	clip := jobs[0]
	if clip.ExceptionId != exceptionId || clip.SessionId == nil || *clip.SessionId != sessionId {
		t.Errorf("clip not linked to both its exception and session: %+v", clip)
	}
	if !strings.HasSuffix(clip.Key, "/sessions/"+sessionId.String()+"/3.json") {
		t.Errorf("a session segment keeps the session key, got %q", clip.Key)
	}
	if jobs[1].ExceptionId != uuid.Nil {
		t.Errorf("a plain session segment has no exception")
	}
	if !strings.HasSuffix(jobs[2].Key, "/"+exceptionId.String()+".json") {
		t.Errorf("a clip without a session is stored under the exception, got %q", jobs[2].Key)
	}
	var stored map[string]json.RawMessage
	if err := json.Unmarshal(clip.Body, &stored); err != nil || string(stored["events"]) != `[{"type":2}]` {
		t.Errorf("segment body not stored as sent: %s", clip.Body)
	}
}

func TestRecordingsIngest_RejectsEmptyAndOversizedBatches(t *testing.T) {
	project := &models.Project{Id: uuid.New()}
	for name, body := range map[string]string{
		"empty":     `{"segments":[]}`,
		"missing":   `{}`,
		"oversized": `{"segments":[` + strings.TrimSuffix(strings.Repeat(`{"sessionId":"x"},`, maxRecordingSegments+1), ",") + `]}`,
	} {
		t.Run(name, func(t *testing.T) {
			recorder, jobs := postRecordings(t, project, body)
			if recorder.Code != http.StatusBadRequest || len(jobs) != 0 {
				t.Fatalf("status = %d, jobs = %d, want 400 and none", recorder.Code, len(jobs))
			}
		})
	}
}

func TestLegacyRecordingKeepsBreadcrumbsWithoutReplayEvents(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	logs := json.RawMessage(`[{"type":"log","timestamp":"2026-11-01T10:00:00Z","level":"info","message":"legacy console"}]`)
	actions := json.RawMessage(`[{"type":"custom","timestamp":"2026-11-01T10:00:01Z","category":"ui","name":"click","data":{"target":"buy"}}]`)
	segment := &clientmodels.ClientSessionRecording{SessionId: uuid.NewString(), SegmentIndex: 2, Logs: logs, Actions: actions}
	job, ok := buildRecordingJob(c, uuid.New(), segment, uuid.Nil, false, nil)
	if !ok {
		t.Fatal("legacy logs/actions-only segment was dropped")
	}
	var stored clientmodels.ClientSessionRecording
	if err := json.Unmarshal(job.Body, &stored); err != nil {
		t.Fatal(err)
	}
	if string(stored.Logs) != string(logs) || string(stored.Actions) != string(actions) {
		t.Fatalf("legacy breadcrumbs changed: %s", job.Body)
	}
}
