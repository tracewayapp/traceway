<script lang="ts">
    import { onMount } from 'svelte';
    import { goto } from '$app/navigation';
    import { api } from '$lib/api';
    import * as Table from "$lib/components/ui/table";
    import { Button } from "$lib/components/ui/button";
    import { Skeleton } from "$lib/components/ui/skeleton";
    import * as Select from "$lib/components/ui/select";
    import { ArrowUpDown, ArrowDown } from "@lucide/svelte";
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

    type SortField = 'count' | 'p50_duration' | 'p95_duration' | 'last_seen';

    let endpoints = $state<EndpointStats[]>([]);
    let loading = $state(true);
    let error = $state('');

    // Pagination State
    let page = $state(1);
    let pageSize = $state(20);
    let total = $state(0);
    let totalPages = $state(0);

    // Date Range State
    let fromDate = $state<CalendarDate>(today(getLocalTimeZone()).subtract({ days: 7 }));
    let toDate = $state<CalendarDate>(today(getLocalTimeZone()));
    let fromTime = $state('00:00');
    let toTime = $state('23:59');

    // Sorting
    let orderBy = $state<SortField>('count');

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

    function handleTimeRangeChange(from: { date: CalendarDate; time: string }, to: { date: CalendarDate; time: string }) {
        fromDate = from.date;
        fromTime = from.time;
        toDate = to.date;
        toTime = to.time;
        page = 1;
        loadData();
    }

    function formatDuration(nanoseconds: number): string {
        const ms = nanoseconds / 1_000_000;
        if (ms < 1) {
            return `${(nanoseconds / 1000).toFixed(2)}µs`;
        } else if (ms < 1000) {
            return `${ms.toFixed(2)}ms`;
        } else {
            return `${(ms / 1000).toFixed(2)}s`;
        }
    }

    async function loadData() {
        loading = true;
        error = '';

        try {
            const requestBody = {
                fromDate: new Date(getFromDateTime()).toISOString(),
                toDate: new Date(getToDateTime()).toISOString(),
                orderBy: orderBy,
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
            loadData();
        }
    }

    function handlePageSizeChange(newPageSize: string) {
        pageSize = parseInt(newPageSize);
        page = 1;
        loadData();
    }

    function handleSort(field: SortField) {
        orderBy = field;
        page = 1;
        loadData();
    }

    function navigateToEndpoint(endpoint: string) {
        const params = new URLSearchParams({
            from: getFromDateTime(),
            to: getToDateTime()
        });
        goto(`/transactions/${encodeURIComponent(endpoint)}?${params.toString()}`);
    }

    onMount(() => {
        loadData();
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
            onApply={handleTimeRangeChange}
        />
    </div>

    <!-- Endpoints Table -->
    <div class="rounded-md border overflow-hidden">
        <Table.Root>
            <Table.Header>
                <Table.Row>
                    <Table.Head>Endpoint</Table.Head>
                    <Table.Head class="w-[120px]">
                        <Button
                            variant="ghost"
                            size="sm"
                            class="h-8 -ml-3 font-medium"
                            onclick={() => handleSort('count')}
                        >
                            Requests
                            {#if orderBy === 'count'}
                                <ArrowDown class="ml-2 h-4 w-4" />
                            {:else}
                                <ArrowUpDown class="ml-2 h-4 w-4" />
                            {/if}
                        </Button>
                    </Table.Head>
                    <Table.Head class="w-[120px]">
                        <Button
                            variant="ghost"
                            size="sm"
                            class="h-8 -ml-3 font-medium"
                            onclick={() => handleSort('p50_duration')}
                        >
                            P50
                            {#if orderBy === 'p50_duration'}
                                <ArrowDown class="ml-2 h-4 w-4" />
                            {:else}
                                <ArrowUpDown class="ml-2 h-4 w-4" />
                            {/if}
                        </Button>
                    </Table.Head>
                    <Table.Head class="w-[120px]">
                        <Button
                            variant="ghost"
                            size="sm"
                            class="h-8 -ml-3 font-medium"
                            onclick={() => handleSort('p95_duration')}
                        >
                            P95
                            {#if orderBy === 'p95_duration'}
                                <ArrowDown class="ml-2 h-4 w-4" />
                            {:else}
                                <ArrowUpDown class="ml-2 h-4 w-4" />
                            {/if}
                        </Button>
                    </Table.Head>
                    <Table.Head class="w-[180px]">
                        <Button
                            variant="ghost"
                            size="sm"
                            class="h-8 -ml-3 font-medium"
                            onclick={() => handleSort('last_seen')}
                        >
                            Last Seen
                            {#if orderBy === 'last_seen'}
                                <ArrowDown class="ml-2 h-4 w-4" />
                            {:else}
                                <ArrowUpDown class="ml-2 h-4 w-4" />
                            {/if}
                        </Button>
                    </Table.Head>
                </Table.Row>
            </Table.Header>
            <Table.Body>
                {#if loading}
                    {#each Array(5) as _}
                        <Table.Row>
                            <Table.Cell><Skeleton class="h-4 w-[250px]" /></Table.Cell>
                            <Table.Cell><Skeleton class="h-4 w-[60px]" /></Table.Cell>
                            <Table.Cell><Skeleton class="h-4 w-[80px]" /></Table.Cell>
                            <Table.Cell><Skeleton class="h-4 w-[80px]" /></Table.Cell>
                            <Table.Cell><Skeleton class="h-4 w-[140px]" /></Table.Cell>
                        </Table.Row>
                    {/each}
                {:else if error}
                    <Table.Row>
                        <Table.Cell colspan={5} class="h-24 text-center text-red-500">
                            {error}
                        </Table.Cell>
                    </Table.Row>
                {:else if endpoints.length === 0}
                    <Table.Row>
                        <Table.Cell colspan={5} class="h-24 text-center">
                            No transactions found in this time range.
                        </Table.Cell>
                    </Table.Row>
                {:else}
                    {#each endpoints as endpoint}
                        <Table.Row
                            class="cursor-pointer hover:bg-muted/50"
                            onclick={() => navigateToEndpoint(endpoint.endpoint)}
                        >
                            <Table.Cell class="font-mono text-sm">
                                {endpoint.endpoint}
                            </Table.Cell>
                            <Table.Cell class="font-medium">
                                {endpoint.count.toLocaleString()}
                            </Table.Cell>
                            <Table.Cell class="font-mono text-sm">
                                {formatDuration(endpoint.p50Duration)}
                            </Table.Cell>
                            <Table.Cell class="font-mono text-sm">
                                {formatDuration(endpoint.p95Duration)}
                            </Table.Cell>
                            <Table.Cell class="text-muted-foreground">
                                {new Date(endpoint.lastSeen).toLocaleString()}
                            </Table.Cell>
                        </Table.Row>
                    {/each}
                {/if}
            </Table.Body>
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
