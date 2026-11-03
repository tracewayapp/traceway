//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package opampcontrollers

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/open-telemetry/opamp-go/client"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/tracewayapp/traceway/backend/app/cache"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/dbtest"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"google.golang.org/protobuf/proto"
)

const opampPath = "/api/otel/v1/opamp"

type opampFixture struct {
	t       *testing.T
	router  *gin.Engine
	project *models.Project
}

func newOpampFixture(t *testing.T) *opampFixture {
	t.Helper()
	dbtest.SetupSQLite(t)
	gin.SetMode(gin.TestMode)
	cache.SdkConfigCache.InvalidateAll()
	t.Cleanup(cache.SdkConfigCache.InvalidateAll)
	project, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.Project, error) {
		org, err := transactional.OrganizationRepository.Create(tx, "opamp-org", "UTC")
		if err != nil {
			return nil, err
		}
		return transactional.ProjectRepository.CreateWithOrganization(tx, "web", "react", org.Id)
	})
	if err != nil {
		t.Fatalf("seed project: %v", err)
	}
	f := &opampFixture{t: t, router: gin.New(), project: project}
	f.router.POST(opampPath, func(c *gin.Context) {
		c.Set(middleware.ProjectIdContextKey, project.Id)
		c.Set(middleware.ProjectContextKey, project)
	}, OpampController.Poll)
	return f
}

func (f *opampFixture) rule(name, conditions, settings string) int {
	f.t.Helper()
	now := time.Now().UTC()
	id, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		rules, err := transactional.SdkConfigRepository.ListRules(tx, f.project.Id)
		if err != nil {
			return 0, err
		}
		return transactional.SdkConfigRepository.CreateRule(tx, &models.SdkConfigRule{
			ProjectId: f.project.Id, Name: name, Position: len(rules), Enabled: true,
			Conditions: models.JSONText(conditions), Settings: models.JSONText(settings), CreatedAt: now, UpdatedAt: now,
		})
	})
	if err != nil {
		f.t.Fatalf("create rule: %v", err)
	}
	cache.SdkConfigCache.Invalidate(f.project.Id)
	return id
}

func (f *opampFixture) post(body []byte, headers map[string]string) *httptest.ResponseRecorder {
	f.t.Helper()
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, opampPath, bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	f.router.ServeHTTP(recorder, req)
	return recorder
}

func (f *opampFixture) poll(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
	f.t.Helper()
	body, err := proto.Marshal(msg)
	if err != nil {
		f.t.Fatal(err)
	}
	recorder := f.post(body, nil)
	if recorder.Code != http.StatusOK {
		f.t.Fatalf("poll = %d %s", recorder.Code, recorder.Body.String())
	}
	var resp protobufs.ServerToAgent
	if err := proto.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		f.t.Fatal(err)
	}
	return &resp
}

func stringKV(key, value string) *protobufs.KeyValue {
	return &protobufs.KeyValue{Key: key, Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: value}}}
}

func browserDescription(version string) *protobufs.AgentDescription {
	return &protobufs.AgentDescription{
		IdentifyingAttributes:    []*protobufs.KeyValue{stringKV("service.name", "shop-web")},
		NonIdentifyingAttributes: []*protobufs.KeyValue{stringKV("service.version", version), stringKV("browser.name", "Chrome")},
	}
}

const sdkCapabilities = uint64(protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus |
	protobufs.AgentCapabilities_AgentCapabilities_AcceptsRemoteConfig |
	protobufs.AgentCapabilities_AgentCapabilities_ReportsRemoteConfig |
	protobufs.AgentCapabilities_AgentCapabilities_ReportsHeartbeat)

