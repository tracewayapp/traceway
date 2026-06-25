<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import { authState } from '$lib/state/auth.svelte';
	import { toast } from 'svelte-sonner';
	import { Button } from '$lib/components/ui/button';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { TimeRangePicker } from '$lib/components/ui/time-range-picker';
	import { ErrorDisplay } from '$lib/components/ui/error-display';
	import * as Tabs from '$lib/components/ui/tabs';
	import * as Select from '$lib/components/ui/select';
	import { ArrowLeft, GitCompareArrows, Download } from '@lucide/svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { projectsState } from '$lib/state/projects.svelte';
	import PageHeader from '$lib/components/issues/page-header.svelte';
	import FlameGraph from '$lib/components/profiles/flame-graph.svelte';
	import TopFunctionsTable, {
		type FunctionStat
	} from '$lib/components/profiles/top-functions-table.svelte';
	import TagFilter from '$lib/components/dashboard/tag-filter.svelte';
	import { mergeFlameTrees } from '$lib/utils/flame-diff';
	import { CalendarDate } from '@internationalized/date';
	import {
		defaultProfileType,
		formatValue,
		humanizeType,
		type ProfileTypeInfo
	} from '$lib/utils/profile-format';
	import {
		presetMinutes,
		getTimeRangeFromPreset,
		parseTimeRangeFromUrl,
		getResolvedTimeRange,
		dateToCalendarDate,
		dateToTimeString,
		updateUrl
	} from '$lib/utils/url-params';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import { toUTCISO, calendarDateTimeToLuxon, parseISO } from '$lib/utils/formatters';
	import type { FlameGraphNode } from 'd3-flame-graph';

	const timezone = $derived(getTimezone());

	type SeriesPoint = { timestamp: string; value: number };

	let { data } = $props();

	let flameData = $state<FlameGraphNode | null>(null);
	let series = $state<SeriesPoint[]>([]);
	let totalValue = $state(0);
	let availableLabels = $state<Record<string, string[]>>({});
	let selectedLabels = $state<Record<string, string>>({});
	let topFunctions = $state<FunctionStat[]>([]);
	let loading = $state(true);
	let error = $state('');
	let notFound = $state(false);

	let compareMode = $state(false);
	let diffView = $state<'differential' | 'sidebyside'>('differential');
	let baseFlameData = $state<FlameGraphNode | null>(null);
	let diffData = $state<FlameGraphNode | null>(null);
	let downloading = $state(false);

	let availableTypes = $state<ProfileTypeInfo[]>([]);
	let activeType = $state(data.type ?? '');

	let tabsListEl = $state<HTMLElement | null>(null);
	let hasOverflow = $state(false);

	function checkOverflow() {
		if (tabsListEl) {
			hasOverflow = tabsListEl.scrollWidth > tabsListEl.clientWidth;
		}
	}

	$effect(() => {
		if (!tabsListEl) return;
		const observer = new ResizeObserver(() => checkOverflow());
		observer.observe(tabsListEl);
		checkOverflow();
		return () => observer.disconnect();
	});

	$effect(() => {
		availableTypes;
		if (tabsListEl) {
			requestAnimationFrame(() => checkOverflow());
		}
	});

	function getInitialRange(): { preset: string | null; from: Date; to: Date } {
		if (data.preset && presetMinutes[data.preset]) {
			const range = getTimeRangeFromPreset(data.preset, timezone);
			return { preset: data.preset, from: range.from, to: range.to };
		}
		if (data.from && data.to) {
			const fromDt = parseISO(data.from, timezone);
			const toDt = parseISO(data.to, timezone);
			if (fromDt.isValid && toDt.isValid) {
				return { preset: null, from: fromDt.toJSDate(), to: toDt.toJSDate() };
			}
		}
		const range = getTimeRangeFromPreset('24h', timezone);
		return { preset: '24h', from: range.from, to: range.to };
	}

	const initialRange = getInitialRange();

	let selectedPreset = $state<string | null>(initialRange.preset);
	let fromDate = $state<CalendarDate>(dateToCalendarDate(initialRange.from, timezone));
	let toDate = $state<CalendarDate>(dateToCalendarDate(initialRange.to, timezone));
	let fromTime = $state(dateToTimeString(initialRange.from, timezone));
	let toTime = $state(dateToTimeString(initialRange.to, timezone));

	const baseInitial = getTimeRangeFromPreset('7d', timezone);
	let baseSelectedPreset = $state<string | null>('7d');
	let baseFromDate = $state<CalendarDate>(dateToCalendarDate(baseInitial.from, timezone));
	let baseToDate = $state<CalendarDate>(dateToCalendarDate(baseInitial.to, timezone));
	let baseFromTime = $state(dateToTimeString(baseInitial.from, timezone));
	let baseToTime = $state(dateToTimeString(baseInitial.to, timezone));

	const activeMeta = $derived<ProfileTypeInfo | undefined>(
		availableTypes.find((t) => t.type === activeType)
	);
	const activeUnit = $derived(activeMeta?.unit ?? '');
	const activeIsGauge = $derived(activeMeta?.isGauge ?? false);
	const seriesPeak = $derived(series.reduce((max, p) => Math.max(max, p.value), 0));

	function updateUrlParams(pushToHistory = true) {
		const params: Record<string, string | undefined> = selectedPreset
			? { preset: selectedPreset }
			: { from: getFromDateTimeUTC(), to: getToDateTimeUTC() };
		params.type = activeType;
		updateUrl(params, { pushToHistory });
	}

	function getFromDateTimeUTC(): string {
		const [hour, minute] = (fromTime || '00:00').split(':').map(Number);
		const dt = calendarDateTimeToLuxon(
			{ year: fromDate.year, month: fromDate.month, day: fromDate.day, hour, minute },
			timezone
		);
		return toUTCISO(dt);
	}

	function getToDateTimeUTC(): string {
		const [hour, minute] = (toTime || '23:59').split(':').map(Number);
		const dt = calendarDateTimeToLuxon(
			{ year: toDate.year, month: toDate.month, day: toDate.day, hour, minute },
			timezone
		).endOf('minute');
		return toUTCISO(dt);
	}

	function intervalMinutes(fromIso: string, toIso: string): number {
		const spanMinutes = Math.max(
			1,
			Math.round((new Date(toIso).getTime() - new Date(fromIso).getTime()) / 60_000)
		);
		return Math.max(1, Math.round(spanMinutes / 60));
	}

	function handleTimeRangeChange(
		from: { date: CalendarDate; time: string },
		to: { date: CalendarDate; time: string },
		preset: string | null
	) {
		fromDate = from.date;
		fromTime = from.time;
		toDate = to.date;
		toTime = to.time;
		selectedPreset = preset;
		loadData(true);
	}

	function handleTypeChange(type: string) {
		activeType = type;
		selectedLabels = {};
		loadData(true);
	}

	function handleFilterChange(filters: Record<string, string>) {
		selectedLabels = filters;
		loadData(true);
	}

	async function loadData(pushToHistory = true) {
		loading = true;
		error = '';
		notFound = false;

		if (selectedPreset) {
			const range = getTimeRangeFromPreset(selectedPreset, timezone);
			fromDate = dateToCalendarDate(range.from, timezone);
			toDate = dateToCalendarDate(range.to, timezone);
			fromTime = dateToTimeString(range.from, timezone);
			toTime = dateToTimeString(range.to, timezone);
		}

		const fromIso = getFromDateTimeUTC();
		const toIso = getToDateTimeUTC();

		try {
			const types = (await api
				.post(
					'/profiles/types',
					{ fromDate: fromIso, toDate: toIso, serviceName: data.service },
					{ projectId: projectsState.currentProjectId ?? undefined }
				)
				.catch(() => [])) as ProfileTypeInfo[];
			availableTypes = types || [];

			const typeNames = availableTypes.map((t) => t.type);
			if (!activeType || !typeNames.includes(activeType)) {
				activeType = defaultProfileType(typeNames);
			}
			const isGauge = availableTypes.find((t) => t.type === activeType)?.isGauge ?? false;

			updateUrlParams(pushToHistory);

			const body = {
				fromDate: fromIso,
				toDate: toIso,
				serviceName: data.service,
				type: activeType
			};

			const [tree, points, labels, top] = await Promise.all([
				api.post(
					'/profiles/flamegraph',
					{ ...body, isGauge, labels: selectedLabels },
					{ projectId: projectsState.currentProjectId ?? undefined }
				),
				api.post(
					'/profiles/series',
					{ ...body, isGauge, intervalMinutes: intervalMinutes(fromIso, toIso) },
					{ projectId: projectsState.currentProjectId ?? undefined }
				),
				api.post('/profiles/labels', body, {
					projectId: projectsState.currentProjectId ?? undefined
				}).catch(() => ({})),
				api
					.post(
						'/profiles/top',
						{ ...body, isGauge, labels: selectedLabels, limit: 25 },
						{ projectId: projectsState.currentProjectId ?? undefined }
					)
					.catch(() => [])
			]);

			totalValue = tree?.value ?? 0;
			flameData = tree?.children?.length ? tree : null;
			series = points || [];
			availableLabels = labels || {};
			topFunctions = (top as FunctionStat[]) || [];

			if (compareMode) await loadCompare(isGauge);
		} catch (e: any) {
			if (e?.status === 404) {
				notFound = true;
			} else {
				console.error(e);
				error = e.message || 'Failed to load profile';
			}
		} finally {
			loading = false;
		}
	}

	function baseRangeFromUTC(): string {
		if (baseSelectedPreset) {
			return getTimeRangeFromPreset(baseSelectedPreset, timezone).from.toISOString();
		}
		const [hour, minute] = (baseFromTime || '00:00').split(':').map(Number);
		return toUTCISO(
			calendarDateTimeToLuxon(
				{ year: baseFromDate.year, month: baseFromDate.month, day: baseFromDate.day, hour, minute },
				timezone
			)
		);
	}

	function baseRangeToUTC(): string {
		if (baseSelectedPreset) {
			return getTimeRangeFromPreset(baseSelectedPreset, timezone).to.toISOString();
		}
		const [hour, minute] = (baseToTime || '23:59').split(':').map(Number);
		return toUTCISO(
			calendarDateTimeToLuxon(
				{ year: baseToDate.year, month: baseToDate.month, day: baseToDate.day, hour, minute },
				timezone
			).endOf('minute')
		);
	}

	async function loadCompare(isGauge: boolean) {
		try {
			const baseTree = await api.post(
				'/profiles/flamegraph',
				{
					fromDate: baseRangeFromUTC(),
					toDate: baseRangeToUTC(),
					serviceName: data.service,
					type: activeType,
					isGauge,
					labels: selectedLabels
				},
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			baseFlameData = baseTree?.children?.length ? baseTree : null;
			diffData = mergeFlameTrees(baseFlameData, flameData);
		} catch (e) {
			console.error(e);
			baseFlameData = null;
			diffData = null;
		}
	}

	function toggleCompare() {
		compareMode = !compareMode;
		if (compareMode) loadCompare(activeIsGauge);
	}

	function handleBaseRangeChange(
		from: { date: CalendarDate; time: string },
		to: { date: CalendarDate; time: string },
		preset: string | null
	) {
		baseFromDate = from.date;
		baseFromTime = from.time;
		baseToDate = to.date;
		baseToTime = to.time;
		baseSelectedPreset = preset;
		loadCompare(activeIsGauge);
	}

	async function downloadPprof() {
		if (!flameData) return;
		downloading = true;
		try {
			const projectId = projectsState.currentProjectId;
			const url = `/api/profiles/pprof${projectId ? `?projectId=${projectId}` : ''}`;
			const res = await fetch(url, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					...(authState.token ? { Authorization: `Bearer ${authState.token}` } : {})
				},
				body: JSON.stringify({
					fromDate: getFromDateTimeUTC(),
					toDate: getToDateTimeUTC(),
					serviceName: data.service,
					type: activeType,
					unit: activeUnit,
					isGauge: activeIsGauge,
					labels: selectedLabels
				})
			});
			if (!res.ok) {
				toast.error('Failed to export profile', { position: 'top-center' });
				return;
			}
			const blob = await res.blob();
			const objectUrl = URL.createObjectURL(blob);
			const anchor = document.createElement('a');
			anchor.href = objectUrl;
			anchor.download = `${data.service}-${activeType.replace(/[^a-zA-Z0-9-_]/g, '_')}.pprof`;
			document.body.appendChild(anchor);
			anchor.click();
			anchor.remove();
			URL.revokeObjectURL(objectUrl);
		} catch (e) {
			console.error(e);
			toast.error('Failed to export profile', { position: 'top-center' });
		} finally {
			downloading = false;
		}
	}

	const sparkPath = $derived.by(() => {
		if (series.length < 2 || seriesPeak === 0) return '';
		const step = 100 / (series.length - 1);
		return series
			.map(
				(p, i) =>
					`${i === 0 ? 'M' : 'L'} ${(i * step).toFixed(2)} ${(30 - (p.value / seriesPeak) * 28).toFixed(2)}`
			)
			.join(' ');
	});

	onMount(() => {
		loadData(false);
	});
