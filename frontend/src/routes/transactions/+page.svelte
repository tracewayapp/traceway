<script lang="ts">
    import { onMount, onDestroy } from 'svelte';
    import { browser } from '$app/environment';
    import { goto } from '$app/navigation';
    import { api } from '$lib/api';
    import { formatDuration, getNow, luxonToCalendarDateTime, parseISO, toUTCISO, calendarDateTimeToLuxon } from '$lib/utils/formatters';
    import { getTimezone } from '$lib/state/timezone.svelte';
    import * as Table from "$lib/components/ui/table";
    import { Button } from "$lib/components/ui/button";
    import { LoadingCircle } from "$lib/components/ui/loading-circle";
    import * as Select from "$lib/components/ui/select";
    import { TracewayTableHeader } from "$lib/components/ui/traceway-table-header";
    import { ImpactBadge } from "$lib/components/ui/impact-badge";
    import { TableEmptyState } from "$lib/components/ui/table-empty-state";
    import { PaginationFooter } from "$lib/components/ui/pagination-footer";
    import { TimeRangePicker } from "$lib/components/ui/time-range-picker";
    import { CalendarDate, getLocalTimeZone, today } from "@internationalized/date";
    import { projectsState } from '$lib/state/projects.svelte';
    import { createRowClickHandler } from '$lib/utils/navigation';
    import { resolve } from '$app/paths';

    const timezone = $derived(getTimezone());

    type EndpointStats = {
        endpoint: string;
        count: number;
        p50Duration: number;
        p95Duration: number;
        avgDuration: number;
        lastSeen: string;
    };

    type SortField = 'count' | 'p50_duration' | 'p95_duration' | 'last_seen' | 'impact';
    type SortDirection = 'asc' | 'desc';

    let endpoints = $state<EndpointStats[]>([]);
    let loading = $state(true);
    let error = $state('');

    // Pagination State
    let page = $state(1);
    let pageSize = $state(20);
    let total = $state(0);
    let totalPages = $state(0);

    // Preset definitions (must match TimeRangePicker)
    const presetMinutes: Record<string, number> = {
        '30m': 30,
        '60m': 60,
        '3h': 180,
        '6h': 360,
        '12h': 720,
        '24h': 1440,
        '3d': 4320,
        '7d': 10080,
        '1M': 43200,
        '3M': 129600,
    };

    // Helper functions
    function getTimeRangeFromPresetLocal(presetValue: string): { from: Date; to: Date } {
        const minutes = presetMinutes[presetValue] || 360;
        const now = getNow(timezone);
        const from = now.minus({ minutes });
        return { from: from.toJSDate(), to: now.toJSDate() };
    }

    function dateToCalendarDate(date: Date): CalendarDate {
        return new CalendarDate(date.getFullYear(), date.getMonth() + 1, date.getDate());
    }

    function dateToTimeString(date: Date): string {
        return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
    }

    // Parse URL params - supports preset OR from/to
    function parseUrlParams(): { preset: string | null; from: Date | null; to: Date | null } {
        if (!browser) return { preset: '6h', from: null, to: null };
        const params = new URLSearchParams(window.location.search);
        const presetParam = params.get('preset');
        const fromParam = params.get('from');
        const toParam = params.get('to');

        // If preset is specified, use it
        if (presetParam && presetMinutes[presetParam]) {
            return { preset: presetParam, from: null, to: null };
        }

        // If custom from/to specified
        if (fromParam && toParam) {
            const fromDt = parseISO(fromParam, timezone);
            const toDt = parseISO(toParam, timezone);
            if (fromDt.isValid && toDt.isValid) {
                return { preset: null, from: fromDt.toJSDate(), to: toDt.toJSDate() };
            }
        }

        // Default to 6h preset
        return { preset: '6h', from: null, to: null };
    }

    // Initialize from URL
    const initialUrlParams = parseUrlParams();
    const initialRange = initialUrlParams.preset
        ? getTimeRangeFromPresetLocal(initialUrlParams.preset)
        : { from: initialUrlParams.from!, to: initialUrlParams.to! };

    // Date Range State
    let selectedPreset = $state<string | null>(initialUrlParams.preset);
    let fromDate = $state<CalendarDate>(dateToCalendarDate(initialRange.from));
    let toDate = $state<CalendarDate>(dateToCalendarDate(initialRange.to));
    let fromTime = $state(dateToTimeString(initialRange.from));
    let toTime = $state(dateToTimeString(initialRange.to));

    // Update URL with current time range
    function updateUrl(pushToHistory = true) {
        if (!browser) return;

        const params = new URLSearchParams();

        if (selectedPreset) {
            params.set('preset', selectedPreset);
        } else {
            params.set('from', getFromDateTimeUTC());
            params.set('to', getToDateTimeUTC());
        }

        const newUrl = `${window.location.pathname}?${params.toString()}`;

        goto(newUrl, {
            replaceState: !pushToHistory,
            noScroll: true,
            keepFocus: true
        });
    }

    // Handle browser back/forward navigation
    function handlePopState() {
        const urlParams = parseUrlParams();

        if (urlParams.preset) {
            selectedPreset = urlParams.preset;
            const range = getTimeRangeFromPresetLocal(urlParams.preset);
            fromDate = dateToCalendarDate(range.from);
            fromTime = dateToTimeString(range.from);
            toDate = dateToCalendarDate(range.to);
            toTime = dateToTimeString(range.to);
        } else if (urlParams.from && urlParams.to) {
            selectedPreset = null;
            fromDate = dateToCalendarDate(urlParams.from);
            fromTime = dateToTimeString(urlParams.from);
            toDate = dateToCalendarDate(urlParams.to);
            toTime = dateToTimeString(urlParams.to);
        }

        page = 1;
        loadData(false);
    }

    // Sorting - default to impact descending
    let orderBy = $state<SortField>('impact');
    let sortDirection = $state<SortDirection>('desc');

    // Page size options
    const pageSizeOptions = [
        { value: "10", label: "10" },
        { value: "20", label: "20" },
        { value: "50", label: "50" },
        { value: "100", label: "100" }
    ];

    const pageSizeLabel = $derived(pageSizeOptions.find(o => o.value === pageSize.toString())?.label ?? pageSize.toString());

    // Combine date and time into UTC ISO datetime string
    function getFromDateTimeUTC(): string {
        const [hour, minute] = (fromTime || '00:00').split(':').map(Number);
        const dt = calendarDateTimeToLuxon({ year: fromDate.year, month: fromDate.month, day: fromDate.day, hour, minute }, timezone);
        return toUTCISO(dt);
    }

    function getToDateTimeUTC(): string {
        const [hour, minute] = (toTime || '23:59').split(':').map(Number);
        const dt = calendarDateTimeToLuxon({ year: toDate.year, month: toDate.month, day: toDate.day, hour, minute }, timezone);
        return toUTCISO(dt);
    }

    function handleTimeRangeChange(from: { date: CalendarDate; time: string }, to: { date: CalendarDate; time: string }, preset: string | null) {
        fromDate = from.date;
        fromTime = from.time;
        toDate = to.date;
        toTime = to.time;
        selectedPreset = preset;
        page = 1;
        loadData(true);
    }

    // Format count with k/m suffixes
    function formatCount(count: number): string {
        if (count >= 1_000_000) {
            return `${(count / 1_000_000).toFixed(1)}m`;
        } else if (count >= 1_000) {
            return `${(count / 1_000).toFixed(1)}k`;
        }
        return count.toLocaleString();
    }

    // Calculate impact level based on call volume and response time variance
    // Returns: 'critical' | 'high' | 'medium' | null (null = not significant)
    function getImpactLevel(count: number, p50: number, p95: number): 'critical' | 'high' | 'medium' | null {
        const varianceMs = (p95 - p50) / 1_000_000;
        const score = count * varianceMs;
        if (score > 100) return 'critical';
        if (score > 10) return 'high';
        if (score > 1) return 'medium';
        return null;
    }

    async function loadData(pushToHistory = true) {
        loading = true;
        error = '';

        // Update URL
        updateUrl(pushToHistory);

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

            const response = await api.post('/transactions/grouped', requestBody, { projectId: projectsState.currentProjectId ?? undefined });

            endpoints = response.data || [];
            total = response.pagination.total;
            totalPages = response.pagination.totalPages;
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
            loadData(false); // Don't push to history for pagination
        }
    }

    function handlePageSizeChange(newPageSize: number) {
        pageSize = newPageSize;
        page = 1;
        loadData(false); // Don't push to history for pagination
    }

    function handleSort(field: SortField) {
        if (orderBy === field) {
            // Toggle direction if clicking the same field
            sortDirection = sortDirection === 'desc' ? 'asc' : 'desc';
        } else {
            // New field, default to descending
            orderBy = field;
            sortDirection = 'desc';
        }
        page = 1;
        loadData(false); // Don't push to history for sorting
    }

    onMount(() => {
        // Add popstate listener for back/forward navigation
        window.addEventListener('popstate', handlePopState);

        // Initial load with replaceState (don't push to history)
        loadData(false);
    });

    onDestroy(() => {
        if (typeof window !== 'undefined') {
            window.removeEventListener('popstate', handlePopState);
        }
    });
