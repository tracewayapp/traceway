<script lang="ts" module>
	export type LogChartSelection = { fromDate: string; toDate: string } & Record<string, unknown>;
</script>

<script lang="ts">
	import { untrack } from 'svelte';
	import { ChartColumn, ChartLine, X } from '@lucide/svelte';
	import { api } from '$lib/api';
	import { projectsState } from '$lib/state/projects.svelte';
	import { getErrorMessage } from '$lib/utils/errors';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import LogAttributeSeriesChart, {
		type LogAttributeSeriesBucket
	} from './log-attribute-series-chart.svelte';
	import LogAttributeDistributionChart, {
		type LogAttributeBin
	} from './log-attribute-distribution-chart.svelte';
	import {
		chartAggregations,
		formatChartNumber,
		toggleAggregation,
		type ChartView,
		type LogAttributeChart
	} from '$lib/utils/log-attribute-chart';
	import { formatLogAttributeTarget } from '$lib/utils/log-filters';

	type SeriesResponse = {
		intervalSeconds: number;
		present: number;
		numeric: number;
		buckets: LogAttributeSeriesBucket[];
	};

	type DistributionResponse = {
		present: number;
		numeric: number;
		integral: boolean;
		min: number;
		max: number;
		avg: number;
		sum: number;
		p50: number;
		p95: number;
		p99: number;
		binWidth: number;
		bins: LogAttributeBin[];
	};

	let {
		chart,
		selection,
		timezone,
		onChange,
		onClose,
		onRangeSelect
	}: {
		chart: LogAttributeChart;
		// The list's current selection; a new object reloads the chart.
		selection: LogChartSelection | null;
		timezone: string;
		onChange: (chart: LogAttributeChart) => void;
		onClose: () => void;
		onRangeSelect?: (from: Date, to: Date) => void;
	} = $props();

	let width = $state(0);
	let loading = $state(false);
	let error = $state('');
	let series = $state<(SeriesResponse & { from: Date; to: Date }) | null>(null);
	let distribution = $state<DistributionResponse | null>(null);
	let generation = 0;

	const label = $derived(formatLogAttributeTarget(chart.target));

	async function load(sel: LogChartSelection, view: ChartView) {
		const gen = ++generation;
		loading = true;
		error = '';
		const body = {
			...sel,
			attribute: { scope: chart.target.scope, key: chart.target.key }
		};
		try {
			if (view === 'series') {
				const maxBuckets = Math.max(20, Math.min(200, Math.floor((width || 960) / 7)));
				const response = (await api.post(
					'/logs/attribute-series',
					{ ...body, maxBuckets },
					{ projectId: projectsState.currentProjectId ?? undefined }
				)) as SeriesResponse;
				if (gen !== generation) return;
				series = { ...response, from: new Date(sel.fromDate), to: new Date(sel.toDate) };
				distribution = null;
			} else {
				const response = (await api.post(
					'/logs/attribute-distribution',
					{ ...body, bins: 40 },
					{ projectId: projectsState.currentProjectId ?? undefined }
				)) as DistributionResponse;
				if (gen !== generation) return;
				distribution = response;
				series = null;
			}
		} catch (e) {
			if (gen !== generation) return;
			error = getErrorMessage(e) || 'Failed to chart this attribute';
		} finally {
			if (gen === generation) loading = false;
		}
	}

	$effect(() => {
		const sel = selection;
		const view = chart.view;
		void chart.target.scope;
		void chart.target.key;
		if (!sel) return;
		untrack(() => load(sel, view));
	});

	const counts = $derived(
		chart.view === 'series'
			? series && { present: series.present, numeric: series.numeric }
			: distribution && { present: distribution.present, numeric: distribution.numeric }
	);

	const summary = $derived.by(() => {
		if (chart.view === 'distribution') {
			if (!distribution || distribution.numeric === 0) return [];
			return [
				{ label: 'Min', value: distribution.min },
				{ label: 'Avg', value: distribution.avg },
				{ label: 'P50', value: distribution.p50 },
				{ label: 'P95', value: distribution.p95 },
				{ label: 'Max', value: distribution.max }
			];
		}
		if (!series || series.buckets.length === 0) return [];
		let count = 0;
		let sum = 0;
		let lo = Infinity;
		let hi = -Infinity;
		for (const b of series.buckets) {
			count += b.count;
			sum += b.sum;
			lo = Math.min(lo, b.min);
			hi = Math.max(hi, b.max);
		}
		return [
			{ label: 'Min', value: lo },
			{ label: 'Avg', value: sum / count },
			{ label: 'Max', value: hi }
		];
	});

	const markers = $derived(
		distribution
			? [
					{ label: 'P50', value: distribution.p50, color: 'var(--chart-2)' },
					{ label: 'P95', value: distribution.p95, color: 'var(--chart-3)' },
					{ label: 'P99', value: distribution.p99, color: 'var(--chart-4)' }
				]
			: []
	);

	const skipped = $derived(counts ? counts.present - counts.numeric : 0);

	function setView(view: ChartView) {
		if (view !== chart.view) onChange({ ...chart, view });
	}
