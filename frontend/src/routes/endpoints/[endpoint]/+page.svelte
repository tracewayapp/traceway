<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { api } from '$lib/api';
	import {
		formatDuration,
		formatDurationMs,
		getStatusColor,
		formatDateTime,
		parseISO,
		toUTCISO,
		calendarDateTimeToLuxon
	} from '$lib/utils/formatters';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import * as Table from '$lib/components/ui/table';
	import { Button } from '$lib/components/ui/button';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { TimeRangePicker } from '$lib/components/ui/time-range-picker';
	import { ArrowLeft, Snail } from '@lucide/svelte';
	import { TracewayTableHeader } from '$lib/components/ui/traceway-table-header';
	import { TableEmptyState } from '$lib/components/ui/table-empty-state';
	import { CalendarDate } from '@internationalized/date';
	import { ErrorDisplay } from '$lib/components/ui/error-display';
	import { projectsState } from '$lib/state/projects.svelte';
	import AttributesDisplay from '$lib/components/attributes-display.svelte';
	import { createRowClickHandler } from '$lib/utils/navigation';
	import { goto } from '$app/navigation';
	import PaginationFooter from '$lib/components/ui/pagination-footer/pagination-footer.svelte';
	import PageHeader from '$lib/components/issues/page-header.svelte';
	import { Input } from '$lib/components/ui/input';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { toast } from 'svelte-sonner';
	import { resolve } from '$app/paths';
	import {
		presetMinutes,
		getTimeRangeFromPreset,
		parseTimeRangeFromUrl,
		getResolvedTimeRange,
		dateToCalendarDate,
		dateToTimeString,
		updateUrl
	} from '$lib/utils/url-params';
	import {
		getSortState,
		setSortState,
		handleSortClick,
		type SortDirection
	} from '$lib/utils/sort-storage';

	const timezone = $derived(getTimezone());

	type EndpointInstance = {
		id: string;
		endpoint: string;
		duration: number;
		recordedAt: string;
		statusCode: number;
		bodySize: number;
		clientIP: string;
		attributes: Record<string, string> | null;
		serverName: string;
		appVersion: string;
	};

	type EndpointStats = {
		count: number;
		avgDuration: number;
		medianDuration: number;
		p95Duration: number;
		p99Duration: number;
		apdex: number;
		errorRate: number;
		throughput: number;
	};

	type SortField = 'recorded_at' | 'duration' | 'status_code' | 'body_size';

	let { data } = $props();

	let transactions = $state<EndpointInstance[]>([]);
	let stats = $state<EndpointStats | null>(null);
	let loading = $state(true);
	let error = $state('');
	let notFound = $state(false);
	let errorStatus = $state<number>(0);

	// Pagination State
	let page = $state(1);
	let pageSize = $state(50);
	let total = $state(0);
	let totalPages = $state(0);

	// Initialize from URL params (from page data)
	function getInitialRange(): { preset: string | null; from: Date; to: Date } {
		// If preset is provided, use it
		if (data.preset && presetMinutes[data.preset]) {
			const range = getTimeRangeFromPreset(data.preset, timezone);
			return { preset: data.preset, from: range.from, to: range.to };
		}

		// If custom from/to provided
		if (data.from && data.to) {
			const fromDt = parseISO(data.from, timezone);
			const toDt = parseISO(data.to, timezone);
			if (fromDt.isValid && toDt.isValid) {
				return { preset: null, from: fromDt.toJSDate(), to: toDt.toJSDate() };
			}
		}

		// Default to 24h preset
		const range = getTimeRangeFromPreset('24h', timezone);
		return { preset: '24h', from: range.from, to: range.to };
	}

	const initialRange = getInitialRange();

	// Date Range State
	let selectedPreset = $state<string | null>(initialRange.preset);
	let fromDate = $state<CalendarDate>(dateToCalendarDate(initialRange.from, timezone));
	let toDate = $state<CalendarDate>(dateToCalendarDate(initialRange.to, timezone));
	let fromTime = $state(dateToTimeString(initialRange.from, timezone));
	let toTime = $state(dateToTimeString(initialRange.to, timezone));

	function updateTimeRangeUrl(pushToHistory = true) {
		updateUrl(
			selectedPreset
				? { preset: selectedPreset }
				: { from: getFromDateTimeUTC(), to: getToDateTimeUTC() },
			{ pushToHistory }
		);
	}

	// Sorting State - persisted to localStorage
	const SORT_STORAGE_KEY = 'endpoint_detail';
	const initialSort = getSortState(SORT_STORAGE_KEY, { field: 'recorded_at', direction: 'desc' });
	let orderBy = $state<SortField>(initialSort.field as SortField);
	let sortDirection = $state<SortDirection>(initialSort.direction);

	// Slow endpoint state
	let offsetMs = $state<number>(0);
	let reason = $state('');
	let showSlowDialog = $state(false);
	let slowLoading = $state(false);
	let offsetInput = $state('');
	let reasonInput = $state('');

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
		);
		return toUTCISO(dt);
	}

	function goBackToEndpoints(event: MouseEvent) {
		const params = new URLSearchParams();
		if (selectedPreset) {
			params.set('preset', selectedPreset);
		} else {
			params.set('from', getFromDateTimeUTC());
			params.set('to', getToDateTimeUTC());
		}
		const href = resolve('/endpoints') + '?' + params.toString();
		if (event.ctrlKey || event.metaKey) {
			window.open(href, '_blank');
		} else {
			goto(href);
		}
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
		loadData();
	}

	function formatBytes(bytes: number): string {
		if (bytes < 1024) {
			return `${bytes} B`;
		} else if (bytes < 1024 * 1024) {
			return `${(bytes / 1024).toFixed(1)} KB`;
		} else {
			return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
		}
	}

	async function loadData(pushToHistory = true) {
		loading = true;
		error = '';
		notFound = false;
		errorStatus = 0;

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
				}
			};

			const response = await api.post(
				`/endpoints/endpoint?endpoint=${encodeURIComponent(data.endpoint)}`,
				requestBody,
				{ projectId: projectsState.currentProjectId ?? undefined }
			);

			transactions = response.data || [];
			stats = response.stats || null;
			total = response.pagination.total;
			totalPages = response.pagination.totalPages;
		} catch (e: any) {
			console.error(e);
			errorStatus = e.status || 0;
			if (e.status === 404) {
				notFound = true;
			} else {
				error = e.message || 'Failed to load data';
			}
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

	async function loadSlowEndpoint() {
		try {
			const response = await api.get(
				`/endpoints/slow?endpoint=${encodeURIComponent(data.endpoint)}`,
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			offsetMs = response.offsetMs ?? 0;
			reason = response.reason ?? '';
		} catch {
			offsetMs = 0;
			reason = '';
		}
	}

	async function saveSlowEndpoint() {
		slowLoading = true;
		try {
			const value = parseInt(offsetInput) || 0;
			await api.post(
				'/endpoints/slow',
				{ endpoint: decodeURIComponent(data.endpoint), offsetMs: Math.max(0, value), reason: reasonInput },
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			offsetMs = Math.max(0, value);
			reason = reasonInput;
			showSlowDialog = false;
			toast.success('Expected performance updated');
		} catch (e: any) {
			toast.error(e.message || 'Failed to save');
		} finally {
			slowLoading = false;
		}
	}

	function handlePopState() {
		const urlParams = parseTimeRangeFromUrl(timezone);
		const range = getResolvedTimeRange(urlParams, timezone);
		selectedPreset = urlParams.preset;
		fromDate = dateToCalendarDate(range.from, timezone);
		toDate = dateToCalendarDate(range.to, timezone);
		fromTime = dateToTimeString(range.from, timezone);
		toTime = dateToTimeString(range.to, timezone);
		page = 1;
		loadData(false);
	}

	onMount(() => {
		window.addEventListener('popstate', handlePopState);
		loadData(false);
		loadSlowEndpoint();
	});

	onDestroy(() => {
		if (typeof window !== 'undefined') {
			window.removeEventListener('popstate', handlePopState);
		}
	});
</script>

<div class="space-y-6">
	{#if notFound}
		<ErrorDisplay
			status={404}
			title="Endpoint Not Found"
			description="The endpoint you're looking for doesn't exist or has no recorded traces."
			onBack={goBackToEndpoints}
			backLabel="Back to Endpoints"
			onRetry={() => loadData(false)}
			identifier={decodeURIComponent(data.endpoint)}
		/>
	{:else if error && !loading}
		<ErrorDisplay
			status={errorStatus === 400 ? 400 : errorStatus === 422 ? 422 : 400}
			title="Failed to Load Traces"
			description={error}
			onBack={goBackToEndpoints}
			backLabel="Back to Endpoints"
			onRetry={() => loadData(false)}
		/>
	{:else}
		<!-- Header with Title and Time Range Filter -->
		<div class="flex flex-col gap-4 sm:flex-row sm:justify-between">
			<PageHeader
				title={decodeURIComponent(data.endpoint)}
				subtitle="Trace instances for this endpoint"
				onBack={goBackToEndpoints}
			/>

			<div class="flex items-start gap-2">
				<Button variant="outline" size="sm" onclick={() => { offsetInput = offsetMs > 0 ? String(offsetMs) : ''; reasonInput = reason; showSlowDialog = true; }}>
					<Snail class="h-4 w-4" />
					{offsetMs > 0 ? `+${offsetMs}ms offset` : 'Expected Performance'}
				</Button>
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

		{#if offsetMs > 0}
			<div class="flex items-center gap-2 rounded-md border border-border bg-muted/50 px-4 py-3 text-sm">
				<Snail class="h-4 w-4 shrink-0 text-muted-foreground" />
				<span>
					<span class="font-medium">Expected Performance:</span> +{offsetMs}ms offset applied.
					{#if reason}
						{reason}
					{/if}
				</span>
			</div>
		{/if}

		<!-- Endpoint Stats -->
		{#if stats}
			<div class="grid grid-cols-2 gap-4 md:grid-cols-4 lg:grid-cols-7">
				<div class="space-y-1">
					<p class="text-2xl font-semibold tracking-tight">{formatDurationMs(stats.avgDuration)}</p>
					<p class="text-xs text-muted-foreground">Average response time</p>
				</div>
				<div class="space-y-1">
					<p class="text-2xl font-semibold tracking-tight">
						{formatDurationMs(stats.medianDuration)}
					</p>
					<p class="text-xs text-muted-foreground">Median response time</p>
				</div>
				<div class="space-y-1">
					<p class="text-2xl font-semibold tracking-tight">{formatDurationMs(stats.p95Duration)}</p>
					<p class="text-xs text-muted-foreground">95th percentile response time</p>
				</div>
				<div class="space-y-1">
					<p class="text-2xl font-semibold tracking-tight">{formatDurationMs(stats.p99Duration)}</p>
					<p class="text-xs text-muted-foreground">99th percentile response time</p>
				</div>
				<div class="space-y-1">
					<p class="text-2xl font-semibold tracking-tight">{stats.apdex.toFixed(2)}</p>
					<p class="text-xs text-muted-foreground">Apdex score</p>
				</div>
				<div class="space-y-1">
					<p class="text-2xl font-semibold tracking-tight">{stats.errorRate.toFixed(2)} %</p>
					<p class="text-xs text-muted-foreground">Average error rate</p>
				</div>
				<div class="space-y-1">
					<p class="text-2xl font-semibold tracking-tight">{stats.throughput.toFixed(0)} rpm</p>
					<p class="text-xs text-muted-foreground">Average throughput</p>
				</div>
			</div>
		{:else if loading}
			<div class="flex items-center justify-center py-8">
				<LoadingCircle size="lg" />
			</div>
		{/if}

		<!-- Traces Table -->
		<div class="overflow-hidden rounded-md border">
			<Table.Root>
				{#if loading || transactions.length > 0}
					<Table.Header>
						<Table.Row>
							<TracewayTableHeader
								label="Recorded At"
								sortField="recorded_at"
								currentSortField={orderBy}
								{sortDirection}
								onSort={(field) => handleSort(field as SortField)}
								class="w-[180px]"
							/>
							<TracewayTableHeader
								label="Duration"
								sortField="duration"
								currentSortField={orderBy}
								{sortDirection}
								onSort={(field) => handleSort(field as SortField)}
								class="w-[120px]"
							/>
							<TracewayTableHeader
								label="Status"
								sortField="status_code"
								currentSortField={orderBy}
								{sortDirection}
								onSort={(field) => handleSort(field as SortField)}
								class="w-[100px]"
							/>
							<TracewayTableHeader
								label="Body Size"
								sortField="body_size"
								currentSortField={orderBy}
								{sortDirection}
								onSort={(field) => handleSort(field as SortField)}
								class="w-[100px]"
							/>
							<TracewayTableHeader label="Client IP" class="w-[140px]" />
							<TracewayTableHeader label="Server" class="w-[120px]" />
							<TracewayTableHeader label="Version" class="w-[100px]" />
							<TracewayTableHeader label="Attributes" />
						</Table.Row>
					</Table.Header>
				{/if}
				<Table.Body>
					{#if loading}
						<Table.Row>
							<Table.Cell colspan={8} class="h-48">
								<div class="flex items-center justify-center">
									<LoadingCircle size="lg" />
								</div>
							</Table.Cell>
						</Table.Row>
					{:else if transactions.length === 0}
						<TableEmptyState colspan={8} message="No traces found in this time range." />
					{:else}
						{#each transactions as transaction}
							<Table.Row
								class="cursor-pointer hover:bg-muted/50"
								onclick={createRowClickHandler(
									`/endpoints/${encodeURIComponent(decodeURIComponent(data.endpoint))}/${transaction.id}`,
									'preset',
									'from',
									'to'
								)}
							>
								<Table.Cell class="text-muted-foreground">
									{formatDateTime(transaction.recordedAt, { timezone })}
								</Table.Cell>
								<Table.Cell class="font-mono text-sm">
									{formatDuration(transaction.duration)}
								</Table.Cell>
								<Table.Cell class="font-mono text-sm {getStatusColor(transaction.statusCode)}">
									{transaction.statusCode}
								</Table.Cell>
								<Table.Cell class="font-mono text-sm">
									{formatBytes(transaction.bodySize)}
								</Table.Cell>
								<Table.Cell class="font-mono text-sm text-muted-foreground">
									{transaction.clientIP}
								</Table.Cell>
								<Table.Cell class="font-mono text-sm text-muted-foreground">
									{transaction.serverName || '-'}
								</Table.Cell>
								<Table.Cell class="font-mono text-sm text-muted-foreground">
									{transaction.appVersion || '-'}
								</Table.Cell>
								<Table.Cell>
									<AttributesDisplay attributes={transaction.attributes} />
								</Table.Cell>
							</Table.Row>
						{/each}
					{/if}
				</Table.Body>
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
			itemLabel="endpoint"
		/>
	{/if}
</div>

<AlertDialog.Root bind:open={showSlowDialog}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Expected Performance</AlertDialog.Title>
			<AlertDialog.Description>
				Set a time offset for endpoints that are expected to be slow (e.g., report generation, data exports).
				The offset adjusts impact score thresholds so the endpoint isn't flagged as unhealthy.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<div class="space-y-4 py-4">
			<div>
				<label for="offset-input" class="text-sm font-medium">Offset (ms)</label>
				<Input id="offset-input" type="number" bind:value={offsetInput} placeholder="e.g., 2000" min="0" class="mt-1.5" />
				<p class="text-xs text-muted-foreground mt-1.5">
					How many extra milliseconds above the default 750ms threshold are acceptable for this endpoint.
					Set to 0 to remove the offset.
				</p>
			</div>
			<div>
				<label for="reason-input" class="text-sm font-medium">Reason</label>
				<Input id="reason-input" type="text" bind:value={reasonInput} placeholder="e.g., generates large PDF reports" class="mt-1.5" />
				<p class="text-xs text-muted-foreground mt-1.5">
					Explain why this endpoint is expected to be slow.
				</p>
			</div>
		</div>
		<AlertDialog.Footer>
			<Button variant="outline" onclick={() => showSlowDialog = false} disabled={slowLoading}>Cancel</Button>
			<Button onclick={saveSlowEndpoint} disabled={slowLoading}>
				{slowLoading ? 'Saving...' : 'Save'}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
