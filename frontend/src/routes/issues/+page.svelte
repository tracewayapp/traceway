<script lang="ts">
    import { onMount } from 'svelte';
    import { goto } from '$app/navigation';
    import { api } from '$lib/api';
    import * as Table from "$lib/components/ui/table";
    import { Input } from "$lib/components/ui/input";
    import { Button } from "$lib/components/ui/button";
    import * as Select from "$lib/components/ui/select";
    import { Skeleton } from "$lib/components/ui/skeleton";

    type ExceptionGroup = {
        exceptionHash: string;
        stackTrace: string;
        lastSeen: string;
        firstSeen: string;
        count: number;
    };

    let exceptions = $state<ExceptionGroup[]>([]);
    let loading = $state(true);
    let error = $state('');

    // Pagination State
    let page = $state(1);
    let pageSize = $state(10);
    let total = $state(0);
    let totalPages = $state(0);
    let selectedCount = $state(0); // For row selection tracking

    // Filters
    let searchQuery = $state('');
    let daysBack = $state("7");

    // Select options for days back
    const daysOptions = [
        { value: "1", label: "24 Hours" },
        { value: "7", label: "7 Days" }
    ];

    // Page size options
    const pageSizeOptions = [
        { value: "10", label: "10" },
        { value: "20", label: "20" },
        { value: "50", label: "50" },
        { value: "100", label: "100" }
    ];

    // Derived labels for select displays
    const daysBackLabel = $derived(daysOptions.find(o => o.value === daysBack)?.label ?? "Select period");
    const pageSizeLabel = $derived(pageSizeOptions.find(o => o.value === pageSize.toString())?.label ?? pageSize.toString());

    async function loadData() {
        loading = true;
        error = '';

        try {
            const now = new Date();
            const fromDate = new Date();
            fromDate.setDate(now.getDate() - parseInt(daysBack));

            const requestBody = {
                fromDate: fromDate.toISOString(),
                toDate: now.toISOString(),
                orderBy: 'last_seen',
                pagination: {
                    page: page,
                    pageSize: pageSize
                },
                search: searchQuery.trim()
            };

            const response = await api.post('/exception-stack-traces', requestBody);

            exceptions = response.data || [];
            total = response.pagination.total;
            totalPages = response.pagination.totalPages;

        } catch (e: any) {
            console.error(e);
            error = e.message || 'Failed to load data';
        } finally {
            loading = false;
        }
    }

    // $effect(() => {
    //     if (page || daysBack) {
    //         loadData();
    //     }
    // });

    function handlePageChange(newPage: number) {
        if (newPage >= 1 && newPage <= totalPages) {
            page = newPage;
            loadData();
        }
    }

    function handlePageSizeChange(newPageSize: string) {
        pageSize = parseInt(newPageSize);
        page = 1; // Reset to first page when changing page size
        loadData();
    }

    // Debounce search input
    let searchTimeout: ReturnType<typeof setTimeout> | null = null;

    function handleSearchInput() {
        if (searchTimeout) clearTimeout(searchTimeout);
        searchTimeout = setTimeout(() => {
            page = 1; // Reset to page 1 on new search
            loadData();
        }, 300);
    }

    onMount(() => {
        loadData();
    });
</script>

<div class="space-y-4">
    <div class="flex items-center justify-between">
        <h2 class="text-3xl font-bold tracking-tight">Issues</h2>
    </div>

    <div class="flex items-center space-x-2">
        <Input
            placeholder="Search exceptions..."
            class="h-8 w-[150px] lg:w-[250px]"
            bind:value={searchQuery}
            oninput={handleSearchInput}
        />

        <Select.Root
            type="single"
            bind:value={daysBack}
            onValueChange={(v) => {
                if (v) {
                    page = 1;
                    loadData();
                }
            }}
        >
            <Select.Trigger class="h-8 w-[120px]">
                {daysBackLabel}
            </Select.Trigger>
            <Select.Content>
                {#each daysOptions as option}
                    <Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
                {/each}
            </Select.Content>
        </Select.Root>

        <div class="ml-auto">
             <Button variant="outline" size="sm" onclick={loadData} class="h-8">Refresh</Button>
        </div>
    </div>

    <div class="rounded-md border">
        <Table.Root>
            <Table.Header>
                <Table.Row>
                    <Table.Head class="w-[30px]"></Table.Head>
                    <Table.Head>Issue</Table.Head>
                    <Table.Head class="w-[100px]">Count</Table.Head>
                    <Table.Head class="w-[200px]">Last Seen</Table.Head>
                </Table.Row>
            </Table.Header>
            <Table.Body>
                {#if loading}
                     {#each Array(5) as _}
                        <Table.Row>
                            <Table.Cell><Skeleton class="h-4 w-[30px]" /></Table.Cell>
                            <Table.Cell><Skeleton class="h-4 w-[250px]" /></Table.Cell>
                            <Table.Cell><Skeleton class="h-4 w-[40px]" /></Table.Cell>
                            <Table.Cell><Skeleton class="h-4 w-[150px]" /></Table.Cell>
                        </Table.Row>
                     {/each}
                {:else if error}
                    <Table.Row>
                        <Table.Cell colspan={4} class="h-24 text-center text-red-500">
                            {error}
                        </Table.Cell>
                    </Table.Row>
                {:else if exceptions.length === 0}
                    <Table.Row>
                        <Table.Cell colspan={4} class="h-24 text-center">
                            No issues found.
                        </Table.Cell>
                    </Table.Row>
                {:else}
                    {#each exceptions as exception}
                        <Table.Row
                            class="cursor-pointer hover:bg-muted/50"
                            onclick={() => goto(`/issues/${exception.exceptionHash}`)}
                        >
                            <Table.Cell class="font-medium truncate max-w-[30px]"></Table.Cell>
                            <Table.Cell class="truncate" title={exception.stackTrace}>
                                {exception.stackTrace.split('\n')[0]}
                            </Table.Cell>
                            <Table.Cell>{exception.count}</Table.Cell>
                            <Table.Cell>{new Date(exception.lastSeen).toLocaleString()}</Table.Cell>
                        </Table.Row>
                    {/each}
                {/if}
            </Table.Body>
        </Table.Root>
    </div>

    <!-- Pagination Footer -->
    <div class="flex items-center justify-between px-2">
        <div class="flex-1 text-sm text-muted-foreground">
            {selectedCount} of {total} row(s) selected.
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
                    <svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 15 15" fill="none" class="lucide lucide-chevron-left h-4 w-4">
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
                    <svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 15 15" fill="none" class="lucide lucide-chevron-right h-4 w-4">
                        <path d="M6.1584 3.13508C6.35985 2.94621 6.67627 2.95642 6.86514 3.15788L10.6151 7.15788C10.7954 7.3502 10.7954 7.64949 10.6151 7.84182L6.86514 11.8418C6.67627 12.0433 6.35985 12.0535 6.1584 11.8646C5.95694 11.6757 5.94673 11.3593 6.1356 11.1579L9.565 7.49985L6.1356 3.84182C5.94673 3.64036 5.95694 3.32394 6.1584 3.13508Z" fill="currentColor" fill-rule="evenodd" clip-rule="evenodd"></path>
                    </svg>
                </Button>
            </div>
        </div>
    </div>
</div>
