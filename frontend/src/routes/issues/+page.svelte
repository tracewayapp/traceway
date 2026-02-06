<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { browser } from '$app/environment';
	import { createRowClickHandler } from '$lib/utils/navigation';
	import { api } from '$lib/api';
	import * as Table from '$lib/components/ui/table';
	import { SearchBar } from '$lib/components/ui/search-bar';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import { projectsState } from '$lib/state/projects.svelte';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import IssueTrendChart from '$lib/components/issue-trend-chart.svelte';
	import Archive from '@lucide/svelte/icons/archive';
	import { toast } from 'svelte-sonner';
	import ArchiveConfirmationDialog from '$lib/components/archive-confirmation-dialog.svelte';
	import { TracewayTableHeader } from '$lib/components/ui/traceway-table-header';
	import { TableEmptyState } from '$lib/components/ui/table-empty-state';
	import { PaginationFooter } from '$lib/components/ui/pagination-footer';
	import { TimeRangePicker } from '$lib/components/ui/time-range-picker';
	import { CalendarDate } from '@internationalized/date';
	import {
		parseTimeRangeFromUrl,
		getResolvedTimeRange,
		dateToCalendarDate,
		dateToTimeString,
		updateUrl
	} from '$lib/utils/url-params';
	import { calendarDateTimeToLuxon, toUTCISO, formatDateTime } from '$lib/utils/formatters';
	import {
		getSortState,
		setSortState,
		handleSortClick,
		type SortDirection
	} from '$lib/utils/sort-storage';

	const timezone = $derived(getTimezone());

	// Sort state (persisted to localStorage)
	const SORT_STORAGE_KEY = 'issues';
	const initialSort = getSortState(SORT_STORAGE_KEY, { field: 'last_seen', direction: 'desc' });
	let sortField = $state(initialSort.field);
	let sortDirection = $state<SortDirection>(initialSort.direction);

	type ExceptionTrendPoint = {
		timestamp: string;
		count: number;
	};

	type ExceptionGroup = {
		exceptionHash: string;
		stackTrace: string;
		lastSeen: string;
		firstSeen: string;
		count: number;
		hourlyTrend: ExceptionTrendPoint[];
	};

	let exceptions = $state<ExceptionGroup[]>([]);
	let loading = $state(true);
	let error = $state('');
	let archiving = $state(false);
	let showArchiveDialog = $state(false);

	// Pagination State
	let page = $state(1);
	let pageSize = $state(20);
	let total = $state(0);
	let totalPages = $state(0);

	// Selection State
	let selectedHashes = $state<Set<string>>(new Set());

	// Derived selection states
	const selectedCount = $derived(selectedHashes.size);
	const allSelected = $derived(exceptions.length > 0 && selectedHashes.size === exceptions.length);
	const someSelected = $derived(selectedHashes.size > 0 && selectedHashes.size < exceptions.length);

	// Parse URL params on init
	function parseIssuesUrlParams() {
		if (!browser) return { preset: '24h', from: null, to: null, search: '', searchType: 'all' };
		const params = new URLSearchParams(window.location.search);
		const timeParams = parseTimeRangeFromUrl(timezone, '24h');
		return {
			...timeParams,
			search: params.get('search') || '',
			searchType: params.get('searchType') || 'all'
		};
	}

	const initialUrlParams = parseIssuesUrlParams();
	const initialRange = getResolvedTimeRange(initialUrlParams, timezone);

	// Time range state
	let selectedPreset = $state<string | null>(initialUrlParams.preset);
	let fromDate = $state<CalendarDate>(dateToCalendarDate(initialRange.from, timezone));
	let toDate = $state<CalendarDate>(dateToCalendarDate(initialRange.to, timezone));
	let fromTime = $state(dateToTimeString(initialRange.from, timezone));
	let toTime = $state(dateToTimeString(initialRange.to, timezone));

	// Search state (manual trigger only)
	let searchQuery = $state(initialUrlParams.search);
	let searchType = $state(initialUrlParams.searchType);

	// Search type options
	const searchTypeOptions = [
		{ value: 'all', label: 'All' },
		{ value: 'issues', label: 'Issues' },
		{ value: 'messages', label: 'Messages' }
	];

	// Page size options
	const pageSizeOptions = [
		{ value: '10', label: '10' },
		{ value: '20', label: '20' },
		{ value: '50', label: '50' },
		{ value: '100', label: '100' }
	];

	// Helper functions for date/time conversion
	function getFromDateTimeUTC(): string {
		const [hour, minute] = fromTime.split(':').map(Number);
		const luxonDt = calendarDateTimeToLuxon(
			{ year: fromDate.year, month: fromDate.month, day: fromDate.day, hour, minute },
			timezone
		);
		return toUTCISO(luxonDt);
	}

	function getToDateTimeUTC(): string {
		const [hour, minute] = toTime.split(':').map(Number);
		const luxonDt = calendarDateTimeToLuxon(
			{ year: toDate.year, month: toDate.month, day: toDate.day, hour, minute },
			timezone
		);
		return toUTCISO(luxonDt);
	}

	function updateIssuesUrl(pushToHistory = true) {
		const params: Record<string, string | null | undefined> = {};
		if (selectedPreset) {
			params.preset = selectedPreset;
		} else {
			params.from = getFromDateTimeUTC();
			params.to = getToDateTimeUTC();
		}
		if (searchQuery.trim()) params.search = searchQuery.trim();
		if (searchType !== 'all') params.searchType = searchType;
		updateUrl(params, { pushToHistory });
	}

	async function loadData(pushToHistory = true) {
		loading = true;
		error = '';

		updateIssuesUrl(pushToHistory);

		try {
			// Build orderBy with direction suffix for backend
			const orderBy = sortDirection === 'asc' ? `${sortField}_asc` : sortField;

			const requestBody = {
				fromDate: getFromDateTimeUTC(),
				toDate: getToDateTimeUTC(),
				orderBy,
				pagination: {
					page: page,
					pageSize: pageSize
				},
				search: searchQuery.trim(),
				searchType: searchType,
				includeArchived: false
			};

			const response = await api.post('/exception-stack-traces', requestBody, {
				projectId: projectsState.currentProjectId ?? undefined
			});

			exceptions = response.data || [];
			total = response.pagination.total;
			totalPages = response.pagination.totalPages;

			// Clear selection when data changes
			selectedHashes = new Set();
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
			loadData(true);
		}
	}

	function handlePageSizeChange(newPageSize: number) {
		pageSize = newPageSize;
		page = 1;
		loadData(true);
	}

	function handleTimeRangeChange(
		from: { date: CalendarDate; time: string },
		to: { date: CalendarDate; time: string },
		preset: string | null
	) {
		fromDate = from.date;
		toDate = to.date;
		fromTime = from.time;
		toTime = to.time;
		selectedPreset = preset;
		page = 1;
		loadData(true);
	}

	function handleSearch() {
		page = 1;
		loadData(true);
	}

	function handleSort(field: string) {
		const newSort = handleSortClick(field, sortField, sortDirection);
		sortField = newSort.field;
		sortDirection = newSort.direction;
		setSortState(SORT_STORAGE_KEY, newSort);
		page = 1;
		loadData(true);
	}

	function handlePopState() {
		const urlParams = parseIssuesUrlParams();
		const range = getResolvedTimeRange(urlParams, timezone);
		selectedPreset = urlParams.preset;
		fromDate = dateToCalendarDate(range.from, timezone);
		toDate = dateToCalendarDate(range.to, timezone);
		fromTime = dateToTimeString(range.from, timezone);
		toTime = dateToTimeString(range.to, timezone);
		searchQuery = urlParams.search;
		searchType = urlParams.searchType;
		page = 1;
		loadData(false);
	}

	// Selection handlers
	function toggleSelectAll() {
		if (allSelected) {
			selectedHashes = new Set();
		} else {
			selectedHashes = new Set(exceptions.map((e) => e.exceptionHash));
		}
	}

	function toggleSelect(hash: string) {
		const newSet = new Set(selectedHashes);
		if (newSet.has(hash)) {
			newSet.delete(hash);
		} else {
			newSet.add(hash);
		}
		selectedHashes = newSet;
	}

	function isSelected(hash: string): boolean {
		return selectedHashes.has(hash);
	}

	// Archive handler
	async function archiveSelected() {
		if (selectedHashes.size === 0) return;

		archiving = true;
		try {
			await api.post(
				'/exception-stack-traces/archive',
				{
					hashes: Array.from(selectedHashes)
				},
				{ projectId: projectsState.currentProjectId ?? undefined }
			);

			toast.success('Successfully archived the Issue' + (selectedHashes.size > 1 ? 's' : ''), {
				position: 'top-center'
			});
			selectedHashes = new Set();
			await loadData();
		} catch (e: any) {
			console.error('Archive failed:', e);
			error = e.message || 'Failed to archive issues';
			throw e;
		} finally {
			archiving = false;
		}
	}

	onMount(() => {
		window.addEventListener('popstate', handlePopState);
		loadData(false);
	});

	onDestroy(() => {
		if (typeof window !== 'undefined') {
			window.removeEventListener('popstate', handlePopState);
		}
	});
