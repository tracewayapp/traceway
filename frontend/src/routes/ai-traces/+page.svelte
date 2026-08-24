<script lang="ts">
	import { getErrorMessage } from '$lib/utils/errors';
	import { onMount, onDestroy } from 'svelte';
	import { api } from '$lib/api';
	import {
		formatDuration,
		formatDateTime,
		toUTCISO,
		calendarDateTimeToLuxon
	} from '$lib/utils/formatters';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import * as Table from '$lib/components/ui/table';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { TracewayTableHeader } from '$lib/components/ui/traceway-table-header';
	import { TableEmptyState } from '$lib/components/ui/table-empty-state';
	import { PaginationFooter } from '$lib/components/ui/pagination-footer';
	import { TimeRangePicker } from '$lib/components/ui/time-range-picker';
	import { SearchBar } from '$lib/components/ui/search-bar';
	import { RootFilter } from '$lib/components/ui/root-filter';
	import { NonRootChip } from '$lib/components/ui/non-root-chip';
	import { browser } from '$app/environment';
	import { CalendarDate } from '@internationalized/date';
	import { projectsState } from '$lib/state/projects.svelte';
	import { createRowClickHandler } from '$lib/utils/navigation';
	import { resolve } from '$app/paths';
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import TableContainer from '$lib/components/traceway/table-container.svelte';
	import { formatCost, formatCount, formatTokens } from '$lib/utils/ai-format';
	import AiNavTabs from '$lib/components/ai/ai-nav-tabs.svelte';
	import {
		getTimeRangeFromPreset,
		dateToCalendarDate,
		dateToTimeString,
		parseTimeRangeFromUrl,
		getResolvedTimeRange,
		updateUrl
	} from '$lib/utils/url-params';
	import {
		getSortState,
		setSortState,
		handleSortClick,
		type SortDirection
	} from '$lib/utils/sort-storage';

	const timezone = $derived(getTimezone());
	const initialTimezone = getTimezone();

	type AiTraceStats = {
		traceName: string;
		count: number;
		p50Duration: number;
		p95Duration: number;
		avgDuration: number;
		totalTokens: number;
		totalCost: number;
		avgInputTokens: number;
		avgOutputTokens: number;
		lastSeen: string;
		hasRoot: boolean;
		hasNonRoot: boolean;
	};

	type SortField =
		| 'count'
		| 'p50_duration'
		| 'p95_duration'
		| 'total_tokens'
		| 'total_cost'
		| 'last_seen';

	let traces = $state<AiTraceStats[]>([]);
	let loading = $state(true);
	let error = $state('');

	// Pagination State
	let page = $state(1);
	let pageSize = $state(50);
	let total = $state(0);
	let totalPages = $state(0);

	// Parse URL params including search + rootFilter
	function parseAiTracesUrlParams() {
		if (!browser) return { preset: '24h', from: null, to: null, search: '', rootFilter: 'all' };
		const params = new URLSearchParams(window.location.search);
		const timeParams = parseTimeRangeFromUrl(timezone, '24h');
		return {
			...timeParams,
			search: params.get('search') || '',
			rootFilter: params.get('rootFilter') || 'all'
		};
	}

	// Initialize from URL
	const initialUrlParams = parseAiTracesUrlParams();
	const initialRange = getResolvedTimeRange(initialUrlParams, initialTimezone);

	// Search + rootFilter state
	let searchQuery = $state(initialUrlParams.search);
	let rootFilter = $state(initialUrlParams.rootFilter);

	// Date Range State
	let selectedPreset = $state<string | null>(initialUrlParams.preset);
	let fromDate = $state<CalendarDate>(dateToCalendarDate(initialRange.from, initialTimezone));
	let toDate = $state<CalendarDate>(dateToCalendarDate(initialRange.to, initialTimezone));
	let fromTime = $state(dateToTimeString(initialRange.from, initialTimezone));
	let toTime = $state(dateToTimeString(initialRange.to, initialTimezone));

	// Update URL with current time range and search
	function updateTimeRangeUrl(pushToHistory = true) {
		const params: Record<string, string | null | undefined> = {};
		if (selectedPreset) {
			params.preset = selectedPreset;
		} else {
			params.from = getFromDateTimeUTC();
			params.to = getToDateTimeUTC();
		}
		if (searchQuery.trim()) {
			params.search = searchQuery.trim();
		}
		if (rootFilter && rootFilter !== 'all') {
			params.rootFilter = rootFilter;
		}
		updateUrl(params, { pushToHistory });
	}

	// Handle browser back/forward navigation
	function handlePopState() {
		const urlParams = parseAiTracesUrlParams();
		const range = getResolvedTimeRange(urlParams, timezone);

		selectedPreset = urlParams.preset;
		fromDate = dateToCalendarDate(range.from, timezone);
		fromTime = dateToTimeString(range.from, timezone);
		toDate = dateToCalendarDate(range.to, timezone);
		toTime = dateToTimeString(range.to, timezone);
		searchQuery = urlParams.search;
		rootFilter = urlParams.rootFilter;

		page = 1;
		loadData(false);
	}

	// Sorting - persisted to localStorage
	const SORT_STORAGE_KEY = 'ai-traces';
	const initialSort = getSortState(SORT_STORAGE_KEY, { field: 'total_cost', direction: 'desc' });
	let orderBy = $state<SortField>(initialSort.field as SortField);
	let sortDirection = $state<SortDirection>(initialSort.direction);

	// Combine date and time into UTC ISO datetime string
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
		loadData(false);
	}

	async function loadData(pushToHistory = true) {
		loading = true;
		error = '';

		if (selectedPreset) {
			const range = getTimeRangeFromPreset(selectedPreset, timezone);
			fromDate = dateToCalendarDate(range.from, timezone);
			toDate = dateToCalendarDate(range.to, timezone);
			fromTime = dateToTimeString(range.from, timezone);
			toTime = dateToTimeString(range.to, timezone);
		}

		// Update URL
		updateTimeRangeUrl(pushToHistory);

		try {
			const requestBody = {
				fromDate: getFromDateTimeUTC(),
				toDate: getToDateTimeUTC(),
				orderBy: orderBy,
				sortDirection: sortDirection,
				pagination: {
					page: page,
					pageSize: pageSize
				},
				search: searchQuery.trim(),
				rootFilter: rootFilter === 'all' ? '' : rootFilter
			};

			const response = await api.post('/ai-traces/grouped', requestBody, {
				projectId: projectsState.currentProjectId ?? undefined
			});

			traces = response.data || [];
			total = response.pagination.total;
			totalPages = response.pagination.totalPages;
		} catch (e) {
			console.error(e);
			error = getErrorMessage(e) || 'Failed to load data';
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

	function handleSearch() {
		page = 1;
		loadData(true);
	}

	onMount(() => {
		// Add popstate listener for back/forward navigation
		window.addEventListener('popstate', handlePopState);

		// Initial load with replaceState (don't push to history)
		loadData(false);
	});

	onDestroy(() => {
		if (typeof window !== 'undefined') {
			window.removeEventListener('popstate', handlePopState);
		}
	});
</script>

<div class="space-y-4">
	<PageHeader title="AI Traces">
		{#snippet actions()}
			<TimeRangePicker
				bind:fromDate
				bind:toDate
				bind:fromTime
				bind:toTime
				bind:preset={selectedPreset}
				onApply={handleTimeRangeChange}
			/>
		{/snippet}
	</PageHeader>

	<AiNavTabs active="traces" />

	<!-- Search -->
	<SearchBar
		placeholder="Search traces..."
		bind:value={searchQuery}
		onSearch={handleSearch}
		disabled={loading}
	>
		{#snippet pillEnd()}
			<RootFilter
				bind:value={rootFilter}
				class="h-9 w-[110px] shrink-0 shadow-none sm:rounded-none sm:border-r-0"
			/>
		{/snippet}
	</SearchBar>

	<!-- AI Traces Table -->
	<TableContainer>
		<Table.Root>
			{#if loading}
				<Table.Body>
					<Table.Row>
						<Table.Cell colspan={7} class="h-48">
							<div class="flex h-full items-center justify-center">
								<LoadingCircle size="xlg" />
							</div>
						</Table.Cell>
					</Table.Row>
				</Table.Body>
			{:else if error}
				<Table.Body>
					<Table.Row>
						<Table.Cell colspan={7} class="h-24 text-center text-red-500">
							{error}
						</Table.Cell>
					</Table.Row>
				</Table.Body>
			{:else if traces.length === 0}
				<Table.Body>
					<TableEmptyState
						colspan={7}
						message="No AI trace data available for your search parameters"
					/>
				</Table.Body>
			{:else}
				<Table.Header>
					<Table.Row>
						<TracewayTableHeader label="Trace Name" tooltip="The AI agent or workflow name" />
						<TracewayTableHeader
							label="Calls"
							tooltip="Total number of AI calls"
							sortField="count"
							currentSortField={orderBy}
							{sortDirection}
							onSort={(field) => handleSort(field as SortField)}
							class="w-[80px]"
						/>
						<TracewayTableHeader
							label="Typical"
							tooltip="Median duration (P50)"
							sortField="p50_duration"
							currentSortField={orderBy}
							{sortDirection}
							onSort={(field) => handleSort(field as SortField)}
							class="w-[100px]"
						/>
						<TracewayTableHeader
							label="Slow"
							tooltip="95th percentile duration"
							sortField="p95_duration"
							currentSortField={orderBy}
							{sortDirection}
							onSort={(field) => handleSort(field as SortField)}
							class="w-[100px]"
						/>
						<TracewayTableHeader
							label="Total Tokens"
							tooltip="Total tokens consumed"
							sortField="total_tokens"
							currentSortField={orderBy}
							{sortDirection}
							onSort={(field) => handleSort(field as SortField)}
							class="w-[110px]"
						/>
						<TracewayTableHeader
							label="Total Cost"
							tooltip="Total cost across all calls"
							sortField="total_cost"
							currentSortField={orderBy}
							{sortDirection}
							onSort={(field) => handleSort(field as SortField)}
							class="w-[100px]"
						/>
						<TracewayTableHeader
							label="Last Seen"
							tooltip="When the last call occurred"
							sortField="last_seen"
							currentSortField={orderBy}
							{sortDirection}
							onSort={(field) => handleSort(field as SortField)}
							class="w-[100px]"
						/>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each traces as trace, __index (__index)}
						<!-- Trace names "conversations" and "users" are shadowed by the
                         static sibling routes, so those two names are unreachable here. -->
						<Table.Row
							class="cursor-pointer"
							onclick={createRowClickHandler(
								resolve(`/ai-traces/${encodeURIComponent(trace.traceName)}`),
								'preset',
								'from',
								'to'
							)}
						>
							<Table.Cell class="font-mono text-sm">
								{trace.traceName}
								{#if trace.hasNonRoot}
									<NonRootChip mixed={trace.hasRoot && trace.hasNonRoot} />
								{/if}
							</Table.Cell>
							<Table.Cell class="tabular-nums">
								{formatCount(trace.count)}
							</Table.Cell>
							<Table.Cell class="font-mono text-sm tabular-nums">
								{formatDuration(trace.p50Duration)}
							</Table.Cell>
							<Table.Cell class="font-mono text-sm tabular-nums">
								{formatDuration(trace.p95Duration)}
							</Table.Cell>
							<Table.Cell class="font-mono text-sm tabular-nums">
								{formatTokens(trace.totalTokens)}
							</Table.Cell>
							<Table.Cell class="font-mono text-sm tabular-nums">
								{formatCost(trace.totalCost)}
							</Table.Cell>
							<Table.Cell class="text-sm text-muted-foreground">
								{formatDateTime(trace.lastSeen, { timezone, format: 'date' })}
							</Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			{/if}
		</Table.Root>
	</TableContainer>

	<!-- Pagination Footer -->
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
</div>
