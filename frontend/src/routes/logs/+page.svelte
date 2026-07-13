<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { browser } from '$app/environment';
	import { goto } from '$app/navigation';
	import { api } from '$lib/api';
	import * as Table from '$lib/components/ui/table';
	import { SearchBar } from '$lib/components/ui/search-bar';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { projectsState } from '$lib/state/projects.svelte';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import { TracewayTableHeader } from '$lib/components/ui/traceway-table-header';
	import { TableEmptyState } from '$lib/components/ui/table-empty-state';
	import { PaginationFooter } from '$lib/components/ui/pagination-footer';
	import { TimeRangePicker } from '$lib/components/ui/time-range-picker';
	import { SeverityBadge } from '$lib/components/ui/severity-badge';
	import * as Select from '$lib/components/ui/select';
	import { Input } from '$lib/components/ui/input';
	import { Button } from '$lib/components/ui/button';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import ExpandedLogRow from '$lib/components/trace-logs/expanded-log-row.svelte';
	import { Plus, X } from '@lucide/svelte';
	import { CalendarDate } from '@internationalized/date';
	import {
		parseTimeRangeFromUrl,
		getResolvedTimeRange,
		getTimeRangeFromPreset,
		dateToCalendarDate,
		dateToTimeString
	} from '$lib/utils/url-params';
	import { calendarDateTimeToLuxon, toUTCISO, formatDateTime } from '$lib/utils/formatters';
	import {
		getSortState,
		setSortState,
		handleSortClick,
		type SortDirection
	} from '$lib/utils/sort-storage';

	const timezone = $derived(getTimezone());

	const SORT_STORAGE_KEY = 'logs';
	const initialSort = getSortState(SORT_STORAGE_KEY, { field: 'timestamp', direction: 'desc' });
	let sortField = $state(initialSort.field);
	let sortDirection = $state<SortDirection>(initialSort.direction);

	type LogRecord = {
		id: string;
		projectId: string;
		timestamp: string;
		traceId: string;
		spanId: string;
		traceFlags: number;
		severityText: string;
		severityNumber: number;
		serviceName: string;
		body: string;
		resourceSchemaUrl: string;
		resourceAttributes: Record<string, string> | null;
		scopeSchemaUrl: string;
		scopeName: string;
		scopeVersion: string;
		scopeAttributes: Record<string, string> | null;
		logAttributes: Record<string, string> | null;
	};

	let logs = $state<LogRecord[]>([]);
	let loading = $state(true);
	let error = $state('');
	let expandedId = $state<string | null>(null);

	let page = $state(1);
	let pageSize = $state(50);
	let total = $state(0);
	let totalPages = $state(0);

	type AttributeFilter = {
		scope: 'resource' | 'scope' | 'log';
		key: string;
		value: string;
	};

	// Parse `resource.service.name=backend-service` (or scope.*/log.*) into a
	// structured filter. Returns null if the input doesn't match the shape.
	function parseAttributeFilter(input: string): AttributeFilter | null {
		const trimmed = input.trim();
		const m = trimmed.match(/^(resource|scope|log)\.([^=]+)=(.*)$/);
		if (!m) return null;
		const key = m[2].trim();
		if (!key) return null;
		return { scope: m[1] as AttributeFilter['scope'], key, value: m[3] };
	}

	function formatAttributeFilter(f: AttributeFilter): string {
		return `${f.scope}.${f.key}=${f.value}`;
	}

	function parseLogsUrlParams() {
		if (!browser) {
			return {
				preset: '24h',
				from: null,
				to: null,
				search: '',
				searchType: 'body',
				minSeverity: 0,
				serviceName: '',
				traceId: '',
				attributeFilters: [] as AttributeFilter[]
			};
		}
		const params = new URLSearchParams(window.location.search);
		const timeParams = parseTimeRangeFromUrl(timezone, '24h');
		const minSev = Number(params.get('minSeverity') || '0');
		const attrs: AttributeFilter[] = [];
		for (const raw of params.getAll('attr')) {
			const parsed = parseAttributeFilter(raw);
			if (parsed) attrs.push(parsed);
		}
		return {
			...timeParams,
			search: params.get('search') || '',
			searchType: params.get('searchType') || 'body',
			minSeverity: Number.isFinite(minSev) ? minSev : 0,
			serviceName: params.get('service') || '',
			traceId: params.get('traceId') || '',
			attributeFilters: attrs
		};
	}

	const initialUrlParams = parseLogsUrlParams();
	const initialRange = getResolvedTimeRange(initialUrlParams, timezone);

	let selectedPreset = $state<string | null>(initialUrlParams.preset);
	let fromDate = $state<CalendarDate>(dateToCalendarDate(initialRange.from, timezone));
	let toDate = $state<CalendarDate>(dateToCalendarDate(initialRange.to, timezone));
	let fromTime = $state(dateToTimeString(initialRange.from, timezone));
	let toTime = $state(dateToTimeString(initialRange.to, timezone));

	let searchQuery = $state(initialUrlParams.search);
	let searchType = $state(initialUrlParams.searchType);
	let minSeverity = $state<number>(initialUrlParams.minSeverity);
	let serviceName = $state(initialUrlParams.serviceName);
	let traceIdFilter = $state(initialUrlParams.traceId);
	let attributeFilters = $state<AttributeFilter[]>(initialUrlParams.attributeFilters);

	// Add-filter dialog
	let addFilterOpen = $state(false);
	let dialogKey = $state('');
	let dialogValue = $state('');
	let dialogError = $state('');

	function openAddFilterDialog() {
		dialogKey = '';
		dialogValue = '';
		dialogError = '';
		addFilterOpen = true;
	}

	function submitDialogFilter() {
		const composed = `${dialogKey.trim()}=${dialogValue}`;
		const parsed = parseAttributeFilter(composed);
		if (!parsed) {
			dialogError = 'Key must start with resource., scope., or log.';
			return;
		}
		const dup = attributeFilters.some(
			(f) => f.scope === parsed.scope && f.key === parsed.key && f.value === parsed.value
		);
		if (!dup) {
			attributeFilters = [...attributeFilters, parsed];
		}
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

	const searchTypeOptions = [
		{ value: 'body', label: 'Message' },
		{ value: 'service', label: 'Service' },
		{ value: 'trace', label: 'Trace ID' }
	];

	const severityOptions = [
		{ value: '0', label: 'All levels' },
		{ value: '1', label: 'TRACE+' },
		{ value: '5', label: 'DEBUG+' },
		{ value: '9', label: 'INFO+' },
		{ value: '13', label: 'WARN+' },
		{ value: '17', label: 'ERROR+' },
		{ value: '21', label: 'FATAL' }
	];

	const severityTriggerLabel = $derived(
		severityOptions.find((o) => Number(o.value) === minSeverity)?.label ?? 'All levels'
	);

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
		).endOf('minute');
		return toUTCISO(luxonDt);
	}

	function updateLogsUrl(pushToHistory = true) {
		if (!browser) return;
		const urlParams = new URLSearchParams();
		if (selectedPreset) {
			urlParams.set('preset', selectedPreset);
		} else {
			urlParams.set('from', getFromDateTimeUTC());
			urlParams.set('to', getToDateTimeUTC());
		}
		if (searchQuery.trim()) urlParams.set('search', searchQuery.trim());
		if (searchType !== 'body') urlParams.set('searchType', searchType);
		if (minSeverity > 0) urlParams.set('minSeverity', String(minSeverity));
		if (serviceName.trim()) urlParams.set('service', serviceName.trim());
		if (traceIdFilter.trim()) urlParams.set('traceId', traceIdFilter.trim());
		for (const f of attributeFilters) {
			urlParams.append('attr', formatAttributeFilter(f));
		}
		const newUrl = `${window.location.pathname}?${urlParams.toString()}`;
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		goto(newUrl, { replaceState: !pushToHistory, noScroll: true, keepFocus: true });
	}

	function removeAttributeFilter(index: number) {
		attributeFilters = attributeFilters.filter((_, i) => i !== index);
		page = 1;
		loadData(true);
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

		updateLogsUrl(pushToHistory);

		try {
			const requestBody = {
				fromDate: getFromDateTimeUTC(),
				toDate: getToDateTimeUTC(),
				orderBy: sortField,
				sortDirection,
				pagination: { page, pageSize },
				search: searchQuery.trim(),
				searchType,
				minSeverity,
				serviceName: serviceName.trim(),
				traceId: traceIdFilter.trim(),
				attributeFilters
			};

			const response = (await api.post('/logs', requestBody, {
				projectId: projectsState.currentProjectId ?? undefined
			})) as { data: LogRecord[]; pagination: { total: number; totalPages: number } };

			logs = response.data || [];
			total = response.pagination.total;
			totalPages = response.pagination.totalPages;
		} catch (e: any) {
			console.error(e);
			error = e.message || 'Failed to load logs';
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

	function handleSeverityChange(value: string) {
		// Keep severity as pending query state - applied on the next Go press,
		// same as the search input. Avoids triggering a re-fetch on every
		// dropdown change.
		minSeverity = Number(value) || 0;
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
		const urlParams = parseLogsUrlParams();
		const range = getResolvedTimeRange(urlParams, timezone);
		selectedPreset = urlParams.preset;
		fromDate = dateToCalendarDate(range.from, timezone);
		toDate = dateToCalendarDate(range.to, timezone);
		fromTime = dateToTimeString(range.from, timezone);
		toTime = dateToTimeString(range.to, timezone);
		searchQuery = urlParams.search;
		searchType = urlParams.searchType;
		minSeverity = urlParams.minSeverity;
		serviceName = urlParams.serviceName;
		traceIdFilter = urlParams.traceId;
		attributeFilters = urlParams.attributeFilters;
		page = 1;
		loadData(false);
	}

	function toggleExpanded(id: string) {
		expandedId = expandedId === id ? null : id;
	}

	function firstLine(body: string): string {
		if (!body) return '';
		const nl = body.indexOf('\n');
		return nl === -1 ? body : body.slice(0, nl);
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
	<div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
		<h2 class="text-3xl font-semibold tracking-tight">Logs</h2>
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

	<div class="flex flex-col gap-3 sm:flex-row sm:items-start">
		<div class="flex-1">
			<SearchBar
				placeholder="Search logs..."
				bind:value={searchQuery}
				bind:typeValue={searchType}
				typeOptions={searchTypeOptions}
				onSearch={handleSearch}
				disabled={loading}
			>
				<Select.Root
					type="single"
					value={String(minSeverity)}
					onValueChange={handleSeverityChange}
				>
					<Select.Trigger class="h-9 w-[130px] rounded-none border-r-0 shadow-none">
						{severityTriggerLabel}
					</Select.Trigger>
					<Select.Content>
						{#each severityOptions as opt (opt.value)}
							<Select.Item value={opt.value} label={opt.label}>{opt.label}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</SearchBar>
		</div>
	</div>

	<div class="flex flex-wrap items-center gap-2">
		<button
			type="button"
			class="inline-flex items-center gap-1 rounded-full border border-dashed px-3 py-0.5 text-xs font-medium text-muted-foreground transition-colors hover:border-foreground/40 hover:text-foreground"
			onclick={openAddFilterDialog}
			disabled={loading}
		>
			<Plus class="h-3 w-3" />
			Add filter
		</button>
		{#each attributeFilters as f, i (i)}
			<span
				class="inline-flex items-center gap-1 rounded-full bg-muted px-2 py-0.5 text-xs font-mono"
			>
				<span class="text-muted-foreground">{f.scope}.{f.key}</span>
				<span>=</span>
				<span>{f.value}</span>
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
				<AlertDialog.Title>Add attribute filter</AlertDialog.Title>
				<AlertDialog.Description>
					Attribute keys must start with <code class="font-mono">resource.</code>,
					<code class="font-mono">scope.</code>, or <code class="font-mono">log.</code>.
				</AlertDialog.Description>
			</AlertDialog.Header>
			<div class="flex flex-col gap-3">
				<label class="flex flex-col gap-1 text-sm">
					<span class="font-medium">Attribute key</span>
					<Input
						placeholder="resource.service.name"
						bind:value={dialogKey}
						onkeydown={handleDialogKeydown}
					/>
				</label>
				<label class="flex flex-col gap-1 text-sm">
					<span class="font-medium">Value</span>
					<Input
						placeholder="backend-service"
						bind:value={dialogValue}
						onkeydown={handleDialogKeydown}
					/>
				</label>
				{#if dialogError}
					<p class="text-xs text-red-500">{dialogError}</p>
				{/if}
			</div>
			<AlertDialog.Footer>
				<Button variant="outline" onclick={() => (addFilterOpen = false)}>Cancel</Button>
				<Button onclick={submitDialogFilter}>
					<Plus class="h-4 w-4" /> Add filter
				</Button>
			</AlertDialog.Footer>
		</AlertDialog.Content>
	</AlertDialog.Root>

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
			{:else if logs.length === 0}
				<Table.Body>
					<TableEmptyState colspan={5} message="No logs found." />
				</Table.Body>
			{:else}
				<Table.Header>
					<Table.Row>
						<TracewayTableHeader
							label="Timestamp"
							tooltip="Log event time"
							class="w-[180px]"
							sortField="timestamp"
							currentSortField={sortField}
							{sortDirection}
							onSort={handleSort}
						/>
						<TracewayTableHeader
							label="Level"
							tooltip="Log severity"
							class="w-[90px]"
							sortField="severity_number"
							currentSortField={sortField}
							{sortDirection}
							onSort={handleSort}
						/>
						<TracewayTableHeader label="Message" tooltip="Log body (click row to expand)" />
						<TracewayTableHeader
							label="Service"
							tooltip="Emitting service"
							class="w-[160px]"
							sortField="service_name"
							currentSortField={sortField}
							{sortDirection}
							onSort={handleSort}
						/>
						<TracewayTableHeader
							label="Trace"
							tooltip="Linked trace ID (if present)"
							class="w-[110px]"
						/>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each logs as log (log.id)}
						<Table.Row
							class="group cursor-pointer hover:bg-muted/50"
							onclick={() => toggleExpanded(log.id)}
						>
							<Table.Cell class="text-muted-foreground tabular-nums">
								{formatDateTime(log.timestamp, { timezone })}
							</Table.Cell>
							<Table.Cell>
								<SeverityBadge
									severityText={log.severityText}
									severityNumber={log.severityNumber}
								/>
							</Table.Cell>
							<Table.Cell class="max-w-[600px] truncate font-mono text-sm">
								{firstLine(log.body)}
							</Table.Cell>
							<Table.Cell class="truncate text-muted-foreground">
								{log.serviceName || '—'}
							</Table.Cell>
							<Table.Cell class="font-mono text-xs">
								{#if log.traceId}
									<span class="text-muted-foreground" title={log.traceId}>
										{log.traceId.slice(0, 8)}…
									</span>
								{:else}
									<span class="text-muted-foreground">—</span>
								{/if}
							</Table.Cell>
						</Table.Row>
						{#if expandedId === log.id}
							<ExpandedLogRow {log} colspan={5} />
						{/if}
					{/each}
				</Table.Body>
			{/if}
		</Table.Root>
	</div>

	<PaginationFooter
		currentPage={page}
		{totalPages}
		{pageSize}
		totalItems={total}
		onPageChange={handlePageChange}
		onPageSizeChange={handlePageSizeChange}
		{loading}
		itemLabel="log"
	/>
</div>
