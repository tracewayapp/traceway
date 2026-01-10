<script lang="ts">
    import { onMount } from 'svelte';
    import { goto } from '$app/navigation';
    import { api } from '$lib/api';
    import * as Table from "$lib/components/ui/table";
    import { Button } from "$lib/components/ui/button";
    import { Skeleton } from "$lib/components/ui/skeleton";
    import { TimeRangePicker } from "$lib/components/ui/time-range-picker";
    import { ArrowLeft, ArrowUpDown, ArrowDown, ArrowUp } from "@lucide/svelte";
    import { CalendarDate, getLocalTimeZone, today } from "@internationalized/date";
    import { ErrorDisplay } from "$lib/components/ui/error-display";
    import { projectsState } from '$lib/state/projects.svelte';
    import ScopeDisplay from '$lib/components/scope-display.svelte';

    type Transaction = {
        id: string;
        endpoint: string;
        duration: number;
        recordedAt: string;
        statusCode: number;
        bodySize: number;
        clientIP: string;
        scope: Record<string, string> | null;
    };

    type SortField = 'recorded_at' | 'duration' | 'status_code' | 'body_size';
    type SortDirection = 'asc' | 'desc';

    let { data } = $props();

    let transactions = $state<Transaction[]>([]);
    let loading = $state(true);
    let error = $state('');
    let notFound = $state(false);
    let errorStatus = $state<number>(0);

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

    // Sorting State
    let orderBy = $state<SortField>('recorded_at');
    let sortDirection = $state<SortDirection>('desc');

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

    function formatBytes(bytes: number): string {
        if (bytes < 1024) {
            return `${bytes} B`;
        } else if (bytes < 1024 * 1024) {
            return `${(bytes / 1024).toFixed(1)} KB`;
        } else {
            return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
        }
    }

    function getStatusColor(statusCode: number): string {
        if (statusCode >= 200 && statusCode < 300) {
            return 'text-green-500';
        } else if (statusCode >= 300 && statusCode < 400) {
            return 'text-blue-500';
        } else if (statusCode >= 400 && statusCode < 500) {
            return 'text-yellow-500';
        } else {
            return 'text-red-500';
        }
    }

    async function loadData() {
        loading = true;
        error = '';
        notFound = false;
        errorStatus = 0;

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

            const response = await api.post(`/transactions/endpoint?endpoint=${encodeURIComponent(data.endpoint)}`, requestBody, { projectId: projectsState.currentProjectId ?? undefined });

            transactions = response.data || [];
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
            loadData();
        }
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
        loadData();
    }

    function goBack() {
        goto('/transactions');
    }

    onMount(() => {
        // Initialize dates from URL params or use defaults (already set in state)
        if (data.from && data.to) {
            // Parse from URL params: "YYYY-MM-DDTHH:MM"
            const fromParts = data.from.split('T');
            const toParts = data.to.split('T');

            if (fromParts[0]) {
                const [year, month, day] = fromParts[0].split('-').map(Number);
                fromDate = new CalendarDate(year, month, day);
                fromTime = fromParts[1] || '00:00';
            }

            if (toParts[0]) {
                const [year, month, day] = toParts[0].split('-').map(Number);
                toDate = new CalendarDate(year, month, day);
                toTime = toParts[1] || '23:59';
            }
        }
        loadData();
    });
</script>

