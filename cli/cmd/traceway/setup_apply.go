package main

import (
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

	"github.com/tracewayapp/traceway/cli/internal/envfile"
	"github.com/tracewayapp/traceway/cli/internal/exitcode"
	"github.com/tracewayapp/traceway/cli/internal/output"
	"github.com/tracewayapp/traceway/cli/pkg/client"
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

func validateSetupPlan(plan *setupPlan) error {
	if len(plan.Projects) == 0 {
		return errors.New("the plan has no projects")
	}
	if len(plan.Projects) > 20 {
		return errors.New("the plan has more than 20 projects")
	}
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
		if p.EnvVar != "" && !setupEnvVarRegex.MatchString(p.EnvVar) {
			return fmt.Errorf("project %q has an invalid envVar %q", p.Name, p.EnvVar)
		}
		switch p.EnvFormat {
		case "", envFormatToken, envFormatConnectionString:
		default:
			return fmt.Errorf("project %q has an invalid envFormat %q (use \"token\" or \"connectionString\")", p.Name, p.EnvFormat)
		}
	}
	return nil
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
		Hint:     "generate a new setup token on the Traceway dashboard's /setup page and retry",
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

func setupEnvValue(project client.SetupProject, format, fallbackURL string) string {
	if format == envFormatConnectionString {
		base := strings.TrimRight(project.BackendURL, "/")
		if base == "" {
			base = strings.TrimRight(fallbackURL, "/")
		}
		return project.Token + "@" + base + "/api/report"
	}
	return project.Token
}

type setupApplyProjectSummary struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Framework          string `json:"framework"`
	Status             string `json:"status"`
	EnvFile            string `json:"envFile,omitempty"`
	EnvVar             string `json:"envVar,omitempty"`
	EnvResult          string `json:"envResult,omitempty"`
	DashboardURL       string `json:"dashboardUrl"`
	DeploymentPlatform string `json:"deploymentPlatform,omitempty"`
}

func runSetupApply(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())

	if setupApplyPlanPath == "" {
		return renderUsageError(cmd.ErrOrStderr(), mode, "no plan file provided", "traceway setup apply --plan <file> --token-stdin")
	}
	plan, err := loadSetupPlan(setupApplyPlanPath)
	if err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(), "traceway setup apply --help")
	}
	if err := validateSetupPlan(plan); err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(), "traceway setup apply --help")
	}

	token, err := resolveSetupToken(cmd)
	if err != nil {
		return renderUsageError(cmd.ErrOrStderr(), mode, err.Error(), "traceway setup apply --help")
	}
	if token == "" {
		return renderUsageError(cmd.ErrOrStderr(), mode, "no setup token provided", "traceway setup apply --plan <file> --token-stdin (or set TRACEWAY_SETUP_TOKEN)")
	}

	url := strings.TrimSpace(setupApplyURL)
	if url == "" {
		url = strings.TrimSpace(plan.URL)
	}
	if url == "" {
		return renderUsageError(cmd.ErrOrStderr(), mode, "no Traceway URL provided", "pass --url or add \"url\" to the plan file")
	}
	url = strings.TrimRight(url, "/")

	c := client.New(url, client.WithJWT(token))

	session, err := c.GetSetupSession(ctx)
	if err != nil {
		return renderSetupAPIError(cmd.ErrOrStderr(), mode, err)
	}
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Applying plan to organization %q on %s as %s\n", session.OrganizationName, url, session.Email)

	if err := c.SubmitSetupPlan(ctx, plan.Projects); err != nil {
		return renderSetupAPIError(cmd.ErrOrStderr(), mode, err)
	}
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Plan submitted. Ask the user to review and approve it at %s/setup (it also appears on the registration page if they still have it open).\nWaiting for approval...\n", url)

	projects, err := waitForSetupDecision(ctx, cmd.ErrOrStderr(), mode, c)
	if err != nil {
		return err
	}

	byName := make(map[string]client.SetupProject, len(projects))
	for _, p := range projects {
		byName[p.Name] = p
	}

	planDir := filepath.Dir(setupApplyPlanPath)
	summaries := make([]setupApplyProjectSummary, 0, len(plan.Projects))
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
			ID:           project.ID,
			Name:         project.Name,
			Framework:    project.Framework,
			Status:       project.Status,
			DashboardURL: url + "/?projectId=" + project.ID,
		}
		if planned.Deployment != nil {
			summary.DeploymentPlatform = planned.Deployment.Platform
		}
		if planned.EnvFile != "" {
			envPath := planned.EnvFile
			if !filepath.IsAbs(envPath) {
				envPath = filepath.Join(planDir, envPath)
			}
			summary.EnvFile = planned.EnvFile
			summary.EnvVar = planned.EnvVar
			result, err := envfile.Upsert(envPath, planned.EnvVar, setupEnvValue(project, planned.EnvFormat, url))
			if err != nil {
				summary.EnvResult = "error: " + err.Error()
				envWriteFailed = true
			} else {
				summary.EnvResult = string(result)
			}
		}
		summaries = append(summaries, summary)
	}

	if err := renderSetupApplySummary(cmd.OutOrStdout(), mode, summaries, plan.Projects, url); err != nil {
		return err
	}
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

func renderSetupApplySummary(out io.Writer, mode output.Mode, summaries []setupApplyProjectSummary, planned []client.SetupPlanProject, url string) error {
	switch mode {
	case output.ModeJSON:
		return output.RenderJSON(out, map[string]any{"projects": summaries}, output.ParseFieldsFlag(flagFields))
	case output.ModeYAML:
		return output.RenderYAML(out, map[string]any{"projects": summaries}, output.ParseFieldsFlag(flagFields))
	}

	tw := output.NewTabWriter(out)
	_, _ = fmt.Fprintln(tw, "NAME\tFRAMEWORK\tSTATUS\tENV\tDASHBOARD")
	for _, s := range summaries {
		env := "-"
		if s.EnvFile != "" {
			env = fmt.Sprintf("%s (%s, %s)", s.EnvFile, s.EnvVar, s.EnvResult)
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", s.Name, s.Framework, s.Status, env, s.DashboardURL)
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
			}
			_, _ = fmt.Fprintf(out, "  - %s: add %s to %s. The exact command with the secret value is on %s/setup.\n", p.Name, credential, p.Deployment.Platform, url)
		}
	}
	return nil
}