</script>

<div class="space-y-4">
	<!-- Row 1: Title + TimeRangePicker -->
	<div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
		<h2 class="text-2xl font-bold tracking-tight">Issues</h2>
		<div class="w-full sm:w-auto">
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

	<!-- Row 2: Search -->
	<SearchBar
		placeholder="Search exceptions..."
		bind:value={searchQuery}
		bind:typeValue={searchType}
		typeOptions={searchTypeOptions}
		onSearch={handleSearch}
		disabled={loading}
	/>

	<!-- Archive Toolbar - shown when items selected -->
	{#if selectedCount > 0}
		<div
			class="flex animate-in items-center gap-3 rounded-lg border bg-muted/50 p-3 duration-200 fade-in slide-in-from-top-1"
		>
			<span class="text-sm font-medium"
				>{selectedCount} issue{selectedCount === 1 ? '' : 's'} selected</span
			>
			<Button
				variant="outline"
				size="sm"
				onclick={() => (showArchiveDialog = true)}
				disabled={archiving}
				class="gap-1.5"
			>
				<Archive class="h-4 w-4" />
				Archive
			</Button>
			<Button variant="ghost" size="sm" onclick={() => (selectedHashes = new Set())}>
				Clear selection
			</Button>
		</div>
	{/if}

	<div class="overflow-hidden rounded-md border">
		<Table.Root>
			{#if loading}
				<Table.Body>
					<Table.Row>
						<Table.Cell colspan={5} class="h-48">
							<div class="flex h-full items-center justify-center">
								<LoadingCircle size="xlg" />
							</div>
						</Table.Cell>
					</Table.Row>
				</Table.Body>
			{:else if error}
				<Table.Body>
					<Table.Row>
						<Table.Cell colspan={5} class="h-24 text-center text-red-500">
							{error}
						</Table.Cell>
					</Table.Row>
				</Table.Body>
			{:else if exceptions.length === 0}
				<Table.Body>
					<TableEmptyState colspan={5} message="No issues found." />
				</Table.Body>
			{:else}
				<Table.Header>
					<Table.Row>
						<Table.Head class="w-[40px] pl-4">
							<Checkbox
								checked={allSelected ? true : someSelected ? 'indeterminate' : false}
								onCheckedChange={toggleSelectAll}
								aria-label="Select all"
							/>
						</Table.Head>
						<TracewayTableHeader
							label="Issue"
							tooltip="The error message or exception that occurred"
						/>
						<TracewayTableHeader
							label="Trend"
							tooltip="Hourly occurrence pattern over the last 24h"
							class="w-[190px]"
						/>
						<TracewayTableHeader
							label="Events"
							tooltip="Total number of times this issue occurred in the selected range"
							align="right"
							class="w-[80px]"
							sortField="count"
							currentSortField={sortField}
							{sortDirection}
							onSort={handleSort}
						/>
						<TracewayTableHeader
							label="Last Seen"
							tooltip="When this issue last occurred"
							class="w-[180px]"
							sortField="last_seen"
							currentSortField={sortField}
							{sortDirection}
							onSort={handleSort}
						/>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each exceptions as exception (exception.exceptionHash)}
						<Table.Row
							class="group cursor-pointer hover:bg-muted/50"
							data-state={isSelected(exception.exceptionHash) ? 'selected' : undefined}
						>
							<Table.Cell class="pl-4" onclick={(e) => e.stopPropagation()}>
								<Checkbox
									checked={isSelected(exception.exceptionHash)}
									onCheckedChange={() => toggleSelect(exception.exceptionHash)}
									aria-label="Select row"
								/>
							</Table.Cell>
							<Table.Cell
								class="max-w-[400px] truncate font-mono text-sm"
								title={exception.stackTrace}
								onclick={createRowClickHandler(
									`/issues/${exception.exceptionHash}`,
									'preset',
									'from',
									'to'
								)}
							>
								<span class="text-foreground">{exception.stackTrace.split('\n')[0]}</span>
							</Table.Cell>
							<Table.Cell
								onclick={createRowClickHandler(
									`/issues/${exception.exceptionHash}`,
									'preset',
									'from',
									'to'
								)}
							>
								<IssueTrendChart trend={exception.hourlyTrend || []} />
							</Table.Cell>
							<Table.Cell
								class="text-right font-medium tabular-nums"
								onclick={createRowClickHandler(
									`/issues/${exception.exceptionHash}`,
									'preset',
									'from',
									'to'
								)}
							>
								{exception.count.toLocaleString()}
							</Table.Cell>
							<Table.Cell
								class="text-muted-foreground"
								onclick={createRowClickHandler(
									`/issues/${exception.exceptionHash}`,
									'preset',
									'from',
									'to'
								)}
							>
								{formatDateTime(exception.lastSeen, { timezone })}
							</Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			{/if}
		</Table.Root>
	</div>

	<!-- Pagination Footer -->
	<PaginationFooter
		currentPage={page}
		{totalPages}
		{pageSize}
		totalItems={total}
		onPageChange={handlePageChange}
		onPageSizeChange={handlePageSizeChange}
		{loading}
		itemLabel="issue"
	/>
</div>

<ArchiveConfirmationDialog
	open={showArchiveDialog}
	onOpenChange={(open) => (showArchiveDialog = open)}
	count={selectedCount}
	onConfirm={archiveSelected}
/>
