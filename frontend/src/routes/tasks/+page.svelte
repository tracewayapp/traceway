<script lang="ts">
    import { onMount, onDestroy } from 'svelte';
    import { api } from '$lib/api';
    import { formatDuration, toUTCISO, calendarDateTimeToLuxon } from '$lib/utils/formatters';
    import { getTimezone } from '$lib/state/timezone.svelte';
    import * as Table from "$lib/components/ui/table";
    import { LoadingCircle } from "$lib/components/ui/loading-circle";
    import { TracewayTableHeader } from "$lib/components/ui/traceway-table-header";
    import { TableEmptyState } from "$lib/components/ui/table-empty-state";
    import { PaginationFooter } from "$lib/components/ui/pagination-footer";
    import { TimeRangePicker } from "$lib/components/ui/time-range-picker";
    import { Badge } from "$lib/components/ui/badge";
    import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
    import { ChevronDown, Check } from 'lucide-svelte';
    import { browser } from '$app/environment';
    import { CalendarDate } from "@internationalized/date";
    import { projectsState } from '$lib/state/projects.svelte';
    import { createRowClickHandler } from '$lib/utils/navigation';
    import { resolve } from '$app/paths';
    import PageHeader from '$lib/components/issues/page-header.svelte';
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

    const timezone = $derived(getTimezone());

    type TaskStats = {
        taskName: string;
        count: number;
        nonRootCount: number;
        p50Duration: number;
        p95Duration: number;
        avgDuration: number;
        lastSeen: string;
    };

    type SortField = 'count' | 'p50_duration' | 'p95_duration' | 'last_seen';

    type RootFilter = 'all' | 'root' | 'nonroot';

    const rootFilterOptions: { value: RootFilter; label: string }[] = [
        { value: 'all', label: 'All' },
        { value: 'root', label: 'Root only' },
        { value: 'nonroot', label: 'Non-root only' }
    ];

    let tasks = $state<TaskStats[]>([]);
    let loading = $state(true);
    let error = $state('');

    // Pagination State
    let page = $state(1);
    let pageSize = $state(50);
    let total = $state(0);
    let totalPages = $state(0);

    // Initialize from URL
    const initialUrlParams = parseTimeRangeFromUrl(timezone);
    const initialRange = getResolvedTimeRange(initialUrlParams, timezone);

    function parseRootFilterFromUrl(): RootFilter {
        if (!browser) return 'all';
        const raw = new URLSearchParams(window.location.search).get('root');
        return raw === 'root' || raw === 'nonroot' ? raw : 'all';
    }

    let rootFilter = $state<RootFilter>(parseRootFilterFromUrl());

    // Date Range State
    let selectedPreset = $state<string | null>(initialUrlParams.preset);
    let fromDate = $state<CalendarDate>(dateToCalendarDate(initialRange.from, timezone));
    let toDate = $state<CalendarDate>(dateToCalendarDate(initialRange.to, timezone));
    let fromTime = $state(dateToTimeString(initialRange.from, timezone));
    let toTime = $state(dateToTimeString(initialRange.to, timezone));

    // Update URL with current time range
    function updateTimeRangeUrl(pushToHistory = true) {
        const params: Record<string, string | null | undefined> = selectedPreset
            ? { preset: selectedPreset }
            : { from: getFromDateTimeUTC(), to: getToDateTimeUTC() };
        params.root = rootFilter === 'all' ? null : rootFilter;
        updateUrl(params, { pushToHistory });
    }

    // Handle browser back/forward navigation
    function handlePopState() {
        const urlParams = parseTimeRangeFromUrl(timezone);
        const range = getResolvedTimeRange(urlParams, timezone);

        selectedPreset = urlParams.preset;
        fromDate = dateToCalendarDate(range.from, timezone);
        fromTime = dateToTimeString(range.from, timezone);
        toDate = dateToCalendarDate(range.to, timezone);
        toTime = dateToTimeString(range.to, timezone);
        rootFilter = parseRootFilterFromUrl();

        page = 1;
        loadData(false);
    }

    // Sorting - persisted to localStorage
    const SORT_STORAGE_KEY = 'tasks';
    const initialSort = getSortState(SORT_STORAGE_KEY, { field: 'count', direction: 'desc' });
    let orderBy = $state<SortField>(initialSort.field as SortField);
    let sortDirection = $state<SortDirection>(initialSort.direction);

    // Combine date and time into UTC ISO datetime string
    function getFromDateTimeUTC(): string {
        const [hour, minute] = (fromTime || '00:00').split(':').map(Number);
        const dt = calendarDateTimeToLuxon({ year: fromDate.year, month: fromDate.month, day: fromDate.day, hour, minute }, timezone);
        return toUTCISO(dt);
    }

    function getToDateTimeUTC(): string {
        const [hour, minute] = (toTime || '23:59').split(':').map(Number);
        const dt = calendarDateTimeToLuxon({ year: toDate.year, month: toDate.month, day: toDate.day, hour, minute }, timezone).endOf('minute');
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

        // Update URL
        updateTimeRangeUrl(pushToHistory);

        try {
            const requestBody: Record<string, unknown> = {
                fromDate: getFromDateTimeUTC(),
                toDate: getToDateTimeUTC(),
                orderBy: orderBy,
                sortDirection: sortDirection,
                pagination: {
                    page: page,
                    pageSize: pageSize
                }
            };
            if (rootFilter === 'root') requestBody.isRoot = true;
            else if (rootFilter === 'nonroot') requestBody.isRoot = false;

            const response = await api.post('/tasks/grouped', requestBody, { projectId: projectsState.currentProjectId ?? undefined });

            tasks = response.data || [];
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
        const newSort = handleSortClick(field, orderBy, sortDirection);
        orderBy = newSort.field as SortField;
        sortDirection = newSort.direction;
        setSortState(SORT_STORAGE_KEY, newSort);
        page = 1;
        loadData(false);
    }

    function handleRootFilterChange(value: RootFilter) {
        rootFilter = value;
        page = 1;
        loadData(true);
    }

    const rootFilterLabel = $derived(rootFilterOptions.find(o => o.value === rootFilter)?.label ?? 'All');

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

        <PageHeader title="Tasks" />

        <div class="flex flex-col gap-2 sm:flex-row sm:items-center">
            <DropdownMenu.Root>
                <DropdownMenu.Trigger
                    class="flex items-center gap-1 rounded-md border border-input bg-background px-3 py-2 text-sm hover:bg-accent"
                >
                    <span class="text-muted-foreground">Roots:</span>
                    <span>{rootFilterLabel}</span>
                    <ChevronDown class="h-3 w-3 text-muted-foreground" />
                </DropdownMenu.Trigger>
                <DropdownMenu.Content align="end" class="w-[180px]">
                    {#each rootFilterOptions as option}
                        <DropdownMenu.Item
                            onclick={() => handleRootFilterChange(option.value)}
                            class="flex items-center justify-between cursor-pointer"
                        >
                            <span>{option.label}</span>
                            {#if option.value === rootFilter}
                                <Check class="h-4 w-4" />
                            {/if}
                        </DropdownMenu.Item>
                    {/each}
                </DropdownMenu.Content>
            </DropdownMenu.Root>
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

    <!-- Tasks Table -->
    <div class="rounded-md border overflow-hidden">
        <Table.Root>
            {#if loading}
            <Table.Body>
                <Table.Row>
                    <Table.Cell colspan={4} class="h-48">
                        <div class="flex justify-center items-center h-full">
                            <LoadingCircle size="xlg" />
                        </div>
                    </Table.Cell>
                </Table.Row>
            </Table.Body>
            {:else if error}
            <Table.Body>
                <Table.Row>
                    <Table.Cell colspan={4} class="h-24 text-center text-red-500">
                        {error}
                    </Table.Cell>
                </Table.Row>
            </Table.Body>
            {:else if tasks.length === 0}
            <Table.Body>
                <TableEmptyState colspan={4} message="No task data received yet" />
            </Table.Body>
            {:else}
            <Table.Header>
                <Table.Row>
                    <TracewayTableHeader
                        label="Task"
                        tooltip="The background job or task name"
                    />
                    <TracewayTableHeader
                        label="Runs"
                        tooltip="Total number of task executions"
                        sortField="count"
                        currentSortField={orderBy}
                        {sortDirection}
                        onSort={(field) => handleSort(field as SortField)}
                        class="w-[100px]"
                    />
                    <TracewayTableHeader
                        label="Typical"
                        tooltip="Median duration (P50)"
                        sortField="p50_duration"
                        currentSortField={orderBy}
                        {sortDirection}
                        onSort={(field) => handleSort(field as SortField)}
                        class="w-[100px]"
                    />
                    <TracewayTableHeader
                        label="Slow"
                        tooltip="95th percentile - slowest 5% of executions"
                        sortField="p95_duration"
                        currentSortField={orderBy}
                        {sortDirection}
                        onSort={(field) => handleSort(field as SortField)}
                        class="w-[100px]"
                    />
                </Table.Row>
            </Table.Header>
            <Table.Body>
                {#each tasks as task}
                    <Table.Row
                        class="cursor-pointer hover:bg-muted/50"
                        onclick={createRowClickHandler(resolve(`/tasks/${encodeURIComponent(task.taskName)}`), 'preset', 'from', 'to')}
                    >
                        <Table.Cell class="font-mono text-sm">
                            <span class="inline-flex items-center gap-2">
                                {task.taskName}
                                {#if task.nonRootCount > 0}
                                    <Badge variant="secondary" class="font-sans text-xs" title="At least one entry was captured as a child span of another endpoint/task/AI trace.">Not Root</Badge>
                                {/if}
                            </span>
                        </Table.Cell>
                        <Table.Cell class="tabular-nums">
                            {formatCount(task.count)}
                        </Table.Cell>
                        <Table.Cell class="font-mono text-sm tabular-nums">
                            {formatDuration(task.p50Duration)}
                        </Table.Cell>
                        <Table.Cell class="font-mono text-sm tabular-nums">
                            {formatDuration(task.p95Duration)}
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
        itemLabel="task"
    />
</div>
