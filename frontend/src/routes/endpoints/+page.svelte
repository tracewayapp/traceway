<script lang="ts">
    import { onMount, onDestroy } from 'svelte';
    import { api } from '$lib/api';
    import { formatDuration, toUTCISO, calendarDateTimeToLuxon } from '$lib/utils/formatters';
    import { getTimezone } from '$lib/state/timezone.svelte';
    import * as Table from "$lib/components/ui/table";
    import * as Card from "$lib/components/ui/card";
    import { Button } from "$lib/components/ui/button";
    import { LoadingCircle } from "$lib/components/ui/loading-circle";
    import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
    import { ChevronDown, Check } from 'lucide-svelte';
    import { TracewayTableHeader } from "$lib/components/ui/traceway-table-header";
    import { ImpactBadge } from "$lib/components/ui/impact-badge";
    import { TableEmptyState } from "$lib/components/ui/table-empty-state";
    import { PaginationFooter } from "$lib/components/ui/pagination-footer";
    import { TimeRangePicker } from "$lib/components/ui/time-range-picker";
    import { CalendarDate } from "@internationalized/date";
    import { projectsState } from '$lib/state/projects.svelte';
    import { createRowClickHandler } from '$lib/utils/navigation';
    import { resolve } from '$app/paths';
    import PageHeader from '$lib/components/issues/page-header.svelte';
    import D3StackedAreaChart from '$lib/components/dashboard/d3-stacked-area-chart.svelte';
    import D3HorizontalBarChart from '$lib/components/dashboard/d3-horizontal-bar-chart.svelte';
    import {
        presetMinutes,
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
	import { ChartLine } from '@lucide/svelte';

    const timezone = $derived(getTimezone());

    type EndpointStats = {
        endpoint: string;
        count: number;
        p50Duration: number;
        p95Duration: number;
        p99Duration: number;
        avgDuration: number;
        lastSeen: string;
        impact: number; // 0-1 Apdex-based impact score from backend
    };

    type SortField = 'count' | 'p50_duration' | 'p95_duration' | 'p99_duration' | 'last_seen' | 'impact';

    type ChartDataPoint = {
        timestamp: Date;
        endpoint: string;
        value: number;
    };

    type ChartResponse = {
        endpoints: string[];
        series: { timestamp: string; endpoint: string; value: number }[];
    };

    // Metric options for the chart
    const metricOptions = [
        { value: 'total_time', label: 'Total time' },
        { value: 'p50', label: 'Average response time' },
        { value: 'p95', label: 'Slow response time' },
        { value: 'p99', label: 'Awful response time' }
    ];

    let endpoints = $state<EndpointStats[]>([]);
    let loading = $state(true);
    let error = $state('');

    // Chart state
    let chartEndpoints = $state<string[]>([]);
    let chartSeries = $state<ChartDataPoint[]>([]);
    let chartLoading = $state(true);
    let selectedMetric = $state('total_time');

    // Pagination State
    let page = $state(1);
    let pageSize = $state(100);
    let total = $state(0);
    let totalPages = $state(0);

    // Initialize from URL
    const initialUrlParams = parseTimeRangeFromUrl(timezone);
    const initialRange = getResolvedTimeRange(initialUrlParams, timezone);

    // Date Range State
    let selectedPreset = $state<string | null>(initialUrlParams.preset);
    let fromDate = $state<CalendarDate>(dateToCalendarDate(initialRange.from));
    let toDate = $state<CalendarDate>(dateToCalendarDate(initialRange.to));
    let fromTime = $state(dateToTimeString(initialRange.from));
    let toTime = $state(dateToTimeString(initialRange.to));

    // Update URL with current time range
    function updateTimeRangeUrl(pushToHistory = true) {
        updateUrl(
            selectedPreset
                ? { preset: selectedPreset }
                : { from: getFromDateTimeUTC(), to: getToDateTimeUTC() },
            { pushToHistory }
        );
    }

    // Handle browser back/forward navigation
    function handlePopState() {
        const urlParams = parseTimeRangeFromUrl(timezone);
        const range = getResolvedTimeRange(urlParams, timezone);

        selectedPreset = urlParams.preset;
        fromDate = dateToCalendarDate(range.from);
        fromTime = dateToTimeString(range.from);
        toDate = dateToCalendarDate(range.to);
        toTime = dateToTimeString(range.to);

        page = 1;
        loadData(false);
    }

    // Sorting - persisted to localStorage
    const SORT_STORAGE_KEY = 'endpoints';
    const initialSort = getSortState(SORT_STORAGE_KEY, { field: 'impact', direction: 'desc' });
    let orderBy = $state<SortField>(initialSort.field as SortField);
    let sortDirection = $state<SortDirection>(initialSort.direction);

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
        loadData(false);
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

    // Calculate interval based on time range
    function calculateInterval(from: Date, to: Date): number {
        const diffMs = to.getTime() - from.getTime();
        const diffHours = diffMs / (1000 * 60 * 60);

        if (diffHours <= 1) return 1; // 1 minute
        if (diffHours <= 6) return 5; // 5 minutes
        if (diffHours <= 24 * 7) return 60; // 1 hour
        return 240; // 4 hours
    }

    async function loadChartData() {
        chartLoading = true;

        try {
            const fromStr = getFromDateTimeUTC();
            const toStr = getToDateTimeUTC();
            const interval = calculateInterval(new Date(fromStr), new Date(toStr));

            const response = await api.post('/endpoints/chart', {
                fromDate: fromStr,
                toDate: toStr,
                metricType: selectedMetric,
                intervalMinutes: interval
            }, { projectId: projectsState.currentProjectId ?? undefined }) as ChartResponse;

            chartEndpoints = response.endpoints || [];
            chartSeries = (response.series || []).map((s: { timestamp: string; endpoint: string; value: number }) => ({
                timestamp: new Date(s.timestamp),
                endpoint: s.endpoint,
                value: s.value
            }));
        } catch (e: any) {
            console.error('Failed to load chart data:', e);
            chartEndpoints = [];
            chartSeries = [];
        } finally {
            chartLoading = false;
        }
    }

    async function loadData(pushToHistory = true) {
        loading = true;
        error = '';

        // Update URL
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

            const response = await api.post('/endpoints/grouped', requestBody, { projectId: projectsState.currentProjectId ?? undefined });

            endpoints = response.data || [];
            total = response.pagination.total;
            totalPages = response.pagination.totalPages;
        } catch (e: any) {
            console.error(e);
            error = e.message || 'Failed to load data';
        } finally {
            loading = false;
        }

        // Also load chart data
        loadChartData();
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
        const newSort = handleSortClick(field, orderBy, sortDirection);
        orderBy = newSort.field as SortField;
        sortDirection = newSort.direction;
        setSortState(SORT_STORAGE_KEY, newSort);
        page = 1;
        loadData(false);
    }

    function handleMetricChange(value: string) {
        selectedMetric = value;
        loadChartData();
    }

    function handleChartRangeSelect(from: Date, to: Date) {
        selectedPreset = null;
        fromDate = new CalendarDate(from.getFullYear(), from.getMonth() + 1, from.getDate());
        fromTime = `${String(from.getHours()).padStart(2, '0')}:${String(from.getMinutes()).padStart(2, '0')}`;
        toDate = new CalendarDate(to.getFullYear(), to.getMonth() + 1, to.getDate());
        toTime = `${String(to.getHours()).padStart(2, '0')}:${String(to.getMinutes()).padStart(2, '0')}`;
        page = 1;
        loadData(true);
    }

    const selectedMetricLabel = $derived(metricOptions.find(o => o.value === selectedMetric)?.label ?? 'Total time');

    // Format total time (input is milliseconds, not nanoseconds)
    function formatTotalTime(ms: number): string {
        if (ms >= 3600000) { // >= 1 hour
            return `${(ms / 3600000).toFixed(1)}h`;
        } else if (ms >= 60000) { // >= 1 minute
            return `${(ms / 60000).toFixed(1)}m`;
        } else if (ms >= 1000) { // >= 1 second
            return `${(ms / 1000).toFixed(1)}s`;
        } else if (ms < 1) {
            return `${(ms * 1000).toFixed(0)}µs`;
        }
        return `${Math.round(ms)}ms`;
    }

    // Check if we should use bar chart (for percentile metrics)
    const useBarChart = $derived(selectedMetric !== 'total_time');

    // Get top 8 endpoints for bar chart based on selected metric
    // Sort by the selected metric to show the worst (slowest) endpoints first
    const barChartData = $derived.by(() => {
        if (!useBarChart || endpoints.length === 0) return [];

        const metricKey = selectedMetric === 'p50' ? 'p50Duration'
            : selectedMetric === 'p95' ? 'p95Duration'
            : 'p99Duration';

        return [...endpoints]
            .sort((a, b) => b[metricKey] - a[metricKey])
            .slice(0, 8)
            .map(ep => ({
                endpoint: ep.endpoint,
                value: ep[metricKey]
            }));
    });


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
     <div class="flex flex-col gap-4 sm:flex-row sm:justify-between">

        <PageHeader title="Endpoints" />

        <div class="flex flex-col">
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

    <!-- Performance Chart -->
    <Card.Root class="pt-2">
        <Card.Header class="flex flex-row items-center justify-between space-y-0 border-b [.border-b]:pb-2 pl-3">
            <Card.Title class="text-base font-medium flex items-center gap-2">
                <ChartLine class="h-3.5 w-3.5 text-muted-foreground" />
                <DropdownMenu.Root>
                    <DropdownMenu.Trigger class="flex items-center text-sm gap-0.5 font-[400] text-muted-foreground">
                        {selectedMetricLabel}
                        <ChevronDown class="h-3 w-3 text-muted-foreground" />
                    </DropdownMenu.Trigger>
                    <DropdownMenu.Content align="start" class="w-[200px]">
                        {#each metricOptions as option}
                            <DropdownMenu.Item
                                onclick={() => handleMetricChange(option.value)}
                                class="flex items-center justify-between cursor-pointer"
                            >
                                <span>{option.label}</span>
                                {#if option.value === selectedMetric}
                                    <Check class="h-4 w-4" />
                                {/if}
                            </DropdownMenu.Item>
                        {/each}
                    </DropdownMenu.Content>
                </DropdownMenu.Root>
            </Card.Title>
        </Card.Header>
        <Card.Content>
            {#if chartLoading && !useBarChart}
                <div class="flex justify-center items-center h-[220px]">
                    <LoadingCircle size="xlg" />
                </div>
            {:else if useBarChart}
                {#if loading}
                    <div class="flex justify-center items-center h-[220px]">
                        <LoadingCircle size="xlg" />
                    </div>
                {:else}
                    <D3HorizontalBarChart
                        data={barChartData}
                        formatValue={formatDuration}
                    />
                {/if}
            {:else}
                <D3StackedAreaChart
                    endpoints={chartEndpoints}
                    series={chartSeries}
                    formatValue={formatTotalTime}
                    onRangeSelect={handleChartRangeSelect}
                />
            {/if}
        </Card.Content>
    </Card.Root>

    <!-- Endpoints Table -->
    <div class="rounded-md border overflow-hidden">
        <Table.Root>
            {#if loading}
            <Table.Body>
                <Table.Row>
                    <Table.Cell colspan={6} class="h-48">
                        <div class="flex justify-center items-center h-full">
                            <LoadingCircle size="xlg" />
                        </div>
                    </Table.Cell>
                </Table.Row>
            </Table.Body>
            {:else if error}
            <Table.Body>
                <Table.Row>
                    <Table.Cell colspan={6} class="h-24 text-center text-red-500">
                        {error}
                    </Table.Cell>
                </Table.Row>
            </Table.Body>
            {:else if endpoints.length === 0}
            <Table.Body>
                <TableEmptyState colspan={6} message="No endpoint data available for your search parameters" />
            </Table.Body>
            {:else}
            <Table.Header>
                <Table.Row>
                    <TracewayTableHeader
                        label="Endpoint"
                        tooltip="The API route or page being accessed"
                        class="max-w-[50%]"
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
                        label="Awful"
                        tooltip="99th percentile - slowest 1% of requests"
                        sortField="p99_duration"
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
                    <Table.Row
                        class="cursor-pointer hover:bg-muted/50"
                        onclick={createRowClickHandler(resolve(`/endpoints/${encodeURIComponent(endpoint.endpoint)}`), 'preset', 'from', 'to')}
                    >
                        <Table.Cell class="font-mono text-sm max-w-[50%] break-all whitespace-normal">
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
                        <Table.Cell class="font-mono text-sm tabular-nums">
                            {formatDuration(endpoint.p99Duration)}
                        </Table.Cell>
                        <Table.Cell class="text-right">
                            <ImpactBadge score={endpoint.impact} />
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
