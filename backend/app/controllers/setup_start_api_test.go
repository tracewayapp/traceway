//go:build !telemetry_ch && !transactional_pg && !telemetry_duckdb

package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/tracewayapp/traceway/backend/v2/app/cache"
	"github.com/tracewayapp/traceway/backend/v2/app/config"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/middleware"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/transactional"
)

func withAppBaseURL(t *testing.T, url string) {
	t.Helper()
	prev := config.Config.AppBaseURL
	config.Config.AppBaseURL = url
	t.Cleanup(func() { config.Config.AppBaseURL = prev })
}

func TestInstanceUrl(t *testing.T) {
	setupSetupControllerDB(t)

	cases := []struct {
		name       string
		appBaseURL string
		host       string
		headers    map[string]string
		want       string
	}{
		{name: "configured", appBaseURL: "https://traceway.acme.test/", host: "10.0.0.5:8082", want: "https://traceway.acme.test"},
		{name: "cloud configured", appBaseURL: "https://cloud.tracewayapp.com", host: "127.0.0.1:8082", want: "https://cloud.tracewayapp.com"},
		{name: "configured without a scheme", appBaseURL: "traceway.acme.test/api/", host: "10.0.0.5:8082", want: "https://traceway.acme.test"},
		{name: "configured under a sub-path", appBaseURL: "https://corp.example/traceway/", host: "10.0.0.5:8082", want: "https://corp.example/traceway"},
		{name: "self-hosted without APP_BASE_URL", host: "traceway.internal:8082", want: "http://traceway.internal:8082"},
		{
			name:    "self-hosted behind a proxy",
			host:    "127.0.0.1:8082",
			headers: map[string]string{"X-Forwarded-Proto": "https", "X-Forwarded-Host": "traceway.acme.test"},
			want:    "https://traceway.acme.test",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withAppBaseURL(t, tc.appBaseURL)
			c, _ := newControllerTestContext(t, nil, 0, http.MethodGet, "/api/projects", "")
			c.Request.Host = tc.host
			for k, v := range tc.headers {
				c.Request.Header.Set(k, v)
			}
			if got := instanceUrl(c); got != tc.want {
				t.Errorf("instanceUrl = %q, want %q", got, tc.want)
			}

			projects := []*models.ProjectWithBackendUrl{{BackendUrl: "https://cloud.tracewayapp.com"}, nil}
			withInstanceUrl(c, projects...)
			if projects[0].BackendUrl != tc.want {
				t.Errorf("withInstanceUrl set %q, want %q", projects[0].BackendUrl, tc.want)
			}
		})
	}
}