</script>

<div class="flex flex-col gap-2 border-b px-3 pt-2 pb-3" bind:clientWidth={width}>
	<div class="flex flex-wrap items-center gap-x-3 gap-y-2">
		<div class="flex min-w-0 items-center gap-1.5">
			<ChartLine class="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
			<span class="truncate font-mono text-xs font-medium" title={label}>{label}</span>
		</div>
		<div class="inline-flex rounded-md border p-0.5 text-xs" role="group" aria-label="Chart view">
			<button
				type="button"
				aria-pressed={chart.view === 'series'}
				class="inline-flex items-center gap-1 rounded px-2 py-0.5 text-muted-foreground transition-colors hover:text-foreground aria-pressed:bg-muted aria-pressed:text-foreground"
				onclick={() => setView('series')}
			>
				<ChartLine class="h-3 w-3" />
				Over time
			</button>
			<button
				type="button"
				aria-pressed={chart.view === 'distribution'}
				class="inline-flex items-center gap-1 rounded px-2 py-0.5 text-muted-foreground transition-colors hover:text-foreground aria-pressed:bg-muted aria-pressed:text-foreground"
				onclick={() => setView('distribution')}
			>
				<ChartColumn class="h-3 w-3" />
				Distribution
			</button>
		</div>
		{#if chart.view === 'series'}
			<div class="flex flex-wrap items-center gap-1" role="group" aria-label="Lines">
				{#each chartAggregations as agg (agg.value)}
					{@const on = chart.aggregations.includes(agg.value)}
					<button
						type="button"
						aria-pressed={on}
						class="inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-[11px] font-medium transition-colors {on
							? 'border-foreground/30 text-foreground'
							: 'border-transparent text-muted-foreground hover:text-foreground'}"
						onclick={() =>
							onChange({
								...chart,
								aggregations: toggleAggregation(chart.aggregations, agg.value)
							})}
					>
						<span
							class="h-2 w-2 rounded-full {on ? '' : 'opacity-30'}"
							style="background-color: {agg.color};"
						></span>
						{agg.label}
					</button>
				{/each}
			</div>
		{/if}
		<div class="ml-auto flex items-center gap-3">
			{#if summary.length > 0}
				<div class="flex flex-wrap items-center gap-x-3 text-xs text-muted-foreground tabular-nums">
					{#if counts}
						<span>
							<span class="font-medium text-foreground">{counts.numeric.toLocaleString()}</span>
							{counts.numeric === 1 ? 'value' : 'values'}
						</span>
					{/if}
					{#each summary as s (s.label)}
						<span>
							{s.label}
							<span class="font-medium text-foreground">{formatChartNumber(s.value)}</span>
						</span>
					{/each}
				</div>
			{/if}
			<button
				type="button"
				aria-label="Close chart"
				title="Close chart"
				class="rounded p-0.5 text-muted-foreground hover:bg-muted hover:text-foreground"
				onclick={onClose}
			>
				<X class="h-3.5 w-3.5" />
			</button>
		</div>
	</div>

	{#if error}
		<div class="flex h-[180px] items-center justify-center text-center text-xs text-red-500">
			{error}
		</div>
	{:else if counts && counts.numeric === 0}
		<div
			class="flex h-[180px] flex-col items-center justify-center gap-1 text-center text-xs text-muted-foreground"
		>
			<span class="font-medium text-foreground">No numbers to chart</span>
			{#if counts.present > 0}
				<span>
					{counts.present.toLocaleString()}
					{counts.present === 1 ? 'log carries' : 'logs carry'}
					<span class="font-mono">{label}</span> in this range, but none of the values is a number.
				</span>
			{:else}
				<span>No log in this range carries <span class="font-mono">{label}</span>.</span>
			{/if}
		</div>
	{:else if chart.view === 'series' && series}
		<LogAttributeSeriesChart
			buckets={series.buckets}
			intervalSeconds={series.intervalSeconds}
			from={series.from}
			to={series.to}
			aggregations={chart.aggregations}
			{timezone}
			{loading}
			{onRangeSelect}
		/>
	{:else if chart.view === 'distribution' && distribution}
		<LogAttributeDistributionChart
			bins={distribution.bins}
			binWidth={distribution.binWidth}
			integral={distribution.integral}
			{markers}
			{loading}
		/>
	{:else}
		<div class="flex h-[180px] items-center justify-center">
			<LoadingCircle size="sm" />
		</div>
	{/if}

	{#if skipped > 0 && counts && counts.numeric > 0}
		<p class="text-[11px] text-muted-foreground">
			{skipped.toLocaleString()} of {counts.present.toLocaleString()} values are not numbers and are left
			out.
		</p>
	{/if}
</div>