</script>

<div class="space-y-4">
	<div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
		<div class="flex items-center gap-3">
			<Button variant="ghost" size="sm" onclick={() => goto(resolve('/profiles'))}>
				<ArrowLeft class="h-4 w-4" />
			</Button>
			<PageHeader title={data.service} subtitle="Profiles" />
		</div>

		<div class="flex flex-col">
			<TimeRangePicker
				bind:fromDate
				bind:toDate
				bind:fromTime
				bind:toTime
				bind:preset={selectedPreset}
				onApply={handleTimeRangeChange}
			/>
		</div>
	</div>

	{#if availableTypes.length}
		<Tabs.Root value={activeType} onValueChange={handleTypeChange}>
			<div class="flex items-center" class:gap-2={!hasOverflow}>
				{#if hasOverflow}
					<Select.Root
						type="single"
						value={activeType}
						onValueChange={(v) => {
							if (v) handleTypeChange(v);
						}}
					>
						<Select.Trigger size="sm" class="w-full" data-testid="type-overflow-select">
							{activeType ? humanizeType(activeType) : 'Select type'}
						</Select.Trigger>
						<Select.Content>
							{#each availableTypes as meta (meta.type)}
								<Select.Item value={meta.type}>{humanizeType(meta.type)}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				{/if}
				<div
					class="flex min-w-0 items-center overflow-hidden"
					class:invisible={hasOverflow}
					class:h-0={hasOverflow}
					class:w-0={hasOverflow}
					bind:this={tabsListEl}
				>
					<Tabs.List>
						{#each availableTypes as meta (meta.type)}
							<Tabs.Trigger value={meta.type}>{humanizeType(meta.type)}</Tabs.Trigger>
						{/each}
					</Tabs.List>
				</div>
			</div>
		</Tabs.Root>
	{/if}

	<TagFilter
		tagKeys={Object.keys(availableLabels)}
		activeFilters={selectedLabels}
		onFilterChange={handleFilterChange}
		onLoadTagValues={(key) => Promise.resolve(availableLabels[key] ?? [])}
	/>

	{#if loading}
		<div class="flex items-center justify-center py-20">
			<LoadingCircle size="xlg" />
		</div>
	{:else if notFound}
		<ErrorDisplay
			status={404}
			title="Not Found"
			description="No profiles found for this service."
			onRetry={() => loadData()}
		/>
	{:else if error}
		<ErrorDisplay status={400} title="Error" description={error} onRetry={() => loadData()} />
	{:else}
		<div class="grid gap-4 sm:grid-cols-3">
			<div class="rounded-lg border p-4">
				<div class="text-sm text-muted-foreground">
					Total {activeType ? humanizeType(activeType) : ''}
				</div>
				<div class="text-2xl font-bold tabular-nums">
					{formatValue(activeUnit, totalValue)}
				</div>
			</div>
			<div class="rounded-lg border p-4">
				<div class="text-sm text-muted-foreground">
					Peak {activeIsGauge ? '(avg/bucket)' : '(sum/bucket)'}
				</div>
				<div class="text-2xl font-bold tabular-nums">
					{formatValue(activeUnit, seriesPeak)}
				</div>
			</div>
			<div class="rounded-lg border p-4">
				<div class="text-sm text-muted-foreground">Trend</div>
				{#if sparkPath}
					<svg viewBox="0 0 100 30" preserveAspectRatio="none" class="mt-1 h-9 w-full text-primary">
						<path
							d={sparkPath}
							fill="none"
							stroke="currentColor"
							stroke-width="1.5"
							vector-effect="non-scaling-stroke"
						/>
					</svg>
				{:else}
					<div class="mt-1 text-sm text-muted-foreground">Not enough data</div>
				{/if}
			</div>
		</div>

		<div class="space-y-3 rounded-lg border p-4">
			<div class="flex flex-wrap items-center gap-2">
				<Button
					variant={compareMode ? 'secondary' : 'outline'}
					size="sm"
					aria-pressed={compareMode}
					onclick={toggleCompare}
					data-testid="compare-toggle"
				>
					<GitCompareArrows class="h-4 w-4" />
					Compare
				</Button>

				{#if compareMode}
					<span class="text-xs text-muted-foreground">vs baseline</span>
					<TimeRangePicker
						bind:fromDate={baseFromDate}
						bind:toDate={baseToDate}
						bind:fromTime={baseFromTime}
						bind:toTime={baseToTime}
						bind:preset={baseSelectedPreset}
						onApply={handleBaseRangeChange}
					/>
				{/if}

				<div class="flex-1"></div>

				{#if compareMode}
					<div class="flex gap-1">
						<Button
							variant={diffView === 'differential' ? 'secondary' : 'outline'}
							size="sm"
							onclick={() => (diffView = 'differential')}
							data-testid="diff-differential"
						>
							Differential
						</Button>
						<Button
							variant={diffView === 'sidebyside' ? 'secondary' : 'outline'}
							size="sm"
							onclick={() => (diffView = 'sidebyside')}
							data-testid="diff-sidebyside"
						>
							Side by side
						</Button>
					</div>
				{/if}

				<Button
					variant="outline"
					size="sm"
					onclick={downloadPprof}
					disabled={downloading || !flameData}
					data-testid="download-pprof"
				>
					<Download class="h-4 w-4" />
					Download .pprof
				</Button>
			</div>

			{#if compareMode && diffView === 'differential'}
				<div class="flex items-center gap-4 text-xs text-muted-foreground">
					<span class="inline-flex items-center gap-1.5">
						<span class="h-2.5 w-3.5 rounded-sm" style="background: hsl(8, 70%, 55%)"></span>
						hotter than baseline
					</span>
					<span class="inline-flex items-center gap-1.5">
						<span class="h-2.5 w-3.5 rounded-sm" style="background: hsl(140, 60%, 50%)"></span>
						cooler than baseline
					</span>
				</div>
				<FlameGraph data={diffData} unit={activeUnit} diff={true} />
			{:else if compareMode}
				<div class="grid gap-4 lg:grid-cols-2">
					<div>
						<div class="mb-1 text-xs font-medium text-muted-foreground">Baseline</div>
						<FlameGraph data={baseFlameData} unit={activeUnit} controls={false} />
					</div>
					<div>
						<div class="mb-1 text-xs font-medium text-muted-foreground">Current</div>
						<FlameGraph data={flameData} unit={activeUnit} controls={false} />
					</div>
				</div>
			{:else}
				<FlameGraph data={flameData} unit={activeUnit} />
			{/if}
		</div>

		<TopFunctionsTable rows={topFunctions} unit={activeUnit} />
	{/if}
</div>
