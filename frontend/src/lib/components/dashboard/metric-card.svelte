<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import * as Chart from "$lib/components/ui/chart/index.js";
	import { LineChart, Points } from 'layerchart';
	import { scaleUtc } from 'd3-scale';
	import type { DashboardMetric, MetricTrendPoint, ServerMetricTrend } from '$lib/types/dashboard';
	import { min, max } from 'd3-array';
	import MetricChartOverlay from './metric-chart-overlay.svelte';

	let { metric, timeDomain = null, onRangeSelect, serverColorMap = {} } = $props<{
		metric: DashboardMetric;
		timeDomain?: [Date, Date] | null;
		onRangeSelect?: (from: Date, to: Date) => void;
		serverColorMap?: Record<string, string>;
	}>();

	// Check if we have multi-server data
	const hasMultiServerData = $derived(
		metric.servers && metric.servers.length > 1
	);


	// Smart number formatting based on unit type
	function formatMetricValue(value: number, unit: string): string {
		// Percentages
		if (unit === '%') {
			if (value === 0) return '0';
			if (Math.abs(value) < 0.1) return value.toFixed(2);
			if (Math.abs(value) < 10) return value.toFixed(1);
			return Math.round(value).toString();
		}

		// Durations (ms)
		if (unit === 'ms') {
			if (value < 1) return (value * 1000).toFixed(0);
			if (value < 10) return value.toFixed(1);
			if (value < 1000) return Math.round(value).toString();
			return (value / 1000).toFixed(1);
		}

		// Counts
		if (unit === 'count' || unit === '') {
			if (value >= 1_000_000) return (value / 1_000_000).toFixed(1) + 'M';
			if (value >= 1_000) return (value / 1_000).toFixed(1) + 'K';
			return Math.round(value).toString();
		}

		// Memory (MB) - convert to GB for large values
		if (unit === 'MB') {
			if (value >= 1024) return (value / 1024).toFixed(1) + ' GB';
			return Math.round(value).toString() + ' MB';
		}

		// Default: round to 1 decimal
		if (Number.isInteger(value)) return value.toString();
		return value.toFixed(1);
	}

	// Generate chart config based on servers or single value
	const chartConfig = $derived(() => {
		if (hasMultiServerData && metric.servers) {
			const config: Chart.ChartConfig = {};
			for (const server of metric.servers) {
				config[server.serverName] = {
					label: server.serverName,
					color: serverColorMap[server.serverName] || 'var(--chart-1)'
				};
			}
			return config;
		}
		return {
			value: { label: "Value", color: "var(--chart-1)" }
		} satisfies Chart.ChartConfig;
	});

	// Calculate yMin/yMax considering multi-server data
	const yMin = $derived((): number => {
		if (hasMultiServerData && metric.servers) {
			const allValues = metric.servers.flatMap((s: ServerMetricTrend) => s.trend.map((t: MetricTrendPoint) => t.value));
			const minVal = min(allValues);
			return typeof minVal === 'number' ? minVal : 0;
		}
		const minVal = min(metric.trend, (d: MetricTrendPoint) => d.value);
		return typeof minVal === 'number' ? minVal : 0;
	});
	const yMax = $derived((): number => {
		if (hasMultiServerData && metric.servers) {
			const allValues = metric.servers.flatMap((s: ServerMetricTrend) => s.trend.map((t: MetricTrendPoint) => t.value));
			const maxVal = max(allValues);
			return typeof maxVal === 'number' ? maxVal : 0;
		}
		const maxVal = max(metric.trend, (d: MetricTrendPoint) => d.value);
		return typeof maxVal === 'number' ? maxVal : 0;
	});
	const padding = $derived((yMax() - yMin()) * 0.1 || 1);

	// Calculate X domain from timeDomain or data
	const xDomainValue = $derived(() => {
		if (timeDomain) {
			return timeDomain;
		}
		// Fallback to data range
		if (metric.trend.length > 0) {
			const minTime = min(metric.trend, (d: MetricTrendPoint) => d.timestamp);
			const maxTime = max(metric.trend, (d: MetricTrendPoint) => d.timestamp);
			if (minTime && maxTime) {
				return [minTime, maxTime] as [Date, Date];
			}
		}
		return undefined;
	});

	// Calculate expected interval from actual data and use 2x as gap threshold
	const gapThresholdMs = $derived(() => {
		if (metric.trend.length < 2) return 3600000; // 1 hour default
		const intervals: number[] = [];
		for (let i = 1; i < Math.min(metric.trend.length, 10); i++) {
			intervals.push(metric.trend[i].timestamp.getTime() - metric.trend[i - 1].timestamp.getTime());
		}
		intervals.sort((a, b) => a - b);
		const median = intervals[Math.floor(intervals.length / 2)];
		return median * 2; // Gap threshold = 2x median interval
	});

	// Create lookup set for gap points - marks points that have a gap before them
	const gapPoints = $derived(() => {
		const gaps = new Set<number>();
		const threshold = gapThresholdMs();
		for (let i = 1; i < metric.trend.length; i++) {
			const gap = metric.trend[i].timestamp.getTime() - metric.trend[i - 1].timestamp.getTime();
			if (gap > threshold) {
				// Mark the point AFTER the gap as "undefined" to break the line
				gaps.add(metric.trend[i].timestamp.getTime());
			}
		}
		return gaps;
	});

	// Function to determine if a point should be connected to the previous point
	// A point is "defined" (line should be drawn TO it) if there's no gap before it
	function isDefined(d: MetricTrendPoint): boolean {
		return !gapPoints().has(d.timestamp.getTime());
	}

	// Calculate isolated points - points that have gaps BOTH before AND after them
	// These are the only points that should show dots
	const isolatedPoints = $derived(() => {
		if (metric.trend.length === 0) return [];
		if (metric.trend.length === 1) return metric.trend; // Single point is always isolated

		const threshold = gapThresholdMs();
		const isolated: MetricTrendPoint[] = [];

		for (let i = 0; i < metric.trend.length; i++) {
			const hasGapBefore = i === 0 ||
				(metric.trend[i].timestamp.getTime() - metric.trend[i - 1].timestamp.getTime() > threshold);
			const hasGapAfter = i === metric.trend.length - 1 ||
				(metric.trend[i + 1].timestamp.getTime() - metric.trend[i].timestamp.getTime() > threshold);

			if (hasGapBefore && hasGapAfter) {
				isolated.push(metric.trend[i]);
			}
		}

		return isolated;
	});

	const hasData = $derived(() => {
		if (hasMultiServerData && metric.servers) {
			return metric.servers.some((s: ServerMetricTrend) => s.trend.length > 0 && s.trend.some((t: MetricTrendPoint) => t.value !== 0));
		}
		return metric.trend.length > 0 && metric.trend.some((d: MetricTrendPoint) => d.value !== 0);
	});

	// Build merged data and series for multi-server charts
	const multiServerChartData = $derived(() => {
		if (!hasMultiServerData || !metric.servers) return [];

		// Merge all server data by timestamp
		const timeMap = new Map<number, Record<string, number>>();

		for (const server of metric.servers) {
			for (const point of server.trend) {
				const timestamp = point.timestamp.getTime();
				if (!timeMap.has(timestamp)) {
					timeMap.set(timestamp, { timestamp } as any);
				}
				timeMap.get(timestamp)![server.serverName] = point.value;
			}
		}

		// Convert to sorted array
		return Array.from(timeMap.values())
			.map(entry => ({
				...entry,
				timestamp: new Date(entry.timestamp as unknown as number)
			}))
			.sort((a, b) => a.timestamp.getTime() - b.timestamp.getTime());
	});

	const multiServerSeries = $derived(() => {
		if (!hasMultiServerData || !metric.servers) return [];
		return metric.servers.map((server: ServerMetricTrend) => ({
			key: server.serverName,
			label: server.serverName,
			color: serverColorMap[server.serverName] || 'var(--chart-1)'
		}));
	});
