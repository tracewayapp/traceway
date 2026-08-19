<script lang="ts">
	import { getErrorMessage } from '$lib/utils/errors';
	import { onMount, onDestroy } from 'svelte';
	import { api } from '$lib/api';
	import { toUTCISO, calendarDateTimeToLuxon, formatDateTime } from '$lib/utils/formatters';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import * as Table from '$lib/components/ui/table';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import { Badge } from '$lib/components/ui/badge';
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
	import ToolbarSelect from '$lib/components/traceway/toolbar-select.svelte';
	import { formatCost, formatCount, formatTokens } from '$lib/utils/ai-format';
	import AiNavTabs from '$lib/components/ai/ai-nav-tabs.svelte';
	import FlaggedBadge from '$lib/components/ai/flagged-badge.svelte';
	import FlaggedFilter from '$lib/components/ai/flagged-filter.svelte';
	import EditProjectModal from '$lib/components/edit-project-modal.svelte';
	import { Button } from '$lib/components/ui/button';
	import { ShieldAlert, TrendingUp, X } from '@lucide/svelte';
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

	type ConversationRow = {
		conversationId: string;
		userId: string;
		turns: number;
		totalTokens: number;
		totalCost: number;
		toolCallCount: number;
		toolNames: string[] | null;
		models: string[] | null;
		flagged: boolean;
		flaggedTerms: string[] | null;
		firstSeen: string;
		lastSeen: string;
	};

	type Thresholds = {
		p95Cost: number;
		p95Turns: number;
	};

	type SortField =
		| 'turns'
		| 'tool_call_count'
		| 'user_id'
		| 'total_tokens'
		| 'total_cost'
		| 'first_seen'
		| 'last_seen';

	let conversations = $state<ConversationRow[]>([]);
	let thresholds = $state<Thresholds | null>(null);
	let facetModels = $state<string[]>([]);
	let facetTools = $state<string[]>([]);
	let loading = $state(true);
	let error = $state('');

	// Pagination State
	let page = $state(1);
	let pageSize = $state(50);
	let total = $state(0);
	let totalPages = $state(0);

	function parseConversationsUrlParams() {
		if (!browser)
			return {
				preset: '24h',
				from: null,
				to: null,
				search: '',
				flagged: 'all',
				userId: '',
				model: '',
				tool: ''
			};
		const params = new URLSearchParams(window.location.search);
		const timeParams = parseTimeRangeFromUrl(timezone, '24h');
		return {
			...timeParams,
			search: params.get('search') || '',
			flagged: params.get('flagged') === '1' ? 'flagged' : 'all',
			userId: params.get('userId') || '',
			model: params.get('model') || '',
			tool: params.get('tool') || ''
		};
	}

	const initialUrlParams = parseConversationsUrlParams();
	const initialRange = getResolvedTimeRange(initialUrlParams, initialTimezone);

	let searchQuery = $state(initialUrlParams.search);
	let flaggedFilter = $state(initialUrlParams.flagged);
	let userIdFilter = $state(initialUrlParams.userId);
	let modelFilter = $state(initialUrlParams.model);
	let toolFilter = $state(initialUrlParams.tool);

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
		params.flagged = flaggedFilter === 'flagged' ? '1' : null;
		params.userId = userIdFilter || null;
		params.model = modelFilter || null;
		params.tool = toolFilter || null;
		updateUrl(params, { pushToHistory });
	}

	function handlePopState() {
		const urlParams = parseConversationsUrlParams();
		const range = getResolvedTimeRange(urlParams, timezone);

		selectedPreset = urlParams.preset;
		fromDate = dateToCalendarDate(range.from, timezone);
		fromTime = dateToTimeString(range.from, timezone);
		toDate = dateToCalendarDate(range.to, timezone);
		toTime = dateToTimeString(range.to, timezone);
		searchQuery = urlParams.search;
		flaggedFilter = urlParams.flagged;
		userIdFilter = urlParams.userId;
		modelFilter = urlParams.model;
		toolFilter = urlParams.tool;

		page = 1;
		loadData(false);
	}

	const SORT_STORAGE_KEY = 'ai-conversations';
	const initialSort = getSortState(SORT_STORAGE_KEY, { field: 'last_seen', direction: 'desc' });
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
				search: searchQuery.trim(),
				userId: userIdFilter,
				model: modelFilter,
				toolName: toolFilter,
				flaggedOnly: flaggedFilter === 'flagged'
			};

			const response = await api.post('/ai-conversations/grouped', requestBody, {
				projectId: projectsState.currentProjectId ?? undefined
			});

			conversations = response.data || [];
			thresholds = response.thresholds || null;
			facetModels = response.facets?.models || [];
			facetTools = response.facets?.tools || [];
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

	function applyFilterChange() {
		page = 1;
		loadData(true);
	}

	type FilterPillKind = 'user' | 'model' | 'tool' | 'flagged';

	const filterPills = $derived([
		...(userIdFilter
			? [{ kind: 'user' as FilterPillKind, label: 'user', value: userIdFilter }]
			: []),
		...(modelFilter
			? [{ kind: 'model' as FilterPillKind, label: 'model', value: modelFilter }]
			: []),
		...(toolFilter ? [{ kind: 'tool' as FilterPillKind, label: 'tool', value: toolFilter }] : []),
		...(flaggedFilter === 'flagged'
			? [{ kind: 'flagged' as FilterPillKind, label: 'flagged', value: 'only' }]
			: [])
	]);

	function removeFilterPill(kind: FilterPillKind) {
		if (kind === 'user') userIdFilter = '';
		else if (kind === 'model') modelFilter = '';
		else if (kind === 'tool') toolFilter = '';
		else if (kind === 'flagged') flaggedFilter = 'all';
		applyFilterChange();
	}

	const hasActiveFilters = $derived(
		!!(
			searchQuery.trim() ||
			userIdFilter ||
			modelFilter ||
			toolFilter ||
			flaggedFilter === 'flagged'
		)
	);

	function shortModelName(model: string): string {
		const idx = model.lastIndexOf('/');
		return idx >= 0 ? model.slice(idx + 1) : model;
	}

	const modelOptions = $derived([
		{ value: '', label: 'All models' },
		...facetModels.map((m) => ({ value: m, label: m }))
	]);
	const toolOptions = $derived([
		{ value: '', label: 'All tools' },
		...facetTools.map((t) => ({ value: t, label: t }))
	]);

	const isCostOutlier = (row: ConversationRow) =>
		!!thresholds && thresholds.p95Cost > 0 && row.totalCost > thresholds.p95Cost;
	const isTurnsOutlier = (row: ConversationRow) =>
		!!thresholds && thresholds.p95Turns > 0 && row.turns > thresholds.p95Turns;

	// Reloads after the settings modal closes so new flagged terms and
	// language packs are reflected (they only affect future ingests, but the
	// project object itself is refreshed).
	let showFlaggedTermsSettings = $state(false);
	function onFlaggedTermsSettingsChange(open: boolean) {
		showFlaggedTermsSettings = open;
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
	<PageHeader title="AI Conversations">
		{#snippet actions()}
			{#if projectsState.canWriteCurrentProject}
				<Button variant="outline" size="sm" onclick={() => (showFlaggedTermsSettings = true)}>
					<ShieldAlert class="mr-2 h-4 w-4" />
					Flagged terms
				</Button>
			{/if}
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

	<AiNavTabs active="conversations" />

	<!-- Search + filters -->
	<SearchBar
		placeholder="Search conversations..."
		bind:value={searchQuery}
		onSearch={handleSearch}
		disabled={loading}
	>
		<FlaggedFilter bind:value={flaggedFilter} onChange={applyFilterChange} />
		<ToolbarSelect
			bind:value={modelFilter}
			options={modelOptions}
			class="w-[160px]"
			onChange={applyFilterChange}
		/>
		<ToolbarSelect
			bind:value={toolFilter}
			options={toolOptions}
			class="w-[150px]"
			onChange={applyFilterChange}
		/>
	</SearchBar>

	{#if filterPills.length > 0}
		<div class="flex flex-wrap items-center gap-2">
			{#each filterPills as pill (pill.kind)}
				<span
					class="inline-flex items-center gap-1 rounded-full bg-muted px-2 py-0.5 font-mono text-xs"
				>
					<span class="text-muted-foreground">{pill.label}</span>
					<span>=</span>
					<span class="max-w-[240px] truncate" title={pill.value}>{pill.value}</span>
					<button
						type="button"
						aria-label="Remove {pill.label} filter"
						class="ml-1 cursor-pointer text-muted-foreground hover:text-foreground"
						onclick={() => removeFilterPill(pill.kind)}
					>
						<X class="h-3 w-3" />
					</button>
				</span>
			{/each}
		</div>
	{/if}

	<!-- Conversations Table -->
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
			{:else if conversations.length === 0}
				<Table.Body>
					<TableEmptyState
						colspan={8}
						message={hasActiveFilters
							? 'No conversations match the current search and filters. Search covers conversation ids, users, models, tool names, and flagged terms.'
							: 'No conversations found. They appear once your AI calls carry a gen_ai.conversation.id or session.id attribute (or share a distributed trace).'}
					/>
				</Table.Body>
			{:else}
				<Table.Header>
					<Table.Row>
						<TracewayTableHeader
							label="Conversation"
							tooltip="The conversation id reported by your app"
						/>
						<TracewayTableHeader
							label="User"
							tooltip="The user.id attribute on the calls"
							sortField="user_id"
							currentSortField={orderBy}
							{sortDirection}
							onSort={(field) => handleSort(field as SortField)}
							class="w-[140px]"
						/>
						<TracewayTableHeader
							label="Turns"
							tooltip="Number of LLM calls in the conversation"
							sortField="turns"
							currentSortField={orderBy}
							{sortDirection}
							onSort={(field) => handleSort(field as SortField)}
							class="w-[90px]"
						/>
						<TracewayTableHeader
							label="Tools"
							tooltip="Total tool calls across the conversation"
							sortField="tool_call_count"
							currentSortField={orderBy}
							{sortDirection}
							onSort={(field) => handleSort(field as SortField)}
							class="w-[90px]"
						/>
						<TracewayTableHeader label="Models" tooltip="Models used in the conversation" />
						<TracewayTableHeader
							label="Tokens"
							tooltip="Total tokens used"
							sortField="total_tokens"
							currentSortField={orderBy}
							{sortDirection}
							onSort={(field) => handleSort(field as SortField)}
							class="w-[100px]"
						/>
						<TracewayTableHeader
							label="Cost"
							tooltip="Total cost of the conversation"
							sortField="total_cost"
							currentSortField={orderBy}
							{sortDirection}
							onSort={(field) => handleSort(field as SortField)}
							class="w-[100px]"
						/>
						<TracewayTableHeader
							label="Last Seen"
							tooltip="When the conversation last had a call"
							sortField="last_seen"
							currentSortField={orderBy}
							{sortDirection}
							onSort={(field) => handleSort(field as SortField)}
							class="w-[130px]"
						/>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each conversations as conv, __index (__index)}
						<Table.Row
							class="cursor-pointer"
							onclick={createRowClickHandler(
								resolve('/ai-traces/conversations/[conversationId]', {
									conversationId: encodeURIComponent(conv.conversationId)
								}),
								'preset',
								'from',
								'to'
							)}
						>
							<Table.Cell class="max-w-[280px] font-mono text-sm">
								<div class="flex items-center gap-2">
									<span class="truncate" title={conv.conversationId}>{conv.conversationId}</span>
									{#if conv.flagged && conv.flaggedTerms?.length}
										<FlaggedBadge terms={conv.flaggedTerms} />
									{/if}
								</div>
							</Table.Cell>
							<Table.Cell class="max-w-[140px] truncate font-mono text-sm" title={conv.userId}>
								{conv.userId || '-'}
							</Table.Cell>
							<Table.Cell class="tabular-nums">
								{#if isTurnsOutlier(conv)}
									<Tooltip.Provider>
										<Tooltip.Root>
											<Tooltip.Trigger>
												<span
													class="inline-flex items-center gap-1 font-semibold text-orange-600 dark:text-orange-400"
												>
													{formatCount(conv.turns)}
													<TrendingUp class="h-3 w-3" />
												</span>
											</Tooltip.Trigger>
											<Tooltip.Content>
												<p class="text-xs">
													Above the 95th percentile turns for this period (P95: {Math.round(
														thresholds?.p95Turns ?? 0
													)})
												</p>
											</Tooltip.Content>
										</Tooltip.Root>
									</Tooltip.Provider>
								{:else}
									{formatCount(conv.turns)}
								{/if}
							</Table.Cell>
							<Table.Cell class="tabular-nums">
								{#if conv.toolNames?.length}
									<Tooltip.Provider>
										<Tooltip.Root>
											<Tooltip.Trigger>
												<span>{formatCount(conv.toolCallCount)}</span>
											</Tooltip.Trigger>
											<Tooltip.Content>
												<p class="max-w-xs text-xs">{conv.toolNames.join(', ')}</p>
											</Tooltip.Content>
										</Tooltip.Root>
									</Tooltip.Provider>
								{:else}
									{formatCount(conv.toolCallCount)}
								{/if}
							</Table.Cell>
							<Table.Cell>
								{#if conv.models?.length}
									<Tooltip.Provider>
										<Tooltip.Root>
											<Tooltip.Trigger>
												<span class="inline-flex items-center gap-1">
													<Badge variant="outline" class="font-mono text-xs"
														>{shortModelName(conv.models[0])}</Badge
													>
													{#if conv.models.length > 1}
														<Badge variant="outline" class="text-xs"
															>+{conv.models.length - 1}</Badge
														>
													{/if}
												</span>
											</Tooltip.Trigger>
											<Tooltip.Content>
												<p class="max-w-xs text-xs">{conv.models.join(', ')}</p>
											</Tooltip.Content>
										</Tooltip.Root>
									</Tooltip.Provider>
								{:else}
									<span class="text-muted-foreground">-</span>
								{/if}
							</Table.Cell>
							<Table.Cell class="font-mono text-sm tabular-nums">
								{formatTokens(conv.totalTokens)}
							</Table.Cell>
							<Table.Cell class="font-mono text-sm tabular-nums">
								{#if isCostOutlier(conv)}
									<Tooltip.Provider>
										<Tooltip.Root>
											<Tooltip.Trigger>
												<span
													class="inline-flex items-center gap-1 font-semibold text-orange-600 dark:text-orange-400"
												>
													{formatCost(conv.totalCost)}
													<TrendingUp class="h-3 w-3" />
												</span>
											</Tooltip.Trigger>
											<Tooltip.Content>
												<p class="text-xs">
													Above the 95th percentile cost for this period (P95: {formatCost(
														thresholds?.p95Cost ?? 0
													)})
												</p>
											</Tooltip.Content>
										</Tooltip.Root>
									</Tooltip.Provider>
								{:else}
									{formatCost(conv.totalCost)}
								{/if}
							</Table.Cell>
							<Table.Cell class="text-sm text-muted-foreground">
								<Tooltip.Provider>
									<Tooltip.Root>
										<Tooltip.Trigger>
											<span>{formatDateTime(conv.lastSeen, { timezone })}</span>
										</Tooltip.Trigger>
										<Tooltip.Content>
											<p class="text-xs">Started {formatDateTime(conv.firstSeen, { timezone })}</p>
										</Tooltip.Content>
									</Tooltip.Root>
								</Tooltip.Provider>
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
		itemLabel="conversation"
	/>
</div>

<EditProjectModal
	open={showFlaggedTermsSettings}
	onOpenChange={onFlaggedTermsSettingsChange}
	project={projectsState.currentProject}
	initialTab="ai"
/>
