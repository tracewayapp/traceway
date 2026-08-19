<script lang="ts">
	import {
		Card,
		CardContent,
		CardDescription,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card';
	import {
		projectsState,
		type ProjectWithToken,
		isJsFramework,
		isOtelFramework,
		isCloudflareFramework,
		supportsSymbolUpload
	} from '$lib/state/projects.svelte';
	import FrameworkIcon from '$lib/components/framework-icon.svelte';
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import go from 'svelte-highlight/languages/go';
	import javascript from 'svelte-highlight/languages/javascript';
	import bash from 'svelte-highlight/languages/bash';
	import php from 'svelte-highlight/languages/php';
	import python from 'svelte-highlight/languages/python';
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
	import CopyableInline from '$lib/components/setup/copyable-inline.svelte';
	import CopyableCodeBlock from '$lib/components/setup/copyable-code-block.svelte';

	let projectWithToken = $derived(projectsState.currentProject);
	let setupMode = $state(getSetupMode());

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
		const fw = projectWithToken.framework;
		if (fw === 'symfony' || fw === 'laravel') return php;
		if (fw === 'django') return python;
		if (fw === 'ios' || fw === 'flutter' || fw === 'android') return javascript;
		if (isJsFramework(fw)) return javascript;
		return go;
	});

	const isJs = $derived(projectWithToken ? isJsFramework(projectWithToken.framework) : false);
	const installDescription = $derived.by(() => {
		const fw = projectWithToken?.framework;
		let suffix = '';
		if (fw === 'symfony' || fw === 'laravel') suffix = ' using composer';
		else if (fw === 'django') suffix = ' using pip';
		else if (fw === 'ios') suffix = ' using Swift Package Manager';
		else if (fw === 'flutter') suffix = ' using pub';
		else if (fw === 'android') suffix = ' using Gradle';
		else if (!isJs) suffix = ' using go get';
		return `Install the required packages${suffix}.`;
	});
	const showSymbolUpload = $derived(
		projectWithToken ? supportsSymbolUpload(projectWithToken.framework) : false
	);
</script>

<div class="space-y-4">
	<PageHeader title="Connection" description="Connect your application to Traceway using the SDK" />

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
							<p class="mb-2 text-sm font-medium">Step 1: OTLP Traces Endpoint</p>
							<p class="mb-2 text-xs text-muted-foreground">
								Enter this URL when creating your OTLP destination in the Cloudflare dashboard.
							</p>
							<CopyableInline value={cloudflareOtelEndpoint} />
						</div>
						<div>
							<p class="mb-2 text-sm font-medium">Step 2: Authorization Header</p>
							<p class="mb-2 text-xs text-muted-foreground">
								Add this as an authorization header in your OTLP destination settings.
							</p>
							<CopyableInline value={cloudflareAuthHeader} />
						</div>
						<div>
							<p class="mb-2 text-sm font-medium">Step 3: wrangler.jsonc</p>
							<p class="mb-2 text-xs text-muted-foreground">
								Enable observability traces in your wrangler.jsonc configuration file.
							</p>
							<CopyableCodeBlock code={wranglerConfig} language={javascript} />
						</div>
						<div>
							<p class="mb-2 text-sm font-medium">Step 4: Deploy</p>
							<CopyableInline value="npx wrangler deploy" />
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
					<CopyableCodeBlock code={sdkCode} language={highlightLanguage} />
				</CardContent>
			</Card>

			<Card>
				<CardHeader>
					<CardTitle>Installation</CardTitle>
					<CardDescription>{installDescription}</CardDescription>
				</CardHeader>
				<CardContent>
					<CopyableCodeBlock code={installCommand} language={bash} />
				</CardContent>
			</Card>
			{#if showSymbolUpload}
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
