<script lang="ts">
	import { getErrorMessage } from '$lib/utils/errors';
	import { onMount, onDestroy } from 'svelte';
	import { api } from '$lib/api';
	import { toUTCISO, calendarDateTimeToLuxon, formatDateTime } from '$lib/utils/formatters';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import * as Table from '$lib/components/ui/table';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { TracewayTableHeader } from '$lib/components/ui/traceway-table-header';
	import { TableEmptyState } from '$lib/components/ui/table-empty-state';
	import { PaginationFooter } from '$lib/components/ui/pagination-footer';
	import { TimeRangePicker } from '$lib/components/ui/time-range-picker';
	import { SearchBar } from '$lib/components/ui/search-bar';
	import { browser } from '$app/environment';
	import { CalendarDate } from '@internationalized/date';
	import { projectsState } from '$lib/state/projects.svelte';
	import { createRowClickHandler } from '$lib/utils/navigation';
	import { resolve } from '$app/paths';
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import TableContainer from '$lib/components/traceway/table-container.svelte';
	import { formatCost, formatCount } from '$lib/utils/ai-format';
	import AiNavTabs from '$lib/components/ai/ai-nav-tabs.svelte';
	import { TriangleAlert } from '@lucide/svelte';
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

	type UserRow = {
		userId: string;
		conversationCount: number;
		totalCalls: number;
		avgTurns: number;
		minTurns: number;
		medianTurns: number;
		avgCostPerConversation: number;
		totalCost: number;
		flaggedConversationCount: number;
		totalTokens: number;
		lastSeen: string;
	};

	type SortField =
		| 'conversation_count'
		| 'total_calls'
		| 'median_turns'
		| 'avg_conversation_cost'
		| 'total_cost'
		| 'total_tokens'
		| 'flagged_conversation_count'
		| 'last_seen';

	let users = $state<UserRow[]>([]);
	let loading = $state(true);
	let error = $state('');

	// Pagination State
	let page = $state(1);
	let pageSize = $state(50);
	let total = $state(0);
	let totalPages = $state(0);

	function parseUsersUrlParams() {
		if (!browser) return { preset: '24h', from: null, to: null, search: '' };
		const params = new URLSearchParams(window.location.search);
		const timeParams = parseTimeRangeFromUrl(timezone, '24h');
		return {
			...timeParams,
			search: params.get('search') || ''
		};
	}

	const initialUrlParams = parseUsersUrlParams();
	const initialRange = getResolvedTimeRange(initialUrlParams, initialTimezone);

	let searchQuery = $state(initialUrlParams.search);

	// Date Range State
	let selectedPreset = $state<string | null>(initialUrlParams.preset);
	let fromDate = $state<CalendarDate>(dateToCalendarDate(initialRange.from, initialTimezone));
	let toDate = $state<CalendarDate>(dateToCalendarDate(initialRange.to, initialTimezone));
	let fromTime = $state(dateToTimeString(initialRange.from, initialTimezone));
	let toTime = $state(dateToTimeString(initialRange.to, initialTimezone));

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
		updateUrl(params, { pushToHistory });
	}

	function handlePopState() {
		const urlParams = parseUsersUrlParams();
		const range = getResolvedTimeRange(urlParams, timezone);

		selectedPreset = urlParams.preset;
		fromDate = dateToCalendarDate(range.from, timezone);
		fromTime = dateToTimeString(range.from, timezone);
		toDate = dateToCalendarDate(range.to, timezone);
		toTime = dateToTimeString(range.to, timezone);
		searchQuery = urlParams.search;

		page = 1;
		loadData(false);
	}

	const SORT_STORAGE_KEY = 'ai-users';
	const initialSort = getSortState(SORT_STORAGE_KEY, { field: 'total_cost', direction: 'desc' });
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
				search: searchQuery.trim()
			};

			const response = await api.post('/ai-users/grouped', requestBody, {
				projectId: projectsState.currentProjectId ?? undefined
			});

			users = response.data || [];
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

	function userConversationsHref(userId: string): string {
		return `${resolve('/ai-traces/conversations')}?userId=${encodeURIComponent(userId)}`;
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
	<PageHeader title="AI Users">
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

	<AiNavTabs active="users" />

	<!-- Search -->
	<SearchBar
		placeholder="Search users..."
		bind:value={searchQuery}
		onSearch={handleSearch}
		disabled={loading}
	/>

	<!-- Users Table -->
	<TableContainer>
		<Table.Root>
			{#if loading}
				<Table.Body>
					<Table.Row>
						<Table.Cell colspan={8} class="h-48">
							<div class="flex h-full items-center justify-center">
								<LoadingCircle size="xlg" />
							</div>
						</Table.Cell>
					</Table.Row>
				</Table.Body>
			{:else if error}
				<Table.Body>
					<Table.Row>
						<Table.Cell colspan={8} class="h-24 text-center text-red-500">
							{error}
						</Table.Cell>
					</Table.Row>
				</Table.Body>
			{:else if users.length === 0}
				<Table.Body>
					<TableEmptyState
						colspan={8}
						message={searchQuery.trim()
							? 'No users match the current search.'
							: 'No users found. Per-user analytics appear once your AI calls carry both a user.id and a conversation id.'}
					/>
				</Table.Body>
			{:else}
				<Table.Header>
					<Table.Row>
						<TracewayTableHeader label="User" tooltip="The user.id attribute on the calls" />
						<TracewayTableHeader
							label="Conversations"
							tooltip="Number of distinct conversations"
							sortField="conversation_count"
							currentSortField={orderBy}
							{sortDirection}
							onSort={(field) => handleSort(field as SortField)}
							class="w-[130px]"
						/>
						<TracewayTableHeader
							label="Calls"
							tooltip="Total LLM calls across all conversations"
							sortField="total_calls"
							currentSortField={orderBy}
							{sortDirection}
							onSort={(field) => handleSort(field as SortField)}
							class="w-[90px]"
						/>
						<TracewayTableHeader
							label="Median Turns"
							tooltip="Median conversation length in LLM calls"
							sortField="median_turns"
							currentSortField={orderBy}
							{sortDirection}
							onSort={(field) => handleSort(field as SortField)}
							class="w-[130px]"
						/>
						<TracewayTableHeader
							label="Avg Cost/Conv"
							tooltip="Average cost per conversation"
							sortField="avg_conversation_cost"
							currentSortField={orderBy}
							{sortDirection}
							onSort={(field) => handleSort(field as SortField)}
							class="w-[130px]"
						/>
						<TracewayTableHeader
							label="Total Cost"
							tooltip="Total cost across all conversations"
							sortField="total_cost"
							currentSortField={orderBy}
							{sortDirection}
							onSort={(field) => handleSort(field as SortField)}
							class="w-[110px]"
						/>
						<TracewayTableHeader
							label="Flagged"
							tooltip="Conversations containing flagged terms"
							sortField="flagged_conversation_count"
							currentSortField={orderBy}
							{sortDirection}
							onSort={(field) => handleSort(field as SortField)}
							class="w-[100px]"
						/>
						<TracewayTableHeader
							label="Last Seen"
							tooltip="When this user last had a call"
							sortField="last_seen"
							currentSortField={orderBy}
							{sortDirection}
							onSort={(field) => handleSort(field as SortField)}
							class="w-[130px]"
						/>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each users as user, __index (__index)}
						<Table.Row
							class="cursor-pointer"
							onclick={createRowClickHandler(
								userConversationsHref(user.userId),
								'preset',
								'from',
								'to'
							)}
						>
							<Table.Cell class="max-w-[240px] truncate font-mono text-sm" title={user.userId}>
								{user.userId}
							</Table.Cell>
							<Table.Cell class="tabular-nums">
								{formatCount(user.conversationCount)}
							</Table.Cell>
							<Table.Cell class="tabular-nums">
								{formatCount(user.totalCalls)}
							</Table.Cell>
							<Table.Cell class="tabular-nums">
								<Tooltip.Provider>
									<Tooltip.Root>
										<Tooltip.Trigger>
											<span>{user.medianTurns.toFixed(1)}</span>
										</Tooltip.Trigger>
										<Tooltip.Content>
											<p class="text-xs">avg {user.avgTurns.toFixed(1)} · min {user.minTurns}</p>
										</Tooltip.Content>
									</Tooltip.Root>
								</Tooltip.Provider>
							</Table.Cell>
							<Table.Cell class="font-mono text-sm tabular-nums">
								{formatCost(user.avgCostPerConversation)}
							</Table.Cell>
							<Table.Cell class="font-mono text-sm tabular-nums">
								{formatCost(user.totalCost)}
							</Table.Cell>
							<Table.Cell class="tabular-nums">
								{#if user.flaggedConversationCount > 0}
									<span
										class="inline-flex items-center gap-1 font-medium text-red-600 dark:text-red-400"
									>
										<TriangleAlert class="h-3 w-3" />
										{formatCount(user.flaggedConversationCount)}
									</span>
								{:else}
									<span class="text-muted-foreground">0</span>
								{/if}
							</Table.Cell>
							<Table.Cell class="text-sm text-muted-foreground">
								{formatDateTime(user.lastSeen, { timezone })}
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
		itemLabel="user"
	/>
</div>