</script>

<Card.Root class="gap-3 pb-0">
	<Card.Header class="pb-2">
		<Card.Title class="text-sm font-medium">
			{metric.name}
		</Card.Title>
	</Card.Header>
	<Card.Content class="p-1 pt-0 pl-3">

			<!-- Large Value Display -->

			<!-- Sparkline Chart -->

				<!-- <Chart
					data={metric.trend}
					x={(d: MetricTrendPoint) => d.timestamp}
					xScale={scaleUtc()}
					y={(d: MetricTrendPoint) => d.value}
					yScale={scaleLinear()}
					padding={{ top: 4, bottom: 4, left: 0, right: 0 }}
				>
					<Svg>
						<Area
							line={{ stroke: 'hsl(var(--chart-1))', 'stroke-width': 2 }}
							area={{ fill: 'none' }}
						/>
					</Svg>
				</Chart> -->

			<MetricChartOverlay
				fromTime={xDomainValue()?.[0] ?? new Date()}
				toTime={xDomainValue()?.[1] ?? new Date()}
				{onRangeSelect}
				data={metric.trend}
				unit={metric.unit}
				formatValue={(v) => metric.formatValue ? metric.formatValue(v) : formatMetricValue(v, metric.unit)}
			>
				<Chart.Container config={chartConfig()}>
					{#if hasData()}
						{#if hasMultiServerData}
							<!-- Multi-server chart with separate lines per server -->
							<LineChart
								data={multiServerChartData()}
								x="timestamp"
								xScale={scaleUtc()}
								xDomain={xDomainValue()}
								series={multiServerSeries()}
								yDomain={[Math.max(0, yMin() - padding), yMax() + padding]}
								seriesLayout="overlap"
								props={{
									xAxis: {
										format: () => ""
									},
									yAxis: {
										format: (a: number) => a > 999 ? (a/1000).toFixed(0) + "k" : `${a}`,
									}
								}}
								tooltip={false}
							/>
						{:else}
							<!-- Single series chart (original behavior) -->
							<LineChart
								data={metric.trend}
								x="timestamp"
								xScale={scaleUtc()}
								xDomain={xDomainValue()}
								series={[
									{
										key: "value",
										label: "Value",
										color: "var(--chart-1)",
									},
								]}
								yDomain={[Math.max(0, yMin() - padding), yMax() + padding]}
								seriesLayout="stack"
								props={{
									xAxis: {
										format: () => "",
									},
									yAxis: {
										format: (a: number) => a > 999 ? (a/1000).toFixed(0) + "k" : `${a}`,
									},
									spline: {
										defined: isDefined
									}
								}}
								tooltip={false}
							>
								{#snippet aboveMarks()}
									<!-- Isolated points (dots only where no line) -->
									{#if isolatedPoints().length > 0}
										<Points
											data={isolatedPoints()}
											x="timestamp"
											y="value"
											r={2}
											fill="var(--chart-1)"
										/>
									{/if}
								{/snippet}
							</LineChart>
						{/if}
					{:else}
						<div class="flex h-[100px] items-center justify-center text-sm text-muted-foreground">
							No data in this period
						</div>
					{/if}
				</Chart.Container>
			</MetricChartOverlay>
	</Card.Content>
</Card.Root>
