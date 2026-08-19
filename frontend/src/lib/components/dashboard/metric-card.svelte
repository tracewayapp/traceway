<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import type { DashboardMetric, MetricTrendPoint, ServerMetricTrend } from '$lib/types/dashboard';
	import { min, max } from 'd3-array';
	import D3LineChart from './d3-line-chart.svelte';
	import { formatMetricLabel } from '$lib/utils/metric-format';

	let {
		metric,
		timeDomain = null,
		onRangeSelect,
		serverColorMap = {},
		sharedHoverTime = null,
		isSourceChart = false,
		onHoverTimeChange
	} = $props<{
		metric: DashboardMetric;
		timeDomain?: [Date, Date] | null;
		onRangeSelect?: (from: Date, to: Date) => void;
		serverColorMap?: Record<string, string>;
		sharedHoverTime?: Date | null;
		isSourceChart?: boolean;
		onHoverTimeChange?: (time: Date | null) => void;
	}>();

	// Check if we have multi-server data
	const hasMultiServerData = $derived(metric.servers && metric.servers.length > 1);

	// Calculate X domain from timeDomain or data
	const xDomainValue = $derived(() => {
		if (timeDomain) {
			return timeDomain;
		}
		if (metric.trend.length > 0) {
			const minTime = min(metric.trend, (d: MetricTrendPoint) => d.timestamp);
			const maxTime = max(metric.trend, (d: MetricTrendPoint) => d.timestamp);
			if (minTime && maxTime) {
				return [minTime, maxTime] as [Date, Date];
			}
		}
		return undefined;
	});

	// Build series array for the chart
	const chartSeries = $derived(() => {
		if (hasMultiServerData && metric.servers) {
			return metric.servers.map((server: ServerMetricTrend) => ({
				key: server.serverName,
				data: server.trend,
				color: serverColorMap[server.serverName] || '#3b82f6'
			}));
		}
		return [
			{
				key: 'value',
				data: metric.trend,
				color: '#3b82f6'
			}
		];
	});

	const hasData = $derived(() => {
		if (hasMultiServerData && metric.servers) {
			return metric.servers.some((s: ServerMetricTrend) => s.trend.length > 0);
		}
		return metric.trend.length > 0;
	});
</script>

<Card.Root class="gap-0 pb-0">
	<Card.Header class="pb-0">
		<Card.Title class="text-sm font-medium">
			{metric.name}
		</Card.Title>
	</Card.Header>
	<Card.Content class="p-1 pt-0">
		{#if hasData()}
			<D3LineChart
				series={chartSeries()}
				xDomain={xDomainValue()}
				height={220}
				padding={{ top: 10, right: 4, bottom: 20, left: 45 }}
				{onRangeSelect}
				data={metric.trend}
				servers={metric.servers}
				{serverColorMap}
				unit={metric.unit}
				formatValue={(v) =>
					metric.formatValue ? metric.formatValue(v) : formatMetricLabel(v, metric.unit)}
				{sharedHoverTime}
				{isSourceChart}
				{onHoverTimeChange}
			/>
		{:else}
			<div class="flex h-[220px] items-center justify-center text-sm text-muted-foreground">
				No data in this period
			</div>
		{/if}
	</Card.Content>
</Card.Root>
