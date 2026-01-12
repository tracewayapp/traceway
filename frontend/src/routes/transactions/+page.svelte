<script lang="ts">
    import { onMount, onDestroy } from 'svelte';
    import { browser } from '$app/environment';
    import { api } from '$lib/api';
    import * as Table from "$lib/components/ui/table";
    import { Button } from "$lib/components/ui/button";
    import { LoadingCircle } from "$lib/components/ui/loading-circle";
    import * as Select from "$lib/components/ui/select";
    import { ArrowUpDown, ArrowDown, ArrowUp, TriangleAlert, CircleHelp } from "@lucide/svelte";
    import * as Tooltip from "$lib/components/ui/tooltip";
    import { TimeRangePicker } from "$lib/components/ui/time-range-picker";
    import { CalendarDate, getLocalTimeZone, today } from "@internationalized/date";
    import { projectsState } from '$lib/state/projects.svelte';

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
    function getTimeRangeFromPreset(presetValue: string): { from: Date; to: Date } {
        const minutes = presetMinutes[presetValue] || 360;
        const now = new Date();
        const from = new Date(now.getTime() - minutes * 60 * 1000);
        return { from, to: now };
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
            const from = new Date(fromParam);
            const to = new Date(toParam);
            if (!isNaN(from.getTime()) && !isNaN(to.getTime())) {
                return { preset: null, from, to };
            }
        }

        // Default to 6h preset
        return { preset: '6h', from: null, to: null };
    }

    // Initialize from URL
    const initialUrlParams = parseUrlParams();
    const initialRange = initialUrlParams.preset
        ? getTimeRangeFromPreset(initialUrlParams.preset)
        : { from: initialUrlParams.from!, to: initialUrlParams.to! };

    // Date Range State
    let selectedPreset = $state<string | null>(initialUrlParams.preset);
    let fromDate = $state<CalendarDate>(dateToCalendarDate(initialRange.from));
    let toDate = $state<CalendarDate>(dateToCalendarDate(initialRange.to));
    let fromTime = $state(dateToTimeString(initialRange.from));
    let toTime = $state(dateToTimeString(initialRange.to));

    // Update URL with current time range
    function updateUrl(pushState = true) {
        if (!browser) return;

        const params = new URLSearchParams();

        if (selectedPreset) {
            params.set('preset', selectedPreset);
        } else {
            const fromDateTime = new Date(getFromDateTime());
            const toDateTime = new Date(getToDateTime());
            params.set('from', fromDateTime.toISOString());
            params.set('to', toDateTime.toISOString());
        }

        const newUrl = `${window.location.pathname}?${params.toString()}`;

        if (pushState) {
            window.history.pushState({}, '', newUrl);
        } else {
            window.history.replaceState({}, '', newUrl);
        }
    }

    // Handle browser back/forward navigation
    function handlePopState() {
        const urlParams = parseUrlParams();

        if (urlParams.preset) {
            selectedPreset = urlParams.preset;
            const range = getTimeRangeFromPreset(urlParams.preset);
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

    // Combine date and time into ISO datetime string
    function getFromDateTime(): string {
        const dateStr = `${fromDate.year}-${String(fromDate.month).padStart(2, '0')}-${String(fromDate.day).padStart(2, '0')}`;
        return `${dateStr}T${fromTime || '00:00'}`;
    }

    function getToDateTime(): string {
        const dateStr = `${toDate.year}-${String(toDate.month).padStart(2, '0')}-${String(toDate.day).padStart(2, '0')}`;
        return `${dateStr}T${toTime || '23:59'}`;
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

    function formatDuration(nanoseconds: number): string {
        const ms = nanoseconds / 1_000_000;
        if (ms < 1) {
            return `${(nanoseconds / 1000).toFixed(0)}µs`;
        } else if (ms < 1000) {
            return `${ms.toFixed(0)}ms`;
        } else {
            return `${(ms / 1000).toFixed(1)}s`;
        }
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
                fromDate: new Date(getFromDateTime()).toISOString(),
                toDate: new Date(getToDateTime()).toISOString(),
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

    function handlePageSizeChange(newPageSize: string) {
        pageSize = parseInt(newPageSize);
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

    function getEndpointUrl(endpoint: string): string {
        const params = new URLSearchParams();
        if (selectedPreset) {
            params.set('preset', selectedPreset);
        } else {
            params.set('from', new Date(getFromDateTime()).toISOString());
            params.set('to', new Date(getToDateTime()).toISOString());
        }
        return `/transactions/${encodeURIComponent(endpoint)}?${params.toString()}`;
    }

    function navigateToEndpoint(endpoint: string, event: MouseEvent) {
        const url = getEndpointUrl(endpoint);
        if (event.ctrlKey || event.metaKey) {
            window.open(url, '_blank');
        } else {
            window.location.href = url;
        }
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

<div class="space-y-6">
    <!-- Header with Title and Time Range Filter -->
    <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <h2 class="text-3xl font-bold tracking-tight">Transactions</h2>
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
                <Table.Row>
                    <Table.Cell colspan={5} class="h-24 text-center text-muted-foreground">
                        No transaction data received yet
                    </Table.Cell>
                </Table.Row>
            </Table.Body>
            {:else}
            <Table.Header>
                <Table.Row>
                    <Table.Head>
                        <span class="flex items-center gap-1.5">
                            Endpoint
                            <Tooltip.Root>
                                <Tooltip.Trigger>
                                    <CircleHelp class="h-3.5 w-3.5 text-muted-foreground/60" />
                                </Tooltip.Trigger>
                                <Tooltip.Content>
                                    <p class="text-xs">The API route or page being accessed</p>
                                </Tooltip.Content>
                            </Tooltip.Root>
                        </span>
                    </Table.Head>
                    <Table.Head class="w-[100px]">
                        <div class="flex items-center">
                            <Button
                                variant="ghost"
                                size="sm"
                                class="h-8 -ml-3 font-medium"
                                onclick={() => handleSort('count')}
                            >
                                Calls
                                {#if orderBy === 'count'}
                                    {#if sortDirection === 'desc'}
                                        <ArrowDown class="ml-2 h-4 w-4" />
                                    {:else}
                                        <ArrowUp class="ml-2 h-4 w-4" />
                                    {/if}
                                {:else}
                                    <ArrowUpDown class="ml-2 h-4 w-4" />
                                {/if}
                            </Button>
                            <Tooltip.Root>
                                <Tooltip.Trigger>
                                    <CircleHelp class="h-3.5 w-3.5 text-muted-foreground/60" />
                                </Tooltip.Trigger>
                                <Tooltip.Content>
                                    <p class="text-xs">Total number of requests</p>
                                </Tooltip.Content>
                            </Tooltip.Root>
                        </div>
                    </Table.Head>
                    <Table.Head class="w-[100px]">
                        <div class="flex items-center">
                            <Button
                                variant="ghost"
                                size="sm"
                                class="h-8 -ml-3 font-medium"
                                onclick={() => handleSort('p50_duration')}
                            >
                                Typical
                                {#if orderBy === 'p50_duration'}
                                    {#if sortDirection === 'desc'}
                                        <ArrowDown class="ml-2 h-4 w-4" />
                                    {:else}
                                        <ArrowUp class="ml-2 h-4 w-4" />
                                    {/if}
                                {:else}
                                    <ArrowUpDown class="ml-2 h-4 w-4" />
                                {/if}
                            </Button>
                            <Tooltip.Root>
                                <Tooltip.Trigger>
                                    <CircleHelp class="h-3.5 w-3.5 text-muted-foreground/60" />
                                </Tooltip.Trigger>
                                <Tooltip.Content>
                                    <p class="text-xs">Median response time (P50)</p>
                                </Tooltip.Content>
                            </Tooltip.Root>
                        </div>
                    </Table.Head>
                    <Table.Head class="w-[100px]">
                        <div class="flex items-center">
                            <Button
                                variant="ghost"
                                size="sm"
                                class="h-8 -ml-3 font-medium"
                                onclick={() => handleSort('p95_duration')}
                            >
                                Slow
                                {#if orderBy === 'p95_duration'}
                                    {#if sortDirection === 'desc'}
                                        <ArrowDown class="ml-2 h-4 w-4" />
                                    {:else}
                                        <ArrowUp class="ml-2 h-4 w-4" />
                                    {/if}
                                {:else}
                                    <ArrowUpDown class="ml-2 h-4 w-4" />
                                {/if}
                            </Button>
                            <Tooltip.Root>
                                <Tooltip.Trigger>
                                    <CircleHelp class="h-3.5 w-3.5 text-muted-foreground/60" />
                                </Tooltip.Trigger>
                                <Tooltip.Content>
                                    <p class="text-xs">95th percentile - slowest 5% of requests</p>
                                </Tooltip.Content>
                            </Tooltip.Root>
                        </div>
                    </Table.Head>
                    <Table.Head class="w-[120px] text-right">
                        <div class="flex items-center justify-end gap-1.5">
                            <Button
                                variant="ghost"
                                size="sm"
                                class="h-8 font-medium"
                                onclick={() => handleSort('impact')}
                            >
                                Impact
                                {#if orderBy === 'impact'}
                                    {#if sortDirection === 'desc'}
                                        <ArrowDown class="ml-2 h-4 w-4" />
                                    {:else}
                                        <ArrowUp class="ml-2 h-4 w-4" />
                                    {/if}
                                {:else}
                                    <ArrowUpDown class="ml-2 h-4 w-4" />
                                {/if}
                            </Button>
                            <Tooltip.Root>
                                <Tooltip.Trigger>
                                    <CircleHelp class="h-3.5 w-3.5 text-muted-foreground/60" />
                                </Tooltip.Trigger>
                                <Tooltip.Content>
                                    <p class="text-xs">Priority based on traffic × response time variance</p>
                                </Tooltip.Content>
                            </Tooltip.Root>
                        </div>
                    </Table.Head>
                </Table.Row>
            </Table.Header>
            <Table.Body>
                {#each endpoints as endpoint}
                    {@const impactLevel = getImpactLevel(endpoint.count, endpoint.p50Duration, endpoint.p95Duration)}
                    <Table.Row
                        class="cursor-pointer hover:bg-muted/50"
                        onclick={(e) => navigateToEndpoint(endpoint.endpoint, e)}
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
                            {#if impactLevel === 'critical'}
                                <span class="inline-flex items-center gap-1 rounded-full bg-red-500/15 px-2 py-0.5 text-xs font-medium text-red-600 dark:text-red-400">
                                    <TriangleAlert class="h-3 w-3" />
                                    Critical
                                </span>
                            {:else if impactLevel === 'high'}
                                <span class="inline-flex items-center gap-1 rounded-full bg-orange-500/15 px-2 py-0.5 text-xs font-medium text-orange-600 dark:text-orange-400">
                                    <TriangleAlert class="h-3 w-3" />
                                    High
                                </span>
                            {:else if impactLevel === 'medium'}
                                <span class="inline-flex items-center gap-1 rounded-full bg-yellow-500/15 px-2 py-0.5 text-xs font-medium text-yellow-600 dark:text-yellow-500">
                                    Medium
                                </span>
                            {/if}
                        </Table.Cell>
                    </Table.Row>
                {/each}
            </Table.Body>
            {/if}
        </Table.Root>
    </div>

    <!-- Pagination Footer -->
    <div class="flex items-center justify-between px-2">
        <div class="flex-1 text-sm text-muted-foreground">
            {total} endpoint(s) found.
        </div>
        <div class="flex items-center space-x-6 lg:space-x-8">
            <div class="flex items-center space-x-2">
                <p class="text-sm font-medium">Rows per page</p>
                <Select.Root
                    type="single"
                    value={pageSize.toString()}
                    onValueChange={(v) => {
                        if (v) {
                            handlePageSizeChange(v);
                        }
                    }}
                >
                    <Select.Trigger class="h-8 w-[70px]">
                        {pageSizeLabel}
                    </Select.Trigger>
                    <Select.Content side="top">
                        {#each pageSizeOptions as option}
                            <Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
                        {/each}
                    </Select.Content>
                </Select.Root>
            </div>
            <div class="flex w-[100px] items-center justify-center text-sm font-medium">
                Page {page} of {totalPages || 1}
            </div>
            <div class="flex items-center space-x-2">
                <Button
                    variant="outline"
                    size="sm"
                    class="h-8 w-8 p-0"
                    onclick={() => handlePageChange(page - 1)}
                    disabled={page <= 1 || loading}
                >
                    <span class="sr-only">Go to previous page</span>
                    <svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 15 15" fill="none" class="h-4 w-4">
                        <path d="M8.84182 3.13514C9.04327 3.32401 9.05348 3.64042 8.86462 3.84188L5.43521 7.49991L8.86462 11.1579C9.05348 11.3594 9.04327 11.6758 8.84182 11.8647C8.64036 12.0535 8.32394 12.0433 8.13508 11.8419L4.38508 7.84188C4.20477 7.64955 4.20477 7.35027 4.38508 7.15794L8.13508 3.15794C8.32394 2.95648 8.64036 2.94628 8.84182 3.13514Z" fill="currentColor" fill-rule="evenodd" clip-rule="evenodd"></path>
                    </svg>
                </Button>
                <Button
                    variant="outline"
                    size="sm"
                    class="h-8 w-8 p-0"
                    onclick={() => handlePageChange(page + 1)}
                    disabled={page >= totalPages || loading}
                >
                    <span class="sr-only">Go to next page</span>
                    <svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 15 15" fill="none" class="h-4 w-4">
                        <path d="M6.1584 3.13508C6.35985 2.94621 6.67627 2.95642 6.86514 3.15788L10.6151 7.15788C10.7954 7.3502 10.7954 7.64949 10.6151 7.84182L6.86514 11.8418C6.67627 12.0433 6.35985 12.0535 6.1584 11.8646C5.95694 11.6757 5.94673 11.3593 6.1356 11.1579L9.565 7.49985L6.1356 3.84182C5.94673 3.64036 5.95694 3.32394 6.1584 3.13508Z" fill="currentColor" fill-rule="evenodd" clip-rule="evenodd"></path>
                    </svg>
                </Button>
            </div>
        </div>
    </div>
</div>
