<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import {
		formatDuration,
		formatDurationMs,
		formatDateTime,
		parseISO,
		toUTCISO,
		calendarDateTimeToLuxon
	} from '$lib/utils/formatters';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import * as Table from '$lib/components/ui/table';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { TimeRangePicker } from '$lib/components/ui/time-range-picker';
	import { TracewayTableHeader } from '$lib/components/ui/traceway-table-header';
	import { TableEmptyState } from '$lib/components/ui/table-empty-state';
	import { CalendarDate } from '@internationalized/date';
	import { ErrorDisplay } from '$lib/components/ui/error-display';
	import { projectsState } from '$lib/state/projects.svelte';
	import { createRowClickHandler } from '$lib/utils/navigation';
	import { createSmartBackHandler } from '$lib/utils/back-navigation';
	import PaginationFooter from '$lib/components/ui/pagination-footer/pagination-footer.svelte';
	import PageHeader from '$lib/components/issues/page-header.svelte';
	import { resolve } from '$app/paths';
	import {
		presetMinutes,
		getTimeRangeFromPreset,
		dateToCalendarDate,
		dateToTimeString,
		updateUrl
	} from '$lib/utils/url-params';
	import {
		getSortState,
		setSortState,
		handleSortClick,
		type SortDirection
	} from '$lib/utils/sort-storage';

	const timezone = $derived(getTimezone());

	type AiTrace = {
		id: string;
		recordedAt: string;
		duration: number;
		model: string;
		provider: string;
		inputTokens: number;
		outputTokens: number;
		totalTokens: number;
		totalCost: number;
		statusCode: number;
	};

	type AiTraceDetailStats = {
		count: number;
		avgDuration: number;
		medianDuration: number;
		p95Duration: number;
		totalTokens: number;
		totalCost: number;
		avgInputTokens: number;
		avgOutputTokens: number;
		throughput: number;
	};

	type SortField = 'recorded_at' | 'duration' | 'total_tokens' | 'total_cost';

	let { data } = $props();

	let traces = $state<AiTrace[]>([]);
	let stats = $state<AiTraceDetailStats | null>(null);
	let loading = $state(true);
	let error = $state('');
	let notFound = $state(false);
	let errorStatus = $state<number>(0);

	let page = $state(1);
	let pageSize = $state(50);
	let total = $state(0);
	let totalPages = $state(0);

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

	function updateTimeRangeUrl(pushToHistory = true) {
		updateUrl(
			selectedPreset
				? { preset: selectedPreset }
				: { from: getFromDateTimeUTC(), to: getToDateTimeUTC() },
			{ pushToHistory }
		);
	}

	const SORT_STORAGE_KEY = 'ai_trace_detail';
	const initialSort = getSortState(SORT_STORAGE_KEY, { field: 'recorded_at', direction: 'desc' });
	let orderBy = $state<SortField>(initialSort.field as SortField);
	let sortDirection = $state<SortDirection>(initialSort.direction);

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
		page = 1;
		loadData();
	}

	function formatCost(cost: number): string {
		if (cost === 0) return '$0';
		if (cost < 0.001) return `$${cost.toFixed(6)}`;
		if (cost < 0.01) return `$${cost.toFixed(4)}`;
		if (cost < 1) return `$${cost.toFixed(3)}`;
		return `$${cost.toFixed(2)}`;
	}

	function formatTokens(tokens: number): string {
		if (tokens >= 1_000_000) return `${(tokens / 1_000_000).toFixed(1)}M`;
		if (tokens >= 1_000) return `${(tokens / 1_000).toFixed(1)}k`;
		return tokens.toLocaleString();
	}

	async function loadData(pushToHistory = true) {
		loading = true;
		error = '';
		notFound = false;
		errorStatus = 0;

		updateTimeRangeUrl(pushToHistory);

		try {
			const requestBody = {
				fromDate: getFromDateTimeUTC(),
				toDate: getToDateTimeUTC(),
				orderBy: orderBy,
				sortDirection: sortDirection,
				pagination: { page, pageSize }
			};

			const response = await api.post(
				`/ai-traces/trace?traceName=${encodeURIComponent(data.traceName)}`,
				requestBody,
				{ projectId: projectsState.currentProjectId ?? undefined }
			);

			traces = response.data || [];
			stats = response.stats || null;
			total = response.pagination.total;
			totalPages = response.pagination.totalPages;
		} catch (e: any) {
			console.error(e);
			errorStatus = e.status || 0;
			if (e.status === 404) {
				notFound = true;
			} else {
				error = e.message || 'Failed to load data';
			}
		} finally {
			loading = false;
		}
	}

	function handlePageChange(newPage: number) {
		if (newPage >= 1 && newPage <= totalPages) {
			page = newPage;
			loadData(false);
		}
	}

	function handlePageSizeChange(newPageSize: number) {
		pageSize = newPageSize;
		page = 1;
		loadData(false);
	}

	function handleSort(field: SortField) {
		const newSort = handleSortClick(field, orderBy, sortDirection);
		orderBy = newSort.field as SortField;
		sortDirection = newSort.direction;
		setSortState(SORT_STORAGE_KEY, newSort);
		page = 1;
		loadData(false);
	}

	onMount(() => {
		loadData(false);
	});
</script>

