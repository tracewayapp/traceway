package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/tracewayapp/traceway/cli/v2/internal/appdir"
	"github.com/tracewayapp/traceway/cli/v2/internal/connstr"
	"github.com/tracewayapp/traceway/cli/v2/internal/envfile"
	"github.com/tracewayapp/traceway/cli/v2/internal/exitcode"
	"github.com/tracewayapp/traceway/cli/v2/internal/output"
	"github.com/tracewayapp/traceway/cli/v2/pkg/client"
)

var setupPollInterval = 3 * time.Second

const (
	envFormatToken            = "token"
	envFormatConnectionString = "connectionString"
)

type setupPlan struct {
	URL      string                    `json:"url"`
	Projects []client.SetupPlanProject `json:"projects"`
}

var setupEnvVarRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// A Gradle local.properties or other .properties file also takes dotted keys (traceway.token).
var setupPropertiesKeyRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z0-9_]+)*$`)

// Bundlers ship variables with these prefixes to the browser, so an upload token never gets one.
var setupPublicEnvPrefixes = []string{"VITE_", "NEXT_PUBLIC_", "PUBLIC_", "REACT_APP_", "EXPO_PUBLIC_", "NUXT_PUBLIC_", "VUE_APP_", "GATSBY_"}

func validSetupEnvVar(name, file string) bool {
	if strings.HasSuffix(strings.ToLower(file), ".properties") {
		return setupPropertiesKeyRegex.MatchString(name)
	}
	return setupEnvVarRegex.MatchString(name)
}

func hasPublicEnvPrefix(name string) bool {
	upper := strings.ToUpper(name)
	for _, prefix := range setupPublicEnvPrefixes {
		if strings.HasPrefix(upper, prefix) {
			return true
		}
	}
	return false
}

func loadSetupPlan(path string) (*setupPlan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading plan file: %w", err)
	}
	var plan setupPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("parsing plan file: %w", err)
	}
	return &plan, nil
}

func setupFilePath(planDir, file string) string {
	if filepath.IsAbs(file) {
		return filepath.Clean(file)
	}
	return filepath.Join(planDir, file)
}

func validateSetupPlan(plan *setupPlan, planDir string) error {
	if len(plan.Projects) == 0 {
		return errors.New("the plan has no projects")
	}
	if len(plan.Projects) > 20 {
		return errors.New("the plan has more than 20 projects")
	}
	files := map[[2]string]bool{} // true for a server URL, which projects may share
	for i, p := range plan.Projects {
		if strings.TrimSpace(p.Name) == "" {
			return fmt.Errorf("projects[%d] has no name", i)
		}
		if strings.TrimSpace(p.Framework) == "" {
			return fmt.Errorf("project %q has no framework", p.Name)
		}
		if p.EnvFile != "" && p.EnvVar == "" {
			return fmt.Errorf("project %q sets envFile without envVar", p.Name)
		}
		if p.EnvVar != "" && !validSetupEnvVar(p.EnvVar, p.EnvFile) {
			return fmt.Errorf("project %q has an invalid envVar %q", p.Name, p.EnvVar)
		}
		if p.URLEnvVar != "" && !validSetupEnvVar(p.URLEnvVar, p.EnvFile) {
			return fmt.Errorf("project %q has an invalid urlEnvVar %q", p.Name, p.URLEnvVar)
		}
		uploadFile := uploadTokenFile(p)
		if (p.UploadTokenEnvVar != "" || p.UploadTokenEnvFile != "") && uploadFile == "" {
			return fmt.Errorf("project %q sets uploadTokenEnvVar without uploadTokenEnvFile or envFile", p.Name)
		}
		if p.UploadTokenEnvVar != "" && !validSetupEnvVar(p.UploadTokenEnvVar, uploadFile) {
			return fmt.Errorf("project %q has an invalid uploadTokenEnvVar %q", p.Name, p.UploadTokenEnvVar)
		}
		if hasPublicEnvPrefix(p.UploadTokenEnvVar) {
			return fmt.Errorf("project %q: uploadTokenEnvVar %q has a public prefix, which would ship the upload token with the app", p.Name, p.UploadTokenEnvVar)
		}
		switch p.EnvFormat {
		case "", envFormatToken, envFormatConnectionString:
		default:
			return fmt.Errorf("project %q has an invalid envFormat %q (use \"token\")", p.Name, p.EnvFormat)
		}
		runtime := setupEnvWrites(p, "", "")
		for _, w := range append(runtime, setupUploadTokenWrites(p, "upload", "", runtime)...) {
			slot := [2]string{setupFilePath(planDir, w.file), w.key}
			if isURL, exists := files[slot]; exists && (!isURL || !w.serverURL) {
				return fmt.Errorf("project %q writes conflicting values for %q in %s", p.Name, w.key, w.file)
			}
			files[slot] = w.serverURL
		}
	}
	return nil
}

func uploadTokenFile(p client.SetupPlanProject) string {
	if p.UploadTokenEnvFile != "" {
		return p.UploadTokenEnvFile
	}
	if p.UploadTokenEnvVar != "" {
		return p.EnvFile
	}
	return ""
}

func resolveSetupToken(cmd *cobra.Command) (string, error) {
	if setupApplyToken != "" && setupApplyTokenStdin {
		return "", errors.New("choose one of --token or --token-stdin, not both")
	}
	if setupApplyToken != "" {
		return strings.TrimSpace(setupApplyToken), nil
	}
	if setupApplyTokenStdin {
		return readTokenStdin(cmd.InOrStdin())
	}
	return strings.TrimSpace(os.Getenv("TRACEWAY_SETUP_TOKEN")), nil
}

func renderSetupTokenError(errOut io.Writer, mode output.Mode) error {
	env := output.ErrorEnvelope{
		Code:     "invalid_setup_token",
		Message:  "the setup token is invalid or expired",
		Hint:     "generate a new setup token on the Traceway dashboard's /setup?mode=add page and retry",
		ExitCode: exitcode.Auth,
	}
	_ = output.RenderError(errOut, mode, env)
	return newCLIError(env.ExitCode, env.Code)
}

func renderSetupAPIError(errOut io.Writer, mode output.Mode, err error) error {
	if errors.Is(err, client.ErrUnauthorized) {
		return renderSetupTokenError(errOut, mode)
	}
	return renderAPIError(errOut, mode, err, false)
}

// setupInstanceURL is the server the app sends to: the one the server names, unless that is the cloud default of
// a server without APP_BASE_URL (older self-hosted servers report it) while this command talks to another one. A
// server without APP_BASE_URL names itself from the request as scheme://host, which loses a path prefix and the
// https of a proxy in front of it, so for the same host the URL this command just reached it at is kept, unless
// that would turn the server's https into http.
func setupInstanceURL(serverURL, cliURL string) string {
	server, serverErr := connstr.NormalizeBaseURL(serverURL)
	if serverErr != nil {
		return cliURL
	}
	if connstr.IsCloud(server) && !connstr.IsCloud(cliURL) {
		return cliURL
	}
	if connstr.SameHost(server, cliURL) && !(strings.HasPrefix(server, "https://") && strings.HasPrefix(cliURL, "http://")) {
		return cliURL
	}
	return server
}

// setupEnvFormat is the form the token variable takes. @tracewayapp/react-native starts only from a connection
// string, so a React Native entry gets one without asking.
func setupEnvFormat(p client.SetupPlanProject) string {
	if p.EnvFormat == "" && p.Framework == "react-native" {
		return envFormatConnectionString
	}
	return p.EnvFormat
}

// urlEnvVarFor names the server URL variable after the token one: VITE_TRACEWAY_TOKEN → VITE_TRACEWAY_URL,
// traceway.token → traceway.url.
func urlEnvVarFor(p client.SetupPlanProject) string {
	if p.URLEnvVar != "" {
		return p.URLEnvVar
	}
	name := p.EnvVar
	switch {
	case strings.HasSuffix(name, "TOKEN"):
		return strings.TrimSuffix(name, "TOKEN") + "URL"
	case strings.HasSuffix(name, "token"):
		return strings.TrimSuffix(name, "token") + "url"
	case strings.Contains(name, "."):
		return name + ".url"
	default:
		return name + "_URL"
	}
}

// The browser source map uploader reads TRACEWAY_SOURCEMAP_TOKEN; the Flutter, Android and iOS tools read
// TRACEWAY_UPLOAD_TOKEN.
func uploadTokenEnvVarFor(p client.SetupPlanProject) string {
	if p.UploadTokenEnvVar != "" {
		return p.UploadTokenEnvVar
	}
	switch p.Framework {
	case "react", "svelte", "vuejs", "jquery", "nextjs":
		return "TRACEWAY_SOURCEMAP_TOKEN"
	default:
		return "TRACEWAY_UPLOAD_TOKEN"
	}
}

type setupEnvWrite struct {
	file          string
	key           string
	value         string
	serverURL     bool
	onlyIfPresent bool
	result        envfile.Result
	err           error
	equivalent    func(existing string) bool
}

func sameBaseURL(want string) func(string) bool {
	return func(existing string) bool {
		got, err := connstr.NormalizeBaseURL(existing)
		return err == nil && got == want
	}
}

func setupServerURL(file, key, instance string) setupEnvWrite {
	return setupEnvWrite{file: file, key: key, value: instance, serverURL: true, onlyIfPresent: connstr.IsCloud(instance), equivalent: sameBaseURL(instance)}
}

func setupEnvWrites(p client.SetupPlanProject, token, instance string) []setupEnvWrite {
	if p.EnvFile == "" {
		return nil
	}
	if setupEnvFormat(p) == envFormatConnectionString {
		value := token + "@" + instance + "/api/report"
		return []setupEnvWrite{{file: p.EnvFile, key: p.EnvVar, value: value, equivalent: func(existing string) bool { return existing == value }}}
	}
	return []setupEnvWrite{{file: p.EnvFile, key: p.EnvVar, value: token}, setupServerURL(p.EnvFile, urlEnvVarFor(p), instance)}
}

// setupUploadTokenWrites lists the upload token and, for a server other than Traceway Cloud, the TRACEWAY_URL the
// upload tools read, when the plan asks for the upload token.
func setupUploadTokenWrites(p client.SetupPlanProject, uploadToken, instance string, envWrites []setupEnvWrite) []setupEnvWrite {
	file := uploadTokenFile(p)
	if uploadToken == "" || file == "" {
		return nil
	}
	writes := []setupEnvWrite{{file: file, key: uploadTokenEnvVarFor(p), value: uploadToken}}
	urlKey := "TRACEWAY_URL"
	if strings.HasSuffix(strings.ToLower(file), ".properties") && strings.Contains(p.EnvVar, ".") {
		urlKey = urlEnvVarFor(p)
	}
	for _, w := range envWrites {
		if w.serverURL && filepath.Clean(w.file) == filepath.Clean(file) && w.key == urlKey {
			return writes
		}
	}
	return append(writes, setupServerURL(file, urlKey, instance))
}

var envResultRank = map[envfile.Result]int{envfile.ResultUnchanged: 0, envfile.ResultUpdated: 1, envfile.ResultAdded: 2, envfile.ResultCreated: 3}

// Commit credentials and their destinations together, including when the runtime
// and uploader share a file. An error in one file does not prevent another from being written.
func applyEnvWrites(planDir string, groups ...[]setupEnvWrite) error {
	files := map[string][]*setupEnvWrite{}
	for _, group := range groups {
		for i := range group {
			w := &group[i]
			path := setupFilePath(planDir, w.file)
			files[path] = append(files[path], w)
		}
	}
	var failures []error
	for path, writes := range files {
		err := envfile.Edit(path, func(file *envfile.File) error {
			for _, w := range writes {
				if w.onlyIfPresent && !file.Has(w.key) {
					continue
				}
				result, err := file.Set(w.key, w.value, w.equivalent)
				if err != nil {
					return err
				}
				w.result = result
			}
			return nil
		})
		if err != nil {
			failures = append(failures, err)
			for _, w := range writes {
				w.err = err
			}
		}
	}
	return errors.Join(failures...)
}

func setupWriteResult(writes []setupEnvWrite) string {
	combined := envfile.ResultUnchanged
	for _, w := range writes {
		if w.err != nil {
			return "error: " + w.err.Error()
		}
		if envResultRank[w.result] > envResultRank[combined] {
			combined = w.result
		}
	}
	return string(combined)
}

type setupApplyProjectSummary struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Framework          string `json:"framework"`
	Status             string `json:"status"`
	EnvFile            string `json:"envFile,omitempty"`
	EnvVar             string `json:"envVar,omitempty"`
	URLEnvVar          string `json:"urlEnvVar,omitempty"`
	EnvResult          string `json:"envResult,omitempty"`
	HasUploadToken     bool   `json:"hasUploadToken"`
	UploadTokenEnvFile string `json:"uploadTokenEnvFile,omitempty"`
	UploadTokenEnvVar  string `json:"uploadTokenEnvVar,omitempty"`
	UploadTokenResult  string `json:"uploadTokenResult,omitempty"`
	DashboardURL       string `json:"dashboardUrl"`
	ConnectionURL      string `json:"connectionUrl"`
	DeploymentPlatform string `json:"deploymentPlatform,omitempty"`
}

func runSetupApply(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())

	if setupApplyPlanPath == "" {
		return renderUsageError(cmd.ErrOrStderr(), mode, "no plan file provided", appdir.Binary+" setup apply --plan <file> --token-stdin")
	}
	plan, err := loadSetupPlan(setupApplyPlanPath)
	if err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(), appdir.Binary+" setup apply --help")
	}
	absPlanPath, err := filepath.Abs(setupApplyPlanPath)
	if err != nil {
		return err
	}
	planDir := filepath.Dir(absPlanPath)
	if err := validateSetupPlan(plan, planDir); err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(), appdir.Binary+" setup apply --help")
	}

	token, err := resolveSetupToken(cmd)
	if err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(), appdir.Binary+" setup apply --help")
	}
	if token == "" {
		return renderUsageError(cmd.ErrOrStderr(), mode, "no setup token provided", appdir.Binary+" setup apply --plan <file> --token-stdin (or set TRACEWAY_SETUP_TOKEN)")
	}

	url := strings.TrimSpace(setupApplyURL)
	if url == "" {
		url = strings.TrimSpace(plan.URL)
	}
	if url == "" {
		return renderUsageError(cmd.ErrOrStderr(), mode, "no Traceway URL provided", "pass --url or add \"url\" to the plan file")
	}
	url, err = connstr.NormalizeBaseURL(url)
	if err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, "invalid Traceway URL: "+err.Error(), "pass the server's address, such as "+connstr.CloudURL)
	}

	c := client.New(url, client.WithJWT(token))

	session, err := c.GetSetupSession(ctx)
	if err != nil {
		return renderSetupAPIError(cmd.ErrOrStderr(), mode, err)
	}
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Applying plan to organization %q on %s as %s\n", session.OrganizationName, url, session.Email)

	if err := c.SubmitSetupPlan(ctx, plan.Projects); err != nil {
		return renderSetupAPIError(cmd.ErrOrStderr(), mode, err)
	}
	setupPage := setupPageURL(url, session.OrganizationID)
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Plan submitted. Ask the user to review and approve it at %s (it also appears on the registration page if they still have it open).\nWaiting for approval...\n", setupPage)

	projects, err := waitForSetupDecision(ctx, cmd.ErrOrStderr(), mode, c)
	if err != nil {
		return err
	}

	byName := make(map[string]client.SetupProject, len(projects))
	for _, p := range projects {
		byName[p.Name] = p
	}

	summaries := make([]setupApplyProjectSummary, 0, len(plan.Projects))
	var mobileURLs []mobileServerURL
	deploymentURLVars := map[string]string{}
	envWriteFailed := false
	projectMissing := false
	for _, planned := range plan.Projects {
		project, ok := byName[strings.TrimSpace(planned.Name)]
		if !ok {
			summaries = append(summaries, setupApplyProjectSummary{Name: planned.Name, Framework: planned.Framework, Status: "missing"})
			projectMissing = true
			continue
		}

		summary := setupApplyProjectSummary{
			ID:             project.ID,
			Name:           project.Name,
			Framework:      project.Framework,
			Status:         project.Status,
			HasUploadToken: project.SourceMapToken != "",
			DashboardURL:   url + "/?projectId=" + project.ID,
			ConnectionURL:  url + "/connection?projectId=" + project.ID,
		}
		instance := setupInstanceURL(cmp.Or(project.BackendURL, session.BackendURL), url)
		if planned.Deployment != nil {
			summary.DeploymentPlatform = planned.Deployment.Platform
			if planned.EnvVar != "" && setupEnvFormat(planned) != envFormatConnectionString && !connstr.IsCloud(instance) {
				deploymentURLVars[planned.Name] = urlEnvVarFor(planned)
			}
		}
		envWrites := setupEnvWrites(planned, project.Token, instance)
		uploadWrites := setupUploadTokenWrites(planned, project.SourceMapToken, instance, envWrites)
		if err := applyEnvWrites(planDir, envWrites, uploadWrites); err != nil {
			envWriteFailed = true
		}
		if len(envWrites) > 0 {
			summary.EnvFile = planned.EnvFile
			summary.EnvVar = planned.EnvVar
			summary.EnvResult = setupWriteResult(envWrites)
			if len(envWrites) > 1 && envWrites[1].result != "" && envWrites[1].err == nil {
				summary.URLEnvVar = envWrites[1].key
			}
			if envWrites[0].err == nil && mobileFrameworks[project.Framework] && writesServerURL(planned, envWrites) {
				mobileURLs = append(mobileURLs, mobileServerURL{project: project.Name, url: instance})
			}
		}
		if len(uploadWrites) > 0 {
			summary.UploadTokenEnvFile = uploadWrites[0].file
			summary.UploadTokenEnvVar = uploadWrites[0].key
			summary.UploadTokenResult = setupWriteResult(uploadWrites)
		}
		summaries = append(summaries, summary)
	}

	if err := renderSetupApplySummary(cmd.OutOrStdout(), mode, summaries, plan.Projects, deploymentURLVars, setupPage); err != nil {
		return err
	}
	renderLocalServerNote(cmd.ErrOrStderr(), mobileURLs)
	if envWriteFailed {
		return newCLIError(exitcode.Generic, "env_write_failed")
	}
	if projectMissing {
		env := output.ErrorEnvelope{
			Code:     "setup_project_missing",
			Message:  "one or more approved projects could not be loaded",
			Hint:     "review the setup in Traceway and rerun the plan to restore the missing project",
			ExitCode: exitcode.Generic,
		}
		_ = output.RenderError(cmd.ErrOrStderr(), mode, env)
		return newCLIError(env.ExitCode, env.Code)
	}
	return nil
}

func waitForSetupDecision(ctx context.Context, errOut io.Writer, mode output.Mode, c *client.Client) ([]client.SetupProject, error) {
	deadline := time.Now().Add(setupApplyWaitTimeout)
	for {
		status, err := c.GetSetupPlan(ctx)
		if err != nil {
			return nil, renderSetupAPIError(errOut, mode, err)
		}
		switch status.Status {
		case "approved":
			return status.Projects, nil
		case "rejected":
			message := "the user rejected the setup plan"
			if status.Reason != "" {
				message += ": " + status.Reason
			}
			env := output.ErrorEnvelope{
				Code:     "setup_plan_rejected",
				Message:  message,
				Hint:     "revise the plan with the user and run setup apply again",
				ExitCode: exitcode.Generic,
			}
			_ = output.RenderError(errOut, mode, env)
			return nil, newCLIError(env.ExitCode, env.Code)
		}

		if time.Now().After(deadline) {
			env := output.ErrorEnvelope{
				Code:     "setup_approval_timeout",
				Message:  "timed out waiting for the plan to be approved",
				Hint:     "run setup apply again once the user is ready; re-submitting replaces the pending draft",
				ExitCode: exitcode.Generic,
			}
			_ = output.RenderError(errOut, mode, env)
			return nil, newCLIError(env.ExitCode, env.Code)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(setupPollInterval):
		}
	}
}

// deploymentURLVars names, per project, the server URL variable a deployment needs next to the token: off Traceway
// Cloud an app that gets only the token would send to Cloud.
func renderSetupApplySummary(out io.Writer, mode output.Mode, summaries []setupApplyProjectSummary, planned []client.SetupPlanProject, deploymentURLVars map[string]string, setupPage string) error {
	switch mode {
	case output.ModeJSON:
		return output.RenderJSON(out, map[string]any{"projects": summaries}, output.ParseFieldsFlag(flagFields))
	case output.ModeYAML:
		return output.RenderYAML(out, map[string]any{"projects": summaries}, output.ParseFieldsFlag(flagFields))
	}

	tw := output.NewTabWriter(out)
	_, _ = fmt.Fprintln(tw, "NAME\tFRAMEWORK\tSTATUS\tENV\tUPLOAD TOKEN\tDASHBOARD")
	for _, s := range summaries {
		env := "-"
		if s.EnvFile != "" {
			vars := s.EnvVar
			if s.URLEnvVar != "" {
				vars += ", " + s.URLEnvVar
			}
			env = fmt.Sprintf("%s (%s: %s)", s.EnvFile, vars, s.EnvResult)
		}
		upload := "-"
		if s.UploadTokenEnvFile != "" {
			upload = fmt.Sprintf("%s (%s: %s)", s.UploadTokenEnvFile, s.UploadTokenEnvVar, s.UploadTokenResult)
		} else if s.HasUploadToken {
			upload = "on the Connection page"
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", s.Name, s.Framework, s.Status, env, upload, s.DashboardURL)
	}
	if err := tw.Flush(); err != nil {
		return err
	}

	withDeployment := []client.SetupPlanProject{}
	for _, p := range planned {
		if p.Deployment != nil {
			withDeployment = append(withDeployment, p)
		}
	}
	if len(withDeployment) > 0 {
		_, _ = fmt.Fprintln(out, "\nNext steps (deployment credentials):")
		for _, p := range withDeployment {
			credential := p.EnvVar
			if credential == "" {
				credential = "its Traceway credential"
			} else if urlVar := deploymentURLVars[p.Name]; urlVar != "" {
				credential += " and " + urlVar + " (the server URL)"
			}
			_, _ = fmt.Fprintf(out, "  - %s: add %s to %s. The exact command with the secret value is on %s.\n", p.Name, credential, p.Deployment.Platform, setupPage)
		}
	}
	return nil
}
