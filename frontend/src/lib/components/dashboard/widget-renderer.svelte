<script lang="ts">
	import D3LineChart from './d3-line-chart.svelte';
	import D3HorizontalBarChart from './d3-horizontal-bar-chart.svelte';
	import WidgetTable from './widget-table.svelte';
	import { api } from '$lib/api';
	import { projectsState } from '$lib/state/projects.svelte';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import type { MetricTrendPoint, MetricQueryResponse } from '$lib/types/dashboard';
	import { formatMetricLabel } from '$lib/utils/metric-format';

	type WidgetSource = {
		type: 'metric';
		name: string;
		tagFilters?: Record<string, string>;
		aggregation: string;
		groupBy?: string;
	};

	type WidgetConfig = {
		sources?: WidgetSource[];
		yAxisLabel?: string;
		showLegend?: boolean;
		unit?: string;
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

	const colors = ['#3b82f6', '#ef4444', '#22c55e', '#f59e0b', '#8b5cf6', '#ec4899'];

	const effectiveUnit = $derived(widget.config.unit ?? resolvedUnit);

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
			for (const result of response.results) {
				if (result.unit) units.add(result.unit);
				for (const [key, points] of Object.entries(result.series)) {
					const label = Object.keys(result.series).length > 1 ? `${result.name} (${key})` : result.name;
					newSeries.push({
						key: label,
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

			if (widget.widgetType === 'single_value' && newSeries.length > 0 && newSeries[0].data.length > 0) {
				singleValue = newSeries[0].data[newSeries[0].data.length - 1].value;
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
</script>

<div class="h-full w-full min-h-[200px]">
	{#if loading}
		<div class="flex h-full items-center justify-center">
			<LoadingCircle size="md" />
		</div>
	{:else if widget.widgetType === 'single_value'}
		<div class="flex h-full flex-col items-center justify-center">
			<span class="text-3xl font-bold">
				{singleValue !== null ? formatMetricLabel(singleValue, effectiveUnit) : '-'}
			</span>
		</div>
	{:else if widget.widgetType === 'bar_chart'}
		{#if barData.length > 0}
			<D3HorizontalBarChart data={barData} height={200} unit={effectiveUnit} formatValue={(v) => formatMetricLabel(v, effectiveUnit)} />
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
				{series}
				xDomain={timeDomain ?? undefined}
				height={200}
				padding={{ top: 10, right: 4, bottom: 20, left: 45 }}
				{onRangeSelect}
				data={series[0]?.data ?? []}
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
	{:else if series.length > 0}
		<D3LineChart
			{series}
			xDomain={timeDomain ?? undefined}
			height={200}
			padding={{ top: 10, right: 4, bottom: 20, left: 45 }}
			{onRangeSelect}
			data={series[0]?.data ?? []}
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
