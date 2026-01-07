<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import * as Chart from "$lib/components/ui/chart/index.js";
	import { TrendingUp, TrendingDown, Minus } from 'lucide-svelte';
	import { AreaChart, LineChart, Svg, Area } from 'layerchart';
	import { scaleUtc, scaleLinear } from 'd3-scale';
	import { curveLinear } from "d3-shape";
	import type { DashboardMetric, MetricTrendPoint } from '$lib/types/dashboard';
	import { min, max } from 'd3-array';

	let { metric, timeDomain = null } = $props<{
		metric: DashboardMetric;
		timeDomain?: [Date, Date] | null;
	}>();

	const statusColors: Record<string, string> = {
		healthy: 'bg-green-500',
		warning: 'bg-yellow-500',
		critical: 'bg-red-500'
	};

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

		// Memory (MB)
		if (unit === 'MB') {
			if (value >= 1024) return (value / 1024).toFixed(1);
			return Math.round(value).toString();
		}

		// Default: round to 1 decimal
		if (Number.isInteger(value)) return value.toString();
		return value.toFixed(1);
	}

	const formattedValue = $derived(
		metric.formatValue
			? metric.formatValue(metric.value)
			: formatMetricValue(metric.value, metric.unit)
	);

	const TrendChangeIcon = $derived(
		metric.change24h > 0 ? TrendingUp : metric.change24h < 0 ? TrendingDown : Minus
	);

	const trendChangeColor = $derived(
		metric.change24h > 0
			? 'text-green-600 dark:text-green-400'
			: metric.change24h < 0
				? 'text-red-600 dark:text-red-400'
				: 'text-muted-foreground'
	);

	const chartConfig = {
		value: { label: "Value", color: "var(--chart-1)" },
	} satisfies Chart.ChartConfig;

	const yMin = $derived(min(metric.trend, (d: MetricTrendPoint) => d.value) ?? 0);
	const yMax = $derived(max(metric.trend, (d: MetricTrendPoint) => d.value) ?? 0);
	const padding = $derived((yMax - yMin) * 0.1 || 1);

	// Calculate xScale with shared time domain
	const xScaleWithDomain = $derived(() => {
		const scale = scaleUtc();
		if (timeDomain) {
			return scale.domain(timeDomain);
		}
		// Fallback to data range
		if (metric.trend.length > 0) {
			const minTime = min(metric.trend, (d: MetricTrendPoint) => d.timestamp);
			const maxTime = max(metric.trend, (d: MetricTrendPoint) => d.timestamp);
			if (minTime && maxTime) {
				return scale.domain([minTime, maxTime]);
			}
		}
		return scale;
	});

	// Detect expected interval and filter out gaps for sparse data
	const expectedIntervalMs = $derived(() => {
		if (metric.trend.length < 2) return 3600000; // 1 hour default
		const intervals: number[] = [];
		for (let i = 1; i < Math.min(metric.trend.length, 10); i++) {
			intervals.push(metric.trend[i].timestamp.getTime() - metric.trend[i - 1].timestamp.getTime());
		}
		// Use median interval * 2 as threshold
		intervals.sort((a, b) => a - b);
		const median = intervals[Math.floor(intervals.length / 2)];
		return median * 2;
	});

	// Split data into continuous segments to avoid drawing lines across gaps
	function splitIntoSegments(data: MetricTrendPoint[], maxGapMs: number): MetricTrendPoint[][] {
		if (data.length === 0) return [];
		const segments: MetricTrendPoint[][] = [];
		let current: MetricTrendPoint[] = [data[0]];

		for (let i = 1; i < data.length; i++) {
			const gap = data[i].timestamp.getTime() - data[i - 1].timestamp.getTime();
			if (gap <= maxGapMs) {
				current.push(data[i]);
			} else {
				if (current.length > 0) segments.push(current);
				current = [data[i]];
			}
		}
		if (current.length > 0) segments.push(current);
		return segments;
	}

	const dataSegments = $derived(splitIntoSegments(metric.trend, expectedIntervalMs()));
	const hasData = $derived(metric.trend.length > 0 && metric.trend.some((d: MetricTrendPoint) => d.value !== 0));
</script>

<Card.Root class="gap-3">
	<Card.Header class="flex flex-row items-center justify-between space-y-0 pb-0">
		<Card.Title class="text-sm font-medium">
			{metric.name}
		</Card.Title>
		<div class="text-2xl font-bold">
			{formattedValue}{#if metric.unit}<span class="ml-1 text-lg text-muted-foreground"
					>{metric.unit}</span
				>{/if}
		</div>
		<!-- <div class={`h-2 w-2 rounded-full ${statusColors[metric.status]}`} title={metric.status}></div> -->
	</Card.Header>
	<Card.Content class="pt-0">

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

			<Chart.Container config={chartConfig}>
				{#if hasData}
					<LineChart
						data={metric.trend}
						x="timestamp"
						xScale={xScaleWithDomain()}
						series={[
							{
								key: "value",
								label: "Value",
								color: chartConfig.value.color,
							},
						]}
						yDomain={[Math.max(0, yMin - padding), yMax + padding]}
						seriesLayout="stack"
						props={{
							xAxis: {
								format: () => ""
							},
							yAxis: {
								format: (a: number) => a > 999 ? (a/1000).toFixed(0) + "k" : `${a}`,
							},
						}}
					>
						{#snippet tooltip()}
							<Chart.Tooltip hideLabel />
						{/snippet}
					</LineChart>
				{:else}
					<div class="flex h-[100px] items-center justify-center text-sm text-muted-foreground">
						No data in this period
					</div>
				{/if}
			</Chart.Container>

			<!-- 24h Change -->
			<div class="flex items-center text-xs {trendChangeColor}">
				<TrendChangeIcon class="mr-1 h-3 w-3" />
				<span class="font-medium">{Math.abs(metric.change24h).toFixed(1)}%</span>
				<span class="ml-1 text-muted-foreground">vs 24h ago</span>
			</div>
	</Card.Content>
</Card.Root>