<div class="space-y-6">
    {#if notFound}
        <ErrorDisplay
            status={404}
            title="Endpoint Not Found"
            description="The endpoint you're looking for doesn't exist or has no recorded transactions."
            backHref="/transactions"
            backLabel="Back to Transactions"
            onRetry={() => loadData()}
            identifier={decodeURIComponent(data.endpoint)}
        />
    {:else if error && !loading}
        <ErrorDisplay
            status={errorStatus === 400 ? 400 : errorStatus === 422 ? 422 : 400}
            title="Failed to Load Transactions"
            description={error}
            backHref="/transactions"
            backLabel="Back to Transactions"
            onRetry={() => loadData()}
        />
    {:else}
    <!-- Header with Title and Time Range Filter -->
    <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div class="flex items-center gap-4">
            <Button variant="ghost" size="sm" onclick={goBack} class="h-8 w-8 p-0">
                <ArrowLeft class="h-4 w-4" />
            </Button>
            <div>
                <h2 class="text-2xl font-bold tracking-tight font-mono">{decodeURIComponent(data.endpoint)}</h2>
                <p class="text-sm text-muted-foreground">Transaction instances for this endpoint</p>
            </div>
        </div>
        <TimeRangePicker
            bind:fromDate
            bind:toDate
            bind:fromTime
            bind:toTime
            onApply={handleTimeRangeChange}
        />
    </div>

    <!-- Transactions Table -->
    <div class="rounded-md border overflow-hidden">
        <Table.Root>
            {#if loading || transactions.length > 0}
            <Table.Header>
                <Table.Row>
                    <Table.Head class="w-[180px]">
                        <Button
                            variant="ghost"
                            size="sm"
                            class="h-8 -ml-3 font-medium"
                            onclick={() => handleSort('recorded_at')}
                        >
                            Recorded At
                            {#if orderBy === 'recorded_at'}
                                {#if sortDirection === 'desc'}
                                    <ArrowDown class="ml-2 h-4 w-4" />
                                {:else}
                                    <ArrowUp class="ml-2 h-4 w-4" />
                                {/if}
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
                            onclick={() => handleSort('duration')}
                        >
                            Duration
                            {#if orderBy === 'duration'}
                                {#if sortDirection === 'desc'}
                                    <ArrowDown class="ml-2 h-4 w-4" />
                                {:else}
                                    <ArrowUp class="ml-2 h-4 w-4" />
                                {/if}
                            {:else}
                                <ArrowUpDown class="ml-2 h-4 w-4" />
                            {/if}
                        </Button>
                    </Table.Head>
                    <Table.Head class="w-[100px]">
                        <Button
                            variant="ghost"
                            size="sm"
                            class="h-8 -ml-3 font-medium"
                            onclick={() => handleSort('status_code')}
                        >
                            Status
                            {#if orderBy === 'status_code'}
                                {#if sortDirection === 'desc'}
                                    <ArrowDown class="ml-2 h-4 w-4" />
                                {:else}
                                    <ArrowUp class="ml-2 h-4 w-4" />
                                {/if}
                            {:else}
                                <ArrowUpDown class="ml-2 h-4 w-4" />
                            {/if}
                        </Button>
                    </Table.Head>
                    <Table.Head class="w-[100px]">
                        <Button
                            variant="ghost"
                            size="sm"
                            class="h-8 -ml-3 font-medium"
                            onclick={() => handleSort('body_size')}
                        >
                            Body Size
                            {#if orderBy === 'body_size'}
                                {#if sortDirection === 'desc'}
                                    <ArrowDown class="ml-2 h-4 w-4" />
                                {:else}
                                    <ArrowUp class="ml-2 h-4 w-4" />
                                {/if}
                            {:else}
                                <ArrowUpDown class="ml-2 h-4 w-4" />
                            {/if}
                        </Button>
                    </Table.Head>
                    <Table.Head class="w-[140px]">Client IP</Table.Head>
                    <Table.Head>Context</Table.Head>
                </Table.Row>
            </Table.Header>
            {/if}
            <Table.Body>
                {#if loading}
                    {#each Array(5) as _}
                        <Table.Row>
                            <Table.Cell><Skeleton class="h-4 w-[140px]" /></Table.Cell>
                            <Table.Cell><Skeleton class="h-4 w-[80px]" /></Table.Cell>
                            <Table.Cell><Skeleton class="h-4 w-[50px]" /></Table.Cell>
                            <Table.Cell><Skeleton class="h-4 w-[60px]" /></Table.Cell>
                            <Table.Cell><Skeleton class="h-4 w-[100px]" /></Table.Cell>
                            <Table.Cell><Skeleton class="h-4 w-[150px]" /></Table.Cell>
                        </Table.Row>
                    {/each}
                {:else if transactions.length === 0}
                    <Table.Row>
                        <Table.Cell colspan={6} class="h-24 text-center">
                            No transactions found in this time range.
                        </Table.Cell>
                    </Table.Row>
                {:else}
                    {#each transactions as transaction}
                        <Table.Row>
                            <Table.Cell class="text-muted-foreground">
                                {new Date(transaction.recordedAt).toLocaleString()}
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
                            <Table.Cell>
                                <ScopeDisplay scope={transaction.scope} />
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
            {total} transaction(s) found.
        </div>
        <div class="flex items-center space-x-6 lg:space-x-8">
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
    {/if}
</div>
