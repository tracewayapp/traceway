<script lang="ts">
	import { onMount } from 'svelte';
	import { createRowClickHandler } from '$lib/utils/navigation';
	import { formatDuration, formatRelativeTime, truncateStackTrace } from '$lib/utils/formatters';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import * as Table from '$lib/components/ui/table';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import { ArrowRight, Gauge, Bug, CircleQuestionMark, CircleCheck, RefreshCw, Copy, Check, Unplug } from 'lucide-svelte';
	import { TracewayTableHeader } from '$lib/components/ui/traceway-table-header';
	import { ImpactBadge } from '$lib/components/ui/impact-badge';
	import { ViewAllTableRow } from '$lib/components/ui/view-all-table-row';
	import { api } from '$lib/api';
	import { ErrorDisplay } from '$lib/components/ui/error-display';
	import { projectsState, type ProjectWithToken } from '$lib/state/projects.svelte';
	import { setSortState } from '$lib/utils/sort-storage';
	import { Button } from '$lib/components/ui/button';
	import Highlight from 'svelte-highlight';
	import go from 'svelte-highlight/languages/go';
	import bash from 'svelte-highlight/languages/bash';
	import { themeState } from '$lib/state/theme.svelte';
	import 'svelte-highlight/styles/github-dark.css';
	import {
		getFrameworkCode,
		getInstallCommand,
		getTestingRouteCode,
		getFrameworkLabel,

		getTestingRouteCode2

	} from '$lib/utils/framework-code';
	import { toast } from 'svelte-sonner';

	const timezone = $derived(getTimezone());

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
		impact: number; // 0-1 Apdex-based impact score from backend
	};

	type DashboardOverview = {
		recentIssues: ExceptionGroup[];
		worstEndpoints: EndpointStats[];
		hasData: boolean;
	};

	let data = $state<DashboardOverview | null>(null);
	let loading = $state(true);
	let error = $state('');
	let errorStatus = $state<number>(0);

	// Filter endpoints to only show those with impact > good (score >= 0.25)
	const impactfulEndpoints = $derived(
		data?.worstEndpoints?.filter((e) => e.impact >= 0.25) ?? []
	);

	let projectWithToken = $derived(projectsState.currentProject);
	let copiedInstall = $state(false);
	let copiedCode = $state(false);
	let copiedTesting = $state(false);
	let copiedTesting2 = $state(false);
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
		projectWithToken
			? getInstallCommand(projectWithToken.framework)
			: 'go get github.com/traceway-io/go-client'
	);

	const testingRouteCode = getTestingRouteCode();
	const testingRouteCode2 = getTestingRouteCode2();

	async function copyInstall() {
		await navigator.clipboard.writeText(installCommand);
		copiedInstall = true;
		setTimeout(() => (copiedInstall = false), 2000);
	}

	async function copyCode() {
		await navigator.clipboard.writeText(sdkCode);
		copiedCode = true;
		setTimeout(() => (copiedCode = false), 2000);
	}

	async function copyTesting() {
		await navigator.clipboard.writeText(testingRouteCode);
		copiedTesting = true;
		setTimeout(() => (copiedTesting = false), 2000);
	}


	async function copyTesting2() {
		await navigator.clipboard.writeText(testingRouteCode2);
		copiedTesting2 = true;
		setTimeout(() => (copiedTesting2 = false), 2000);
	}

	async function checkAgain() {
		checking = true;
		const hadDataBefore = data?.hasData ?? false;
		await loadDashboard(false);
		checking = false;

		// Show success toast if data was received
		if (!hadDataBefore && data?.hasData) {
			toast.success('Integration successful! Data received from your application.');
		} else if (!data?.hasData) {
			toast.warning('No data received yet', {
				position: 'top-center'
			});
		}
	}

	async function loadDashboard(showFullPageLoading = true) {
		if (showFullPageLoading) {
			loading = true;
		}
		error = '';
		errorStatus = 0;

		try {
			const response = await api.get('/dashboard/overview', {
				projectId: projectsState.currentProjectId ?? undefined
			});
			data = response;
		} catch (e: any) {
			errorStatus = e.status || 0;
			error = e.message || 'Failed to load dashboard data';
			console.error(e);
		} finally {
			if (showFullPageLoading) {
				loading = false;
			}
		}
	}

	onMount(() => {
		loadDashboard();
	});

	function resetEndpointsSortToImpact() {
		setSortState('endpoints', { field: 'impact', direction: 'desc' });
	}
