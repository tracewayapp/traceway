<script lang="ts">
	import D3LineChart from './d3-line-chart.svelte';
	import D3HorizontalBarChart from './d3-horizontal-bar-chart.svelte';
	import D3StackedAreaChart from './d3-stacked-area-chart.svelte';
	import D3Gauge from './d3-gauge.svelte';
	import WidgetTable from './widget-table.svelte';
	import Sparkline from './sparkline.svelte';
	import { activeThresholdColor, type ThresholdStep } from './gauge-thresholds';
	import { SvelteSet } from 'svelte/reactivity';
	import { api } from '$lib/api';
	import { projectsState } from '$lib/state/projects.svelte';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import type { MetricTrendPoint, MetricQueryResponse } from '$lib/types/dashboard';
	import { formatMetricLabel, formatMetricValue } from '$lib/utils/metric-format';

	type WidgetSource = {
		type: 'metric';
		name: string;
		tagFilters?: Record<string, string>;
		aggregation: string;
		groupBy?: string;
		label?: string;
	};

	type WidgetConfig = {
		sources?: WidgetSource[];
		yAxisLabel?: string;
		showLegend?: boolean;
		unit?: string;
		colSpan?: 1 | 2 | 3;
		size?: 'sm' | 'md' | 'lg';
		showSparkline?: boolean;
		min?: number;
		max?: number;
		baseColor?: string;
		thresholds?: ThresholdStep[];
	};

	let {
		widget,
		fromDateUTC,
		toDateUTC,
		timeDomain = null,
		onRangeSelect,
		sharedHoverTime = null,
		isSourceChart = false,
		onHoverTimeChange
	} = $props<{
		widget: {
			id: number;
			title: string;
			widgetType: string;
			config: WidgetConfig;
		};
		fromDateUTC: string;
		toDateUTC: string;
		timeDomain: [Date, Date] | null;
		onRangeSelect?: (from: Date, to: Date) => void;
		sharedHoverTime?: Date | null;
		isSourceChart?: boolean;
		onHoverTimeChange?: (time: Date | null) => void;
	}>();

	let series = $state<Array<{ key: string; data: MetricTrendPoint[]; color: string }>>([]);
	let loading = $state(true);
	let singleValue = $state<number | null>(null);
	let resolvedUnit = $state('');

	const colors = [
		'var(--chart-1)',
		'var(--chart-2)',
		'var(--chart-3)',
		'var(--chart-4)',
		'var(--chart-5)',
		'var(--crit)'
	];

	const effectiveUnit = $derived(widget.config.unit ?? resolvedUnit);

	// Fallbacks for the first render; once mounted the chart fills the measured card space
	const chartHeights: Record<string, number> = { sm: 200, md: 300, lg: 420 };
	let chartAreaHeight = $state(0);
	const chartHeight = $derived(
		chartAreaHeight > 0 ? chartAreaHeight : (chartHeights[widget.config.size ?? 'sm'] ?? 200)
	);

	function reduceSeries(points: MetricTrendPoint[], aggregation: string): number | null {
		if (points.length === 0) return null;
		const values = points.map((p) => p.value);
		switch (aggregation) {
			case 'max':
				return Math.max(...values);
			case 'min':
				return Math.min(...values);
			case 'sum':
			case 'count':
				return values.reduce((a, b) => a + b, 0);
			case 'avg':
				return values.reduce((a, b) => a + b, 0) / values.length;
			default:
				return values[values.length - 1];
		}
	}

	let hiddenSeries = new SvelteSet<string>();
	const visibleSeries = $derived(series.filter((s) => !hiddenSeries.has(s.key)));

	function toggleSeries(key: string) {
		if (hiddenSeries.has(key)) {
			hiddenSeries.delete(key);
		} else {
			hiddenSeries.add(key);
		}
	}

	const singleValueColor = $derived(
		widget.widgetType === 'single_value' &&
			singleValue !== null &&
			(widget.config.thresholds?.length || widget.config.baseColor)
			? activeThresholdColor(singleValue, widget.config.baseColor, widget.config.thresholds)
			: null
	);

	const legendVisible = $derived(
		['line_chart', 'area_chart', 'stacked_area'].includes(widget.widgetType) &&
			series.length > 0 &&
			(widget.config.showLegend === true ||
				(widget.config.showLegend !== false && series.length > 1))
	);

	async function loadData() {
		const sources = widget.config.sources;
		if (!sources || sources.length === 0) {
			loading = false;
			return;
		}

		loading = true;
		try {
			const newSeries: Array<{ key: string; data: MetricTrendPoint[]; color: string }> = [];
			let colorIdx = 0;

			const queries = sources.map((s: WidgetSource) => ({
				name: s.name,
				aggregation: s.aggregation || 'avg',
				tagFilters: s.tagFilters,
				groupBy: s.groupBy
			}));

			const response: MetricQueryResponse = await api.post(
				'/metrics/query',
				{ queries, from: fromDateUTC, to: toDateUTC },
				{ projectId: projectsState.currentProjectId ?? undefined }
			);

			const units = new Set<string>();
			const usedKeys = new Set<string>();
			for (const [idx, result] of response.results.entries()) {
				if (result.unit) units.add(result.unit);
				const baseName = sources[idx]?.label?.trim() || result.name;
				for (const [key, points] of Object.entries(result.series)) {
					const label = Object.keys(result.series).length > 1 ? `${baseName} (${key})` : baseName;
					let uniqueKey = label;
					for (let n = 2; usedKeys.has(uniqueKey); n++) {
						uniqueKey = `${label} (${n})`;
					}
					usedKeys.add(uniqueKey);
					newSeries.push({
						key: uniqueKey,
						data: points.map((p) => ({
							timestamp: new Date(p.timestamp),
							value: p.value
						})),
						color: colors[colorIdx % colors.length]
					});
					colorIdx++;
				}
			}

			resolvedUnit = units.size === 1 ? [...units][0] : '';
			series = newSeries;

			if (widget.widgetType === 'single_value' || widget.widgetType === 'gauge') {
				const aggregation = widget.config.sources?.[0]?.aggregation || 'avg';
				singleValue = newSeries.length > 0 ? reduceSeries(newSeries[0].data, aggregation) : null;
			}
		} catch {
			// keep empty
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		if (fromDateUTC && toDateUTC) {
			loadData();
		}
	});

	const barData = $derived(
		series.map((s) => ({
			endpoint: s.key,
			value: s.data.length > 0 ? s.data[s.data.length - 1].value : 0
		}))
	);

	const stackedPoints = $derived(
		visibleSeries.flatMap((s) =>
			s.data.map((p) => ({ timestamp: p.timestamp, endpoint: s.key, value: p.value }))
		)
	);

	// Widen the y-axis gutter to fit the largest tick label (e.g. "139.7 GB"),
	// which the charts' fixed default would clip
	const axisMaxValue = $derived.by(() => {
		if (widget.widgetType === 'stacked_area') {
			const sums = new Map<number, number>();
			for (const s of visibleSeries) {
				for (const p of s.data) {
					const t = p.timestamp.getTime();
					sums.set(t, (sums.get(t) ?? 0) + p.value);
				}
			}
			return sums.size > 0 ? Math.max(...sums.values()) * 1.1 : 0;
		}
		let m = 0;
		for (const s of visibleSeries) {
			for (const p of s.data) {
				if (p.value > m) m = p.value;
			}
		}
		return m * 1.1;
	});

	const chartPadding = $derived.by(() => {
		// Ticks show the scaled number; the unit renders once above the axis
		const { text, unit: axisUnit } = formatMetricValue(axisMaxValue || 0, effectiveUnit);
		const chars = Math.max(text.length, axisUnit.length);
		return { top: 10, right: 4, bottom: 20, left: Math.max(45, Math.round(chars * 6.5) + 16) };
	});
