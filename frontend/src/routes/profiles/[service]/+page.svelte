<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import { Button } from '$lib/components/ui/button';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { TimeRangePicker } from '$lib/components/ui/time-range-picker';
	import { ErrorDisplay } from '$lib/components/ui/error-display';
	import * as Tabs from '$lib/components/ui/tabs';
	import { ArrowLeft } from '@lucide/svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { projectsState } from '$lib/state/projects.svelte';
	import PageHeader from '$lib/components/issues/page-header.svelte';
	import FlameGraph from '$lib/components/profiles/flame-graph.svelte';
	import TagFilter from '$lib/components/dashboard/tag-filter.svelte';
	import { CalendarDate } from '@internationalized/date';
	import {
		PROFILE_TYPES,
		defaultProfileType,
		formatProfileValue,
		getProfileTypeMeta,
		type ProfileTypeMeta
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
	let loading = $state(true);
	let error = $state('');
	let notFound = $state(false);

	const knownTypes = PROFILE_TYPES.map((t) => t.type);
	let activeType = $state(
		data.type && knownTypes.includes(data.type) ? data.type : defaultProfileType(knownTypes)
	);

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

	const activeMeta = $derived<ProfileTypeMeta | undefined>(getProfileTypeMeta(activeType));
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

		updateUrlParams(pushToHistory);

		const fromIso = getFromDateTimeUTC();
		const toIso = getToDateTimeUTC();
		const body = {
			fromDate: fromIso,
			toDate: toIso,
			serviceName: data.service,
			type: activeType
		};

		try {
			const [tree, points, labels] = await Promise.all([
				api.post(
					'/profiles/flamegraph',
					{ ...body, labels: selectedLabels },
					{ projectId: projectsState.currentProjectId ?? undefined }
				),
				api.post(
					'/profiles/series',
					{ ...body, intervalMinutes: intervalMinutes(fromIso, toIso) },
					{ projectId: projectsState.currentProjectId ?? undefined }
				),
				api.post('/profiles/labels', body, {
					projectId: projectsState.currentProjectId ?? undefined
				}).catch(() => ({}))
			]);

			totalValue = tree?.value ?? 0;
			flameData = tree?.children?.length ? tree : null;
			series = points || [];
			availableLabels = labels || {};
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

	<Tabs.Root value={activeType} onValueChange={handleTypeChange}>
		<Tabs.List>
			{#each PROFILE_TYPES as meta (meta.type)}
				<Tabs.Trigger value={meta.type}>{meta.label}</Tabs.Trigger>
			{/each}
		</Tabs.List>
	</Tabs.Root>

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
					Total {activeMeta?.category === 'heap' ? 'Memory' : 'CPU Time'}
				</div>
				<div class="text-2xl font-bold tabular-nums">
					{formatProfileValue(activeType, totalValue)}
				</div>
			</div>
			<div class="rounded-lg border p-4">
				<div class="text-sm text-muted-foreground">
					Peak {activeMeta?.isGauge ? '(avg/bucket)' : '(sum/bucket)'}
				</div>
				<div class="text-2xl font-bold tabular-nums">
					{formatProfileValue(activeType, seriesPeak)}
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

		<div class="rounded-lg border p-4">
			<FlameGraph data={flameData} type={activeType} />
		</div>
	{/if}
</div>