func TestSetupSessionNeverHandsASelfHostedServerTheCloudUrl(t *testing.T) {
	setupSetupControllerDB(t)
	withAppBaseURL(t, "")

	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	userId, orgId := createSetupTestAccount(t, tx, "selfhosted@example.com", "owner")

	c, recorder := newControllerTestContext(t, tx, userId, http.MethodGet, "/api/setup/session", "")
	c.Request.Host = "traceway.internal:8082"
	c.Set(middleware.SetupOrganizationIdContextKey, orgId)
	SetupController.GetSession(c)
	if recorder.Code != http.StatusOK {
		t.Fatalf("session: status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var session SetupSessionResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &session); err != nil {
		t.Fatalf("parse session: %v", err)
	}
	if session.BackendUrl != "http://traceway.internal:8082" {
		t.Errorf("backendUrl = %q, want the server the request reached", session.BackendUrl)
	}
}

func TestBatchCreateGivesEveryArtifactPlatformAnUploadToken(t *testing.T) {
	setupSetupControllerDB(t)

	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	userId, orgId := createSetupTestAccount(t, tx, "upload@example.com", "owner")

	want := map[string]bool{
		"react": true, "svelte": true, "vuejs": true, "jquery": true,
		"flutter": true, "android": true, "ios": true,
		"opentelemetry": false, "react-native": false,
	}
	inputs := []BatchProjectInput{}
	for framework := range want {
		inputs = append(inputs, BatchProjectInput{Name: "App " + framework, Framework: framework})
	}
	results, err := batchCreateProjects(tx, orgId, userId, inputs)
	if err != nil {
		t.Fatalf("batch create: %v", err)
	}
	for _, r := range results {
		has := r.Project.SourceMapToken != nil && *r.Project.SourceMapToken != ""
		if has != want[r.Project.Framework] {
			t.Errorf("%s: has upload token = %v, want %v", r.Project.Framework, has, want[r.Project.Framework])
		}
		if has && *r.Project.SourceMapToken == r.Project.Token {
			t.Errorf("%s: the upload token must differ from the ingest token", r.Project.Framework)
		}
	}
}

func TestBatchCreateAddsAnUploadTokenToAnExistingProjectOnlyOnce(t *testing.T) {
	setupSetupControllerDB(t)

	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	userId, orgId := createSetupTestAccount(t, tx, "existing@example.com", "owner")

	old, err := transactional.ProjectRepository.CreateWithOrganization(tx, "Old Web", "react", orgId)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if _, err := tx.Exec("UPDATE projects SET source_map_token = NULL WHERE name = ?", "Old Web"); err != nil {
		t.Fatalf("clear upload token: %v", err)
	}
	cachedOld := *old
	cachedOld.SourceMapToken = nil
	cache.ProjectCache.AddProject(&cachedOld)
	t.Cleanup(func() { cache.ProjectCache.RemoveProject(old.Id) })

	results, err := batchCreateProjects(tx, orgId, userId, []BatchProjectInput{{Name: "Old Web", Framework: "react"}})
	if err != nil {
		t.Fatalf("batch create: %v", err)
	}
	first := results[0]
	if first.Status != "existing" || first.Project.Id != old.Id {
		t.Fatalf("result = %+v, want the existing project", first)
	}
	if !first.UploadTokenAdded || first.Project.SourceMapToken == nil {
		t.Fatalf("existing project without an upload token should get one: %+v", first)
	}
	stored, err := transactional.ProjectRepository.FindById(tx, old.Id)
	if err != nil || stored == nil || stored.SourceMapToken == nil || *stored.SourceMapToken != *first.Project.SourceMapToken {
		t.Fatalf("stored upload token = %+v (err %v), want %q", stored, err, *first.Project.SourceMapToken)
	}

	rerun, err := batchCreateProjects(tx, orgId, userId, []BatchProjectInput{{Name: "Old Web", Framework: "react"}})
	if err != nil {
		t.Fatalf("rerun: %v", err)
	}
	if rerun[0].UploadTokenAdded || *rerun[0].Project.SourceMapToken != *first.Project.SourceMapToken {
		t.Errorf("a rerun must keep the upload token: %+v", rerun[0])
	}

	c, _ := newControllerTestContext(t, tx, userId, http.MethodPost, "/api/projects/batch", "")
	cacheCreatedProjectsOnCommit(c, results)
	hooks, _ := c.Get("txCommitHooks")
	commitHooks, _ := hooks.([]func())
	if len(commitHooks) == 0 {
		t.Fatal("no commit hook queued for the added upload token")
	}
	for _, fn := range commitHooks {
		fn()
	}
	if p := cache.ProjectCache.GetBySourceMapToken(*first.Project.SourceMapToken); p == nil || p.Id != old.Id {
		t.Error("the added upload token should resolve to the project after commit")
	}
}

func TestSubmitPlanStartApiFields(t *testing.T) {
	setupSetupControllerDB(t)

	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	userId, orgId := createSetupTestAccount(t, tx, "startapi@example.com", "owner")

	rejected := map[string]string{
		"dotted key outside a properties file": `{"projects":[{"name":"App","framework":"android","envFile":"app/.env","envVar":"traceway.token"}]}`,
		"bad url env var":                      `{"projects":[{"name":"App","framework":"react","envFile":".env","envVar":"VITE_TRACEWAY_TOKEN","urlEnvVar":"1URL"}]}`,
		"public upload token var":              `{"projects":[{"name":"App","framework":"react","envFile":".env","envVar":"VITE_TRACEWAY_TOKEN","uploadTokenEnvVar":"VITE_TRACEWAY_UPLOAD_TOKEN"}]}`,
		"next public upload token var":         `{"projects":[{"name":"App","framework":"react","uploadTokenEnvFile":".env","uploadTokenEnvVar":"next_public_upload"}]}`,
		"url var over the token var":           `{"projects":[{"name":"App","framework":"react","envFile":".env","envVar":"VITE_TRACEWAY_TOKEN","urlEnvVar":"VITE_TRACEWAY_TOKEN"}]}`,
		"upload token var over the token var":  `{"projects":[{"name":"App","framework":"ios","envFile":"Traceway.xcconfig","envVar":"TRACEWAY_TOKEN","uploadTokenEnvVar":"TRACEWAY_TOKEN"}]}`,
		"url var over the token var of a deployment": `{"projects":[{"name":"App","framework":"react","envVar":"VITE_TRACEWAY","urlEnvVar":"VITE_TRACEWAY",
			"deployment":{"platform":"Vercel","instructions":"vercel env add VITE_TRACEWAY <token>"}}]}`,
	}
	for name, body := range rejected {
		t.Run(name, func(t *testing.T) {
			c, recorder := newControllerTestContext(t, tx, userId, http.MethodPut, "/api/setup/plan", body)
			c.Set(middleware.SetupOrganizationIdContextKey, orgId)
			SetupController.SubmitPlan(c)
			if recorder.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want 422 (body: %s)", recorder.Code, recorder.Body.String())
			}
		})
	}

	submitTestPlan(t, tx, userId, orgId, `{"projects":[
		{"name":"Mobile Android","framework":"android","envFile":"local.properties","envVar":"traceway.token","urlEnvVar":"traceway.url",
		 "uploadTokenEnvVar":"TRACEWAY_UPLOAD_TOKEN"},
		{"name":"Mobile iOS","framework":"ios","envFile":"ios/Traceway.xcconfig","envVar":"TRACEWAY_TOKEN",
		 "uploadTokenEnvFile":"ios/Traceway.secrets.xcconfig"}
	]}`)

	c, recorder := newControllerTestContext(t, tx, userId, http.MethodGet, "/api/setup/drafts?organizationId="+strconv.Itoa(orgId), "")
	SetupController.ListDraft(c)
	body := recorder.Body.String()
	for _, want := range []string{`"urlEnvVar":"traceway.url"`, `"uploadTokenEnvVar":"TRACEWAY_UPLOAD_TOKEN"`, `"uploadTokenEnvFile":"ios/Traceway.secrets.xcconfig"`} {
		if !strings.Contains(body, want) {
			t.Errorf("draft should keep %s: %s", want, body)
		}
	}

	draftId := latestDraftId(t, tx, userId, orgId)
	c, recorder = newControllerTestContext(t, tx, userId, http.MethodPost, "/api/setup/drafts/"+draftId+"/approve", "")
	c.Request.Host = "traceway.internal"
	c.Params = gin.Params{{Key: "id", Value: draftId}}
	withAppBaseURL(t, "")
	SetupController.ApproveDraft(c)
	if recorder.Code != http.StatusOK {
		t.Fatalf("approve: status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var approved struct {
		Projects []BatchProjectResponseItem `json:"projects"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &approved); err != nil {
		t.Fatalf("parse approve: %v", err)
	}
	for _, p := range approved.Projects {
		if p.SourceMapToken == nil || *p.SourceMapToken == "" {
			t.Errorf("%s: approved mobile project has no upload token", p.Name)
		}
		if p.BackendUrl != "http://traceway.internal" {
			t.Errorf("%s: backendUrl = %q, want the server the request reached", p.Name, p.BackendUrl)
		}
	}
}

func TestSetupPlanRejectsInferredFileKeyCollisions(t *testing.T) {
	for _, projects := range [][]models.SetupPlanProject{
		{{Name: "Web", Framework: "react", UploadTokenEnvFile: ".env.upload", UploadTokenEnvVar: "TRACEWAY_URL"}},
		{{Name: "Web", Framework: "react", EnvFile: ".env", EnvVar: "TRACEWAY_URL", UploadTokenEnvFile: "./.env"}},
		{{Name: "Web", Framework: "react", EnvFile: ".env", EnvVar: "VITE_TOKEN"}, {Name: "Other", Framework: "ios", EnvFile: "./.env", EnvVar: "VITE_URL"}},
	} {
		if msg := validateSetupPlanPayload(&models.SetupPlanPayload{Projects: projects}); msg == "" {
			t.Fatalf("accepted conflicting plan %+v", projects)
		}
	}
}

func TestSetupPlanReactNativeDefaultsToAConnectionString(t *testing.T) {
	reactNative := models.SetupPlanProject{Name: "App", Framework: "react-native", EnvFile: ".env", EnvVar: "EXPO_PUBLIC_TRACEWAY_CONNECTION_STRING"}
	if keys := setupFileKeys(&reactNative); len(keys) != 1 || keys[0].key != reactNative.EnvVar || keys[0].serverURL {
		t.Errorf("keys = %+v, want the connection string alone: the apply step writes no URL variable for React Native", keys)
	}
	// The name another project's default URL variable would take stays free.
	other := models.SetupPlanProject{Name: "Api", Framework: "opentelemetry", EnvFile: ".env", EnvVar: "EXPO_PUBLIC_TRACEWAY_CONNECTION_STRING_URL"}
	if msg := validateSetupPlanPayload(&models.SetupPlanPayload{Projects: []models.SetupPlanProject{reactNative, other}}); msg != "" {
		t.Errorf("plan rejected: %s", msg)
	}

	token := reactNative
	token.EnvFormat = setupEnvFormatToken
	if keys := setupFileKeys(&token); len(keys) != 2 {
		t.Errorf("an explicit token format keeps the URL variable: %+v", keys)
	}
}

func TestSetupPlanAcceptsUnusedUrlEnvVar(t *testing.T) {
	// A connection string carries the server, so its urlEnvVar is never written.
	for _, p := range []models.SetupPlanProject{
		{Name: "Old", Framework: "react", EnvFile: ".env", EnvVar: "DSN", UrlEnvVar: "DSN", EnvFormat: setupEnvFormatConnectionString},
		{Name: "Old", Framework: "react", EnvVar: "DSN", UrlEnvVar: "DSN", EnvFormat: setupEnvFormatConnectionString},
	} {
		if msg := validateSetupPlanPayload(&models.SetupPlanPayload{Projects: []models.SetupPlanProject{p}}); msg != "" {
			t.Errorf("%+v rejected: %s", p, msg)
		}
	}
}
