<script lang="ts">
	import { onMount } from 'svelte';
	import { createRowClickHandler } from '$lib/utils/navigation';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import * as Table from '$lib/components/ui/table';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import { ArrowRight, Info, Gauge, Bug, CircleQuestionMark, TriangleAlert } from 'lucide-svelte';
	import { api } from '$lib/api';
	import { ErrorDisplay } from '$lib/components/ui/error-display';
	import { projectsState } from '$lib/state/projects.svelte';

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
	};

	type DashboardOverview = {
		recentIssues: ExceptionGroup[];
		worstEndpoints: EndpointStats[];
	};

	let data = $state<DashboardOverview | null>(null);
	let loading = $state(true);
	let error = $state('');
	let errorStatus = $state<number>(0);

	async function loadDashboard() {
		loading = true;
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
			loading = false;
		}
	}

	onMount(() => {
		loadDashboard();
	});

	function formatRelativeTime(dateStr: string): string {
		const date = new Date(dateStr);
		const now = new Date();
		const diffMs = now.getTime() - date.getTime();
		const diffMins = Math.floor(diffMs / 60000);
		const diffHours = Math.floor(diffMs / 3600000);
		const diffDays = Math.floor(diffMs / 86400000);

		if (diffMins < 1) return 'just now';
		if (diffMins < 60) return `${diffMins}m`;
		if (diffHours < 24) return `${diffHours}h`;
		return `${diffDays}d`;
	}

	function formatDuration(nanoseconds: number): string {
		const ms = nanoseconds / 1_000_000;
		if (ms < 1) {
			return `${(nanoseconds / 1000).toFixed(0)}µs`;
		} else if (ms < 1000) {
			return `${ms.toFixed(0)}ms`;
		} else {
			return `${(ms / 1000).toFixed(1)}s`;
		}
	}

	// Calculate impact level based on call volume and response time variance
	// Returns: 'critical' | 'high' | 'medium' | null (null = not significant)
	function getImpactLevel(
		p50: number,
		p95: number
	): 'critical' | 'high' | 'medium' | null {
		const varianceMs = (p95 - p50) / 1_000_000;
		const score = count * varianceMs;
		if (score > 100) return 'critical';
		if (score > 10) return 'high';
		if (score > 1) return 'medium';
		return null;
	}

	function truncateStackTrace(stackTrace: string): string {
		const firstLine = stackTrace.split('\n')[0];
		if (firstLine.length > 70) {
			return firstLine.slice(0, 70) + '...';
		}
		return firstLine;
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
								Most impactful endpoints to optimize based on traffic and response time variance
							</p>
						</Tooltip.Content>
					</Tooltip.Root>
				</div>
				<div class="overflow-hidden rounded-md border">
					<Table.Root>
						{#if data?.worstEndpoints && data.worstEndpoints.length > 0}
							<Table.Header>
								<Table.Row class="hover:bg-transparent">
									<Table.Head class="h-10 text-sm">
										<span class="flex items-center gap-1.5">
											Endpoint
											<Tooltip.Root>
												<Tooltip.Trigger>
													<CircleQuestionMark class="h-3.5 w-3.5 text-muted-foreground/60" />
												</Tooltip.Trigger>
												<Tooltip.Content>
													<p class="text-xs">The API route or page being accessed</p>
												</Tooltip.Content>
											</Tooltip.Root>
										</span>
									</Table.Head>
									<Table.Head class="h-10 w-[70px] text-right text-sm">
										<span class="flex items-center justify-end gap-1.5">
											Calls
											<Tooltip.Root>
												<Tooltip.Trigger>
													<CircleQuestionMark class="h-3.5 w-3.5 text-muted-foreground/60" />
												</Tooltip.Trigger>
												<Tooltip.Content>
													<p class="text-xs">Total number of requests</p>
												</Tooltip.Content>
											</Tooltip.Root>
										</span>
									</Table.Head>
									<Table.Head class="h-10 w-[80px] text-right text-sm">
										<span class="flex items-center justify-end gap-1.5">
											Typical
											<Tooltip.Root>
												<Tooltip.Trigger>
													<CircleQuestionMark class="h-3.5 w-3.5 text-muted-foreground/60" />
												</Tooltip.Trigger>
												<Tooltip.Content>
													<p class="text-xs">Median response time (P50)</p>
												</Tooltip.Content>
											</Tooltip.Root>
										</span>
									</Table.Head>
									<Table.Head class="h-10 w-[70px] text-right text-sm">
										<span class="flex items-center justify-end gap-1.5">
											Slow
											<Tooltip.Root>
												<Tooltip.Trigger>
													<CircleQuestionMark class="h-3.5 w-3.5 text-muted-foreground/60" />
												</Tooltip.Trigger>
												<Tooltip.Content>
													<p class="text-xs">95th percentile - slowest 5%</p>
												</Tooltip.Content>
											</Tooltip.Root>
										</span>
									</Table.Head>
									<Table.Head class="h-10 w-[80px] text-right text-sm">
										<span class="flex items-center justify-end gap-1.5">
											Impact
											<Tooltip.Root>
												<Tooltip.Trigger>
													<CircleQuestionMark class="h-3.5 w-3.5 text-muted-foreground/60" />
												</Tooltip.Trigger>
												<Tooltip.Content>
													<p class="text-xs">Priority based on traffic × variance</p>
												</Tooltip.Content>
											</Tooltip.Root>
										</span>
									</Table.Head>
								</Table.Row>
							</Table.Header>
							<Table.Body>
								{#each data.worstEndpoints as endpoint}
									{@const impactLevel = getImpactLevel(
										endpoint.count,
										endpoint.p50Duration,
										endpoint.p95Duration
									)}
									<Table.Row
										class="cursor-pointer hover:bg-muted/50"
										onclick={createRowClickHandler(
											`/transactions/${encodeURIComponent(endpoint.endpoint)}?preset=24h`
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
											{#if impactLevel === 'critical'}
												<span
													class="inline-flex items-center gap-1 rounded-full bg-red-500/15 px-2 py-0.5 text-xs font-medium text-red-600 dark:text-red-400"
												>
													<TriangleAlert class="h-3 w-3" />
													Critical
												</span>
											{:else if impactLevel === 'high'}
												<span
													class="inline-flex items-center gap-1 rounded-full bg-orange-500/15 px-2 py-0.5 text-xs font-medium text-orange-600 dark:text-orange-400"
												>
													<TriangleAlert class="h-3 w-3" />
													High
												</span>
											{:else if impactLevel === 'medium'}
												<span
													class="inline-flex items-center gap-1 rounded-full bg-yellow-500/15 px-2 py-0.5 text-xs font-medium text-yellow-600 dark:text-yellow-500"
												>
													Medium
												</span>
											{/if}
										</Table.Cell>
									</Table.Row>
								{/each}
								<Table.Row
									class="cursor-pointer bg-muted/50 hover:bg-muted"
									onclick={createRowClickHandler('/transactions')}
								>
									<Table.Cell colspan={5} class="py-2 text-center text-sm text-muted-foreground">
										View all transactions <ArrowRight class="inline h-3.5 w-3.5" />
									</Table.Cell>
								</Table.Row>
							</Table.Body>
						{:else}
							<Table.Body>
								<Table.Row>
									<Table.Cell colspan={5} class="h-24 text-center text-muted-foreground">
										No transaction data received yet
									</Table.Cell>
								</Table.Row>
							</Table.Body>
						{/if}
					</Table.Root>
				</div>
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
				<div class="overflow-hidden rounded-md border">
					<Table.Root>
						{#if data?.recentIssues && data.recentIssues.length > 0}
							<Table.Header>
								<Table.Row class="hover:bg-transparent">
									<Table.Head class="h-10 text-sm">
										<span class="flex items-center gap-1.5">
											Issue
											<Tooltip.Root>
												<Tooltip.Trigger>
													<CircleQuestionMark class="h-3.5 w-3.5 text-muted-foreground/60" />
												</Tooltip.Trigger>
												<Tooltip.Content>
													<p class="text-xs">The error message or exception that occurred</p>
												</Tooltip.Content>
											</Tooltip.Root>
										</span>
									</Table.Head>
									<Table.Head class="h-10 w-[70px] text-right text-sm">
										<span class="flex items-center justify-end gap-1.5">
											Count
											<Tooltip.Root>
												<Tooltip.Trigger>
													<CircleQuestionMark class="h-3.5 w-3.5 text-muted-foreground/60" />
												</Tooltip.Trigger>
												<Tooltip.Content>
													<p class="text-xs">Number of times this issue occurred</p>
												</Tooltip.Content>
											</Tooltip.Root>
										</span>
									</Table.Head>
									<Table.Head class="h-10 w-[70px] text-right text-sm">
										<span class="flex items-center justify-end gap-1.5">
											When
											<Tooltip.Root>
												<Tooltip.Trigger>
													<CircleQuestionMark class="h-3.5 w-3.5 text-muted-foreground/60" />
												</Tooltip.Trigger>
												<Tooltip.Content>
													<p class="text-xs">When this issue last occurred</p>
												</Tooltip.Content>
											</Tooltip.Root>
										</span>
									</Table.Head>
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
											{formatRelativeTime(issue.lastSeen)}
										</Table.Cell>
									</Table.Row>
								{/each}
								<Table.Row
									class="cursor-pointer bg-muted/50 hover:bg-muted"
									onclick={createRowClickHandler('/issues')}
								>
									<Table.Cell colspan={3} class="py-2 text-center text-sm text-muted-foreground">
										View all issues <ArrowRight class="inline h-3.5 w-3.5" />
									</Table.Cell>
								</Table.Row>
							</Table.Body>
						{:else}
							<Table.Body>
								<Table.Row>
									<Table.Cell colspan={3} class="h-24 text-center text-muted-foreground">
										No issues in the last 24 hours
									</Table.Cell>
								</Table.Row>
							</Table.Body>
						{/if}
					</Table.Root>
				</div>
			</div>
		</div>
	{/if}
</div>
