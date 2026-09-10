//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package slack

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/tracewayapp/traceway/backend/app/agent"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/dbtest"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/notifications"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/secrets"
)

const (
	testSecret  = "8f742231b10e8888abcd99yyyzzz85a5"
	testHash    = "0123456789abcdef"
	otherHash   = "fedcba9876543210"
	ownerSlack  = "U0OWNER"
	readerSlack = "U0READER"
	strangerId  = "U0STRANGER"
)

type post struct {
	Channel   string
	ThreadTs  string
	Text      string
	Blocks    string
	Ephemeral bool
	User      string
}

type member struct {
	name, real, email string
}

// fakeSlack is the Slack Web API and, for Socket Mode, the websocket the
// app connects to.
type fakeSlack struct {
	server *httptest.Server
	ws     *httptest.Server

	mu       sync.Mutex
	posts    []post
	failNext int
	users    map[string]member
	connects int
	frames   []string
	acks     chan string
}

func newFakeSlack(t *testing.T) *fakeSlack {
	t.Helper()
	f := &fakeSlack{users: map[string]member{
		ownerSlack:  {name: "dev", real: "Dev Owner", email: "dev@example.com"},
		readerSlack: {name: "reader", real: "Read Only", email: "reader@example.com"},
		strangerId:  {name: "stranger", real: "Some Stranger", email: "nobody@example.com"},
	}, acks: make(chan string, 16)}

	mux := http.NewServeMux()
	mux.HandleFunc("/chat.postMessage", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.failNext > 0 {
			f.failNext--
			writeJSON(w, map[string]any{"ok": false, "error": "ratelimited"})
			return
		}
		f.posts = append(f.posts, post{Channel: r.FormValue("channel"), ThreadTs: r.FormValue("thread_ts"), Text: r.FormValue("text"), Blocks: r.FormValue("blocks")})
		writeJSON(w, map[string]any{"ok": true, "channel": r.FormValue("channel"), "ts": fmt.Sprintf("1700000000.%06d", len(f.posts))})
	})
	mux.HandleFunc("/chat.postEphemeral", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.mu.Lock()
		f.posts = append(f.posts, post{Channel: r.FormValue("channel"), Text: r.FormValue("text"), Ephemeral: true, User: r.FormValue("user")})
		f.mu.Unlock()
		writeJSON(w, map[string]any{"ok": true, "message_ts": "1700000000.000001"})
	})
	mux.HandleFunc("/users.info", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		id := r.FormValue("user")
		u, ok := f.users[id]
		if !ok {
			writeJSON(w, map[string]any{"ok": false, "error": "user_not_found"})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "user": map[string]any{"id": id, "name": u.name, "real_name": u.real, "profile": map[string]any{"email": u.email}}})
	})
	mux.HandleFunc("/chat.getPermalink", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		writeJSON(w, map[string]any{"ok": true, "permalink": "https://acme.slack.com/archives/" + r.FormValue("channel") + "/p" + strings.ReplaceAll(r.FormValue("message_ts"), ".", "")})
	})
	mux.HandleFunc("/apps.connections.open", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.connects++
		f.mu.Unlock()
		writeJSON(w, map[string]any{"ok": true, "url": "ws" + strings.TrimPrefix(f.ws.URL, "http") + "/link"})
	})
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)

	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	f.ws = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"hello","num_connections":1}`))
		f.mu.Lock()
		frames := f.frames
		f.frames = nil
		f.mu.Unlock()
		for _, frame := range frames {
			if err := conn.WriteMessage(websocket.TextMessage, []byte(frame)); err != nil {
				return
			}
		}
		for range frames {
			_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
			_, ack, err := conn.ReadMessage()
			if err != nil {
				return
			}
			f.acks <- string(ack)
		}
		if len(frames) > 0 {
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(time.Minute))
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	t.Cleanup(f.ws.Close)
	return f
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (f *fakeSlack) snapshot() []post {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]post{}, f.posts...)
}

func (f *fakeSlack) connectCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.connects
}

type fixture struct {
	app      *App
	fake     *fakeSlack
	in       *models.Integration
	project  *models.Project
	ownerId  int
	readerId int
}

func setup(t *testing.T, appToken string) fixture {
	t.Helper()
	config.LoggingEnabled = false
	t.Cleanup(func() { config.LoggingEnabled = true })
	dbtest.SetupSQLite(t)
	key, _, err := secrets.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	secrets.Init(key)
	t.Cleanup(func() { secrets.Init(nil) })

	fake := newFakeSlack(t)
	app := &App{APIBase: fake.server.URL + "/", Client: fake.server.Client(), DashboardURL: func() string { return "https://tw.example.com" }}
	Register(app)

	fx, err := db.ExecuteTransaction(func(tx *sql.Tx) (fixture, error) {
		owner, err := transactional.UserRepository.Create(tx, "dev@example.com", "Dev", "hashed")
		if err != nil {
			return fixture{}, err
		}
		reader, err := transactional.UserRepository.Create(tx, "reader@example.com", "Reader", "hashed")
		if err != nil {
			return fixture{}, err
		}
		org, err := transactional.OrganizationRepository.Create(tx, "Org", "UTC")
		if err != nil {
			return fixture{}, err
		}
		if _, err := transactional.OrganizationRepository.AddUser(tx, org.Id, owner.Id, "owner"); err != nil {
			return fixture{}, err
		}
		if _, err := transactional.OrganizationRepository.AddUser(tx, org.Id, reader.Id, "readonly"); err != nil {
			return fixture{}, err
		}
		project, err := transactional.ProjectRepository.CreateWithOrganization(tx, "Project", "opentelemetry", org.Id)
		if err != nil {
			return fixture{}, err
		}
		raw := fmt.Sprintf(`{"botToken":"xoxb-test","signingSecret":%q,"appToken":%q,"channel":"C0DEFAULT1"}`, testSecret, appToken)
		encrypted, err := secrets.EncryptFields(json.RawMessage(raw), secretFields)
		if err != nil {
			return fixture{}, err
		}
		now := time.Now().UTC()
		in := &models.Integration{OrganizationId: org.Id, Provider: Provider, Kinds: models.StringSlice{agent.KindChat, agent.KindTrigger}, Name: "Acme Slack", Config: models.JSONText(encrypted), Enabled: true, CreatedAt: now, UpdatedAt: now}
		id, err := transactional.IntegrationRepository.Create(tx, in)
		if err != nil {
			return fixture{}, err
		}
		in.Id = id
		return fixture{app: app, fake: fake, in: in, project: project, ownerId: owner.Id, readerId: reader.Id}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return fx
}

func (fx fixture) issueURL(hash string) string {
	return fmt.Sprintf("https://tw.example.com/issues/%s?projectId=%s", hash, fx.project.Id)
}

func (fx fixture) recordException(t *testing.T, hash string) {
	t.Helper()
	err := telemetry.ExceptionStackTraceRepository.InsertAsync(context.Background(), []models.ExceptionStackTrace{{
		Id: uuid.New(), ProjectId: fx.project.Id, TraceType: "endpoint", ExceptionHash: hash, StackTrace: "RuntimeError: boom", RecordedAt: time.Now().UTC(), Attributes: map[string]string{},
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func sign(secret string, ts int64, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("v0:%d:%s", ts, body)))
	return "v0=" + hex.EncodeToString(mac.Sum(nil))
}

func signedRequest(secret string, contentType string, body string, at time.Time) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/integrations/slack/inbound/1", strings.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Slack-Request-Timestamp", strconv.FormatInt(at.Unix(), 10))
	req.Header.Set("X-Slack-Signature", sign(secret, at.Unix(), body))
	return req
}

func (fx fixture) signed(contentType string, body string) *http.Request {
	return signedRequest(testSecret, contentType, body, time.Now())
}

func eventBody(inner map[string]any) string {
	body, _ := json.Marshal(map[string]any{"token": "t", "team_id": "T1", "api_app_id": "A1", "type": "event_callback", "event_id": "Ev1", "event_time": 1, "event": inner})
	return string(body)
}

func mentionBody(text string) string {
	return eventBody(map[string]any{"type": "app_mention", "user": ownerSlack, "text": text, "ts": "2.100", "channel": "C1", "event_ts": "2.100"})
}

func threadMessageBody(user string, text string, ts string, threadTs string) string {
	return eventBody(map[string]any{"type": "message", "user": user, "text": text, "ts": ts, "thread_ts": threadTs, "channel": "C1", "channel_type": "channel"})
}

func actionBody(user string, actionId string, value string) string {
	payload, _ := json.Marshal(map[string]any{
		"type":         "block_actions",
		"user":         map[string]any{"id": user, "username": "someone"},
		"channel":      map[string]any{"id": "C1", "name": "alerts"},
		"message":      map[string]any{"type": "message", "ts": "1.100", "text": "alert"},
		"actions":      []map[string]any{{"action_id": actionId, "block_id": "traceway_actions", "type": "button", "value": value}},
		"response_url": "https://hooks.slack.com/actions/x",
	})
	return url.Values{"payload": {string(payload)}}.Encode()
}

func slashBody(user string, text string) string {
	return url.Values{"command": {"/traceway"}, "text": {text}, "user_id": {user}, "channel_id": {"C1"}, "response_url": {"https://hooks.slack.com/commands/x"}}.Encode()
}

func (fx fixture) target(hash string) string {
	value, _ := json.Marshal(issueTarget{ProjectId: fx.project.Id.String(), Hash: hash})
	return string(value)
}

func (fx fixture) activeAttempt(t *testing.T, hash string) *models.AgentAttempt {
	t.Helper()
	attempt, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.AgentAttempt, error) {
		return transactional.AgentAttemptRepository.FindActiveBySubject(tx, fx.project.Id, models.SubjectKindTracewayException, hash)
	})
	if err != nil {
		t.Fatal(err)
	}
	return attempt
}

func (fx fixture) links(t *testing.T, attemptId uuid.UUID) map[string]*models.AgentLink {
	t.Helper()
	rows, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.AgentLink, error) {
		return transactional.AgentLinkRepository.FindByAttempt(tx, attemptId)
	})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]*models.AgentLink{}
	for _, row := range rows {
		out[row.Kind] = row
	}
	return out
}

func (fx fixture) inbound(t *testing.T, req *http.Request) agent.InboundResult {
	t.Helper()
	result, err := agent.HandleInbound(context.Background(), Provider, fx.in, req)
	if err != nil {
		t.Fatalf("inbound: %v", err)
	}
	return result
}

func waitFor(t *testing.T, timeout time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

const formType = "application/x-www-form-urlencoded"

func TestSignatureVerification(t *testing.T) {
	fx := setup(t, "")
	body := eventBody(map[string]any{"type": "app_mention", "user": ownerSlack, "text": "hi", "ts": "1.1", "channel": "C1"})

	if _, err := fx.app.parseRequest(fx.in, fx.signed("application/json", body)); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	_, err := fx.app.parseRequest(fx.in, signedRequest(testSecret, "application/json", body, time.Now().Add(-10*time.Minute)))
	if err == nil || !strings.Contains(err.Error(), "signature rejected") {
		t.Fatalf("expired timestamp accepted: %v", err)
	}
	_, err = fx.app.parseRequest(fx.in, signedRequest("wrong-secret", "application/json", body, time.Now()))
	if err == nil || !strings.Contains(err.Error(), "signature rejected") {
		t.Fatalf("wrong secret accepted: %v", err)
	}
	unsigned := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	if _, err := fx.app.parseRequest(fx.in, unsigned); err == nil {
		t.Fatal("unsigned request accepted")
	}
	if _, err := agent.HandleInbound(context.Background(), Provider, fx.in, signedRequest("wrong-secret", "application/json", body, time.Now())); err == nil {
		t.Fatal("HandleInbound accepted a bad signature")
	}
}

func TestURLVerificationChallenge(t *testing.T) {
	fx := setup(t, "")
	result := fx.inbound(t, fx.signed("application/json", `{"token":"t","challenge":"abc123","type":"url_verification"}`))
	if string(result.Response) != `{"challenge":"abc123"}` {
		t.Fatalf("challenge response = %s", result.Response)
	}
}

func TestFixItButtonStartsAttemptUnderTheAlert(t *testing.T) {
	fx := setup(t, "")
	result := fx.inbound(t, fx.signed(formType, actionBody(ownerSlack, actionFixIt, fx.target(testHash))))
	if result.Requests != 1 {
		t.Fatalf("requests = %+v", result)
	}
	attempt := fx.activeAttempt(t, testHash)
	if attempt == nil || attempt.Status != models.AttemptQueued || attempt.RequestedBy == nil || *attempt.RequestedBy != fx.ownerId {
		t.Fatalf("attempt = %+v", attempt)
	}
	links := fx.links(t, attempt.Id)
	if links[models.LinkKindAlert] == nil || links[models.LinkKindAlert].ExternalRef != "C1/1.100" || links[models.LinkKindAlert].URL == "" {
		t.Fatalf("alert link = %+v", links[models.LinkKindAlert])
	}
	thread := links[models.LinkKindThread]
	if thread == nil || thread.ExternalRef != "C1/1.100" || thread.IntegrationId == nil || *thread.IntegrationId != fx.in.Id {
		t.Fatalf("thread link = %+v", thread)
	}
	posts := fx.fake.snapshot()
	if len(posts) != 1 || posts[0].Channel != "C1" || posts[0].ThreadTs != "1.100" || !strings.Contains(posts[0].Text, "Attempt 1") || !strings.Contains(posts[0].Text, "/agent/"+attempt.Id.String()) {
		t.Fatalf("thread opener = %+v", posts)
	}
	identity, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.Identity, error) {
		return transactional.IdentityRepository.FindByExternalId(tx, Provider, fmt.Sprintf("%d:%s", fx.in.Id, ownerSlack))
	})
	if err != nil || identity == nil || identity.UserId != fx.ownerId || identity.Display != "Dev Owner" {
		t.Fatalf("identity = %+v, %v", identity, err)
	}

	again := fx.inbound(t, fx.signed(formType, actionBody(ownerSlack, actionFixIt, fx.target(testHash))))
	if again.Requests != 0 || len(fx.fake.snapshot()) != 1 {
		t.Fatalf("second press started another attempt: %+v", again)
	}
}

func TestArchiveButtonArchivesAndReplies(t *testing.T) {
	fx := setup(t, "")
	fx.recordException(t, testHash)
	result := fx.inbound(t, fx.signed(formType, actionBody(ownerSlack, actionArchive, fx.target(testHash))))
	if result.Requests != 0 {
		t.Fatalf("archive started an attempt: %+v", result)
	}
	archived, err := telemetry.ExceptionStackTraceRepository.IsArchived(context.Background(), fx.project.Id, testHash)
	if err != nil || !archived {
		t.Fatalf("archived = %v, %v", archived, err)
	}
	posts := fx.fake.snapshot()
	if len(posts) != 1 || posts[0].ThreadTs != "1.100" || !strings.Contains(posts[0].Text, "Archived by <@"+ownerSlack+">") {
		t.Fatalf("reply = %+v", posts)
	}
}

func TestViewButtonIsIgnored(t *testing.T) {
	fx := setup(t, "")
	result := fx.inbound(t, fx.signed(formType, actionBody(ownerSlack, actionView, "")))
	if result.Requests != 0 || len(fx.fake.snapshot()) != 0 {
		t.Fatalf("view did something: %+v %+v", result, fx.fake.snapshot())
	}
}

func TestMentionWithIssueLinkStartsAttemptInThread(t *testing.T) {
	fx := setup(t, "")
	result := fx.inbound(t, fx.signed("application/json", mentionBody("<@UBOT> fix <"+fx.issueURL(testHash)+"|link>")))
	if result.Requests != 1 {
		t.Fatalf("requests = %+v", result)
	}
	attempt := fx.activeAttempt(t, testHash)
	if attempt == nil {
		t.Fatal("no attempt")
	}
	links := fx.links(t, attempt.Id)
	if links[models.LinkKindOrigin] == nil || links[models.LinkKindOrigin].ExternalRef != "C1/2.100" {
		t.Fatalf("origin = %+v", links[models.LinkKindOrigin])
	}
	posts := fx.fake.snapshot()
	if len(posts) != 1 || posts[0].ThreadTs != "2.100" {
		t.Fatalf("opener = %+v", posts)
	}
}

func TestSlashCommandWithBareHashUsesTheCommandChannel(t *testing.T) {
	fx := setup(t, "")
	fx.recordException(t, testHash)
	result := fx.inbound(t, fx.signed(formType, slashBody(ownerSlack, "fix "+testHash)))
	if result.Requests != 1 {
		t.Fatalf("requests = %+v", result)
	}
	attempt := fx.activeAttempt(t, testHash)
	if attempt == nil {
		t.Fatal("no attempt")
	}
	posts := fx.fake.snapshot()
	if len(posts) != 1 || posts[0].Channel != "C1" || posts[0].ThreadTs != "" {
		t.Fatalf("opener = %+v", posts)
	}
	if fx.links(t, attempt.Id)[models.LinkKindThread].ExternalRef != "C1/1700000000.000001" {
		t.Fatalf("thread = %+v", fx.links(t, attempt.Id)[models.LinkKindThread])
	}
}

func TestMistakesAreAnsweredNotStarted(t *testing.T) {
	cases := []struct {
		name  string
		body  string
		ctype string
		want  string
	}{
		{"usage", mentionBody("<@UBOT> hello there"), "application/json", "Tell me which issue"},
		{"unknown hash", mentionBody("<@UBOT> fix " + otherHash), "application/json", "can't find issue"},
		{"other org link", mentionBody("<@UBOT> fix https://tw.example.com/issues/" + testHash + "?projectId=" + uuid.NewString()), "application/json", "another organization"},
		{"readonly member", actionBody(readerSlack, actionFixIt, ""), formType, "read-only"},
		{"unlinked account", actionBody(strangerId, actionFixIt, ""), formType, "not linked"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := setup(t, "")
			body := tc.body
			if strings.Contains(body, "payload=") {
				body = actionBody(map[string]string{"readonly member": readerSlack, "unlinked account": strangerId}[tc.name], actionFixIt, fx.target(testHash))
			}
			result := fx.inbound(t, fx.signed(tc.ctype, body))
			if result.Requests != 0 || fx.activeAttempt(t, testHash) != nil {
				t.Fatalf("attempt started: %+v", result)
			}
			posts := fx.fake.snapshot()
			if len(posts) != 1 || !strings.Contains(posts[0].Text, tc.want) {
				t.Fatalf("reply = %+v, want %q", posts, tc.want)
			}
		})
	}
}

func TestSlashMistakeIsEphemeral(t *testing.T) {
	fx := setup(t, "")
	fx.inbound(t, fx.signed(formType, slashBody(ownerSlack, "fix "+otherHash)))
	posts := fx.fake.snapshot()
	if len(posts) != 1 || !posts[0].Ephemeral || posts[0].User != ownerSlack {
		t.Fatalf("reply = %+v", posts)
	}
}

func TestThreadReplyResumesTheAttempt(t *testing.T) {
	fx := setup(t, "")
	fx.inbound(t, fx.signed(formType, actionBody(ownerSlack, actionFixIt, fx.target(testHash))))
	attempt := fx.activeAttempt(t, testHash)

	result := fx.inbound(t, fx.signed("application/json", threadMessageBody(ownerSlack, "It is Postgres 16.", "1.200", "1.100")))
	if result.Messages != 1 {
		t.Fatalf("messages = %+v", result)
	}
	messages, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.AgentMessage, error) {
		return transactional.AgentMessageRepository.ListAfter(tx, attempt.Id, 0, "", 10)
	})
	if err != nil || len(messages) != 1 || messages[0].Provider != Provider || messages[0].Direction != models.MessageInbound || messages[0].Body != "It is Postgres 16." || messages[0].IdentityId == nil {
		t.Fatalf("messages = %+v, %v", messages, err)
	}

	retry := fx.inbound(t, fx.signed("application/json", threadMessageBody(ownerSlack, "It is Postgres 16.", "1.200", "1.100")))
	if retry.Messages != 0 || retry.Ignored != 1 {
		t.Fatalf("redelivery = %+v", retry)
	}
	elsewhere := fx.inbound(t, fx.signed("application/json", threadMessageBody(ownerSlack, "unrelated", "9.200", "9.100")))
	if elsewhere.Messages != 0 || elsewhere.Ignored != 0 || len(fx.fake.snapshot()) != 1 {
		t.Fatalf("foreign thread = %+v %+v", elsewhere, fx.fake.snapshot())
	}
	bot := eventBody(map[string]any{"type": "message", "bot_id": "B1", "text": "echo", "ts": "1.300", "thread_ts": "1.100", "channel": "C1"})
	if r := fx.inbound(t, fx.signed("application/json", bot)); r.Messages != 0 {
		t.Fatalf("bot message accepted: %+v", r)
	}
	stranger := fx.inbound(t, fx.signed("application/json", threadMessageBody(strangerId, "me too", "1.400", "1.100")))
	posts := fx.fake.snapshot()
	if stranger.Messages != 0 || len(posts) != 2 || posts[1].ThreadTs != "1.100" || !strings.Contains(posts[1].Text, "not linked") {
		t.Fatalf("stranger reply = %+v %+v", stranger, posts)
	}
}

func TestMirroredMessagesReachTheThread(t *testing.T) {
	fx := setup(t, "")
	fx.inbound(t, fx.signed(formType, actionBody(ownerSlack, actionFixIt, fx.target(testHash))))
	attempt := fx.activeAttempt(t, testHash)
	now := time.Now().UTC()
	author := agent.Identity{UserId: fx.ownerId, Provider: "web", Display: "Dev"}
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.AgentMessage, error) {
		if _, err := agent.Post(tx, attempt, agent.Message{Direction: models.MessageOutbound, Provider: agent.ProviderAgent, Kind: models.MessageKindQuestion, Body: "Which database <version>?"}, nil, "", now); err != nil {
			return nil, err
		}
		return agent.Post(tx, attempt, agent.Message{Direction: models.MessageInbound, Provider: "web", Kind: models.MessageKindAnswer, Body: "Postgres 16", Author: &author}, nil, "", now)
	})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.OutboxDelivery, error) {
		return transactional.OutboxRepository.FindDue(tx, time.Now().UTC().Add(time.Minute), 10)
	})
	if err != nil {
		t.Fatal(err)
	}
	send := agent.OutboxSender(notifications.AdapterSend)
	for _, row := range rows {
		var msg models.NotificationMessage
		_ = json.Unmarshal(row.Message, &msg)
		if err := send(context.Background(), row.AdapterType, json.RawMessage(row.AdapterConfig), msg); err != nil {
			t.Fatalf("send %s: %v", row.AdapterType, err)
		}
	}
	posts := fx.fake.snapshot()
	if len(posts) != 3 {
		t.Fatalf("posts = %+v", posts)
	}
	if posts[1].ThreadTs != "1.100" || !strings.HasPrefix(posts[1].Text, ":question: *The agent needs an answer.*") || !strings.Contains(posts[1].Text, "&lt;version&gt;") {
		t.Fatalf("question = %+v", posts[1])
	}
	if posts[2].ThreadTs != "1.100" || posts[2].Text != "_Dev via web_\nPostgres 16" {
		t.Fatalf("answer = %+v", posts[2])
	}
}

func TestBrowserStartDurablyOpensSlackAndReplaysEarlyQuestion(t *testing.T) {
	fx := setup(t, "")
	started, err := db.ExecuteTransaction(func(tx *sql.Tx) (*agent.StartResult, error) {
		started, err := agent.StartAttempt(tx, fx.project, agent.Subject{ProjectId: fx.project.Id, Kind: models.SubjectKindTracewayException, Ref: testHash}, agent.StartOptions{})
		if err != nil {
			return nil, err
		}
		_, err = agent.Post(tx, started.Attempt, agent.Message{Provider: agent.ProviderAgent, Direction: models.MessageOutbound, Kind: models.MessageKindQuestion, Body: "Early question"}, nil, "", time.Now().UTC())
		return started, err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(fx.fake.snapshot()) != 0 {
		t.Fatal("provider called before durable delivery")
	}
	rows, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.OutboxDelivery, error) {
		return transactional.OutboxRepository.FindDue(tx, time.Now().UTC().Add(time.Minute), 20)
	})
	if err != nil {
		t.Fatal(err)
	}
	send := agent.OutboxSender(notifications.AdapterSend)
	var opening *models.OutboxDelivery
	for _, row := range rows {
		if row.AdapterType == "agent-open-thread" {
			opening = row
		}
	}
	if opening == nil {
		t.Fatal("browser start did not enqueue chat opening")
	}
	for i := 0; i < 2; i++ {
		if err := send(context.Background(), opening.AdapterType, json.RawMessage(opening.AdapterConfig), models.NotificationMessage{}); err != nil {
			t.Fatal(err)
		}
	}
	if len(fx.fake.snapshot()) != 1 {
		t.Fatal("retry opened duplicate thread")
	}
	rows, err = db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.OutboxDelivery, error) {
		return transactional.OutboxRepository.FindDue(tx, time.Now().UTC().Add(time.Minute), 20)
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.AdapterType == "agent:slack" {
			if err := send(context.Background(), row.AdapterType, json.RawMessage(row.AdapterConfig), models.NotificationMessage{}); err != nil {
				t.Fatal(err)
			}
		}
	}
	posts := fx.fake.snapshot()
	if len(posts) != 2 || !strings.Contains(posts[1].Text, "Early question") {
		t.Fatalf("early question not replayed for %s: %+v", started.Attempt.Id, posts)
	}
}

func TestAlertAdapterPostsButtonsAndRetries(t *testing.T) {
	fx := setup(t, "")
	adapter, err := notifications.NewAdapter(ChannelType, json.RawMessage(fmt.Sprintf(`{"integrationId":%d,"channel":"C0ALERTS01"}`, fx.in.Id)))
	if err != nil {
		t.Fatal(err)
	}
	if err := adapter.Validate(); err != nil {
		t.Fatal(err)
	}
	bound, ok := adapter.(notifications.IntegrationBound)
	if !ok || bound.IntegrationId() != fx.in.Id {
		t.Fatalf("adapter is not integration bound: %T", adapter)
	}
	msg := notifications.Message{Subject: "New error: RuntimeError", Body: "boom <details>", Severity: notifications.SeverityCritical, RuleName: "New errors", URL: fx.issueURL(testHash), ProjectId: fx.project.Id.String()}

	fx.fake.mu.Lock()
	fx.fake.failNext = 1
	fx.fake.mu.Unlock()
	if err := adapter.Send(context.Background(), msg); err == nil {
		t.Fatal("first send succeeded despite the API error")
	}
	if err := adapter.Send(context.Background(), msg); err != nil {
		t.Fatalf("retry failed: %v", err)
	}
	posts := fx.fake.snapshot()
	if len(posts) != 1 || posts[0].Channel != "C0ALERTS01" {
		t.Fatalf("posts = %+v", posts)
	}
	var blocks []map[string]any
	if err := json.Unmarshal([]byte(posts[0].Blocks), &blocks); err != nil {
		t.Fatalf("blocks: %v (%s)", err, posts[0].Blocks)
	}
	actions := blocks[len(blocks)-1]
	elements, _ := actions["elements"].([]any)
	var ids []string
	for _, element := range elements {
		button := element.(map[string]any)
		ids = append(ids, button["action_id"].(string))
		if button["action_id"] == actionFixIt {
			var target issueTarget
			_ = json.Unmarshal([]byte(button["value"].(string)), &target)
			if target.Hash != testHash || target.ProjectId != fx.project.Id.String() {
				t.Fatalf("fix it target = %+v", target)
			}
		}
		if button["action_id"] == actionView && button["url"] != fx.issueURL(testHash) {
			t.Fatalf("view url = %v", button["url"])
		}
	}
	if strings.Join(ids, ",") != actionView+","+actionFixIt+","+actionArchive {
		t.Fatalf("buttons = %v", ids)
	}
	if section := blocks[1]["text"].(map[string]any)["text"]; section != "boom &lt;details&gt;" {
		t.Fatalf("body not escaped: %v", section)
	}

	plain := notifications.Message{Subject: "Error rate high", Body: "5%", Severity: notifications.SeverityWarning, URL: "/endpoints"}
	if err := adapter.Send(context.Background(), plain); err != nil {
		t.Fatal(err)
	}
	if last := fx.fake.snapshot()[1].Blocks; strings.Contains(last, actionFixIt) || strings.Contains(last, actionView) {
		t.Fatalf("non-issue alert got issue buttons: %s", last)
	}

	_, err = db.ExecuteTransaction(func(tx *sql.Tx) (any, error) {
		fx.in.Enabled = false
		return nil, transactional.IntegrationRepository.Update(tx, fx.in)
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := adapter.Send(context.Background(), msg); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("disabled integration send = %v", err)
	}
}

func TestValidation(t *testing.T) {
	app := New()
	good := map[string]string{"botToken": "xoxb-1", "signingSecret": "s", "appToken": "xapp-1", "channel": "C0123456789"}
	if err := app.Validate(good); err != nil {
		t.Fatal(err)
	}
	for name, bad := range map[string]map[string]string{
		"bot token":   {"botToken": "xoxp-1", "signingSecret": "s", "channel": "C0123456789"},
		"secret":      {"botToken": "xoxb-1", "signingSecret": " ", "channel": "C0123456789"},
		"app token":   {"botToken": "xoxb-1", "signingSecret": "s", "appToken": "xoxb-2", "channel": "C0123456789"},
		"channel":     {"botToken": "xoxb-1", "signingSecret": "s", "channel": "#alerts"},
		"no app path": {"botToken": "xoxb-1", "signingSecret": "s", "appToken": "", "channel": "alerts"},
	} {
		if err := app.Validate(bad); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	for _, cfg := range []string{`{"integrationId":0,"channel":"C1"}`, `{"integrationId":3,"channel":"general"}`} {
		adapter, err := app.newAdapter(json.RawMessage(cfg))
		if err != nil {
			t.Fatal(err)
		}
		if err := adapter.Validate(); err == nil {
			t.Fatalf("%s accepted", cfg)
		}
	}
	if ref, ok := commandRef("<@U1> fix <https://x/issues/0123456789abcdef|https://x/issues/0123456789abcdef>"); !ok || ref != "https://x/issues/0123456789abcdef" {
		t.Fatalf("commandRef = %q %v", ref, ok)
	}
	if _, ok := commandRef("<@U1> please fix everything"); ok {
		t.Fatal("chatter parsed as a command")
	}
	if got := mirrorText(agent.Message{Provider: agent.ProviderAgent, Kind: models.MessageKindPR, Body: "Opened https://x"}); got != ":rocket: Opened https://x" {
		t.Fatalf("mirrorText = %q", got)
	}
	if got := truncate("héllo", 2); got != "h…" {
		t.Fatalf("truncate = %q", got)
	}
}

func TestSocketModeDeliversAcksReconnectsAndStopsWhenDisabled(t *testing.T) {
	fx := setup(t, "xapp-test")
	fx.fake.mu.Lock()
	fx.fake.frames = []string{`{"envelope_id":"env-1","type":"events_api","accepts_response_payload":false,"payload":` + mentionBody("<@UBOT> fix "+fx.issueURL(testHash)) + `}`}
	fx.fake.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	manager := fx.app.StartSocketMode(ctx, time.Hour)

	waitFor(t, 15*time.Second, "the attempt from the socket frame", func() bool { return fx.activeAttempt(t, testHash) != nil })
	select {
	case ack := <-fx.fake.acks:
		if !strings.Contains(ack, "env-1") {
			t.Fatalf("ack = %s", ack)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no ack")
	}
	waitFor(t, 30*time.Second, "a reconnect after the server closed the socket", func() bool { return fx.fake.connectCount() >= 2 })
	if running := manager.Running(); len(running) != 1 || running[0] != fx.in.Id {
		t.Fatalf("running = %v", running)
	}

	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (any, error) {
		fx.in.Enabled = false
		return nil, transactional.IntegrationRepository.Update(tx, fx.in)
	})
	if err != nil {
		t.Fatal(err)
	}
	agent.IntegrationsChanged()
	waitFor(t, 10*time.Second, "the loop to stop", func() bool { return len(manager.Running()) == 0 })

	_, err = db.ExecuteTransaction(func(tx *sql.Tx) (any, error) {
		fx.in.Enabled = true
		return nil, transactional.IntegrationRepository.Update(tx, fx.in)
	})
	if err != nil {
		t.Fatal(err)
	}
	before := fx.fake.connectCount()
	manager.Kick()
	waitFor(t, 10*time.Second, "the loop to restart", func() bool { return len(manager.Running()) == 1 && fx.fake.connectCount() > before })
}

func TestApprovalMessageAndButtons(t *testing.T) {
	fx := setup(t, "")
	attempt, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.AgentAttempt, error) {
		started, err := agent.StartAttempt(tx, fx.project, agent.Subject{Kind: models.SubjectKindTracewayException, Ref: testHash, ProjectId: fx.project.Id}, agent.StartOptions{RequireApproval: true})
		if err != nil {
			return nil, err
		}
		return started.Attempt, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := agent.RequestApproval(context.Background(), attempt); err != nil {
		t.Fatal(err)
	}
	posts := fx.fake.snapshot()
	if len(posts) != 1 || posts[0].Channel != "C0DEFAULT1" || !strings.Contains(posts[0].Blocks, actionApprove) || !strings.Contains(posts[0].Blocks, actionDismiss) {
		t.Fatalf("approval message = %+v", posts)
	}
	links, _ := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.AgentLink, error) {
		return transactional.AgentLinkRepository.FindByAttempt(tx, attempt.Id)
	})
	if len(links) != 1 || links[0].Kind != models.LinkKindThread || links[0].ExternalRef != "C0DEFAULT1/1700000000.000001" {
		t.Fatalf("thread link = %+v", links)
	}
	value, _ := json.Marshal(approvalTarget{AttemptId: attempt.Id.String()})

	stranger := fx.inbound(t, fx.signed(formType, actionBody(strangerId, actionApprove, string(value))))
	if stranger.Requests != 0 || !strings.Contains(fx.fake.snapshot()[1].Text, "not linked") {
		t.Fatalf("unlinked approver = %+v %+v", stranger, fx.fake.snapshot())
	}
	reader := fx.inbound(t, fx.signed(formType, actionBody(readerSlack, actionApprove, string(value))))
	if reader.Requests != 0 || !strings.Contains(fx.fake.snapshot()[2].Text, "read-only") {
		t.Fatalf("readonly approver = %+v", fx.fake.snapshot())
	}
	if row := fx.activeAttempt(t, testHash); row.Status != models.AttemptPendingApproval {
		t.Fatalf("attempt moved without approval: %s", row.Status)
	}

	fx.inbound(t, fx.signed(formType, actionBody(ownerSlack, actionApprove, string(value))))
	if row := fx.activeAttempt(t, testHash); row.Status != models.AttemptQueued || row.ApprovedBy == nil || *row.ApprovedBy != fx.ownerId {
		t.Fatalf("after approve = %+v", row)
	}
	if !strings.Contains(fx.fake.snapshot()[3].Text, "Approved by <@"+ownerSlack+">") {
		t.Fatalf("approve reply = %+v", fx.fake.snapshot()[3])
	}
	fx.inbound(t, fx.signed(formType, actionBody(ownerSlack, actionApprove, string(value))))
	if !strings.Contains(fx.fake.snapshot()[4].Text, "not waiting for approval") {
		t.Fatalf("second approve reply = %+v", fx.fake.snapshot()[4])
	}

	fx.inbound(t, fx.signed(formType, actionBody(ownerSlack, actionDismiss, string(value))))
	if fx.activeAttempt(t, testHash) != nil {
		t.Fatal("dismiss must cancel the attempt")
	}
	if !strings.Contains(fx.fake.snapshot()[5].Text, "Dismissed by") {
		t.Fatalf("dismiss reply = %+v", fx.fake.snapshot()[5])
	}
}
