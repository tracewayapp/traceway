<script lang="ts">
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { ErrorDisplay } from '$lib/components/ui/error-display';
	import MetricCard from './metric-card.svelte';
	import type { DashboardMetric, ServerMetricTrend, MetricTrendPoint } from '$lib/types/dashboard';

	let {
		metrics = null,
		loading = false,
		error = '',
		errorTitle = 'Failed to Load Metrics',
		onRetry,
		timeDomain = null,
		onRangeSelect,
		serverColorMap = {},
		selectedServers = [],
		availableServers = []
	} = $props<{
		metrics: DashboardMetric[] | null;
		loading: boolean;
		error: string;
		errorTitle?: string;
		onRetry: () => void;
		timeDomain: [Date, Date] | null;
		onRangeSelect?: (from: Date, to: Date) => void;
		serverColorMap: Record<string, string>;
		selectedServers: string[];
		availableServers: string[];
	}>();

	// Marker for "none selected" state (must match server-filter.svelte)
	const NONE_SELECTED = '__none__';

	// Client-side filtering: Filter metrics by selected servers
	function filterMetricsByServer(metricsToFilter: DashboardMetric[]): DashboardMetric[] {
		// Check for "none selected" state
		const isNoneSelected = selectedServers.length === 1 && selectedServers[0] === NONE_SELECTED;

		// If no servers selected or all servers selected, show all data
		if (!isNoneSelected && (selectedServers.length === 0 || selectedServers.length === availableServers.length)) {
			return metricsToFilter;
		}

		return metricsToFilter.map(metric => {
			// If metric doesn't have server breakdown, return as-is
			if (!metric.servers || metric.servers.length === 0) {
				return metric;
			}

			// Filter to only selected servers
			const filteredServers = metric.servers.filter(
				s => selectedServers.includes(s.serverName)
			);

			// Recalculate aggregate value from filtered servers
			const newValue = filteredServers.length > 0
				? filteredServers.reduce((sum, s) => sum + s.value, 0) / filteredServers.length
				: 0;

			// Recalculate aggregate trend from filtered servers
			const newTrend = recalculateAggregateTrend(filteredServers);

			return {
				...metric,
				servers: filteredServers,
				value: newValue,
				trend: newTrend
			};
		});
	}

	// Helper to recalculate aggregate trend from filtered server data
	function recalculateAggregateTrend(servers: ServerMetricTrend[]): MetricTrendPoint[] {
		if (servers.length === 0) return [];

		const timestampValues = new Map<number, number[]>();
		for (const server of servers) {
			for (const point of server.trend) {
				const ts = point.timestamp.getTime();
				if (!timestampValues.has(ts)) timestampValues.set(ts, []);
				timestampValues.get(ts)!.push(point.value);
			}
		}

		return Array.from(timestampValues.entries())
			.sort(([a], [b]) => a - b)
			.map(([ts, values]) => ({
				timestamp: new Date(ts),
				value: values.reduce((a, b) => a + b, 0) / values.length
			}));
	}

	// Derived filtered metrics
	const filteredMetrics = $derived(
		metrics ? filterMetricsByServer(metrics) : []
	);

	// Hover state coordination for shadow tooltip lines across charts
	let currentHoverTime = $state<Date | null>(null);
	let currentHoverChartId = $state<string | null>(null);

	function handleChartHoverChange(chartId: string, time: Date | null) {
		if (time !== null) {
			currentHoverTime = time;
			currentHoverChartId = chartId;
		} else if (currentHoverChartId === chartId) {
			// Only clear if the chart that's leaving is the current source
			currentHoverTime = null;
			currentHoverChartId = null;
		}
	}
</script>

{#if error && !loading}
	<ErrorDisplay
		status={400}
		title={errorTitle}
		description={error}
		{onRetry}
	/>
{:else if loading}
	<div class="flex items-center justify-center py-20">
		<LoadingCircle size="xlg" />
	</div>
{:else if filteredMetrics.length > 0}
	<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
		{#each filteredMetrics as metric (metric.id)}
			<MetricCard
				{metric}
				{timeDomain}
				{onRangeSelect}
				{serverColorMap}
				sharedHoverTime={currentHoverTime}
				isSourceChart={currentHoverChartId === metric.id}
				onHoverTimeChange={(time) => handleChartHoverChange(metric.id, time)}
			/>
		{/each}
	</div>
{:else}
	<div class="flex items-center justify-center py-20 text-muted-foreground">
		No metrics data available
	</div>
{/if}
