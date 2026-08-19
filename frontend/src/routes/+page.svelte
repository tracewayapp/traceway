<script lang="ts">
	import { getErrorMessage, getErrorStatus } from '$lib/utils/errors';
	import { resolve } from '$app/paths';
	import { onMount } from 'svelte';
	import { createRowClickHandler } from '$lib/utils/navigation';
	import { formatDuration, formatRelativeTime } from '$lib/utils/formatters';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import * as Table from '$lib/components/ui/table';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import {
		ArrowRight,
		Gauge,
		Bug,
		CircleQuestionMark,
		CircleCheck,
		RefreshCw,
		Star,
		Unplug
	} from '@lucide/svelte';
	import WidgetGrid from '$lib/components/dashboard/widget-grid.svelte';
	import { TracewayTableHeader } from '$lib/components/ui/traceway-table-header';
	import { ImpactBadge } from '$lib/components/ui/impact-badge';
	import EndpointName from '$lib/components/endpoint-name.svelte';
	import { ViewAllTableRow } from '$lib/components/ui/view-all-table-row';
	import { api } from '$lib/api';
	import { ErrorDisplay } from '$lib/components/ui/error-display';
	import {
		projectsState,
		isFrontendFramework,
		isCloudflareFramework,
		isOtelFramework
	} from '$lib/state/projects.svelte';
	import { setSortState } from '$lib/utils/sort-storage';
	import { Button } from '$lib/components/ui/button';
	import go from 'svelte-highlight/languages/go';
	import javascript from 'svelte-highlight/languages/javascript';
	import bash from 'svelte-highlight/languages/bash';
	import php from 'svelte-highlight/languages/php';
	import python from 'svelte-highlight/languages/python';
	import {
		getFrameworkCode,
		getInstallCommand,
		getTestingRouteCode,
		getFrameworkLabel,
		getTestingRouteCode2,
		getCodeLanguage
	} from '$lib/utils/framework-code';
	import { toast } from 'svelte-sonner';
	import { goto } from '$app/navigation';
	import { OTEL_SDKS } from '$lib/utils/otel-sdks';
	import { getSetupMode } from '$lib/utils/setup-storage';
	import SetupModeTabs from '$lib/components/setup/setup-mode-tabs.svelte';
	import AiSetupSteps from '$lib/components/setup/ai-setup-steps.svelte';
	import OtelSetupSteps from '$lib/components/setup/otel-setup-steps.svelte';
	import OtelExporterConfig from '$lib/components/setup/otel-exporter-config.svelte';
	import CopyableInline from '$lib/components/setup/copyable-inline.svelte';
	import CopyableCodeBlock from '$lib/components/setup/copyable-code-block.svelte';
	import CopyButton from '$lib/components/traceway/copy-button.svelte';
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import type { DashboardWidgetConfig } from '$lib/types/dashboard';

	const timezone = $derived(getTimezone());

	$effect(() => {
		if (
			projectsState.currentProject &&
			isFrontendFramework(projectsState.currentProject.framework)
		) {
			// index redirects to issues on a frontend project
			goto(resolve('/issues'));
		}
	});

	type ExceptionGroup = {
		exceptionHash: string;
		stackTrace: string;
		lastSeen: string;
		firstSeen: string;
		count: number;
	};

	type EndpointStats = {
		endpoint: string;
		count: number;
		p50Duration: number;
		p95Duration: number;
		avgDuration: number;
		lastSeen: string;
		impact: number;
		impactReason: string;
	};

	type DashboardOverview = {
		recentIssues: ExceptionGroup[];
		worstEndpoints: EndpointStats[];
		hasData: boolean;
	};

	type StarredWidgetResponse = {
		id: number;
		dashboardId: number;
		widgetId: string;
		title: string;
		widgetType: string;
		config: DashboardWidgetConfig;
		homePosition: number;
		homeColSpan: number;
		homeSize: string;
	};

	type StarredWidget = {
		id: number;
		dashboardId: number;
		widgetId: string;
		title: string;
		widgetType: string;
		config: DashboardWidgetConfig;
		position: number;
		isStarred: boolean;
	};

	let data = $state<DashboardOverview | null>(null);
	let starredWidgets = $state<StarredWidget[]>([]);
	let loading = $state(true);
	let error = $state('');
	let errorStatus = $state<number>(0);

	const STARRED_WINDOW_MS = 24 * 60 * 60 * 1000;
	let starredFromUTC = $state(new Date(Date.now() - STARRED_WINDOW_MS).toISOString());
	let starredToUTC = $state(new Date().toISOString());

	// Filter endpoints to only show those with impact > good (score >= 0.25)
	const impactfulEndpoints = $derived(data?.worstEndpoints?.filter((e) => e.impact >= 0.25) ?? []);

	let projectWithToken = $derived(projectsState.currentProject);
	let setupMode = $state(getSetupMode());
	let checking = $state(false);

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

	const isFrontend = $derived(
		projectWithToken ? isFrontendFramework(projectWithToken.framework) : false
	);

	const codeLanguage = $derived(
		projectWithToken ? getCodeLanguage(projectWithToken.framework) : ('go' as const)
	);

	const highlightLanguage = $derived(
		codeLanguage === 'javascript'
			? javascript
			: codeLanguage === 'php'
				? php
				: codeLanguage === 'python'
					? python
					: codeLanguage === 'bash'
						? bash
						: go
	);

	const testingRouteCode = $derived(getTestingRouteCode(projectWithToken?.framework));
	const testingRouteCode2 = $derived(getTestingRouteCode2(projectWithToken?.framework));

	const isCloudflare = $derived(
		projectWithToken ? isCloudflareFramework(projectWithToken.framework) : false
	);
	const cfOtelEndpoint = $derived(
		projectWithToken ? `${projectWithToken.backendUrl}/api/otel/v1/traces` : ''
	);
	const cfAuthHeader = $derived(projectWithToken ? `Bearer ${projectWithToken.token}` : '');
	const cfWranglerConfig = $derived(`{
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
}`);

	const isOtel = $derived(projectWithToken ? isOtelFramework(projectWithToken.framework) : false);
	const isOtelGeneric = $derived(projectWithToken?.framework === 'opentelemetry');
	const otelBaseEndpoint = $derived(
		projectWithToken ? `${projectWithToken.backendUrl}/api/otel` : ''
	);
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

	async function checkAgain() {
		checking = true;
		const hadDataBefore = data?.hasData ?? false;
		await loadDashboard(false);
		checking = false;

		// Show success toast if data was received
		if (!hadDataBefore && data?.hasData) {
			toast.success('Integration successful! Data received from your application.');
		} else if (!data?.hasData) {
			toast.warning('No data received yet');
		}
	}

	async function loadDashboard(showFullPageLoading = true) {
		if (showFullPageLoading) {
			loading = true;
		}
		error = '';
		errorStatus = 0;

		try {
			const projectId = projectsState.currentProjectId ?? undefined;
			starredFromUTC = new Date(Date.now() - STARRED_WINDOW_MS).toISOString();
			starredToUTC = new Date().toISOString();
			const [overview, starred] = await Promise.all([
				api.get('/dashboard/overview', { projectId }),
				api.get('/dashboards/starred', { projectId }).catch(() => [])
			]);
			data = overview;
			starredWidgets = ((starred as StarredWidgetResponse[]) ?? []).map((w) => ({
				id: w.id,
				dashboardId: w.dashboardId,
				widgetId: w.widgetId,
				title: w.title,
				widgetType: w.widgetType,
				config: { ...w.config, colSpan: w.homeColSpan, size: w.homeSize },
				position: w.homePosition,
				isStarred: true
			}));
		} catch (e) {
			errorStatus = getErrorStatus(e) || 0;
			error = getErrorMessage(e) || 'Failed to load dashboard data';
			console.error(e);
		} finally {
			if (showFullPageLoading) {
				loading = false;
			}
		}
	}

	async function handleReorderStarred(reorderedIds: (number | string)[]) {
		const ids = reorderedIds.map(Number);
		const previousPositions = new Map(starredWidgets.map((w) => [w.id, w.position]));
		for (const w of starredWidgets) {
			w.position = ids.indexOf(w.id);
		}
		try {
			await api.put(
				'/starred-widgets/reorder',
				{ ids },
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
		} catch (e) {
			for (const w of starredWidgets) {
				w.position = previousPositions.get(w.id) ?? w.position;
			}
			if (getErrorStatus(e) !== 403) {
				toast.error(getErrorMessage(e) || 'Failed to reorder widgets');
			}
			await loadDashboard(false);
		}
	}

	async function handleResizeStarred(
		widget: { id: number | string },
		layout: { colSpan: number; size: string }
	) {
		const target = starredWidgets.find((w) => w.id === Number(widget.id));
		if (!target) return;
		const previous = { colSpan: target.config.colSpan, size: target.config.size };
		target.config.colSpan = layout.colSpan;
		target.config.size = layout.size;
		try {
			await api.put(`/starred-widgets/${target.id}`, layout, {
				projectId: projectsState.currentProjectId ?? undefined
			});
		} catch (e) {
			target.config.colSpan = previous.colSpan;
			target.config.size = previous.size;
			if (getErrorStatus(e) !== 403) {
				toast.error(getErrorMessage(e) || 'Failed to resize widget');
			}
		}
	}

	async function handleUnstar(widget: { id: number | string }) {
		const target = starredWidgets.find((w) => w.id === Number(widget.id));
		if (!target) return;
		const previous = starredWidgets;
		starredWidgets = starredWidgets.filter((w) => w.id !== target.id);
		try {
			await api.put(
				`/dashboards/${target.dashboardId}/widgets/${target.widgetId}/star`,
				{},
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
		} catch (e) {
			starredWidgets = previous;
			if (getErrorStatus(e) !== 403) {
				toast.error(getErrorMessage(e) || 'Failed to unstar widget');
			}
		}
	}

	let lastProjectId = $state(projectsState.currentProjectId);

	onMount(() => {
		loadDashboard();
	});

	$effect(() => {
		const currentId = projectsState.currentProjectId;
		if (currentId !== lastProjectId) {
			lastProjectId = currentId;
			loadDashboard();
		}
	});

	function resetEndpointsSortToImpact() {
		setSortState('endpoints', { field: 'impact', direction: 'desc' });
	}
</script>

<div class="space-y-4">
	<PageHeader title="Dashboard" />

	{#if error && !loading}
		<ErrorDisplay
			status={errorStatus === 404
				? 404
				: errorStatus === 400
					? 400
					: errorStatus === 422
						? 422
						: 400}
			title="Failed to Load Dashboard"
			description={error}
			onRetry={() => loadDashboard()}
		/>
	{/if}

	{#if loading}
		<div class="flex items-center justify-center py-20">
			<LoadingCircle size="xlg" />
		</div>
	{:else if !error && data && !data.hasData}
		<!-- Integration Not Connected -->
		<div class="space-y-6">
			<div class="rounded-md border bg-card">
				<div class="flex flex-col items-center justify-center px-6 py-8 text-center">
					<div class="mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-muted">
						<Unplug class="h-6 w-6 text-muted-foreground" />
					</div>
					<h3 class="mb-2 text-lg font-semibold">Connect Your Application</h3>
					<p class="mb-4 max-w-md text-sm text-muted-foreground">
						No data has been received yet. Follow the steps below to integrate Traceway into your
						application.
					</p>
					<Button variant="outline" onclick={checkAgain} disabled={checking}>
						{#if checking}
							<RefreshCw class="mr-2 h-4 w-4 animate-spin" />
						{:else}
							<RefreshCw class="mr-2 h-4 w-4" />
						{/if}
						Check Again
					</Button>
				</div>
			</div>

			{#if projectWithToken}
				<SetupModeTabs mode={setupMode} onModeChange={(m) => (setupMode = m)} />
				{#if setupMode === 'ai'}
					<AiSetupSteps backendUrl={projectWithToken.backendUrl} token={projectWithToken.token} />
				{:else if isCloudflare}
					<!-- Cloudflare Step 1: Create a Destination -->
					<div class="rounded-md border bg-card">
						<div class="border-b px-4 py-3">
							<div class="flex items-center gap-3">
								<div
									class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-sm font-medium text-primary-foreground"
								>
									1
								</div>
								<h3 class="font-semibold">Create a Destination</h3>
							</div>
							<p class="mt-1 ml-9 text-sm text-muted-foreground">
								In the Cloudflare dashboard, create an OTLP destination with the endpoint and
								authorization header below.
							</p>
						</div>
						<div class="space-y-4 p-4">
							<div>
								<p class="mb-2 text-sm font-medium">OTLP Traces Endpoint</p>
								<CopyableInline value={cfOtelEndpoint} />
							</div>
							<div>
								<p class="mb-2 text-sm font-medium">Authorization Header</p>
								<CopyableInline value={cfAuthHeader} />
							</div>
						</div>
					</div>

					<!-- Cloudflare Step 2: Enable in wrangler.jsonc -->
					<div class="rounded-md border bg-card">
						<div class="border-b px-4 py-3">
							<div class="flex items-center gap-3">
								<div
									class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-sm font-medium text-primary-foreground"
								>
									2
								</div>
								<h3 class="font-semibold">Enable in wrangler.jsonc</h3>
							</div>
							<p class="mt-1 ml-9 text-sm text-muted-foreground">
								Add the observability configuration to your wrangler.jsonc file.
							</p>
						</div>
						<div class="p-4">
							<CopyableCodeBlock code={cfWranglerConfig} language={javascript} />
						</div>
					</div>

					<!-- Cloudflare Step 3: Deploy -->
					<div class="rounded-md border bg-card">
						<div class="border-b px-4 py-3">
							<div class="flex items-center gap-3">
								<div
									class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-sm font-medium text-primary-foreground"
								>
									3
								</div>
								<h3 class="font-semibold">Deploy</h3>
							</div>
							<p class="mt-1 ml-9 text-sm text-muted-foreground">
								Deploy your worker to start sending traces.
							</p>
						</div>
						<div class="p-4">
							<CopyableInline value="npx wrangler deploy" />
						</div>
					</div>
				{:else if isOtelGeneric}
					<OtelSetupSteps backendUrl={projectWithToken.backendUrl} token={projectWithToken.token} />
				{:else if isOtel}
					<!-- OTel Step 1: Install an OTel SDK -->
					<div class="rounded-md border bg-card">
						<div class="border-b px-4 py-3">
							<div class="flex items-center gap-3">
								<div
									class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-sm font-medium text-primary-foreground"
								>
									1
								</div>
								<h3 class="font-semibold">Install an OpenTelemetry SDK</h3>
							</div>
							<p class="mt-1 ml-9 text-sm text-muted-foreground">
								Choose the OTel SDK for your language. Any language that supports OTLP/HTTP export
								will work.
							</p>
						</div>
						<div class="space-y-2 p-4">
							{#each OTEL_SDKS as sdk (sdk.id)}
								<div class="flex items-center gap-2">
									<span class="w-16 shrink-0 text-sm font-medium">{sdk.label}</span>
									<code class="flex-1 rounded-md bg-muted px-3 py-2 font-mono text-xs break-all"
										>{sdk.installCommand}</code
									>
									<CopyButton text={sdk.installCommand} />
								</div>
							{/each}
							<p class="ml-16 pt-1 text-xs text-muted-foreground">
								<a
									href="https://opentelemetry.io/docs/languages/"
									target="_blank"
									rel="noopener noreferrer"
									class="underline hover:text-foreground">View all supported languages</a
								>
							</p>
						</div>
					</div>

					<!-- OTel Step 2: Configure the Exporter -->
					<div class="rounded-md border bg-card">
						<div class="border-b px-4 py-3">
							<div class="flex items-center gap-3">
								<div
									class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-sm font-medium text-primary-foreground"
								>
									2
								</div>
								<h3 class="font-semibold">Configure the OTLP Exporter</h3>
							</div>
							<p class="mt-1 ml-9 text-sm text-muted-foreground">
								Point your OTLP/HTTP exporter at Traceway using the endpoint and token below.
							</p>
						</div>
						<div class="p-4">
							<OtelExporterConfig
								endpoint={otelBaseEndpoint}
								authHeader={otelAuthHeader}
								collectorConfig={otelCollectorConfig}
							/>
						</div>
					</div>

					<!-- OTel Step 3: Run Your Application -->
					<div class="rounded-md border bg-card">
						<div class="border-b px-4 py-3">
							<div class="flex items-center gap-3">
								<div
									class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-sm font-medium text-primary-foreground"
								>
									3
								</div>
								<h3 class="font-semibold">Run Your Application</h3>
							</div>
							<p class="mt-1 ml-9 text-sm text-muted-foreground">
								Start your application with OpenTelemetry instrumentation enabled. The SDK will
								automatically export traces and metrics to Traceway via OTLP/HTTP.
							</p>
						</div>
					</div>
				{:else}
					<!-- Step 1: Install -->
					<div class="rounded-md border bg-card">
						<div class="border-b px-4 py-3">
							<div class="flex items-center gap-3">
								<div
									class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-sm font-medium text-primary-foreground"
								>
									1
								</div>
								<h3 class="font-semibold">Install the SDK</h3>
							</div>
						</div>
						<div class="p-4">
							<CopyableCodeBlock code={installCommand} language={bash} />
						</div>
					</div>

					<!-- Step 2: Setup Integration -->
					<div class="rounded-md border bg-card">
						<div class="border-b px-4 py-3">
							<div class="flex items-center gap-3">
								<div
									class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-sm font-medium text-primary-foreground"
								>
									2
								</div>
								<h3 class="font-semibold">
									{getFrameworkLabel(projectWithToken.framework)} Integration
								</h3>
							</div>
							<p class="mt-1 ml-9 text-sm text-muted-foreground">
								Add the Traceway middleware to your application.
							</p>
						</div>
						<div class="p-4">
							<CopyableCodeBlock code={sdkCode} language={highlightLanguage} />
						</div>
					</div>

					<!-- Step 3: Add Testing Route -->
					<div class="rounded-md border bg-card">
						<div class="border-b px-4 py-3">
							<div class="flex items-center gap-3">
								<div
									class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-sm font-medium text-primary-foreground"
								>
									3
								</div>
								<h3 class="font-semibold">Add a Test Route</h3>
							</div>
							<p class="mt-1 ml-9 text-sm text-muted-foreground">
								Add this route to verify your integration, then visit <code
									class="rounded bg-muted px-1 py-0.5 font-mono text-xs">GET /testing</code
								> in your browser.
							</p>
						</div>
						<div class="p-4">
							<CopyableCodeBlock code={testingRouteCode} language={highlightLanguage} />

							{#if testingRouteCode2}
								<div class="flex justify-center p-2 italic">or</div>

								<CopyableCodeBlock code={testingRouteCode2} language={highlightLanguage} />
							{/if}
						</div>
					</div>
				{/if}
				<!-- Bottom Check Again -->
				<div class="rounded-md border bg-card">
					<div class="flex flex-col items-center justify-center px-6 py-6 text-center">
						<div
							class="mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-destructive/10"
						>
							<Unplug class="h-6 w-6 text-destructive" />
						</div>
						<p class="mb-4 text-sm text-muted-foreground">
							{#if setupMode === 'ai' || isOtel || isCloudflare}
								Once you've completed the steps above and sent some traffic through your
								application, click below to verify.
							{:else}
								Once you've completed the steps above and triggered the <code
									class="rounded bg-muted px-1 py-0.5 font-mono text-xs">/testing</code
								> endpoint, click below to verify.
							{/if}
						</p>
						<Button variant="outline" onclick={checkAgain} disabled={checking}>
							{#if checking}
								<RefreshCw class="mr-2 h-4 w-4 animate-spin" />
							{:else}
								<RefreshCw class="mr-2 h-4 w-4" />
							{/if}
							Check Again
						</Button>
					</div>
				</div>
			{/if}
		</div>
	{:else if !error}
		<div class="space-y-6">
			{#if starredWidgets.length > 0}
				<div>
					<div class="mb-4 flex items-center gap-1">
						<div class="mr-2 flex h-8 w-8 items-center justify-center rounded-md bg-yellow-500/10">
							<Star class="h-5 w-5 fill-yellow-400 text-yellow-400" />
						</div>
						<h2 class="text-2xl font-semibold tracking-tight">Starred</h2>
						<Tooltip.Root>
							<Tooltip.Trigger class="pt-1">
								<CircleQuestionMark class="h-4 w-4 text-muted-foreground/60" />
							</Tooltip.Trigger>
							<Tooltip.Content>
								<p>Widgets you've starred from the Dashboards page (last 24h)</p>
							</Tooltip.Content>
						</Tooltip.Root>
					</div>
					<WidgetGrid
						widgets={starredWidgets}
						fromDateUTC={starredFromUTC}
						toDateUTC={starredToUTC}
						timeDomain={null}
						onReorderWidgets={handleReorderStarred}
						onResizeWidget={handleResizeStarred}
						onToggleStar={handleUnstar}
					/>
				</div>
			{/if}
			{#if !isFrontend}
				<!-- Endpoints -->
				<div>
					<div class="mb-4 flex items-center gap-1">
						<div class="mr-2 flex h-8 w-8 items-center justify-center rounded-md bg-chart-1/10">
							<Gauge class="h-5 w-5 text-chart-1" />
						</div>
						<h2 class="text-2xl font-semibold tracking-tight">Endpoints</h2>
						<Tooltip.Root>
							<Tooltip.Trigger class="pt-1">
								<CircleQuestionMark class="h-4 w-4 text-muted-foreground/60" />
							</Tooltip.Trigger>
							<Tooltip.Content>
								<p>Endpoints needing attention based on response time and error rates</p>
							</Tooltip.Content>
						</Tooltip.Root>
					</div>
					{#if impactfulEndpoints.length > 0}
						<div class="overflow-hidden rounded-md border">
							<Table.Root>
								<Table.Header>
									<Table.Row class="hover:bg-transparent">
										<TracewayTableHeader
											label="Endpoint"
											tooltip="The API route or page being accessed"
										/>
										<TracewayTableHeader
											label="Calls"
											tooltip="Total number of requests"
											align="right"
											class="w-[70px]"
										/>
										<TracewayTableHeader
											label="Typical"
											tooltip="Median response time (P50)"
											align="right"
											class="w-[80px]"
										/>
										<TracewayTableHeader
											label="Slow"
											tooltip="95th percentile - slowest 5%"
											align="right"
											class="w-[70px]"
										/>
										<TracewayTableHeader
											label="Impact"
											tooltip="Priority based on response time and error rates"
											align="right"
											class="w-[80px]"
										/>
									</Table.Row>
								</Table.Header>
								<Table.Body>
									{#each impactfulEndpoints as endpoint, __index (__index)}
										<Table.Row
											class="cursor-pointer"
											onclick={createRowClickHandler(
												`/endpoints/${encodeURIComponent(endpoint.endpoint)}?preset=24h`
											)}
										>
											<Table.Cell
												class="max-w-[300px] truncate py-3 font-mono text-sm"
												title={endpoint.endpoint}
											>
												<EndpointName endpoint={endpoint.endpoint} />
											</Table.Cell>
											<Table.Cell class="py-3 text-right tabular-nums">
												{endpoint.count.toLocaleString()}
											</Table.Cell>
											<Table.Cell class="py-3 text-right font-mono text-sm tabular-nums">
												{formatDuration(endpoint.p50Duration)}
											</Table.Cell>
											<Table.Cell class="py-3 text-right font-mono text-sm tabular-nums">
												{formatDuration(endpoint.p95Duration)}
											</Table.Cell>
											<Table.Cell class="py-3 text-right">
												<ImpactBadge score={endpoint.impact} reason={endpoint.impactReason} />
											</Table.Cell>
										</Table.Row>
									{/each}
									<ViewAllTableRow
										colspan={5}
										href="/endpoints"
										label="View all endpoints"
										onBeforeNavigate={resetEndpointsSortToImpact}
									/>
								</Table.Body>
							</Table.Root>
						</div>
					{:else}
						<!-- Empty state card for endpoints -->
						<div class="rounded-md border bg-card">
							<div class="flex flex-col items-center justify-center px-6 py-12 text-center">
								<div class="mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-muted">
									<CircleCheck class="h-6 w-6 text-green-500 dark:text-green-400" />
								</div>
								<h3 class="mb-2 text-lg font-semibold">All Endpoints Healthy</h3>
								<p class="mb-4 max-w-sm text-sm text-muted-foreground">
									No endpoints have been experiencing performance issues in the last 24h. Endpoints
									with slow response times or high error rates will appear here when detected.
								</p>
								<a
									href={resolve('/endpoints')}
									class="inline-flex items-center gap-1 text-sm font-medium text-primary hover:underline"
									onclick={resetEndpointsSortToImpact}
								>
									View all endpoints
									<ArrowRight class="h-4 w-4" />
								</a>
							</div>
						</div>
					{/if}
				</div>
			{/if}

			<!-- Issues Section -->
			<div>
				<div class="mb-4 flex items-center gap-1">
					<div class="mr-2 flex h-8 w-8 items-center justify-center rounded-md bg-destructive/10">
						<Bug class="h-5 w-5 text-destructive" />
					</div>
					<h2 class="text-2xl font-semibold tracking-tight">Issues</h2>
					<Tooltip.Root>
						<Tooltip.Trigger class="pt-1">
							<CircleQuestionMark class="h-4 w-4 text-muted-foreground/60" />
						</Tooltip.Trigger>
						<Tooltip.Content>
							<p>Latest exceptions and errors to address from the last 24 hours</p>
						</Tooltip.Content>
					</Tooltip.Root>
				</div>
				{#if data?.recentIssues && data.recentIssues.length > 0}
					<div class="overflow-hidden rounded-md border">
						<Table.Root>
							<Table.Header>
								<Table.Row class="hover:bg-transparent">
									<TracewayTableHeader
										label="Issue"
										tooltip="The error message or exception that occurred"
									/>
									<TracewayTableHeader
										label="Count"
										tooltip="Number of times this issue occurred"
										align="right"
										class="w-[70px]"
									/>
									<TracewayTableHeader
										label="When"
										tooltip="When this issue last occurred"
										align="right"
										class="w-[70px]"
									/>
								</Table.Row>
							</Table.Header>
							<Table.Body>
								{#each data.recentIssues as issue, __index (__index)}
									{@const issueFirstLine = issue.stackTrace.split('\n')[0]}
									{@const issueColon = issueFirstLine.indexOf(':')}
									<Table.Row
										class="cursor-pointer"
										onclick={createRowClickHandler(`/issues/${issue.exceptionHash}`)}
									>
										<Table.Cell class="max-w-[480px] py-3" title={issue.stackTrace}>
											<div class="min-w-0">
												<div class="truncate text-[15px]/6 font-semibold text-foreground">
													{issueColon > 0 ? issueFirstLine.slice(0, issueColon) : issueFirstLine}
												</div>
												{#if issueColon > 0}
													<div class="truncate text-sm text-muted-foreground">
														{issueFirstLine.slice(issueColon + 1).trim()}
													</div>
												{/if}
											</div>
										</Table.Cell>
										<Table.Cell class="py-3 text-right font-medium tabular-nums">
											{issue.count}
										</Table.Cell>
										<Table.Cell class="py-3 text-right text-sm text-muted-foreground tabular-nums">
											{formatRelativeTime(issue.lastSeen, timezone)}
										</Table.Cell>
									</Table.Row>
								{/each}
								<ViewAllTableRow colspan={3} href="/issues" label="View all issues" />
							</Table.Body>
						</Table.Root>
					</div>
				{:else}
					<!-- Empty state card for issues -->
					<div class="rounded-md border bg-card">
						<div class="flex flex-col items-center justify-center px-6 py-12 text-center">
							<div class="mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-muted">
								<CircleCheck class="h-6 w-6 text-green-500 dark:text-green-400" />
							</div>
							<h3 class="mb-2 text-lg font-semibold">No Issues Found</h3>
							<p class="mb-4 max-w-sm text-sm text-muted-foreground">
								No Issues have been recorded in the last 24 hours. When issues occur in your
								application, they will appear here for quick triage.
							</p>
							<a
								href={resolve('/issues')}
								class="inline-flex items-center gap-1 text-sm font-medium text-primary hover:underline"
							>
								View all issues
								<ArrowRight class="h-4 w-4" />
							</a>
						</div>
					</div>
				{/if}
			</div>
		</div>
	{/if}
</div>
