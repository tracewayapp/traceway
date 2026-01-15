<script lang="ts">
	import { onMount } from 'svelte';
	import { createRowClickHandler } from '$lib/utils/navigation';
	import { formatDuration, formatRelativeTime, truncateStackTrace } from '$lib/utils/formatters';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import * as Table from '$lib/components/ui/table';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import { ArrowRight, Info, Gauge, Bug, CircleQuestionMark } from 'lucide-svelte';
	import { TracewayTableHeader } from '$lib/components/ui/traceway-table-header';
	import { ImpactBadge } from '$lib/components/ui/impact-badge';
	import { TableEmptyState } from '$lib/components/ui/table-empty-state';
	import { ViewAllTableRow } from '$lib/components/ui/view-all-table-row';
	import { api } from '$lib/api';
	import { ErrorDisplay } from '$lib/components/ui/error-display';
	import { projectsState } from '$lib/state/projects.svelte';

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

	// Calculate impact level based on call volume and response time variance
	// Returns: 'critical' | 'high' | 'medium' | null (null = not significant)
	function getImpactLevel(
		count: number,
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
					<h2 class="text-2xl font-bold tracking-tight">Transaction</h2>
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
										tooltip="Priority based on traffic × variance"
										align="right"
										class="w-[80px]"
									/>
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
											<ImpactBadge level={impactLevel} />
										</Table.Cell>
									</Table.Row>
								{/each}
								<ViewAllTableRow colspan={5} href="/transactions" label="View all transactions" />
							</Table.Body>
						{:else}
							<Table.Body>
								<TableEmptyState colspan={5} message="No transaction data received yet" />
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
						{:else}
							<Table.Body>
								<TableEmptyState colspan={3} message="No issues in the last 24 hours" />
							</Table.Body>
						{/if}
					</Table.Root>
				</div>
			</div>
		</div>
	{/if}
</div>
