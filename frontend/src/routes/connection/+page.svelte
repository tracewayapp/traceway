<script lang="ts">
	import {
		Card,
		CardContent,
		CardDescription,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card';
	import { Button } from '$lib/components/ui/button';
	import { Copy, Check } from 'lucide-svelte';
	import {
		projectsState,
		type ProjectWithToken,
		isJsFramework,
		isOtelFramework,
		isCloudflareFramework
	} from '$lib/state/projects.svelte';
	import FrameworkIcon from '$lib/components/framework-icon.svelte';
	import Highlight from 'svelte-highlight';
	import go from 'svelte-highlight/languages/go';
	import javascript from 'svelte-highlight/languages/javascript';
	import bash from 'svelte-highlight/languages/bash';
	import php from 'svelte-highlight/languages/php';
	import python from 'svelte-highlight/languages/python';
	import { themeState } from '$lib/state/theme.svelte';
	import 'svelte-highlight/styles/github-dark.css';
	import {
		getFrameworkCode,
		getInstallCommand,
		getFrameworkLabel
	} from '$lib/utils/framework-code';
	import { getSetupMode } from '$lib/utils/setup-storage';
	import SetupModeTabs from '$lib/components/setup/setup-mode-tabs.svelte';
	import AiSetupSteps from '$lib/components/setup/ai-setup-steps.svelte';
	import OtelSetupSteps from '$lib/components/setup/otel-setup-steps.svelte';
	import OtelExporterConfig from '$lib/components/setup/otel-exporter-config.svelte';
	import SourceMapUploadCard from '$lib/components/setup/source-map-upload-card.svelte';

	let projectWithToken = $derived(projectsState.currentProject);
	let setupMode = $state(getSetupMode());
	let copiedCode = $state(false);
	let copiedInstall = $state(false);
	let copiedCfEndpoint = $state(false);
	let copiedCfAuth = $state(false);
	let copiedCfWrangler = $state(false);
	let copiedCfDeploy = $state(false);

	const isOtel = $derived(projectWithToken ? isOtelFramework(projectWithToken.framework) : false);
	const isOtelGeneric = $derived(projectWithToken?.framework === 'opentelemetry');
	const isCloudflare = $derived(
		projectWithToken ? isCloudflareFramework(projectWithToken.framework) : false
	);
	const otelEndpoint = $derived(projectWithToken ? `${projectWithToken.backendUrl}/api/otel` : '');
	const otelAuthHeader = $derived(projectWithToken ? `Bearer ${projectWithToken.token}` : '');
	const otelCollectorConfig = $derived(
		projectWithToken
			? `exporters:
  otlphttp:
    endpoint: "${projectWithToken.backendUrl}/api/otel"
    headers:
      Authorization: "Bearer ${projectWithToken.token}"

service:
  pipelines:
    traces:
      exporters: [otlphttp]
    metrics:
      exporters: [otlphttp]`
			: ''
	);
	const cloudflareOtelEndpoint = $derived(
		projectWithToken ? `${projectWithToken.backendUrl}/api/otel/v1/traces` : ''
	);
	const cloudflareAuthHeader = $derived(projectWithToken ? `Bearer ${projectWithToken.token}` : '');
	const wranglerConfig = $derived(
		projectWithToken
			? `{
  "observability": {
    "traces": {
      "enabled": true,
      "head_sample_rate": 1,
      "destinations": [
        {
          "name": "traceway",
          "type": "otlp"
        }
      ]
    }
  }
}`
			: ''
	);

	const sdkCode = $derived(
		projectWithToken
			? getFrameworkCode(
					projectWithToken.framework,
					projectWithToken.token,
					projectWithToken.backendUrl
				)
			: ''
	);

	const installCommand = $derived(
		projectWithToken ? getInstallCommand(projectWithToken.framework) : 'go get go.tracewayapp.com'
	);

	const highlightLanguage = $derived.by(() => {
		if (!projectWithToken) return go;
		if (projectWithToken.framework === 'symfony') return php;
		if (projectWithToken.framework === 'laravel') return php;
		if (projectWithToken.framework === 'django') return python;
		if (isJsFramework(projectWithToken.framework)) return javascript;
		return go;
	});

	const isJs = $derived(projectWithToken ? isJsFramework(projectWithToken.framework) : false);

	async function copyCode() {
		await navigator.clipboard.writeText(sdkCode);
		copiedCode = true;
		setTimeout(() => (copiedCode = false), 2000);
	}

	async function copyInstall() {
		await navigator.clipboard.writeText(installCommand);
		copiedInstall = true;
		setTimeout(() => (copiedInstall = false), 2000);
	}

	async function copyCfEndpoint() {
		await navigator.clipboard.writeText(cloudflareOtelEndpoint);
		copiedCfEndpoint = true;
		setTimeout(() => (copiedCfEndpoint = false), 2000);
	}

	async function copyCfAuth() {
		await navigator.clipboard.writeText(cloudflareAuthHeader);
		copiedCfAuth = true;
		setTimeout(() => (copiedCfAuth = false), 2000);
	}

	async function copyCfWrangler() {
		await navigator.clipboard.writeText(wranglerConfig);
		copiedCfWrangler = true;
		setTimeout(() => (copiedCfWrangler = false), 2000);
	}

	async function copyCfDeploy() {
		await navigator.clipboard.writeText('npx wrangler deploy');
		copiedCfDeploy = true;
		setTimeout(() => (copiedCfDeploy = false), 2000);
	}
</script>

<div class="space-y-4">
	<div>
		<h2 class="text-2xl font-bold tracking-tight">Connection</h2>
		<p class="text-muted-foreground">Connect your application to Traceway using the SDK</p>
	</div>

	{#if projectWithToken}
		<SetupModeTabs mode={setupMode} onModeChange={(m) => (setupMode = m)} />
		{#if setupMode === 'ai'}
			<AiSetupSteps backendUrl={projectWithToken.backendUrl} token={projectWithToken.token} />
		{:else if isOtelGeneric}
			<OtelSetupSteps backendUrl={projectWithToken.backendUrl} token={projectWithToken.token} />
		{:else if isOtel}
			<Card>
				<CardHeader>
					<CardTitle class="flex items-center gap-2">
						<FrameworkIcon framework={projectWithToken.framework} />
						Configure the OTLP Exporter
					</CardTitle>
					<CardDescription>
						Point your OTLP/HTTP exporter at Traceway using the endpoint and token below.
					</CardDescription>
				</CardHeader>
				<CardContent>
					<OtelExporterConfig
						endpoint={otelEndpoint}
						authHeader={otelAuthHeader}
						collectorConfig={otelCollectorConfig}
					/>
				</CardContent>
			</Card>
		{:else if isCloudflare}
			<Card>
				<CardHeader>
					<CardTitle class="flex items-center gap-2">
						<FrameworkIcon framework={projectWithToken.framework} />
						Cloudflare Workers Integration
					</CardTitle>
					<CardDescription>
						Use Cloudflare's native Observability Destinations to send traces to Traceway
					</CardDescription>
				</CardHeader>
				<CardContent>
					<div class="space-y-6">
						<div>
							<p class="mb-2 text-sm font-medium">Step 1 — OTLP Traces Endpoint</p>
							<p class="mb-2 text-xs text-muted-foreground">
								Enter this URL when creating your OTLP destination in the Cloudflare dashboard.
							</p>
							<div class="flex items-center gap-2">
								<code class="flex-1 rounded-md bg-muted px-3 py-2 font-mono text-sm break-all"
									>{cloudflareOtelEndpoint}</code
								>
								<Button variant="outline" size="sm" onclick={copyCfEndpoint}>
									{#if copiedCfEndpoint}
										<Check class="h-4 w-4 text-green-500" />
									{:else}
										<Copy class="h-4 w-4" />
									{/if}
								</Button>
							</div>
						</div>
						<div>
							<p class="mb-2 text-sm font-medium">Step 2 — Authorization Header</p>
							<p class="mb-2 text-xs text-muted-foreground">
								Add this as an authorization header in your OTLP destination settings.
							</p>
							<div class="flex items-center gap-2">
								<code class="flex-1 rounded-md bg-muted px-3 py-2 font-mono text-sm break-all"
									>{cloudflareAuthHeader}</code
								>
								<Button variant="outline" size="sm" onclick={copyCfAuth}>
									{#if copiedCfAuth}
										<Check class="h-4 w-4 text-green-500" />
									{:else}
										<Copy class="h-4 w-4" />
									{/if}
								</Button>
							</div>
						</div>
						<div>
							<p class="mb-2 text-sm font-medium">Step 3 — wrangler.jsonc</p>
							<p class="mb-2 text-xs text-muted-foreground">
								Enable observability traces in your wrangler.jsonc configuration file.
							</p>
							<div class="relative">
								<div class="absolute top-2 right-2 z-10">
									<Button variant="outline" size="sm" onclick={copyCfWrangler}>
										{#if copiedCfWrangler}
											<Check class="mr-2 h-4 w-4 text-green-500" />
											Copied!
										{:else}
											<Copy class="mr-2 h-4 w-4" />
											Copy
										{/if}
									</Button>
								</div>
								<div
									class="overflow-x-auto rounded-lg text-sm {themeState.isDark
										? 'dark-code'
										: 'light-code'}"
								>
									<Highlight language={javascript} code={wranglerConfig} />
								</div>
							</div>
						</div>
						<div>
							<p class="mb-2 text-sm font-medium">Step 4 — Deploy</p>
							<div class="flex items-center gap-2">
								<code class="flex-1 rounded-md bg-muted px-3 py-2 font-mono text-sm break-all"
									>npx wrangler deploy</code
								>
								<Button variant="outline" size="sm" onclick={copyCfDeploy}>
									{#if copiedCfDeploy}
										<Check class="h-4 w-4 text-green-500" />
									{:else}
										<Copy class="h-4 w-4" />
									{/if}
								</Button>
							</div>
						</div>
					</div>
				</CardContent>
			</Card>
		{:else}
			<Card>
				<CardHeader>
					<CardTitle class="flex items-center gap-2">
						<FrameworkIcon framework={projectWithToken.framework} />
						{getFrameworkLabel(projectWithToken.framework)} Integration
					</CardTitle>
					<CardDescription>
						Add Traceway to your application with just a few lines of code.
					</CardDescription>
				</CardHeader>
				<CardContent>
					<div class="relative">
						<div class="absolute top-2 right-2 z-10">
							<Button variant="outline" size="sm" onclick={copyCode}>
								{#if copiedCode}
									<Check class="mr-2 h-4 w-4 text-green-500" />
									Copied!
								{:else}
									<Copy class="mr-2 h-4 w-4" />
									Copy
								{/if}
							</Button>
						</div>
						<div
							class="overflow-x-auto rounded-lg text-sm {themeState.isDark
								? 'dark-code'
								: 'light-code'}"
						>
							<Highlight language={highlightLanguage} code={sdkCode} />
						</div>
					</div>
				</CardContent>
			</Card>

			<Card>
				<CardHeader>
					<CardTitle>Installation</CardTitle>
					<CardDescription
						>Install the required packages{isJs ? '' : projectWithToken?.framework === 'symfony' || projectWithToken?.framework === 'laravel' ? ' using composer' : projectWithToken?.framework === 'django' ? ' using pip' : ' using go get'}.</CardDescription
					>
				</CardHeader>
				<CardContent>
					<div class="relative">
						<div class="absolute top-2 right-2 z-10">
							<Button variant="outline" size="sm" onclick={copyInstall}>
								{#if copiedInstall}
									<Check class="mr-2 h-4 w-4 text-green-500" />
									Copied!
								{:else}
									<Copy class="mr-2 h-4 w-4" />
									Copy
								{/if}
							</Button>
						</div>
						<div
							class="overflow-x-auto rounded-lg text-sm {themeState.isDark
								? 'dark-code'
								: 'light-code'}"
						>
							<Highlight language={bash} code={installCommand} />
						</div>
					</div>
				</CardContent>
			</Card>
			{#if isJs}
				<SourceMapUploadCard />
			{/if}
		{/if}
	{:else}
		<Card>
			<CardContent class="p-6 text-center">
				<p class="text-muted-foreground">
					No project selected. Please select or create a project from the dropdown above.
				</p>
			</CardContent>
		</Card>
	{/if}
</div>

<style>
	/* Light theme - override dark theme defaults */
	:global(.light-code .hljs-name) {
		color: #4ba3f7;
	}
	:global(.light-code .hljs) {
		background: #f6f8fa;
		color: #24292e;
	}
	:global(.light-code .hljs-keyword),
	:global(.light-code .hljs-selector-tag) {
		color: #d73a49;
	}
	:global(.light-code .hljs-string),
	:global(.light-code .hljs-attr) {
		color: #032f62;
	}
	:global(.light-code .hljs-function),
	:global(.light-code .hljs-title) {
		color: #6f42c1;
	}
	:global(.light-code .hljs-comment) {
		color: #6a737d;
	}
	:global(.light-code .hljs-built_in) {
		color: #005cc5;
	}
	:global(.light-code .hljs-meta) {
		color: #d73a49;
	}
	:global(.light-code .hljs-variable) {
		color: #24292e;
	}

	/* Dark theme - ensure dark styles apply */
	:global(.dark-code .hljs) {
		background: #0d1117;
		color: #c9d1d9;
	}
	:global(.dark-code .hljs-keyword),
	:global(.dark-code .hljs-selector-tag) {
		color: #ff7b72;
	}
	:global(.dark-code .hljs-string),
	:global(.dark-code .hljs-attr) {
		color: #a5d6ff;
	}
	:global(.dark-code .hljs-function),
	:global(.dark-code .hljs-title) {
		color: #d2a8ff;
	}
	:global(.dark-code .hljs-comment) {
		color: #8b949e;
	}
	:global(.dark-code .hljs-built_in) {
		color: #79c0ff;
	}
	:global(.dark-code .hljs-meta) {
		color: #ff7b72;
	}
</style>