</script>

<div class="space-y-4">
    <!-- Header with Title and Time Range Filter -->
    <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <h2 class="text-2xl font-bold tracking-tight">Transactions</h2>
        <TimeRangePicker
            bind:fromDate
            bind:toDate
            bind:fromTime
            bind:toTime
            bind:preset={selectedPreset}
            onApply={handleTimeRangeChange}
        />
    </div>

    <!-- Endpoints Table -->
    <div class="rounded-md border overflow-hidden">
        <Table.Root>
            {#if loading}
            <Table.Body>
                <Table.Row>
                    <Table.Cell colspan={5} class="h-48">
                        <div class="flex justify-center items-center h-full">
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
            {:else if endpoints.length === 0}
            <Table.Body>
                <TableEmptyState colspan={5} message="No transaction data received yet" />
            </Table.Body>
            {:else}
            <Table.Header>
                <Table.Row>
                    <TracewayTableHeader
                        label="Endpoint"
                        tooltip="The API route or page being accessed"
                    />
                    <TracewayTableHeader
                        label="Calls"
                        tooltip="Total number of requests"
                        sortField="count"
                        currentSortField={orderBy}
                        {sortDirection}
                        onSort={(field) => handleSort(field as SortField)}
                        class="w-[100px]"
                    />
                    <TracewayTableHeader
                        label="Typical"
                        tooltip="Median response time (P50)"
                        sortField="p50_duration"
                        currentSortField={orderBy}
                        {sortDirection}
                        onSort={(field) => handleSort(field as SortField)}
                        class="w-[100px]"
                    />
                    <TracewayTableHeader
                        label="Slow"
                        tooltip="95th percentile - slowest 5% of requests"
                        sortField="p95_duration"
                        currentSortField={orderBy}
                        {sortDirection}
                        onSort={(field) => handleSort(field as SortField)}
                        class="w-[100px]"
                    />
                    <TracewayTableHeader
                        label="Impact"
                        tooltip="Priority based on traffic × response time variance"
                        sortField="impact"
                        currentSortField={orderBy}
                        {sortDirection}
                        onSort={(field) => handleSort(field as SortField)}
                        align="right"
                        class="w-[120px]"
                    />
                </Table.Row>
            </Table.Header>
            <Table.Body>
                {#each endpoints as endpoint}
                    {@const impactLevel = getImpactLevel(endpoint.count, endpoint.p50Duration, endpoint.p95Duration)}
                    <Table.Row
                        class="cursor-pointer hover:bg-muted/50"
                        onclick={createRowClickHandler(resolve(`/transactions/${encodeURIComponent(endpoint.endpoint)}`), 'preset', 'from', 'to')}
                    >
                        <Table.Cell class="font-mono text-sm">
                            {endpoint.endpoint}
                        </Table.Cell>
                        <Table.Cell class="tabular-nums">
                            {formatCount(endpoint.count)}
                        </Table.Cell>
                        <Table.Cell class="font-mono text-sm tabular-nums">
                            {formatDuration(endpoint.p50Duration)}
                        </Table.Cell>
                        <Table.Cell class="font-mono text-sm tabular-nums">
                            {formatDuration(endpoint.p95Duration)}
                        </Table.Cell>
                        <Table.Cell class="text-right">
                            <ImpactBadge level={impactLevel} />
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
        itemLabel="endpoint"
    />
</div>
