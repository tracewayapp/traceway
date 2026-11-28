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

// TW-1: the dashboard reads every item of a segment's events, logs and actions as an object. Ingest keeps the
// objects of an array and drops a value that is no array at all; it does not judge the rrweb shape of an event.
func TestRecordingsIngest_DropsItemsThatAreNotObjects(t *testing.T) {
	project := &models.Project{Id: uuid.New(), Framework: "react"}
	s := uuid.NewString()
	body := `{"segments":[
		{"sessionId":"` + s + `","segmentIndex":0,"events":[{"type":4,"timestamp":1,"data":{}},null,7,"x",[1]],"logs":{"level":"error"},"actions":[null,{"type":"custom","timestamp":"2026-11-27T10:00:00Z"}]},
		{"sessionId":"` + s + `","segmentIndex":1,"events":[null,1],"logs":"abc","actions":5}
	]}`
	recorder, jobs := postRecordings(t, project, body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", recorder.Code, recorder.Body.String())
	}
	if len(jobs) != 1 {
		t.Fatalf("got %d jobs, want 1 (a segment with nothing but malformed parts is dropped)", len(jobs))
	}
	var stored map[string]json.RawMessage
	if err := json.Unmarshal(jobs[0].Body, &stored); err != nil {
		t.Fatal(err)
	}
	if got := string(stored["events"]); got != `[{"type":4,"timestamp":1,"data":{}}]` {
		t.Errorf("events = %s", got)
	}
	if got, ok := stored["logs"]; ok && string(got) != "null" && string(got) != "[]" {
		t.Errorf("logs that are not an array must not be stored, got %s", got)
	}
	if got := string(stored["actions"]); got != `[{"type":"custom","timestamp":"2026-11-27T10:00:00Z"}]` {
		t.Errorf("actions = %s", got)
	}
}

// Crash safety: whatever a client sends, ingest answers 200 or 400 and never panics, and what it stores holds
// only objects in its lists.
func TestRecordingsIngest_NeverFailsOnHostileBodies(t *testing.T) {
	project := &models.Project{Id: uuid.New(), Framework: "react"}
	s := uuid.NewString()
	for name, body := range map[string]string{
		"null segment":       `{"segments":[null]}`,
		"segments not array": `{"segments":"x"}`,
		"segment not object": `{"segments":[5]}`,
		"events object":      `{"segments":[{"sessionId":"` + s + `","events":{"a":1}}]}`,
		"events string":      `{"segments":[{"sessionId":"` + s + `","events":"flutter_video"}]}`,
		"nested arrays":      `{"segments":[{"sessionId":"` + s + `","events":[[[[{}]]]],"logs":[[null]],"actions":[[]]}]}`,
		"negative index":     `{"segments":[{"sessionId":"` + s + `","segmentIndex":-1,"events":[{"type":2}]}]}`,
		"index overflow":     `{"segments":[{"sessionId":"` + s + `","segmentIndex":99999999999,"events":[{"type":2}]}]}`,
		"bad startedAt":      `{"segments":[{"sessionId":"` + s + `","startedAt":"yesterday","events":[{"type":2}]}]}`,
		"log entry types":    `{"segments":[{"sessionId":"` + s + `","logs":[{"level":5,"message":{}},{"level":"error","message":null},{"level":"error"}]}]}`,
		"not json":           `{"segments":[{"sessionId":`,
		"debugIds wrong":     `{"debugIds":[1],"segments":[{"sessionId":"` + s + `","events":[{"type":2}]}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			recorder, jobs := postRecordings(t, project, body)
			if recorder.Code != http.StatusOK && recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d: %s", recorder.Code, recorder.Body.String())
			}
			for _, job := range jobs {
				var stored clientmodels.ClientSessionRecording
				if err := json.Unmarshal(job.Body, &stored); err != nil {
					t.Fatalf("stored body does not parse: %s", job.Body)
				}
				for _, list := range []json.RawMessage{stored.Events, stored.Logs, stored.Actions} {
					var items []json.RawMessage
					if len(list) > 0 && json.Unmarshal(list, &items) != nil {
						t.Errorf("stored a list that is no array: %s", list)
					}
					for _, item := range items {
						if !isJSONObject(item) {
							t.Errorf("stored an item that is no object: %s", job.Body)
						}
					}
				}
			}
		})
	}
}
