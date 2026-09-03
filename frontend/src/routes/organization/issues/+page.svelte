<script lang="ts">
	import { onDestroy, onMount, untrack } from 'svelte';
	import { browser } from '$app/environment';
	import { api } from '$lib/api';
	import { organizationContext } from '$lib/state/organization-context.svelte';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import * as Table from '$lib/components/ui/table';
	import { SearchBar } from '$lib/components/ui/search-bar';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { TracewayTableHeader } from '$lib/components/ui/traceway-table-header';
	import { TableEmptyState } from '$lib/components/ui/table-empty-state';
	import { PaginationFooter } from '$lib/components/ui/pagination-footer';
	import { TimeRangePicker } from '$lib/components/ui/time-range-picker';
	import IssueTrendChart from '$lib/components/issue-trend-chart.svelte';
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import TableContainer from '$lib/components/traceway/table-container.svelte';
	import CountPill from '../count-pill.svelte';
	import PartialNotice from '../partial-notice.svelte';
	import { getErrorMessage } from '$lib/utils/errors';
	import { formatDateTime } from '$lib/utils/formatters';
	import { createRowClickHandler } from '$lib/utils/navigation';
	import { TimeRangeState } from '$lib/utils/time-range-state.svelte';
	import { updateUrl } from '$lib/utils/url-params';
	import {
		getSortState,
		setSortState,
		handleSortClick,
		type SortDirection
	} from '$lib/utils/sort-storage';

	interface OrgIssueRow {
		exceptionHash: string;
		stackTrace: string;
		lastSeen: string;
		firstSeen: string;
		count: number;
		hourlyTrend: { timestamp: string; count: number }[];
		projectId: string;
		projectName: string;
	}

	const timezone = $derived(getTimezone());
	const range = new TimeRangeState('24h');

	const SORT_STORAGE_KEY = 'org-issues';
	const initialSort = getSortState(SORT_STORAGE_KEY, { field: 'last_seen', direction: 'desc' });
	let sortField = $state(initialSort.field);
	let sortDirection = $state<SortDirection>(initialSort.direction);

	const searchTypeOptions = [
		{ value: 'all', label: 'All' },
		{ value: 'issues', label: 'Issues' },
		{ value: 'messages', label: 'Messages' }
	];

	function readSearchParams() {
		if (!browser) return { search: '', searchType: 'all' };
		const params = new URLSearchParams(window.location.search);
		return {
			search: params.get('search') || '',
			searchType: params.get('searchType') || 'all'
		};
	}

	const initialSearch = readSearchParams();
	let searchQuery = $state(initialSearch.search);
	let searchType = $state(initialSearch.searchType);

	let issues = $state<OrgIssueRow[]>([]);
	let loading = $state(true);
	let error = $state('');
	let partial = $state(false);
	let page = $state(1);
	let pageSize = $state(20);
	let total = $state(0);
	let totalPages = $state(0);
	let loadGeneration = 0;
	let activeOrganizationId: number | null = null;

	function syncUrl(pushToHistory: boolean) {
		updateUrl(
			{
				...range.urlParams(),
				search: searchQuery.trim() || null,
				searchType: searchType !== 'all' ? searchType : null
			},
			{ pushToHistory }
		);
	}

	async function loadData(pushToHistory = true) {
		const organizationId = organizationContext.organizationId;
		if (organizationId === null) return;
		const generation = ++loadGeneration;
		loading = true;
		error = '';
		range.resolvePreset();
		syncUrl(pushToHistory);
		try {
			const orderBy = sortDirection === 'asc' ? `${sortField}_asc` : sortField;
			const response = await api.post(`/organizations/${organizationId}/overview/issues`, {
				fromDate: range.fromUTC(),
				toDate: range.toUTC(),
				orderBy,
				pagination: { page, pageSize },
				search: searchQuery.trim(),
				searchType
			});
			if (generation !== loadGeneration) return;
			issues = response.data || [];
			total = response.pagination?.total || 0;
			totalPages = response.pagination?.totalPages || 0;
			partial = response.partial === true;
		} catch (e) {
			if (generation !== loadGeneration) return;
			error = getErrorMessage(e) || 'Failed to load issues';
		} finally {
			if (generation === loadGeneration) loading = false;
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
		from: Parameters<TimeRangeState['apply']>[0],
		to: Parameters<TimeRangeState['apply']>[1],
		preset: string | null
	) {
		range.apply(from, to, preset);
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
		range.readUrl();
		const params = readSearchParams();
		searchQuery = params.search;
		searchType = params.searchType;
		page = 1;
		loadData(false);
	}

	$effect(() => {
		const organizationId = organizationContext.organizationId;
		if (organizationId === null || organizationId === activeOrganizationId) return;
		activeOrganizationId = organizationId;
		untrack(() => {
			page = 1;
			loadData(false);
		});
	});

	onMount(() => {
		window.addEventListener('popstate', handlePopState);
	});

	onDestroy(() => {
		if (typeof window !== 'undefined') {
			window.removeEventListener('popstate', handlePopState);
		}
	});

	function issueTitle(row: OrgIssueRow): { type: string; message: string } {
		const firstLine = row.stackTrace.split('\n')[0];
		const colonIndex = firstLine.indexOf(':');
		return {
			type: colonIndex > 0 ? firstLine.slice(0, colonIndex) : firstLine,
			message: colonIndex > 0 ? firstLine.slice(colonIndex + 1).trim() : ''
		};
	}
</script>

<div class="space-y-4">
	<PageHeader
		title="Issues"
		description="Issues from every project in the selected range, with the project each belongs to."
	>
		{#snippet trailing()}
			{#if !loading && !error}
				<CountPill count={total} />
			{/if}
		{/snippet}
		{#snippet actions()}
			<TimeRangePicker
				bind:fromDate={range.fromDate}
				bind:toDate={range.toDate}
				bind:fromTime={range.fromTime}
				bind:toTime={range.toTime}
				bind:preset={range.preset}
				onApply={handleTimeRangeChange}
			/>
		{/snippet}
	</PageHeader>

	<SearchBar
		placeholder="Search issues..."
		bind:value={searchQuery}
		bind:typeValue={searchType}
		typeOptions={searchTypeOptions}
		onSearch={handleSearch}
		disabled={loading}
	/>

	{#if partial}
		<PartialNotice />
	{/if}

	<TableContainer minWidth="880px">
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
						<Table.Cell colspan={5} class="h-24 text-center text-red-500">{error}</Table.Cell>
					</Table.Row>
				</Table.Body>
			{:else if issues.length === 0}
				<Table.Body>
					<TableEmptyState colspan={5} message="No issues found in the selected range." />
				</Table.Body>
			{:else}
				<Table.Header>
					<Table.Row>
						<TracewayTableHeader
							label="Issue"
							tooltip="The error message or exception that occurred"
						/>
						<TracewayTableHeader label="Project" class="w-[180px]" />
						<TracewayTableHeader
							label="Trend"
							tooltip="Hourly occurrence pattern over the last 24h"
							class="w-[190px]"
						/>
						<TracewayTableHeader
							label="Events"
							tooltip="Total number of times this issue occurred in the selected range"
							align="right"
							class="w-[90px]"
							sortField="count"
							currentSortField={sortField}
							{sortDirection}
							onSort={handleSort}
						/>
						<TracewayTableHeader
							label="Last seen"
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
					{#each issues as issue (issue.projectId + issue.exceptionHash)}
						{@const title = issueTitle(issue)}
						<Table.Row
							class="group cursor-pointer"
							onclick={createRowClickHandler(
								`/issues/${issue.exceptionHash}?projectId=${issue.projectId}`
							)}
						>
							<Table.Cell class="max-w-[520px] py-3">
								<div class="min-w-0">
									<div
										class="truncate text-[15px]/6 font-semibold text-foreground group-hover:text-primary dark:group-hover:text-blue-300"
									>
										{title.type}
									</div>
									{#if title.message}
										<div class="truncate text-sm text-muted-foreground">{title.message}</div>
									{/if}
								</div>
							</Table.Cell>
							<Table.Cell class="text-sm text-muted-foreground">{issue.projectName}</Table.Cell>
							<Table.Cell>
								<IssueTrendChart trend={issue.hourlyTrend || []} />
							</Table.Cell>
							<Table.Cell class="text-right text-[15px] font-semibold tabular-nums">
								{issue.count.toLocaleString()}
							</Table.Cell>
							<Table.Cell class="text-muted-foreground tabular-nums">
								{formatDateTime(issue.lastSeen, { timezone })}
							</Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			{/if}
		</Table.Root>
	</TableContainer>

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
