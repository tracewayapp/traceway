<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import * as Chart from "$lib/components/ui/chart/index.js";
	import { TrendingUp, TrendingDown, Minus } from 'lucide-svelte';
	import { AreaChart, LineChart, Svg, Area } from 'layerchart';
	import { scaleUtc, scaleLinear } from 'd3-scale';
	import { curveLinear } from "d3-shape";
	import type { DashboardMetric, MetricTrendPoint } from '$lib/types/dashboard';
	import { min, max } from 'd3-array';

	let { metric } = $props<{ metric: DashboardMetric }>();

	const statusColors: Record<string, string> = {
		healthy: 'bg-green-500',
		warning: 'bg-yellow-500',
		critical: 'bg-red-500'
	};

	const formattedValue = $derived(
		metric.formatValue ? metric.formatValue(metric.value) : metric.value.toString()
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

	const yMin = $derived(min(metric.trend, (d: MetricTrendPoint)  => d.value) ?? 0);
  const yMax = $derived(max(metric.trend, (d: MetricTrendPoint) => d.value) ?? 0);
  const padding = $derived((yMax - yMin) * 0.1);
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
				<LineChart
					data={metric.trend}
					x="timestamp"
					xScale={scaleUtc()}
					series={[
						{
							key: "value",
							label: "Value",
							color: chartConfig.value.color,
						},
					]}
					yDomain={[yMin - padding, yMax + padding]}
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
			</Chart.Container>

			<!-- 24h Change -->
			<div class="flex items-center text-xs {trendChangeColor}">
				<TrendChangeIcon class="mr-1 h-3 w-3" />
				<span class="font-medium">{Math.abs(metric.change24h).toFixed(1)}%</span>
				<span class="ml-1 text-muted-foreground">vs 24h ago</span>
			</div>
	</Card.Content>
</Card.Root>
