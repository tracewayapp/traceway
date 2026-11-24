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
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import {
		FilterBar,
		type FilterApply,
		type FilterField,
		describeTimeRange,
		attributeConfig,
		traceIdField,
		valueOptions
	} from '$lib/components/traceway/filter-bar';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { TracewayTableHeader } from '$lib/components/ui/traceway-table-header';
	import { TableEmptyState } from '$lib/components/ui/table-empty-state';
	import { PaginationFooter } from '$lib/components/ui/pagination-footer';
	import { TimeRangePicker } from '$lib/components/ui/time-range-picker';
	import { WarningCallout } from '$lib/components/ui/warning-callout';
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import TableContainer from '$lib/components/traceway/table-container.svelte';
	import FormField from '$lib/components/traceway/form-field.svelte';
	import SavedFiltersMenu from '$lib/components/traceway/saved-filters-menu.svelte';
	import ChartPanel, { type ChartSelection } from '$lib/components/value-chart/chart-panel.svelte';
	import { Check, ChartLine, ChevronDown, CircleAlert, Plus, Timer } from '@lucide/svelte';
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
		type SpanChart,
		type SpanChartTarget,
		type SpanSearchFilters,
		DEFAULT_SPAN_SEARCH_PRESET,
		SPAN_ATTRIBUTE_RULES,
		readSpanChart,
		readSpanSearchFilters,
		spanChartBody,
		spanChartDefaults,
		spanChartLabel,
		spanChartUrlParams,
		spanSearchBody,
		spanSearchUrlParams,
		spanTraceHref
	} from '$lib/utils/span-explorer';
	import { formatChartMs, formatChartNumber, type ChartLines } from '$lib/utils/value-chart';
	import { paramsToApply, toQueryString, type SavedFilter } from '$lib/utils/saved-filters';

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
	let chart = $state<SpanChart | null>(readSpanChart(browser ? window.location.search : ''));
	// What the table shows; the chart reloads when it changes.
	let chartSelection = $state<ChartSelection | null>(null);
	// The applied filters as a query string, for the saved filters menu.
	let appliedQuery = $state('');

	let selectedPreset = $state<string | null>(initialUrlParams.preset);
	let fromDate = $state<CalendarDate>(dateToCalendarDate(initialRange.from, initialTimezone));
	let toDate = $state<CalendarDate>(dateToCalendarDate(initialRange.to, initialTimezone));
	let fromTime = $state(dateToTimeString(initialRange.from, initialTimezone));
	let toTime = $state(dateToTimeString(initialRange.to, initialTimezone));

	const SORT_STORAGE_KEY = 'spans';
	const initialSort = getSortState(SORT_STORAGE_KEY, { field: 'start_time', direction: 'desc' });
	let orderBy = $state<SortField>(initialSort.field === 'duration' ? 'duration' : 'start_time');
	let sortDirection = $state<SortDirection>(initialSort.direction);

	const filterFields = $derived<FilterField[]>([
		{
			kind: 'select',
			key: 'service',
			label: 'Service',
			options: valueOptions(services),
			searchable: true
		},
		{
			kind: 'select',
			key: 'kind',
			label: 'Kind',
			options: SPAN_KIND_OPTIONS
		},
		{
			kind: 'select',
			key: 'status',
			label: 'Status',
			options: SPAN_STATUS_OPTIONS
		},
		{
			kind: 'range',
			key: 'duration',
			label: 'Duration',
			minKey: 'minMs',
			maxKey: 'maxMs',
			unit: 'ms'
		},
		traceIdField('traceId', 'Every span of one trace.')
	]);
	const filterValues = $derived({
		service: filters.service,
		kind: filters.kind,
		status: filters.status,
		minMs: filters.minMs,
		maxMs: filters.maxMs,
		traceId: filters.traceId
	});

	function handleFilterApply(next: FilterApply) {
		filters = {
			name: next.search,
			service: next.values.service ?? '',
			kind: next.values.kind ?? '',
			status: next.values.status ?? '',
			minMs: next.values.minMs ?? '',
			maxMs: next.values.maxMs ?? '',
			traceId: next.values.traceId ?? '',
			attributes: next.attributes
		};
		applyFilters();
	}

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
		const params = { ...range, ...spanSearchUrlParams(filters), ...spanChartUrlParams(chart) };
		appliedQuery = toQueryString(params);
		updateUrl(params, { pushToHistory });
	}

	function applyUrlState(params: URLSearchParams) {
		const urlParams = parseTimeRangeFromUrl(timezone, DEFAULT_SPAN_SEARCH_PRESET, params);
		const range = getResolvedTimeRange(urlParams, timezone);
		selectedPreset = urlParams.preset;
		fromDate = dateToCalendarDate(range.from, timezone);
		fromTime = dateToTimeString(range.from, timezone);
		toDate = dateToCalendarDate(range.to, timezone);
		toTime = dateToTimeString(range.to, timezone);
		filters = readSpanSearchFilters(params.toString());
		chart = readSpanChart(params);
		page = 1;
	}

	function handlePopState() {
		applyUrlState(new URLSearchParams(window.location.search));
		loadData(false);
	}

	function applySavedFilter(filter: SavedFilter) {
		applyUrlState(
			paramsToApply(filter, {
				preset: selectedPreset,
				from: getFromDateTimeUTC(),
				to: getToDateTimeUTC()
			})
		);
		loadData(true);
	}

	const timeRangeLabel = $derived(describeTimeRange(selectedPreset));

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
			const selection = {
				fromDate: getFromDateTimeUTC(),
				toDate: getToDateTimeUTC(),
				...spanSearchBody(filters)
			};
			// The chart loads beside the table, never before it.
			chartSelection = selection;
			const response = (await api.post(
				'/spans/search',
				{
					...selection,
					orderBy: `${orderBy} ${sortDirection}`,
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

	const SPAN_CHART_ENDPOINTS = { series: '/spans/series', distribution: '/spans/distribution' };

	function setChartTarget(target: SpanChartTarget) {
		chart = {
			target,
			view: chart?.view ?? 'series',
			aggregations: [...spanChartDefaults(target)]
		};
		updateExplorerUrl(true);
	}

	// View and line changes replace the history entry instead of adding one per click.
	function setChartLines(lines: ChartLines) {
		if (!chart) return;
		chart = { ...chart, view: lines.view, aggregations: lines.aggregations };
		updateExplorerUrl(false);
	}

	function closeChart() {
		chart = null;
		updateExplorerUrl(true);
	}

	function handleChartRangeSelect(from: Date, to: Date) {
		handleTimeRangeChange(
			{ date: dateToCalendarDate(from, timezone), time: dateToTimeString(from, timezone) },
			{ date: dateToCalendarDate(to, timezone), time: dateToTimeString(to, timezone) },
			null
		);
	}

	let chartDialogOpen = $state(false);
	let chartDialogKey = $state('');
	let chartDialogError = $state('');

	function openChartDialog() {
		chartDialogKey = chart?.target.kind === 'attribute' ? chart.target.key : '';
		chartDialogError = '';
		chartDialogOpen = true;
	}

	function submitChartDialog() {
		const key = chartDialogKey.trim();
		if (!key) {
			chartDialogError = 'Enter an attribute key.';
			return;
		}
		if (/["\\]/.test(key)) {
			chartDialogError = 'Attribute keys cannot contain quotes or backslashes.';
			return;
		}
		chartDialogOpen = false;
		setChartTarget({ kind: 'attribute', key });
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

	<FilterBar
		search={filters.name}
		searchPlaceholder="Search span names..."
		fields={filterFields}
		values={filterValues}
		attributes={attributeConfig(filters.attributes, SPAN_ATTRIBUTE_RULES)}
		onApply={handleFilterApply}
		disabled={loading}
		noun="spans"
		rangeLabel={timeRangeLabel}
	>
		{#snippet actions()}
			{#if !chart}
				<Button
					variant="outline"
					size="sm"
					class="h-8"
					onclick={() => setChartTarget({ kind: 'duration' })}
				>
					<ChartLine class="h-3.5 w-3.5" />
					Chart
				</Button>
			{/if}
			<SavedFiltersMenu
				page="spans"
				currentQuery={appliedQuery}
				{timeRangeLabel}
				namePlaceholder="Slow checkout queries"
				onApply={applySavedFilter}
				disabled={loading}
			/>
		{/snippet}

		<!-- An empty search draws no chart: a flat line over empty buckets says nothing. -->
		{#if chart && (loading || spans.length > 0)}
			<ChartPanel
				class="rounded-md border"
				label={spanChartLabel(chart.target)}
				heading={chartHeading}
				lines={chart}
				endpoints={SPAN_CHART_ENDPOINTS}
				body={spanChartBody(chart.target)}
				selection={chartSelection}
				{timezone}
				format={chart.target.kind === 'duration' ? formatChartMs : formatChartNumber}
				rows={{ one: 'span', many: 'spans' }}
				values={chart.target.kind === 'duration'
					? { one: 'span', many: 'spans' }
					: { one: 'value', many: 'values' }}
				emptyText={chart.target.kind === 'duration'
					? 'No spans match in this time range.'
					: undefined}
				onChange={setChartLines}
				onClose={closeChart}
				onRangeSelect={handleChartRangeSelect}
			/>
		{/if}

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
	</FilterBar>

	<AlertDialog.Root open={chartDialogOpen} onOpenChange={(open) => (chartDialogOpen = open)}>
		<AlertDialog.Content>
			<AlertDialog.Header>
				<AlertDialog.Title>Chart an attribute</AlertDialog.Title>
				<AlertDialog.Description>
					Charts the numeric values of one attribute across the spans the filters match, such as
					<code class="font-mono">db.response.returned_rows</code>.
				</AlertDialog.Description>
			</AlertDialog.Header>
			<div class="flex flex-col gap-3">
				<FormField label="Attribute key">
					<Input
						placeholder="db.response.returned_rows"
						bind:value={chartDialogKey}
						onkeydown={(e) => {
							if (e.key === 'Enter') {
								e.preventDefault();
								submitChartDialog();
							}
						}}
					/>
				</FormField>
				{#if chartDialogError}
					<p class="text-xs text-red-500">{chartDialogError}</p>
				{/if}
			</div>
			<AlertDialog.Footer>
				<Button variant="outline" onclick={() => (chartDialogOpen = false)}>Cancel</Button>
				<Button onclick={submitChartDialog}><ChartLine class="h-4 w-4" /> Chart values</Button>
			</AlertDialog.Footer>
		</AlertDialog.Content>
	</AlertDialog.Root>

	{#snippet chartHeading()}
		<DropdownMenu.Root>
			<DropdownMenu.Trigger
				class="-ml-1 inline-flex min-w-0 items-center gap-1.5 rounded px-1 py-0.5 text-xs hover:bg-muted"
				aria-label="What to chart"
				title="What to chart"
			>
				{#if chart?.target.kind === 'attribute'}
					<ChartLine class="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
					<span class="truncate font-mono font-medium">{chart.target.key}</span>
				{:else}
					<Timer class="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
					<span class="font-medium">Duration</span>
				{/if}
				<ChevronDown class="h-3 w-3 shrink-0 opacity-60" />
			</DropdownMenu.Trigger>
			<DropdownMenu.Content align="start" class="min-w-52">
				<DropdownMenu.Label class="text-xs font-normal text-muted-foreground"
					>Chart</DropdownMenu.Label
				>
				<DropdownMenu.Item onclick={() => setChartTarget({ kind: 'duration' })}>
					<Check class="h-4 w-4 {chart?.target.kind === 'duration' ? '' : 'opacity-0'}" />
					Duration
				</DropdownMenu.Item>
				{#if chart?.target.kind === 'attribute'}
					<DropdownMenu.Item onclick={openChartDialog}>
						<Check class="h-4 w-4" />
						<span class="truncate font-mono">{chart.target.key}</span>
					</DropdownMenu.Item>
				{/if}
				<DropdownMenu.Separator />
				<DropdownMenu.Item onclick={openChartDialog}>
					<Plus class="h-4 w-4" />
					A numeric attribute...
				</DropdownMenu.Item>
			</DropdownMenu.Content>
		</DropdownMenu.Root>
	{/snippet}
</div>
