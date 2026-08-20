package main

import (
	"time"

	"github.com/spf13/cobra"
)

var (
	setupApplyURL         string
	setupApplyToken       string
	setupApplyTokenStdin  bool
	setupApplyPlanPath    string
	setupApplyWaitTimeout time.Duration
)

func newSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Provision Traceway projects from a setup plan",
	}
	cmd.AddCommand(newSetupApplyCmd())
	return cmd
}

func newSetupApplyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Submit a setup plan for approval and write the resulting tokens into env files",
		Long: "Submit a setup plan for approval and write the resulting tokens into env files.\n\n" +
			"The plan is proposed as a draft: nothing is created until a logged-in user\n" +
			"approves it in the Traceway dashboard (the /setup page, or step 2 of\n" +
			"registration). This command waits for that decision, then writes each\n" +
			"project's ingest token into the env file named by the plan. Token values are\n" +
			"never printed.\n\n" +
			"Plan file format (JSON):\n\n" +
			"  {\n" +
			"    \"url\": \"https://traceway.example.com\",\n" +
			"    \"projects\": [\n" +
			"      {\"name\": \"Product Backend\", \"framework\": \"opentelemetry\",\n" +
			"       \"envFile\": \"backend/.env\", \"envVar\": \"TRACEWAY_BACKEND_TOKEN\"},\n" +
			"      {\"name\": \"Product Web\", \"framework\": \"svelte\",\n" +
			"       \"envFile\": \"web/.env\", \"envVar\": \"PUBLIC_TRACEWAY_CONNECTION_STRING\",\n" +
			"       \"envFormat\": \"connectionString\",\n" +
			"       \"deployment\": {\"platform\": \"Vercel\", \"instructions\": \"vercel env add ... <token>\"}}\n" +
			"    ]\n" +
			"  }\n\n" +
			"envFormat \"token\" (default) writes the raw project token; \"connectionString\"\n" +
			"writes <token>@<instance>/api/report for the browser and mobile SDKs. Relative\n" +
			"envFile paths resolve against the plan file's directory. A deployment block\n" +
			"marks a credential that must live in the hosting platform instead; its exact\n" +
			"command (with the secret value) is shown on the dashboard's /setup page after\n" +
			"approval. The setup token is short-lived; pass it via --token-stdin or the\n" +
			"TRACEWAY_SETUP_TOKEN env var rather than --token to keep it out of shell\n" +
			"history. Re-running is safe: a pending draft is replaced and approved projects\n" +
			"are matched by name instead of duplicated.",
		RunE: runSetupApply,
	}
	cmd.Flags().StringVar(&setupApplyURL, "url", "", "Traceway base URL (default: the plan file's \"url\")")
	cmd.Flags().StringVar(&setupApplyToken, "token", "", "Setup token (tws_...); prefer --token-stdin or TRACEWAY_SETUP_TOKEN")
	cmd.Flags().BoolVar(&setupApplyTokenStdin, "token-stdin", false, "Read the setup token from stdin")
	cmd.Flags().StringVar(&setupApplyPlanPath, "plan", "", "Path to the setup plan JSON file (required)")
	cmd.Flags().DurationVar(&setupApplyWaitTimeout, "wait-timeout", 30*time.Minute, "How long to wait for the user to approve the plan")
	return cmd
}
