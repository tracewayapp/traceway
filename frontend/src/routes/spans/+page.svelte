<script lang="ts">
	import { getErrorMessage, getErrorStatus } from '$lib/utils/errors';
	import { onMount, onDestroy } from 'svelte';
	import { browser } from '$app/environment';
	import { api } from '$lib/api';
	import {
		formatDuration,
		formatDateTime,
		toUTCISO,
		calendarDateTimeToLuxon
	} from '$lib/utils/formatters';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import { projectsState } from '$lib/state/projects.svelte';
	import * as Table from '$lib/components/ui/table';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { TracewayTableHeader } from '$lib/components/ui/traceway-table-header';
	import { TableEmptyState } from '$lib/components/ui/table-empty-state';
	import { PaginationFooter } from '$lib/components/ui/pagination-footer';
	import { TimeRangePicker } from '$lib/components/ui/time-range-picker';
	import { SearchBar } from '$lib/components/ui/search-bar';
	import { WarningCallout } from '$lib/components/ui/warning-callout';
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import TableContainer from '$lib/components/traceway/table-container.svelte';
	import ToolbarSelect from '$lib/components/traceway/toolbar-select.svelte';
	import FormField from '$lib/components/traceway/form-field.svelte';
	import { CircleAlert, Plus, X } from '@lucide/svelte';
	import { CalendarDate } from '@internationalized/date';
	import { createRowClickHandler } from '$lib/utils/navigation';
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
	import type { Span } from '$lib/types/spans';
	import {
		SPAN_KIND_OPTIONS,
		SPAN_STATUS_OPTIONS,
		isErrorSpan,
		spanKindLabel,
		spanStatusLabel
	} from '$lib/utils/span-fields';
	import {
		type SpanAttributeFilter,
		type SpanSearchFilters,
		DEFAULT_SPAN_SEARCH_PRESET,
		parseSpanAttributeFilter,
		readSpanSearchFilters,
		spanSearchBody,
		spanSearchUrlParams,
		spanTraceHref
	} from '$lib/utils/span-explorer';

	const timezone = $derived(getTimezone());
	const initialTimezone = getTimezone();

	type SortField = 'start_time' | 'duration';
	type SpanSearchResponse = {
		data: Span[] | null;
		pagination: { total: number; totalPages: number };
		services: string[] | null;
	};

	let spans = $state<Span[]>([]);
	let services = $state<string[]>([]);
	let loading = $state(true);
	let loadSequence = 0;
	let error = $state('');
	// A 422 is advice about the search itself (range too long, search too slow), not a failure.
	let guidance = $state('');

	let page = $state(1);
	let pageSize = $state(50);
	let total = $state(0);
	let totalPages = $state(0);

	const initialUrlParams = parseTimeRangeFromUrl(initialTimezone, DEFAULT_SPAN_SEARCH_PRESET);
	const initialRange = getResolvedTimeRange(initialUrlParams, initialTimezone);

	function filtersFromUrl(): SpanSearchFilters {
		return readSpanSearchFilters(browser ? window.location.search : '');
	}

	let filters = $state<SpanSearchFilters>(filtersFromUrl());

	let selectedPreset = $state<string | null>(initialUrlParams.preset);
	let fromDate = $state<CalendarDate>(dateToCalendarDate(initialRange.from, initialTimezone));
	let toDate = $state<CalendarDate>(dateToCalendarDate(initialRange.to, initialTimezone));
	let fromTime = $state(dateToTimeString(initialRange.from, initialTimezone));
	let toTime = $state(dateToTimeString(initialRange.to, initialTimezone));

	const SORT_STORAGE_KEY = 'spans';
	const initialSort = getSortState(SORT_STORAGE_KEY, { field: 'start_time', direction: 'desc' });
	let orderBy = $state<SortField>(initialSort.field === 'duration' ? 'duration' : 'start_time');
	let sortDirection = $state<SortDirection>(initialSort.direction);

	const serviceOptions = $derived([
		{ value: '', label: 'All services' },
		...[...new Set([...services, filters.service].filter((name) => !!name))].map((name) => ({
			value: name,
			label: name
		}))
	]);
	const kindOptions = [{ value: '', label: 'All kinds' }, ...SPAN_KIND_OPTIONS];
	const statusOptions = [{ value: '', label: 'Any status' }, ...SPAN_STATUS_OPTIONS];

	function getFromDateTimeUTC(): string {
		const [hour, minute] = (fromTime || '00:00').split(':').map(Number);
		return toUTCISO(
			calendarDateTimeToLuxon(
				{ year: fromDate.year, month: fromDate.month, day: fromDate.day, hour, minute },
				timezone
			)
		);
	}

	function getToDateTimeUTC(): string {
		const [hour, minute] = (toTime || '23:59').split(':').map(Number);
		return toUTCISO(
			calendarDateTimeToLuxon(
				{ year: toDate.year, month: toDate.month, day: toDate.day, hour, minute },
				timezone
			).endOf('minute')
		);
	}

	function updateExplorerUrl(pushToHistory: boolean) {
		const range: Record<string, string | string[] | undefined> = selectedPreset
			? { preset: selectedPreset }
			: { from: getFromDateTimeUTC(), to: getToDateTimeUTC() };
		updateUrl({ ...range, ...spanSearchUrlParams(filters) }, { pushToHistory });
	}

	function handlePopState() {
		const urlParams = parseTimeRangeFromUrl(timezone, DEFAULT_SPAN_SEARCH_PRESET);
		const range = getResolvedTimeRange(urlParams, timezone);
		selectedPreset = urlParams.preset;
		fromDate = dateToCalendarDate(range.from, timezone);
		fromTime = dateToTimeString(range.from, timezone);
		toDate = dateToCalendarDate(range.to, timezone);
		toTime = dateToTimeString(range.to, timezone);
		filters = filtersFromUrl();
		page = 1;
		loadData(false);
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

	async function loadData(pushToHistory = true) {
		const sequence = ++loadSequence;
		loading = true;
		error = '';
		guidance = '';

		if (selectedPreset) {
			const range = getTimeRangeFromPreset(selectedPreset, timezone);
			fromDate = dateToCalendarDate(range.from, timezone);
			toDate = dateToCalendarDate(range.to, timezone);
			fromTime = dateToTimeString(range.from, timezone);
			toTime = dateToTimeString(range.to, timezone);
		}

		updateExplorerUrl(pushToHistory);

		try {
			const response = (await api.post(
				'/spans/search',
				{
					fromDate: getFromDateTimeUTC(),
					toDate: getToDateTimeUTC(),
					orderBy: `${orderBy} ${sortDirection}`,
					...spanSearchBody(filters),
					pagination: { page, pageSize }
				},
				{ projectId: projectsState.currentProjectId ?? undefined }
			)) as SpanSearchResponse;
			if (sequence !== loadSequence) return;

			spans = response.data || [];
			services = response.services || [];
			total = response.pagination.total;
			totalPages = response.pagination.totalPages;
		} catch (e) {
			if (sequence !== loadSequence) return;
			console.error(e);
			spans = [];
			total = 0;
			totalPages = 0;
			if (getErrorStatus(e) === 422) {
				guidance = getErrorMessage(e);
			} else {
				error = getErrorMessage(e) || 'Failed to load spans';
			}
		} finally {
			if (sequence === loadSequence) loading = false;
		}
	}

	function applyFilters() {
		page = 1;
		loadData(true);
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

	let filterDialogOpen = $state(false);
	let dialogKey = $state('');
	let dialogValue = $state('');
	let dialogError = $state('');

	function openFilterDialog() {
		dialogKey = '';
		dialogValue = '';
		dialogError = '';
		filterDialogOpen = true;
	}

	function submitFilterDialog() {
		const parsed = parseSpanAttributeFilter(dialogKey, dialogValue, filters.attributes);
		if (typeof parsed === 'string') {
			dialogError = parsed;
			return;
		}
		filters.attributes = [...filters.attributes, parsed];
		filterDialogOpen = false;
		applyFilters();
	}

	function handleDialogKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			e.preventDefault();
			submitFilterDialog();
		}
	}

	function removeAttributeFilter(filter: SpanAttributeFilter) {
		filters.attributes = filters.attributes.filter((existing) => existing !== filter);
		applyFilters();
	}

	function handleInputKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter') applyFilters();
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
	<PageHeader title="Spans" description="Every OpenTelemetry span this project has received.">
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
		placeholder="Search span names..."
		bind:value={filters.name}
		onSearch={applyFilters}
		disabled={loading}
	>
		{#snippet pillEnd()}
			<ToolbarSelect
				bind:value={filters.service}
				options={serviceOptions}
				ariaLabel="Service"
				class="h-9 w-[170px] shrink-0 shadow-none sm:rounded-none sm:border-r-0"
				onChange={applyFilters}
			/>
		{/snippet}
	</SearchBar>

	<div class="flex flex-wrap items-center gap-2">
		<ToolbarSelect
			bind:value={filters.kind}
			options={kindOptions}
			ariaLabel="Span kind"
			class="w-[130px]"
			onChange={applyFilters}
		/>
		<ToolbarSelect
			bind:value={filters.status}
			options={statusOptions}
			ariaLabel="Span status"
			class="w-[130px]"
			onChange={applyFilters}
		/>
		<Input
			inputmode="decimal"
			placeholder="Min ms"
			aria-label="Minimum duration in milliseconds"
			class="h-9 w-[100px]"
			bind:value={filters.minMs}
			onchange={applyFilters}
			onkeydown={handleInputKeydown}
		/>
		<Input
			inputmode="decimal"
			placeholder="Max ms"
			aria-label="Maximum duration in milliseconds"
			class="h-9 w-[100px]"
			bind:value={filters.maxMs}
			onchange={applyFilters}
			onkeydown={handleInputKeydown}
		/>
		<Input
			placeholder="Trace ID"
			aria-label="Trace ID"
			class="h-9 w-[280px] font-mono text-xs"
			bind:value={filters.traceId}
			onchange={applyFilters}
			onkeydown={handleInputKeydown}
		/>
	</div>

	<div class="flex flex-wrap items-center gap-2">
		<button
			type="button"
			class="inline-flex items-center gap-1 rounded-full border border-dashed px-3 py-0.5 text-xs font-medium text-muted-foreground transition-colors hover:border-foreground/40 hover:text-foreground"
			onclick={openFilterDialog}
			disabled={loading}
		>
			<Plus class="h-3 w-3" />
			Add attribute filter
		</button>
		{#each filters.attributes as filter (filter.key + '=' + filter.value)}
			<span
				class="inline-flex items-center gap-1 rounded-full bg-muted px-2 py-0.5 font-mono text-xs"
			>
				<span class="text-muted-foreground">{filter.key}</span>
				<span>=</span>
				<span>{filter.value}</span>
				<button
					type="button"
					aria-label="Remove filter"
					class="ml-1 cursor-pointer text-muted-foreground hover:text-foreground"
					onclick={() => removeAttributeFilter(filter)}
				>
					<X class="h-3 w-3" />
				</button>
			</span>
		{/each}
	</div>

	<AlertDialog.Root open={filterDialogOpen} onOpenChange={(open) => (filterDialogOpen = open)}>
		<AlertDialog.Content>
			<AlertDialog.Header>
				<AlertDialog.Title>Add attribute filter</AlertDialog.Title>
				<AlertDialog.Description>
					Matches spans whose attribute has exactly this value. Keys are the OpenTelemetry names,
					such as <code class="font-mono">http.request.method</code>.
				</AlertDialog.Description>
			</AlertDialog.Header>
			<div class="flex flex-col gap-3">
				<FormField label="Attribute key">
					<Input
						placeholder="http.request.method"
						bind:value={dialogKey}
						onkeydown={handleDialogKeydown}
					/>
				</FormField>
				<FormField label="Value">
					<Input placeholder="GET" bind:value={dialogValue} onkeydown={handleDialogKeydown} />
				</FormField>
				{#if dialogError}
					<p class="text-xs text-red-500">{dialogError}</p>
				{/if}
			</div>
			<AlertDialog.Footer>
				<Button variant="outline" onclick={() => (filterDialogOpen = false)}>Cancel</Button>
				<Button onclick={submitFilterDialog}><Plus class="h-4 w-4" /> Add filter</Button>
			</AlertDialog.Footer>
		</AlertDialog.Content>
	</AlertDialog.Root>

	{#if guidance}
		<WarningCallout title="Narrow the search">{guidance}</WarningCallout>
	{/if}

	<TableContainer minWidth="860px" empty={loading || !!error || spans.length === 0}>
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
			{:else if spans.length === 0}
				<Table.Body>
					<TableEmptyState
						colspan={7}
						message={guidance ? 'No results to show' : 'No spans match in this time range'}
					/>
				</Table.Body>
			{:else}
				<Table.Header>
					<Table.Row>
						<TracewayTableHeader
							label="Started"
							tooltip="When the span started"
							sortField="start_time"
							currentSortField={orderBy}
							{sortDirection}
							onSort={(field) => handleSort(field as SortField)}
							class="w-[210px]"
						/>
						<TracewayTableHeader label="Service" tooltip="The service.name of the resource" />
						<TracewayTableHeader label="Span" tooltip="The span name" />
						<TracewayTableHeader
							label="Kind"
							tooltip="Server, client, producer, consumer or internal"
							class="w-[100px]"
						/>
						<TracewayTableHeader label="Status" tooltip="The span status" class="w-[90px]" />
						<TracewayTableHeader
							label="Duration"
							tooltip="How long the span took"
							sortField="duration"
							currentSortField={orderBy}
							{sortDirection}
							onSort={(field) => handleSort(field as SortField)}
							class="w-[110px]"
						/>
						<TracewayTableHeader
							label="Trace"
							tooltip="Start of the trace ID. Open the row to see the whole trace"
							class="w-[110px]"
						/>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each spans as span (`${span.traceId}:${span.spanId}`)}
						<Table.Row
							class="cursor-pointer"
							onclick={createRowClickHandler(spanTraceHref(span, { focusSpan: true }) ?? '')}
						>
							<Table.Cell class="text-sm whitespace-nowrap text-muted-foreground tabular-nums">
								{formatDateTime(span.startTime, { timezone, format: 'datetime-seconds' })}
							</Table.Cell>
							<Table.Cell class="text-sm">{span.serviceName || '-'}</Table.Cell>
							<Table.Cell class="max-w-[420px] truncate font-mono text-sm">
								{#if isErrorSpan(span)}<CircleAlert
										class="mr-1 inline h-3.5 w-3.5 align-[-2px] text-destructive"
										aria-label="Error"
									/>{/if}{span.name}
							</Table.Cell>
							<Table.Cell class="text-sm text-muted-foreground">
								{spanKindLabel(span.spanKind) || '-'}
							</Table.Cell>
							<Table.Cell class={isErrorSpan(span) ? 'text-sm text-destructive' : 'text-sm'}>
								{spanStatusLabel(span.statusCode)}
							</Table.Cell>
							<Table.Cell class="font-mono text-sm tabular-nums">
								{formatDuration(span.duration)}
							</Table.Cell>
							<Table.Cell class="font-mono text-xs text-muted-foreground">
								{span.traceId.slice(0, 8)}
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
		itemLabel="span"
	/>
</div>