</script>

<div class="space-y-4">
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
				<div class="flex flex-col items-center justify-center py-8 px-6 text-center">
					<div class="flex h-12 w-12 items-center justify-center rounded-full bg-muted mb-4">
						<Unplug class="h-6 w-6 text-muted-foreground" />
					</div>
					<h3 class="text-lg font-semibold mb-2">Connect Your Application</h3>
					<p class="text-sm text-muted-foreground max-w-md mb-4">
						No data has been received yet. Follow the steps below to integrate Traceway into your application.
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
				<!-- Step 1: Install -->
				<div class="rounded-md border bg-card">
					<div class="border-b px-4 py-3">
						<div class="flex items-center gap-3">
							<div class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-primary-foreground text-sm font-medium">
								1
							</div>
							<h3 class="font-semibold">Install the SDK</h3>
						</div>
					</div>
					<div class="p-4">
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
					</div>
				</div>

				<!-- Step 2: Setup Integration -->
				<div class="rounded-md border bg-card">
					<div class="border-b px-4 py-3">
						<div class="flex items-center gap-3">
							<div class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-primary-foreground text-sm font-medium">
								2
							</div>
							<h3 class="font-semibold">{getFrameworkLabel(projectWithToken.framework)} Integration</h3>
						</div>
						<p class="text-sm text-muted-foreground mt-1 ml-9">
							Add the Traceway middleware to your application.
						</p>
					</div>
					<div class="p-4">
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
								<Highlight language={go} code={sdkCode} />
							</div>
						</div>
					</div>
				</div>

				<!-- Step 3: Add Testing Route -->
				<div class="rounded-md border bg-card">
					<div class="border-b px-4 py-3">
						<div class="flex items-center gap-3">
							<div class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-primary-foreground text-sm font-medium">
								3
							</div>
							<h3 class="font-semibold">Add a Test Route</h3>
						</div>
						<p class="text-sm text-muted-foreground mt-1 ml-9">
							Add this route to verify your integration, then visit <code class="rounded bg-muted px-1 py-0.5 text-xs font-mono">GET /testing</code> in your browser.
						</p>
					</div>
					<div class="p-4">
						<div class="relative">
							<div class="absolute top-2 right-2 z-10">
								<Button variant="outline" size="sm" onclick={copyTesting}>
									{#if copiedTesting}
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
								<Highlight language={go} code={testingRouteCode} />
							</div>
						</div>

						<div class="flex justify-center p-2 italic">or</div>

						<div class="relative">
							<div class="absolute top-2 right-2 z-10">
								<Button variant="outline" size="sm" onclick={copyTesting2}>
									{#if copiedTesting2}
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
								<Highlight language={go} code={testingRouteCode2} />
							</div>
						</div>
					</div>
				</div>

				<!-- Bottom Check Again -->
				<div class="rounded-md border bg-card">
					<div class="flex flex-col items-center justify-center py-6 px-6 text-center">
						<div class="flex h-12 w-12 items-center justify-center rounded-full bg-destructive/10 mb-4">
							<Unplug class="h-6 w-6 text-destructive" />
						</div>
						<p class="text-sm text-muted-foreground mb-4">
							Once you've completed the steps above and triggered the <code class="rounded bg-muted px-1 py-0.5 text-xs font-mono">/testing</code> endpoint, click below to verify.
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
			<!-- Endpoints -->
			<div>
				<div class="items-bottom mb-4 flex gap-1">
					<div class="mr-2 flex h-8 w-8 items-center justify-center rounded-md bg-chart-1/10">
						<Gauge class="h-5 w-5 text-chart-1" />
					</div>
					<h2 class="text-2xl font-bold tracking-tight">Endpoints</h2>
					<Tooltip.Root>
						<Tooltip.Trigger class="pt-1">
							<CircleQuestionMark class="h-4 w-4 text-muted-foreground/60" />
						</Tooltip.Trigger>
						<Tooltip.Content>
							<p>
								Endpoints needing attention based on response time and error rates
							</p>
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
								{#each impactfulEndpoints as endpoint}
									<Table.Row
										class="cursor-pointer hover:bg-muted/50"
										onclick={createRowClickHandler(
											`/endpoints/${encodeURIComponent(endpoint.endpoint)}?preset=24h`
										)}
									>
										<Table.Cell
											class="max-w-[300px] truncate py-3 font-mono text-sm"
											title={endpoint.endpoint}
										>
											{endpoint.endpoint}
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
											<ImpactBadge score={endpoint.impact} />
										</Table.Cell>
									</Table.Row>
								{/each}
								<ViewAllTableRow colspan={5} href="/endpoints" label="View all endpoints" onBeforeNavigate={resetEndpointsSortToImpact} />
							</Table.Body>
						</Table.Root>
					</div>
				{:else}
					<!-- Empty state card for endpoints -->
					<div class="rounded-md border bg-card">
						<div class="flex flex-col items-center justify-center py-12 px-6 text-center">
							<div class="flex h-12 w-12 items-center justify-center rounded-full mb-4">
								<CircleCheck class="h-12 w-12 text-green-500 dark:text-green-400" />
							</div>
							<h3 class="text-lg font-semibold mb-2">All Endpoints Healthy</h3>
							<p class="text-sm text-muted-foreground max-w-sm mb-4">
								No endpoints have been experiencing performance issues in the last 24h. Endpoints with slow response times or high error rates will appear here when detected.
							</p>
							<a
								href="/endpoints"
								class="text-sm font-medium text-primary hover:underline inline-flex items-center gap-1"
								onclick={resetEndpointsSortToImpact}
							>
								View all endpoints
								<ArrowRight class="h-4 w-4" />
							</a>
						</div>
					</div>
				{/if}
			</div>

			<!-- Issues Section -->
			<div>
				<div class="mb-4 flex items-center gap-1">
					<div class="mr-2 flex h-8 w-8 items-center justify-center rounded-md bg-destructive/10">
						<Bug class="h-5 w-5 text-destructive" />
					</div>
					<h2 class="text-2xl font-bold tracking-tight">Issues</h2>
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
								{#each data.recentIssues as issue}
									<Table.Row
										class="cursor-pointer hover:bg-muted/50"
										onclick={createRowClickHandler(`/issues/${issue.exceptionHash}`)}
									>
										<Table.Cell class="py-3 font-mono text-sm" title={issue.stackTrace}>
											{truncateStackTrace(issue.stackTrace)}
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
						<div class="flex flex-col items-center justify-center py-12 px-6 text-center">
							<div class="flex h-12 w-12 items-center justify-center rounded-full mb-4">
								<CircleCheck class="h-12 w-12 text-green-500 dark:text-green-400" />
							</div>
							<h3 class="text-lg font-semibold mb-2">No Issues Found</h3>
							<p class="text-sm text-muted-foreground max-w-sm mb-4">
								No Issues have been recorded in the last 24 hours. When issues occur in your application, they will appear here for quick triage.
							</p>
							<a
								href="/issues"
								class="text-sm font-medium text-primary hover:underline inline-flex items-center gap-1"
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

<style>
	/* Light theme - override dark theme defaults */
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
</style>