</script>

<div class="flex h-full min-h-[200px] w-full flex-col">
	<div class="min-h-0 w-full flex-1" bind:clientHeight={chartAreaHeight}>
		{#if loading}
			<div class="flex h-full items-center justify-center">
				<LoadingCircle size="md" />
			</div>
		{:else if widget.widgetType === 'single_value'}
			<div class="flex h-full flex-col items-center justify-center gap-2">
				<span
					class="text-3xl font-bold"
					style={singleValueColor ? `color: ${singleValueColor};` : ''}
				>
					{singleValue !== null ? formatMetricLabel(singleValue, effectiveUnit) : '-'}
				</span>
				{#if widget.config.showSparkline && series[0]?.data && series[0].data.length > 1}
					<div class="w-full px-6">
						<Sparkline data={series[0].data} color={singleValueColor ?? series[0].color} />
					</div>
				{/if}
			</div>
		{:else if widget.widgetType === 'gauge'}
			{#if singleValue !== null}
				<D3Gauge
					value={singleValue}
					min={widget.config.min ?? 0}
					max={widget.config.max ?? 100}
					baseColor={widget.config.baseColor}
					thresholds={widget.config.thresholds}
					formatValue={(v) => formatMetricLabel(v, effectiveUnit)}
				/>
			{:else}
				<div class="flex h-full items-center justify-center text-sm text-muted-foreground">
					No data
				</div>
			{/if}
		{:else if widget.widgetType === 'bar_chart'}
			{#if barData.length > 0}
				<D3HorizontalBarChart
					data={barData}
					height={chartHeight}
					unit={effectiveUnit}
					formatValue={(v) => formatMetricLabel(v, effectiveUnit)}
				/>
			{:else}
				<div class="flex h-full items-center justify-center text-sm text-muted-foreground">
					No data
				</div>
			{/if}
		{:else if widget.widgetType === 'table'}
			{#if series.length > 0}
				<WidgetTable {series} unit={effectiveUnit} />
			{:else}
				<div class="flex h-full items-center justify-center text-sm text-muted-foreground">
					No data
				</div>
			{/if}
		{:else if widget.widgetType === 'area_chart'}
			{#if series.length > 0}
				<D3LineChart
					series={visibleSeries}
					xDomain={timeDomain ?? undefined}
					height={chartHeight}
					padding={chartPadding}
					{onRangeSelect}
					data={visibleSeries[0]?.data ?? []}
					areaFill={true}
					unit={effectiveUnit}
					formatValue={(v) => formatMetricLabel(v, effectiveUnit)}
					{sharedHoverTime}
					{isSourceChart}
					{onHoverTimeChange}
				/>
			{:else}
				<div class="flex h-full items-center justify-center text-sm text-muted-foreground">
					No data
				</div>
			{/if}
		{:else if widget.widgetType === 'stacked_area'}
			{#if series.length > 0}
				<D3StackedAreaChart
					endpoints={visibleSeries.map((s) => s.key)}
					series={stackedPoints}
					height={chartHeight}
					padding={chartPadding}
					unit={effectiveUnit}
					formatValue={(v) => formatMetricLabel(v, effectiveUnit)}
					{onRangeSelect}
					colors={visibleSeries.map((s) => s.color)}
					showBuiltinLegend={false}
				/>
			{:else}
				<div class="flex h-full items-center justify-center text-sm text-muted-foreground">
					No data
				</div>
			{/if}
		{:else if series.length > 0}
			<D3LineChart
				series={visibleSeries}
				xDomain={timeDomain ?? undefined}
				height={chartHeight}
				padding={chartPadding}
				{onRangeSelect}
				data={visibleSeries[0]?.data ?? []}
				unit={effectiveUnit}
				formatValue={(v) => formatMetricLabel(v, effectiveUnit)}
				{sharedHoverTime}
				{isSourceChart}
				{onHoverTimeChange}
			/>
		{:else}
			<div class="flex h-full items-center justify-center text-sm text-muted-foreground">
				No data
			</div>
		{/if}
	</div>

	{#if legendVisible && !loading}
		<div class="flex flex-wrap gap-x-3 gap-y-1 px-2 pt-1">
			{#each series as s (s.key)}
				<button
					type="button"
					class="flex items-center gap-1.5 text-xs transition-opacity {hiddenSeries.has(s.key)
						? 'opacity-40'
						: ''}"
					onclick={() => toggleSeries(s.key)}
					title={s.key}
				>
					<span class="h-2 w-2 flex-shrink-0 rounded-full" style="background-color: {s.color};"
					></span>
					<span
						class="max-w-[180px] truncate text-muted-foreground {hiddenSeries.has(s.key)
							? 'line-through'
							: ''}">{s.key}</span
					>
				</button>
			{/each}
		</div>
	{/if}
</div>