func remoteDocument(t *testing.T, resp *protobufs.ServerToAgent) map[string]any {
	t.Helper()
	object := resp.GetRemoteConfig().GetConfig().GetConfigMap()[ConfigMapKey]
	if object == nil || object.ContentType != "application/json" {
		t.Fatalf("no traceway config in %v", resp)
	}
	var doc map[string]any
	if err := json.Unmarshal(object.Body, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestPollDeliversMatchingRulesOnceAndHeartbeat(t *testing.T) {
	f := newOpampFixture(t)
	f.rule("Stream replay for 2.x", `{"all":[{"key":"service.version","op":"version","value":">=2"}]}`, `{"replay":{"mode":"stream"}}`)
	uid := uuid.Must(uuid.NewV7())

	first := f.poll(&protobufs.AgentToServer{InstanceUid: uid[:], Capabilities: sdkCapabilities, AgentDescription: browserDescription("2.3.1")})
	doc := remoteDocument(t, first)
	if doc["replay"].(map[string]any)["mode"] != "stream" {
		t.Fatalf("rule not applied: %v", doc)
	}
	if first.GetConnectionSettings().GetOpamp().GetHeartbeatIntervalSeconds() != 300 {
		t.Errorf("heartbeat = %d, want the default 300", first.GetConnectionSettings().GetOpamp().GetHeartbeatIntervalSeconds())
	}
	if !bytes.Equal(first.InstanceUid, uid[:]) || first.Capabilities&uint64(protobufs.ServerCapabilities_ServerCapabilities_OffersRemoteConfig) == 0 {
		t.Errorf("unexpected envelope: %v", first)
	}

	applied := f.poll(&protobufs.AgentToServer{
		InstanceUid: uid[:], Capabilities: sdkCapabilities, AgentDescription: browserDescription("2.3.1"),
		RemoteConfigStatus: &protobufs.RemoteConfigStatus{LastRemoteConfigHash: first.RemoteConfig.ConfigHash, Status: protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED},
	})
	if applied.RemoteConfig != nil {
		t.Errorf("config resent although the client has its hash")
	}

	old := f.poll(&protobufs.AgentToServer{
		InstanceUid: uid[:], Capabilities: sdkCapabilities, AgentDescription: browserDescription("1.9.0"),
		RemoteConfigStatus: &protobufs.RemoteConfigStatus{LastRemoteConfigHash: first.RemoteConfig.ConfigHash},
	})
	if doc := remoteDocument(t, old); len(doc) != 1 || doc["schemaVersion"].(float64) != 1 {
		t.Errorf("a client the rule does not match gets the empty overlay, got %v", doc)
	}
}

func TestPollWithoutDescriptionAsksForFullState(t *testing.T) {
	f := newOpampFixture(t)
	uid := uuid.Must(uuid.NewV7())
	resp := f.poll(&protobufs.AgentToServer{InstanceUid: uid[:], Capabilities: sdkCapabilities})
	if resp.Flags&uint64(protobufs.ServerToAgentFlags_ServerToAgentFlags_ReportFullState) == 0 || resp.RemoteConfig != nil {
		t.Fatalf("expected a ReportFullState request, got %v", resp)
	}

	assigned := f.poll(&protobufs.AgentToServer{InstanceUid: uid[:], Flags: uint64(protobufs.AgentToServerFlags_AgentToServerFlags_RequestInstanceUid)})
	if len(assigned.GetAgentIdentification().GetNewInstanceUid()) != 16 {
		t.Fatalf("no new instance uid: %v", assigned)
	}
}

func TestPollGivesCapturedClientsAShortHeartbeat(t *testing.T) {
	f := newOpampFixture(t)
	uid := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		return struct{}{}, transactional.SdkConfigRepository.CreateCapture(tx, &models.SdkCapture{
			Id: uuid.Must(uuid.NewV7()), ProjectId: f.project.Id, Label: "device",
			Conditions: models.JSONText(`{"all":[{"key":"service.instance.id","op":"eq","value":"` + uid.String() + `"}]}`),
			Actions:    models.JSONText(`{"lookbackSeconds":600,"signals":["*"],"replay":true}`),
			ExpiresAt:  now.Add(time.Hour), CreatedAt: now,
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	description := browserDescription("2.0.0")
	description.IdentifyingAttributes = append(description.IdentifyingAttributes, stringKV("service.instance.id", uid.String()))
	resp := f.poll(&protobufs.AgentToServer{InstanceUid: uid[:], Capabilities: sdkCapabilities, AgentDescription: description})
	captures := remoteDocument(t, resp)["captures"].([]any)
	if len(captures) != 1 || resp.ConnectionSettings.Opamp.HeartbeatIntervalSeconds != captureHeartbeat {
		t.Fatalf("captured client got %v with heartbeat %d", captures, resp.ConnectionSettings.Opamp.HeartbeatIntervalSeconds)
	}
}

func TestPollRejectsMalformedRequests(t *testing.T) {
	f := newOpampFixture(t)
	uid := uuid.Must(uuid.NewV7())
	valid, _ := proto.Marshal(&protobufs.AgentToServer{InstanceUid: uid[:], AgentDescription: browserDescription("1.0.0")})

	if got := f.post([]byte("not protobuf at all \xff\xff"), nil); got.Code != http.StatusBadRequest {
		t.Errorf("garbage = %d, want 400", got.Code)
	}
	short, _ := proto.Marshal(&protobufs.AgentToServer{InstanceUid: []byte{1, 2, 3}})
	if got := f.post(short, nil); got.Code != http.StatusBadRequest {
		t.Errorf("short uid = %d, want 400", got.Code)
	}
	if got := f.post(valid, map[string]string{instanceUidHeader: uuid.New().String()}); got.Code != http.StatusBadRequest {
		t.Errorf("mismatched header = %d, want 400", got.Code)
	}
	if got := f.post(valid, map[string]string{instanceUidHeader: uid.String()}); got.Code != http.StatusOK {
		t.Errorf("matching header = %d, want 200", got.Code)
	}
	if got := f.post(bytes.Repeat([]byte{0}, maxBodyBytes+1), nil); got.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized = %d, want 413", got.Code)
	}
	wrongType := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, opampPath, bytes.NewReader(valid))
	req.Header.Set("Content-Type", "application/json")
	f.router.ServeHTTP(wrongType, req)
	if wrongType.Code != http.StatusUnsupportedMediaType {
		t.Errorf("json = %d, want 415", wrongType.Code)
	}

	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	_, _ = w.Write(valid)
	_ = w.Close()
	if got := f.post(gz.Bytes(), map[string]string{"Content-Encoding": "gzip"}); got.Code != http.StatusOK {
		t.Errorf("gzip = %d, want 200", got.Code)
	}
}

type nopLogger struct{}

func (nopLogger) Debugf(context.Context, string, ...interface{}) {}
func (nopLogger) Errorf(context.Context, string, ...interface{}) {}

// TestOpampGoClientInterop drives the handler with opamp-go's own HTTP client, which omits unchanged state from
// its polls: the server's ReportFullState answer must bring the description back so rule changes still arrive.
func TestOpampGoClientInterop(t *testing.T) {
	f := newOpampFixture(t)
	ruleId := f.rule("Console off", `{}`, `{"signals":{"console":{"mode":"off"}}}`)
	server := httptest.NewServer(f.router)
	defer server.Close()

	for _, compression := range []bool{false, true} {
		configs := make(chan *protobufs.AgentRemoteConfig, 8)
		opamp := client.NewHTTP(nopLogger{})
		if err := opamp.SetAgentDescription(browserDescription("2.0.0")); err != nil {
			t.Fatal(err)
		}
		opamp.SetPollingInterval(50 * time.Millisecond)
		err := opamp.Start(context.Background(), types.StartSettings{
			OpAMPServerURL:    server.URL + opampPath,
			InstanceUid:       types.InstanceUid(uuid.Must(uuid.NewV7())),
			EnableCompression: compression,
			Capabilities: protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus |
				protobufs.AgentCapabilities_AgentCapabilities_AcceptsRemoteConfig |
				protobufs.AgentCapabilities_AgentCapabilities_ReportsRemoteConfig,
			Callbacks: types.Callbacks{
				OnMessage: func(_ context.Context, msg *types.MessageData) {
					if msg.RemoteConfig != nil {
						configs <- msg.RemoteConfig
					}
				},
			},
		})
		if err != nil {
			t.Fatal(err)
		}

		first := waitForConfig(t, configs)
		if !bytes.Contains(first.Config.ConfigMap[ConfigMapKey].Body, []byte(`"console":{"mode":"off"}`)) {
			t.Fatalf("unexpected first config: %s", first.Config.ConfigMap[ConfigMapKey].Body)
		}
		_ = opamp.SetRemoteConfigStatus(&protobufs.RemoteConfigStatus{
			LastRemoteConfigHash: first.ConfigHash, Status: protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED,
		})

		_, err = db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
			rule, err := transactional.SdkConfigRepository.FindRule(tx, f.project.Id, ruleId)
			if err != nil {
				return struct{}{}, err
			}
			rule.Settings = models.JSONText(`{"signals":{"console":{"mode":"stream"}}}`)
			return struct{}{}, transactional.SdkConfigRepository.UpdateRule(tx, rule)
		})
		if err != nil {
			t.Fatal(err)
		}
		cache.SdkConfigCache.Invalidate(f.project.Id)

		second := waitForConfig(t, configs)
		if bytes.Equal(second.ConfigHash, first.ConfigHash) || !bytes.Contains(second.Config.ConfigMap[ConfigMapKey].Body, []byte(`"console":{"mode":"stream"}`)) {
			t.Fatalf("rule change not delivered (compression=%v): %s", compression, second.Config.ConfigMap[ConfigMapKey].Body)
		}
		_ = opamp.Stop(context.Background())

		_, _ = db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
			rule, _ := transactional.SdkConfigRepository.FindRule(tx, f.project.Id, ruleId)
			rule.Settings = models.JSONText(`{"signals":{"console":{"mode":"off"}}}`)
			return struct{}{}, transactional.SdkConfigRepository.UpdateRule(tx, rule)
		})
		cache.SdkConfigCache.Invalidate(f.project.Id)
	}
}

func waitForConfig(t *testing.T, configs <-chan *protobufs.AgentRemoteConfig) *protobufs.AgentRemoteConfig {
	t.Helper()
	select {
	case cfg := <-configs:
		return cfg
	case <-time.After(5 * time.Second):
		t.Fatal("no remote config within 5s")
		return nil
	}
}

func TestOpampRejectsNonBrowserProjects(t *testing.T) {
	f := newOpampFixture(t)
	for _, framework := range []string{"go", "nestjs", "nextjs", "react-native", "flutter", "ios", "android", ""} {
		t.Run(framework, func(t *testing.T) {
			f.project.Framework = framework
			response := f.post(nil, nil)
			if response.Code != http.StatusForbidden {
				t.Fatalf("got %d: %s", response.Code, response.Body.String())
			}
		})
	}
}
