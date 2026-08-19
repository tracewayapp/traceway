<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { api } from '$lib/api';
	import { formatDuration, toUTCISO, calendarDateTimeToLuxon } from '$lib/utils/formatters';
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
	import { CalendarDate } from '@internationalized/date';
	import { browser } from '$app/environment';
	import { projectsState } from '$lib/state/projects.svelte';
	import { createRowClickHandler } from '$lib/utils/navigation';
	import { resolve } from '$app/paths';
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import TableContainer from '$lib/components/traceway/table-container.svelte';
	import {
		presetMinutes,
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

	type TaskStats = {
		taskName: string;
		count: number;
		p50Duration: number;
		p95Duration: number;
		avgDuration: number;
		lastSeen: string;
		hasRoot: boolean;
		hasNonRoot: boolean;
	};

	type SortField = 'count' | 'p50_duration' | 'p95_duration' | 'last_seen';

	let tasks = $state<TaskStats[]>([]);
	let loading = $state(true);
	let error = $state('');

	// Pagination State
	let page = $state(1);
	let pageSize = $state(50);
	let total = $state(0);
	let totalPages = $state(0);

	// Initialize from URL
	const initialUrlParams = parseTimeRangeFromUrl(initialTimezone);
	const initialRange = getResolvedTimeRange(initialUrlParams, initialTimezone);

	function readSearchAndFilterFromUrl() {
		if (!browser) return { search: '', rootFilter: 'all' };
		const params = new URLSearchParams(window.location.search);
		return {
			search: params.get('search') || '',
			rootFilter: params.get('rootFilter') || 'all'
		};
	}

	const initialSearchState = readSearchAndFilterFromUrl();
	let searchQuery = $state(initialSearchState.search);
	let rootFilter = $state(initialSearchState.rootFilter);

	// Date Range State
	let selectedPreset = $state<string | null>(initialUrlParams.preset);
	let fromDate = $state<CalendarDate>(dateToCalendarDate(initialRange.from, initialTimezone));
	let toDate = $state<CalendarDate>(dateToCalendarDate(initialRange.to, initialTimezone));
	let fromTime = $state(dateToTimeString(initialRange.from, initialTimezone));
	let toTime = $state(dateToTimeString(initialRange.to, initialTimezone));

	// Update URL with current time range + search + rootFilter
	function updateTimeRangeUrl(pushToHistory = true) {
		const params: Record<string, string | undefined> = selectedPreset
			? { preset: selectedPreset }
			: { from: getFromDateTimeUTC(), to: getToDateTimeUTC() };
		if (searchQuery) params.search = searchQuery;
		if (rootFilter && rootFilter !== 'all') params.rootFilter = rootFilter;
		updateUrl(params, { pushToHistory });
	}

	// Handle browser back/forward navigation
	function handlePopState() {
		const urlParams = parseTimeRangeFromUrl(timezone);
		const range = getResolvedTimeRange(urlParams, timezone);

		selectedPreset = urlParams.preset;
		fromDate = dateToCalendarDate(range.from, timezone);
		fromTime = dateToTimeString(range.from, timezone);
		toDate = dateToCalendarDate(range.to, timezone);
		toTime = dateToTimeString(range.to, timezone);

		page = 1;
		loadData(false);
	}

	// Sorting - persisted to localStorage
	const SORT_STORAGE_KEY = 'tasks';
	const initialSort = getSortState(SORT_STORAGE_KEY, { field: 'count', direction: 'desc' });
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
		loadData(true);
	}

	// Format count with k/m suffixes
	function formatCount(count: number): string {
		if (count >= 1_000_000) {
			return `${(count / 1_000_000).toFixed(1)}m`;
		} else if (count >= 1_000) {
			return `${(count / 1_000).toFixed(1)}k`;
		}
		return count.toLocaleString();
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
				search: searchQuery,
				rootFilter: rootFilter === 'all' ? '' : rootFilter,
				pagination: {
					page: page,
					pageSize: pageSize
				}
			};

			const response = await api.post('/tasks/grouped', requestBody, {
				projectId: projectsState.currentProjectId ?? undefined
			});

			tasks = response.data || [];
			total = response.pagination.total;
			totalPages = response.pagination.totalPages;
		} catch (e: any) {
			console.error(e);
			error = e.message || 'Failed to load data';
		} finally {
			loading = false;
		}
	}

	function handlePageChange(newPage: number) {
		if (newPage >= 1 && newPage <= totalPages) {
			page = newPage;
			loadData(false); // Don't push to history for pagination
		}
	}

	function handlePageSizeChange(newPageSize: number) {
		pageSize = newPageSize;
		page = 1;
		loadData(false); // Don't push to history for pagination
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
	<PageHeader title="Tasks">
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

	<!-- Search -->
	<SearchBar
		placeholder="Search tasks..."
		bind:value={searchQuery}
		onSearch={handleSearch}
		disabled={loading}
	>
		{#snippet children()}
			<RootFilter bind:value={rootFilter} />
		{/snippet}
	</SearchBar>

	<!-- Tasks Table -->
	<TableContainer minWidth="640px">
		<Table.Root>
			{#if loading}
				<Table.Body>
					<Table.Row>
						<Table.Cell colspan={4} class="h-48">
							<div class="flex h-full items-center justify-center">
								<LoadingCircle size="xlg" />
							</div>
						</Table.Cell>
					</Table.Row>
				</Table.Body>
			{:else if error}
				<Table.Body>
					<Table.Row>
						<Table.Cell colspan={4} class="h-24 text-center text-red-500">
							{error}
						</Table.Cell>
					</Table.Row>
				</Table.Body>
			{:else if tasks.length === 0}
				<Table.Body>
					<TableEmptyState colspan={4} message="No task data received yet" />
				</Table.Body>
			{:else}
				<Table.Header>
					<Table.Row>
						<TracewayTableHeader label="Task" tooltip="The background job or task name" />
						<TracewayTableHeader
							label="Runs"
							tooltip="Total number of task executions"
							sortField="count"
							currentSortField={orderBy}
							{sortDirection}
							onSort={(field) => handleSort(field as SortField)}
							class="w-[100px]"
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
							tooltip="95th percentile - slowest 5% of executions"
							sortField="p95_duration"
							currentSortField={orderBy}
							{sortDirection}
							onSort={(field) => handleSort(field as SortField)}
							class="w-[100px]"
						/>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each tasks as task, __index (__index)}
						<Table.Row
							class="cursor-pointer"
							onclick={createRowClickHandler(
								resolve(`/tasks/${encodeURIComponent(task.taskName)}`),
								'preset',
								'from',
								'to'
							)}
						>
							<Table.Cell class="font-mono text-sm">
								{task.taskName}
								{#if task.hasNonRoot}
									<NonRootChip mixed={task.hasRoot && task.hasNonRoot} />
								{/if}
							</Table.Cell>
							<Table.Cell class="tabular-nums">
								{formatCount(task.count)}
							</Table.Cell>
							<Table.Cell class="font-mono text-sm tabular-nums">
								{formatDuration(task.p50Duration)}
							</Table.Cell>
							<Table.Cell class="font-mono text-sm tabular-nums">
								{formatDuration(task.p95Duration)}
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
		itemLabel="task"
	/>
</div>
