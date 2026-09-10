<script lang="ts">
	import { getErrorMessage } from '$lib/utils/errors';
	import { onMount, onDestroy } from 'svelte';
	import { api } from '$lib/api';
	import {
		formatDuration,
		formatRelativeTime,
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
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import * as Select from '$lib/components/ui/select';
	import Check from '@lucide/svelte/icons/check';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import Plus from '@lucide/svelte/icons/plus';
	import X from '@lucide/svelte/icons/x';
	import Info from '@lucide/svelte/icons/info';
	import { CalendarDate } from '@internationalized/date';
	import { browser } from '$app/environment';
	import { projectsState } from '$lib/state/projects.svelte';
	import { createRowClickHandler } from '$lib/utils/navigation';
	import { resolve } from '$app/paths';
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import TableContainer from '$lib/components/traceway/table-container.svelte';
	import FormField from '$lib/components/traceway/form-field.svelte';
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
		parseAttributeFilter,
		filterOperator,
		type AttributeFilter,
		type FilterOperator
	} from '$lib/utils/session-filters';

	const timezone = $derived(getTimezone());
	const initialTimezone = getTimezone();

	type Session = {
		id: string;
		startedAt: string;
		endedAt?: string | null;
		duration: number;
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
		if (!browser) return { ...timeParams, search: '', attributeFilters: [] as AttributeFilter[] };
		const params = new URLSearchParams(window.location.search);
		const attrs: AttributeFilter[] = [];
		for (const raw of params.getAll('attr')) {
			const parsed = parseAttributeFilter(raw);
			if (parsed) attrs.push(parsed);
		}
		return {
			...timeParams,
			search: params.get('search') ?? '',
			attributeFilters: attrs
		};
	}

	const initialUrlParams = parseSessionsUrlParams();
	const initialRange = getResolvedTimeRange(initialUrlParams, initialTimezone);

	let selectedPreset = $state<string | null>(initialUrlParams.preset);
	let fromDate = $state<CalendarDate>(dateToCalendarDate(initialRange.from, initialTimezone));
	let toDate = $state<CalendarDate>(dateToCalendarDate(initialRange.to, initialTimezone));
	let fromTime = $state(dateToTimeString(initialRange.from, initialTimezone));
	let toTime = $state(dateToTimeString(initialRange.to, initialTimezone));
	let searchQuery = $state(initialUrlParams.search);
	let attributeFilters = $state<AttributeFilter[]>(initialUrlParams.attributeFilters);

	let addFilterOpen = $state(false);
	let dialogKey = $state('');
	let dialogValue = $state('');
	let dialogError = $state('');
	let dialogOperator = $state<FilterOperator>('=');
	let dialogEditIndex = $state<number | null>(null);
	const operatorOptions: { value: FilterOperator; label: string }[] = [
		{ value: '=', label: 'Equals' },
		{ value: '!=', label: 'Not equals' },
		{ value: '~=', label: 'Contains' },
		{ value: '!~=', label: 'Not contains' }
	];

	function openAddFilterDialog(key = '', value = '') {
		dialogKey = key;
		dialogValue = value;
		dialogOperator = '=';
		dialogEditIndex = null;
		dialogError = '';
		addFilterOpen = true;
	}

	function openEditFilterDialog(index: number) {
		const filter = attributeFilters[index];
		dialogKey = filter.key;
		dialogValue = filter.value;
		dialogOperator = filterOperator(filter);
		dialogEditIndex = index;
		dialogError = '';
		addFilterOpen = true;
	}

	function submitDialogFilter() {
		const key = dialogKey.trim();
		if (!key) {
			dialogError = 'Attribute key is required';
			return;
		}
		const filter: AttributeFilter = {
			key,
			value: dialogValue,
			exclude: dialogOperator.startsWith('!'),
			contains: dialogOperator.includes('~')
		};
		const others = attributeFilters.filter((_, i) => i !== dialogEditIndex);
		const duplicate = others.some(
			(f) =>
				f.key === filter.key &&
				f.value === filter.value &&
				f.exclude === filter.exclude &&
				f.contains === filter.contains
		);
		attributeFilters = duplicate
			? others
			: dialogEditIndex === null
				? [...others, filter]
				: attributeFilters.map((f, i) => (i === dialogEditIndex ? filter : f));
		addFilterOpen = false;
		page = 1;
		loadData(true);
	}

	function handleDialogKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			e.preventDefault();
			submitDialogFilter();
		}
	}

	function removeAttributeFilter(index: number) {
		attributeFilters = attributeFilters.filter((_, i) => i !== index);
		page = 1;
		loadData(true);
	}

	function updateSessionsUrl(pushToHistory = true) {
		updateUrl(
			{
				preset: selectedPreset || null,
				from: selectedPreset ? null : getFromDateTimeUTC(),
				to: selectedPreset ? null : getToDateTimeUTC(),
				search: searchQuery.trim() || null,
				attr: attributeFilters.map((f) => `${f.key}${filterOperator(f)}${f.value}`)
			},
			{ pushToHistory }
		);
	}

	function handlePopState() {
		const urlParams = parseSessionsUrlParams();
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

	function handleSearch() {
		page = 1;
		loadData(true);
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

	const ABANDONED_AFTER_MS = 15 * 60_000;

	function durationLabel(s: Session): string {
		if (s.endedAt) return formatDuration(s.duration);
		const startedMs = Date.parse(s.startedAt);
		if (Number.isFinite(startedMs) && Date.now() - startedMs >= ABANDONED_AFTER_MS) {
			return 'Abandoned';
		}
		return 'in progress';
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

			const response = await api.post('/sessions', requestBody, {
				projectId: projectsState.currentProjectId ?? undefined
			});

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
						Sessions are full recordings of user interactions and are only recorded when manually
						enabled (<code class="font-mono">recordAllSessions: true</code> in the SDK init options).
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

	<SearchBar
		placeholder="Search session ID, user, or attribute value..."
		bind:value={searchQuery}
		onSearch={handleSearch}
		disabled={loading}
	/>

	<div class="flex flex-wrap items-center gap-2">
		<button
			type="button"
			class="inline-flex items-center gap-1 rounded-full border border-dashed px-3 py-0.5 text-xs font-medium text-muted-foreground transition-colors hover:border-foreground/40 hover:text-foreground"
			onclick={() => openAddFilterDialog()}
			disabled={loading}
		>
			<Plus class="h-3 w-3" />
			Add filter
		</button>
		{#each attributeFilters as f, i (i)}
			<span
				class="inline-flex items-center gap-1 rounded-full bg-muted px-2 py-0.5 font-mono text-xs"
			>
				<button
					type="button"
					aria-label="Edit filter"
					class="inline-flex cursor-pointer items-center gap-1 hover:text-primary"
					onclick={() => openEditFilterDialog(i)}
				>
					<span class="text-muted-foreground">{f.key}</span>
					<span class={f.exclude ? 'text-red-500' : ''}>{filterOperator(f)}</span>
					<span class="max-w-64 truncate" title={f.value}>{f.value || '""'}</span>
				</button>
				<button
					type="button"
					aria-label="Remove filter"
					class="ml-1 text-muted-foreground hover:text-foreground"
					onclick={() => removeAttributeFilter(i)}
				>
					<X class="h-3 w-3" />
				</button>
			</span>
		{/each}
	</div>

	<AlertDialog.Root open={addFilterOpen} onOpenChange={(open) => (addFilterOpen = open)}>
		<AlertDialog.Content>
			<AlertDialog.Header>
				<AlertDialog.Title
					>{dialogEditIndex === null
						? 'Add attribute filter'
						: 'Edit attribute filter'}</AlertDialog.Title
				>
				<AlertDialog.Description>
					Filter by attributes such as <code class="font-mono">userId</code>,
					<code class="font-mono">email</code>, or <code class="font-mono">client.ip</code>. All
					filters must match. Contains ignores case; excluded filters also include sessions without
					the attribute.
				</AlertDialog.Description>
			</AlertDialog.Header>
			<div class="flex flex-col gap-3">
				<FormField label="Attribute key">
					<Input placeholder="userId" bind:value={dialogKey} onkeydown={handleDialogKeydown} />
				</FormField>
				<FormField label="Operator">
					<Select.Root
						type="single"
						value={dialogOperator}
						onValueChange={(v) => (dialogOperator = v as FilterOperator)}
					>
						<Select.Trigger class="w-full"
							>{operatorOptions.find((o) => o.value === dialogOperator)?.label}</Select.Trigger
						>
						<Select.Content>
							{#each operatorOptions as option (option.value)}
								<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				</FormField>
				<FormField label="Value">
					<Input placeholder="u_42" bind:value={dialogValue} onkeydown={handleDialogKeydown} />
				</FormField>
				{#if dialogError}
					<p class="text-xs text-red-500">{dialogError}</p>
				{/if}
			</div>
			<AlertDialog.Footer>
				<Button variant="outline" onclick={() => (addFilterOpen = false)}>Cancel</Button>
				<Button onclick={submitDialogFilter}>
					{#if dialogEditIndex === null}
						<Plus class="h-4 w-4" /> Add filter
					{:else}
						<Check class="h-4 w-4" /> Update filter
					{/if}
				</Button>
			</AlertDialog.Footer>
		</AlertDialog.Content>
	</AlertDialog.Root>

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
						colspan={5}
						message={searchQuery.trim() || attributeFilters.length
							? 'No sessions match your search and filters.'
							: 'No sessions recorded in this time range. Enable recordAllSessions in the SDK to start capturing them.'}
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
												openAddFilterDialog(key, value);
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
											>{attributesExpanded ? 'Show less' : `+${attributes.length - 3} more`}</button
										>
									{/if}
								</div>
							</Table.Cell>
							<Table.Cell class="text-sm"
								>{formatRelativeTime(session.startedAt, timezone)}</Table.Cell
							>
							<Table.Cell class="font-mono text-sm tabular-nums">
								{durationLabel(session)}
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
</div>
