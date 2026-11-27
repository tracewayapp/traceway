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
			"approves it in the Traceway dashboard (the /setup?mode=add page, or step 2\n" +
			"of registration). This command waits for that decision, then writes each\n" +
			"project's token into the env file named by the plan. Token values are\n" +
			"never printed.\n\n" +
			"Plan file format (JSON):\n\n" +
			"  {\n" +
			"    \"url\": \"https://cloud.tracewayapp.com\",\n" +
			"    \"projects\": [\n" +
			"      {\"name\": \"Product Backend\", \"framework\": \"opentelemetry\",\n" +
			"       \"envFile\": \"backend/.env\", \"envVar\": \"TRACEWAY_TOKEN\"},\n" +
			"      {\"name\": \"Product Web\", \"framework\": \"svelte\",\n" +
			"       \"envFile\": \"web/.env\", \"envVar\": \"PUBLIC_TRACEWAY_TOKEN\",\n" +
			"       \"uploadTokenEnvFile\": \"web/.env.sourcemaps\",\n" +
			"       \"deployment\": {\"platform\": \"Vercel\", \"instructions\": \"vercel env add ... <token>\"}},\n" +
			"      {\"name\": \"Product iOS\", \"framework\": \"ios\",\n" +
			"       \"envFile\": \"ios/Traceway.xcconfig\", \"envVar\": \"TRACEWAY_TOKEN\"}\n" +
			"    ]\n" +
			"  }\n\n" +
			"envVar gets the project token. On a server other than Traceway Cloud the same\n" +
			"file also gets the server URL, under urlEnvVar (default: envVar with TOKEN\n" +
			"replaced by URL, so PUBLIC_TRACEWAY_TOKEN gets PUBLIC_TRACEWAY_URL); on Cloud the\n" +
			"SDKs need no URL. A .properties file (Android local.properties) also takes\n" +
			"dotted keys such as traceway.token, and an .xcconfig file stores \"//\" as\n" +
			"\"/$()/\" so Xcode does not read the rest of the line as a comment. Missing\n" +
			"directories are created.\n\n" +
			"Projects whose release builds upload artifacts (browser source maps, Flutter\n" +
			"symbols, Android mappings, iOS dSYMs) get an upload token. It is written only\n" +
			"when the plan names uploadTokenEnvFile or uploadTokenEnvVar (default\n" +
			"TRACEWAY_SOURCEMAP_TOKEN for browsers, TRACEWAY_UPLOAD_TOKEN otherwise; a public\n" +
			"prefix such as VITE_ is refused), together with TRACEWAY_URL off Cloud;\n" +
			"otherwise it stays on the project's Connection page.\n\n" +
			"envFormat \"connectionString\" (deprecated, from 1.x plans) writes\n" +
			"<token>@<server> instead of the token; an existing value naming the same token\n" +
			"and server in an older spelling (such as .../api/report) is left as it is.\n" +
			"Relative paths resolve against the plan file's directory. A deployment block\n" +
			"marks a credential that must live in the hosting platform instead; its exact\n" +
			"command (with the secret value) is shown on the dashboard's /setup?mode=add\n" +
			"page after approval. The setup token is short-lived; pass it via --token-stdin\n" +
			"or the TRACEWAY_SETUP_TOKEN env var rather than --token to keep it out of\n" +
			"shell history. Re-running is safe: a pending draft is replaced and approved\n" +
			"projects are matched by name instead of duplicated.\n\n" +
			"A mobile app (Flutter, Android, iOS) whose written server URL is localhost or\n" +
			"plain http:// gets a note on stderr: emulators and phones reach the computer\n" +
			"by another address, and both platforms block cleartext http by default.",
		RunE: runSetupApply,
	}
	cmd.Flags().StringVar(&setupApplyURL, "url", "", "Traceway base URL (default: the plan file's \"url\")")
	cmd.Flags().StringVar(&setupApplyToken, "token", "", "Setup token (tws_...); prefer --token-stdin or TRACEWAY_SETUP_TOKEN")
	cmd.Flags().BoolVar(&setupApplyTokenStdin, "token-stdin", false, "Read the setup token from stdin")
	cmd.Flags().StringVar(&setupApplyPlanPath, "plan", "", "Path to the setup plan JSON file (required)")
	cmd.Flags().DurationVar(&setupApplyWaitTimeout, "wait-timeout", 30*time.Minute, "How long to wait for the user to approve the plan")
	return cmd
}
