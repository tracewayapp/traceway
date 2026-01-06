<script lang="ts">
    import { onMount } from 'svelte';
    import { goto } from '$app/navigation';
    import { api } from '$lib/api';
    import * as Table from "$lib/components/ui/table";
    import { Button } from "$lib/components/ui/button";
    import { Skeleton } from "$lib/components/ui/skeleton";
    import { Input } from "$lib/components/ui/input";
    import { Label } from "$lib/components/ui/label";
    import { ArrowLeft, ArrowUpDown, ArrowDown } from "@lucide/svelte";
    import { ErrorDisplay } from "$lib/components/ui/error-display";
    import { projectsState } from '$lib/state/projects.svelte';

    type Transaction = {
        id: string;
        endpoint: string;
        duration: number;
        recordedAt: string;
        statusCode: number;
        bodySize: number;
        clientIP: string;
    };

    type SortField = 'recorded_at' | 'duration' | 'status_code' | 'body_size';

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

    // Date Range State (separate date and time)
    let fromDateValue = $state('');
    let fromTimeValue = $state('');
    let toDateValue = $state('');
    let toTimeValue = $state('');

    // Sorting State
    let orderBy = $state<SortField>('recorded_at');

    // Combine date and time into ISO datetime string
    function getFromDateTime(): string {
        if (!fromDateValue) return '';
        return `${fromDateValue}T${fromTimeValue || '00:00'}`;
    }

    function getToDateTime(): string {
        if (!toDateValue) return '';
        return `${toDateValue}T${toTimeValue || '23:59'}`;
    }

    // Parse datetime string into date and time parts
    function parseDateTimeParts(dateTimeStr: string): { date: string; time: string } {
        if (!dateTimeStr) return { date: '', time: '' };
        const parts = dateTimeStr.split('T');
        return {
            date: parts[0] || '',
            time: parts[1] || '00:00'
        };
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
        orderBy = field;
        page = 1;
        loadData();
    }

    function goBack() {
        goto('/transactions');
    }

    onMount(() => {
        // Initialize dates from URL params or default to last 7 days
        if (data.from && data.to) {
            const fromParts = parseDateTimeParts(data.from);
            const toParts = parseDateTimeParts(data.to);
            fromDateValue = fromParts.date;
            fromTimeValue = fromParts.time;
            toDateValue = toParts.date;
            toTimeValue = toParts.time;
        } else {
            const now = new Date();
            const sevenDaysAgo = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000);
            toDateValue = now.toISOString().slice(0, 10);
            toTimeValue = now.toTimeString().slice(0, 5);
            fromDateValue = sevenDaysAgo.toISOString().slice(0, 10);
            fromTimeValue = sevenDaysAgo.toTimeString().slice(0, 5);
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
    <div class="flex items-center gap-4">
        <Button variant="ghost" size="sm" onclick={goBack} class="h-8 w-8 p-0">
            <ArrowLeft class="h-4 w-4" />
        </Button>
        <div>
            <h2 class="text-2xl font-bold tracking-tight font-mono">{decodeURIComponent(data.endpoint)}</h2>
            <p class="text-sm text-muted-foreground">Transaction instances for this endpoint</p>
        </div>
    </div>

    <!-- Date Range Filters -->
    <div class="flex flex-wrap items-end gap-4 p-4 rounded-lg border bg-card">
        <div class="flex flex-col gap-1.5">
            <Label for="from-date" class="text-xs text-muted-foreground">From Date</Label>
            <Input
                id="from-date"
                type="date"
                class="h-9 w-[150px]"
                bind:value={fromDateValue}
            />
        </div>
        <div class="flex flex-col gap-1.5">
            <Label for="from-time" class="text-xs text-muted-foreground">From Time</Label>
            <Input
                id="from-time"
                type="time"
                class="h-9 w-[120px]"
                bind:value={fromTimeValue}
            />
        </div>
        <div class="flex flex-col gap-1.5">
            <Label for="to-date" class="text-xs text-muted-foreground">To Date</Label>
            <Input
                id="to-date"
                type="date"
                class="h-9 w-[150px]"
                bind:value={toDateValue}
            />
        </div>
        <div class="flex flex-col gap-1.5">
            <Label for="to-time" class="text-xs text-muted-foreground">To Time</Label>
            <Input
                id="to-time"
                type="time"
                class="h-9 w-[120px]"
                bind:value={toTimeValue}
            />
        </div>
        <Button variant="default" size="sm" onclick={loadData} class="h-9">
            Go
        </Button>
    </div>

    <!-- Transactions Table -->
    <div class="rounded-md border">
        <Table.Root>
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
                            onclick={() => handleSort('duration')}
                        >
                            Duration
                            {#if orderBy === 'duration'}
                                <ArrowDown class="ml-2 h-4 w-4" />
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
                                <ArrowDown class="ml-2 h-4 w-4" />
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
                                <ArrowDown class="ml-2 h-4 w-4" />
                            {:else}
                                <ArrowUpDown class="ml-2 h-4 w-4" />
                            {/if}
                        </Button>
                    </Table.Head>
                    <Table.Head class="w-[140px]">Client IP</Table.Head>
                </Table.Row>
            </Table.Header>
            <Table.Body>
                {#if loading}
                    {#each Array(5) as _}
                        <Table.Row>
                            <Table.Cell><Skeleton class="h-4 w-[140px]" /></Table.Cell>
                            <Table.Cell><Skeleton class="h-4 w-[80px]" /></Table.Cell>
                            <Table.Cell><Skeleton class="h-4 w-[50px]" /></Table.Cell>
                            <Table.Cell><Skeleton class="h-4 w-[60px]" /></Table.Cell>
                            <Table.Cell><Skeleton class="h-4 w-[100px]" /></Table.Cell>
                        </Table.Row>
                    {/each}
                {:else if transactions.length === 0}
                    <Table.Row>
                        <Table.Cell colspan={5} class="h-24 text-center">
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
