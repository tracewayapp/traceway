<script lang="ts">
	import SessionRecordingReasons from '$lib/components/issues/session-recording-reasons.svelte';
	import { getErrorMessage } from '$lib/utils/errors';
	import { onMount, onDestroy } from 'svelte';
	import { api } from '$lib/api';
	import { formatRelativeTimeAgo, toUTCISO, calendarDateTimeToLuxon } from '$lib/utils/formatters';
	import { sessionDurationLabel } from '$lib/utils/session-duration';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import * as Table from '$lib/components/ui/table';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { TracewayTableHeader } from '$lib/components/ui/traceway-table-header';
	import { TableEmptyState } from '$lib/components/ui/table-empty-state';
	import { PaginationFooter } from '$lib/components/ui/pagination-footer';
	import { TimeRangePicker } from '$lib/components/ui/time-range-picker';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import Info from '@lucide/svelte/icons/info';
	import {
		FilterBar,
		type FilterApply,
		type FilterField,
		type FilterOption,
		describeTimeRange,
		attributeConfig,
		storedValueRules
	} from '$lib/components/traceway/filter-bar';
	import { CalendarDate } from '@internationalized/date';
	import { browser } from '$app/environment';
	import { projectsState, SDK_CONFIG_FRAMEWORKS } from '$lib/state/projects.svelte';
	import { createRowClickHandler } from '$lib/utils/navigation';
	import { resolve } from '$app/paths';
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import TableContainer from '$lib/components/traceway/table-container.svelte';
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

	import {
		formatAttributeFilter,
		parseAttributeFilter,
		operatorFlags,
		type AttributeFilter
	} from '$lib/utils/attribute-filters';

	const timezone = $derived(getTimezone());
	const initialTimezone = getTimezone();

	type Reason = { triggerId: string; name: string; sequence: string; revision: string };
	let reasonOptions = $state<Reason[]>([]);
	type RuleOption = { id: number; name: string; triggers: number };
	let ruleOptions = $state<RuleOption[]>([]);
	let rulesProjectId: string | null = null;
	const hasRules = $derived(
		!!projectsState.currentProject &&
			SDK_CONFIG_FRAMEWORKS.includes(projectsState.currentProject.framework)
	);

	// The rules of Data Collection, to filter by. A failure only hides the filter's names, never the list.
	async function loadRuleOptions() {
		const projectId = projectsState.currentProjectId;
		if (!projectId || !hasRules || rulesProjectId === projectId) return;
		rulesProjectId = projectId;
		try {
			const response = await api.get('/sdk-config/rules', { projectId });
			ruleOptions = (response.data ?? []).map(
				(rule: { id: number; name: string; settings?: { triggers?: unknown[] } }) => ({
					id: rule.id,
					name: rule.name,
					triggers: rule.settings?.triggers?.length ?? 0
				})
			);
		} catch {
			ruleOptions = [];
			rulesProjectId = null;
		}
	}
	type Session = {
		recordingReasons?: Reason[];
		id: string;
		startedAt: string;
		endedAt?: string | null;
		duration: number;
		hasRecording?: boolean;
		clientIP: string;
		attributes?: Record<string, string>;
		appVersion: string;
		serverName: string;
	};

	type SortField = 'started_at' | 'duration';

	let sessions = $state<Session[]>([]);
	let loading = $state(true);
	let error = $state('');
	let expandedAttributes = $state<string[]>([]);

	function sessionAttributes(session: Session) {
		const identityKeys = ['userId', 'user.id', 'user_id', 'email', 'user.email'];
		return Object.entries(session.attributes ?? {}).sort(
			([a], [b]) =>
				Number(identityKeys.includes(b)) - Number(identityKeys.includes(a)) || a.localeCompare(b)
		);
	}

	let page = $state(1);
	let pageSize = $state(50);
	let total = $state(0);
	let totalPages = $state(0);

	function parseSessionsUrlParams() {
		const timeParams = parseTimeRangeFromUrl(timezone);
		if (!browser)
			return {
				...timeParams,
				search: '',
				triggerId: '',
				ruleId: '',
				attributeFilters: [] as AttributeFilter[]
			};
		const params = new URLSearchParams(window.location.search);
		const attrs: AttributeFilter[] = [];
		for (const raw of params.getAll('attr')) {
			const parsed = parseAttributeFilter(raw);
			// The session search matches values only; it has no existence filter.
			if (parsed && !parsed.exists) attrs.push(parsed);
		}
		return {
			...timeParams,
			search: params.get('search') ?? '',
			triggerId: params.get('triggerId') ?? '',
			ruleId: /^\d+$/.test(params.get('ruleId') ?? '') ? params.get('ruleId')! : '',
			attributeFilters: attrs
		};
	}

	const initialUrlParams = parseSessionsUrlParams();
	let triggerId = $state(initialUrlParams.triggerId);
	let ruleId = $state(initialUrlParams.ruleId);
	const initialRange = getResolvedTimeRange(initialUrlParams, initialTimezone);

	let selectedPreset = $state<string | null>(initialUrlParams.preset);
	let fromDate = $state<CalendarDate>(dateToCalendarDate(initialRange.from, initialTimezone));
	let toDate = $state<CalendarDate>(dateToCalendarDate(initialRange.to, initialTimezone));
	let fromTime = $state(dateToTimeString(initialRange.from, initialTimezone));
	let toTime = $state(dateToTimeString(initialRange.to, initialTimezone));
	let searchQuery = $state(initialUrlParams.search);
	let attributeFilters = $state<AttributeFilter[]>(initialUrlParams.attributeFilters);

	let filterBar = $state<ReturnType<typeof FilterBar>>();
	const attributeRules = storedValueRules('userId', 'u_42', 'sessions');

	// Clicking an attribute in a row opens the drawer with that filter filled in.
	function filterByAttribute(key: string, value: string) {
		filterBar?.openAttribute({ key, value, ...operatorFlags('=') });
	}

	const ruleFieldOptions = $derived<FilterOption[]>([
		...(ruleId && !ruleOptions.some((r) => String(r.id) === ruleId)
			? [{ value: ruleId, label: `Rule ${ruleId}` }]
			: []),
		...ruleOptions.map((rule) => ({
			value: String(rule.id),
			label: `${rule.name}${rule.triggers === 0 ? ' (no triggers)' : ''}`
		}))
	]);
	const triggerFieldOptions = $derived<FilterOption[]>(
		reasonOptions.map((reason) => ({ value: reason.triggerId, label: reason.name }))
	);
	const filterFields = $derived.by<FilterField[]>(() => {
		const trigger: FilterField = {
			kind: 'select',
			key: 'triggerId',
			label: 'Recording trigger',
			anyLabel: 'Any trigger',
			hint: 'The trigger that started the recording.',
			options: triggerFieldOptions
		};
		if (!hasRules && !ruleId) return [trigger];
		return [
			{
				kind: 'select',
				key: 'ruleId',
				label: 'Recorded by rule',
				anyLabel: 'Any rule',
				options: ruleFieldOptions,
				hint: 'Sessions in which a trigger of this rule asked for a recording.'
			},
			trigger
		];
	});
	const filterValues = $derived({ ruleId, triggerId });

	function handleFilterApply(next: FilterApply) {
		searchQuery = next.search;
		ruleId = next.values.ruleId ?? '';
		triggerId = next.values.triggerId ?? '';
		attributeFilters = next.attributes;
		page = 1;
		loadData(true);
	}

	const timeRangeLabel = $derived(describeTimeRange(selectedPreset));

	function updateSessionsUrl(pushToHistory = true) {
		updateUrl(
			{
				preset: selectedPreset || null,
				from: selectedPreset ? null : getFromDateTimeUTC(),
				to: selectedPreset ? null : getToDateTimeUTC(),
				search: searchQuery.trim() || null,
				triggerId: triggerId || null,
				ruleId: ruleId || null,
				attr: attributeFilters.map(formatAttributeFilter)
			},
			{ pushToHistory }
		);
	}

	function handlePopState() {
		const urlParams = parseSessionsUrlParams();
		triggerId = urlParams.triggerId;
		ruleId = urlParams.ruleId;
		const range = getResolvedTimeRange(urlParams, timezone);
		selectedPreset = urlParams.preset;
		fromDate = dateToCalendarDate(range.from, timezone);
		fromTime = dateToTimeString(range.from, timezone);
		toDate = dateToCalendarDate(range.to, timezone);
		toTime = dateToTimeString(range.to, timezone);
		searchQuery = urlParams.search;
		attributeFilters = urlParams.attributeFilters;
		page = 1;
		loadData(false);
	}

	const SORT_STORAGE_KEY = 'sessions';
	const initialSort = getSortState(SORT_STORAGE_KEY, { field: 'started_at', direction: 'desc' });
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
		loadData(true);
	}

	function shortId(id: string): string {
		return id.split('-')[0] ?? id.slice(0, 8);
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

		updateSessionsUrl(pushToHistory);

		try {
			const requestBody = {
				fromDate: getFromDateTimeUTC(),
				toDate: getToDateTimeUTC(),
				orderBy,
				sortDirection,
				search: searchQuery.trim(),
				attributeFilters,
				pagination: { page, pageSize }
			};

			void loadRuleOptions();
			const reasonsResponse = await api.get('/sessions/recording-reasons', {
				projectId: projectsState.currentProjectId!
			});
			reasonOptions = [
				...new Map<string, Reason>(
					[...(reasonsResponse.reasons ?? [])].reverse().map((r: Reason) => [r.triggerId, r])
				).values()
			];
			const response = await api.post(
				'/sessions',
				{ ...requestBody, triggerId, ruleId: ruleId ? Number(ruleId) : undefined },
				{
					projectId: projectsState.currentProjectId ?? undefined
				}
			);

			sessions = response.data || [];
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
	<PageHeader title="Sessions">
		{#snippet trailing()}
			<Tooltip.Root>
				<Tooltip.Trigger class="text-muted-foreground/60 hover:text-muted-foreground">
					<Info class="h-4 w-4" />
				</Tooltip.Trigger>
				<Tooltip.Content side="right" class="max-w-xs">
					<p class="text-xs">
						Sessions group a visit. Replay is uploaded when collection settings or a recording
						trigger request it. A trigger tag records the request; recording availability confirms
						whether replay arrived.
					</p>
				</Tooltip.Content>
			</Tooltip.Root>
		{/snippet}
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

	<FilterBar
		search={searchQuery}
		searchPlaceholder="Search session ID, user, or attribute value..."
		fields={filterFields}
		values={filterValues}
		bind:this={filterBar}
		attributes={attributeConfig(attributeFilters, attributeRules)}
		onApply={handleFilterApply}
		disabled={loading}
		noun="sessions"
		rangeLabel={timeRangeLabel}
	>
		{#if ruleId}
			<p class="text-xs text-muted-foreground">
				Sessions in which a trigger of this rule asked for a recording.
			</p>
		{/if}

		<TableContainer>
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
				{:else if sessions.length === 0}
					<Table.Body>
						<TableEmptyState
							colspan={6}
							message={searchQuery.trim() || attributeFilters.length
								? 'No sessions match your search and filters.'
								: ruleId
									? ruleOptions.find((r) => String(r.id) === ruleId)?.triggers === 0
										? 'This rule has no triggers, so it records no sessions. Add a recording trigger to it in Data Collection.'
										: 'No session was recorded by a trigger of this rule in this time range.'
									: 'No sessions reported in this time range. Check the SDK session collection settings.'}
						/>
					</Table.Body>
				{:else}
					<Table.Header>
						<Table.Row>
							<TracewayTableHeader
								label="Session"
								tooltip="Session UUID. Open to play back the recording."
							/>
							<TracewayTableHeader
								label="Attributes"
								tooltip="User and session context. Click an attribute to filter sessions."
							/>
							<TracewayTableHeader
								label="Recorded by"
								tooltip="The triggers that asked for a recording of this session. Hover a chip for its steps; click it to show the sessions that trigger recorded."
								class="w-[260px]"
							/>
							<TracewayTableHeader
								label="Started"
								tooltip="When the session began"
								sortField="started_at"
								currentSortField={orderBy}
								{sortDirection}
								onSort={(field) => handleSort(field as SortField)}
								class="w-[160px]"
							/>
							<TracewayTableHeader
								label="Duration"
								tooltip="Wall-clock length of the session"
								sortField="duration"
								currentSortField={orderBy}
								{sortDirection}
								onSort={(field) => handleSort(field as SortField)}
								class="w-[120px]"
							/>
							<TracewayTableHeader
								label="Version"
								tooltip="App version reported by the SDK"
								class="w-[120px]"
							/>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each sessions as session (session.id)}
							{@const attributes = sessionAttributes(session)}
							{@const attributesExpanded = expandedAttributes.includes(session.id)}
							<Table.Row
								class="cursor-pointer"
								onclick={createRowClickHandler(
									`${resolve(`/sessions/${session.id}`)}?t=${encodeURIComponent(session.startedAt)}`,
									'preset',
									'from',
									'to'
								)}
							>
								<Table.Cell class="w-[140px] font-mono text-sm">{shortId(session.id)}</Table.Cell>
								<Table.Cell>
									<div class="flex max-w-2xl flex-wrap gap-1">
										{#each attributesExpanded ? attributes : attributes.slice(0, 3) as [key, value] (key)}
											<button
												type="button"
												class="max-w-64 truncate rounded-full bg-muted px-2 py-0.5 font-mono text-xs hover:text-primary"
												title={`${key}=${value}`}
												aria-label={`Filter by ${key}=${value}`}
												onclick={(event) => {
													event.stopPropagation();
													filterByAttribute(key, value);
												}}
											>
												<span class="text-muted-foreground">{key}</span>={value || '""'}
											</button>
										{:else}
											<span class="text-muted-foreground">—</span>
										{/each}
										{#if attributes.length > 3}
											<button
												type="button"
												class="px-1 text-xs text-muted-foreground hover:text-foreground"
												aria-expanded={attributesExpanded}
												onclick={(event) => {
													event.stopPropagation();
													expandedAttributes = attributesExpanded
														? expandedAttributes.filter((id) => id !== session.id)
														: [...expandedAttributes, session.id];
												}}
												>{attributesExpanded
													? 'Show less'
													: `+${attributes.length - 3} more`}</button
											>
										{/if}
									</div>
								</Table.Cell>
								<Table.Cell class="w-[260px]">
									{#if session.recordingReasons?.length}
										<SessionRecordingReasons reasons={session.recordingReasons} />
									{:else}
										<span class="text-muted-foreground">—</span>
									{/if}
								</Table.Cell>
								<Table.Cell class="text-sm"
									>{formatRelativeTimeAgo(session.startedAt, timezone)}</Table.Cell
								>
								<Table.Cell class="font-mono text-sm tabular-nums">
									{sessionDurationLabel(session)}
								</Table.Cell>
								<Table.Cell class="font-mono text-sm">{session.appVersion || '—'}</Table.Cell>
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
			itemLabel="session"
		/>
	</FilterBar>
</div>