<div class="space-y-6">
	{#if notFound}
		<ErrorDisplay
			status={404}
			title="AI Trace Not Found"
			description="The AI trace you're looking for doesn't exist or has no recorded calls."
			onBack={createSmartBackHandler({ fallbackPath: resolve('/ai-traces') })}
			backLabel="Back to AI Traces"
			onRetry={() => loadData(false)}
			identifier={decodeURIComponent(data.traceName)}
		/>
	{:else if error && !loading}
		<ErrorDisplay
			status={errorStatus === 400 ? 400 : 400}
			title="Failed to Load AI Traces"
			description={error}
			onBack={createSmartBackHandler({ fallbackPath: resolve('/ai-traces') })}
			backLabel="Back to AI Traces"
			onRetry={() => loadData(false)}
		/>
	{:else}
		<div class="flex flex-col gap-4 sm:flex-row sm:justify-between">
			<PageHeader
				title={decodeURIComponent(data.traceName)}
				subtitle="AI trace instances"
				onBack={createSmartBackHandler({ fallbackPath: resolve('/ai-traces') })}
			/>
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

		{#if stats}
			<div class="grid grid-cols-2 gap-4 md:grid-cols-4 lg:grid-cols-5">
				<div class="space-y-1">
					<p class="text-2xl font-semibold tracking-tight">{formatDurationMs(stats.avgDuration)}</p>
					<p class="text-xs text-muted-foreground">Average duration</p>
				</div>
				<div class="space-y-1">
					<p class="text-2xl font-semibold tracking-tight">{formatDurationMs(stats.medianDuration)}</p>
					<p class="text-xs text-muted-foreground">Median duration</p>
				</div>
				<div class="space-y-1">
					<p class="text-2xl font-semibold tracking-tight">{formatDurationMs(stats.p95Duration)}</p>
					<p class="text-xs text-muted-foreground">95th percentile</p>
				</div>
				<div class="space-y-1">
					<p class="text-2xl font-semibold tracking-tight">{formatCost(stats.totalCost)}</p>
					<p class="text-xs text-muted-foreground">Total cost</p>
				</div>
				<div class="space-y-1">
					<p class="text-2xl font-semibold tracking-tight">{formatTokens(stats.totalTokens)}</p>
					<p class="text-xs text-muted-foreground">Total tokens</p>
				</div>
			</div>
		{:else if loading}
			<div class="flex items-center justify-center py-8">
				<LoadingCircle size="lg" />
			</div>
		{/if}

		<div class="overflow-hidden rounded-md border">
			<Table.Root>
				{#if loading || traces.length > 0}
					<Table.Header>
						<Table.Row>
							<TracewayTableHeader
								label="Recorded At"
								sortField="recorded_at"
								currentSortField={orderBy}
								{sortDirection}
								onSort={(field) => handleSort(field as SortField)}
								class="w-[180px]"
							/>
							<TracewayTableHeader
								label="Duration"
								sortField="duration"
								currentSortField={orderBy}
								{sortDirection}
								onSort={(field) => handleSort(field as SortField)}
								class="w-[100px]"
							/>
							<TracewayTableHeader label="Model" class="w-[200px]" />
							<TracewayTableHeader label="Tokens In" class="w-[90px]" />
							<TracewayTableHeader label="Tokens Out" class="w-[90px]" />
							<TracewayTableHeader
								label="Cost"
								sortField="total_cost"
								currentSortField={orderBy}
								{sortDirection}
								onSort={(field) => handleSort(field as SortField)}
								class="w-[90px]"
							/>
							<TracewayTableHeader label="Provider" class="w-[100px]" />
						</Table.Row>
					</Table.Header>
				{/if}
				<Table.Body>
					{#if loading}
						<Table.Row>
							<Table.Cell colspan={7} class="h-48">
								<div class="flex items-center justify-center">
									<LoadingCircle size="lg" />
								</div>
							</Table.Cell>
						</Table.Row>
					{:else if traces.length === 0}
						<TableEmptyState colspan={7} message="No AI traces found in this time range." />
					{:else}
						{#each traces as trace}
							<Table.Row
								class="cursor-pointer hover:bg-muted/50"
								onclick={createRowClickHandler(
									`/ai-traces/${encodeURIComponent(decodeURIComponent(data.traceName))}/${trace.id}?t=${encodeURIComponent(trace.recordedAt)}`,
									'preset',
									'from',
									'to'
								)}
							>
								<Table.Cell class="text-muted-foreground">
									{formatDateTime(trace.recordedAt, { timezone })}
								</Table.Cell>
								<Table.Cell class="font-mono text-sm">
									{formatDuration(trace.duration)}
								</Table.Cell>
								<Table.Cell class="font-mono text-sm">
									{trace.model || '-'}
								</Table.Cell>
								<Table.Cell class="font-mono text-sm tabular-nums">
									{trace.inputTokens.toLocaleString()}
								</Table.Cell>
								<Table.Cell class="font-mono text-sm tabular-nums">
									{trace.outputTokens.toLocaleString()}
								</Table.Cell>
								<Table.Cell class="font-mono text-sm tabular-nums">
									{formatCost(trace.totalCost)}
								</Table.Cell>
								<Table.Cell class="font-mono text-sm text-muted-foreground">
									{trace.provider || '-'}
								</Table.Cell>
							</Table.Row>
						{/each}
					{/if}
				</Table.Body>
			</Table.Root>
		</div>

		<PaginationFooter
			currentPage={page}
			{totalPages}
			{pageSize}
			totalItems={total}
			onPageChange={handlePageChange}
			onPageSizeChange={handlePageSizeChange}
			{loading}
			itemLabel="trace"
		/>
	{/if}
</div>
