<script lang="ts">
    import { api } from '$lib/api';
    import * as Table from "$lib/components/ui/table";
    import { Input } from "$lib/components/ui/input";
    import { Button } from "$lib/components/ui/button";
    import * as Select from "$lib/components/ui/select";
    import { Skeleton } from "$lib/components/ui/skeleton";

    type ExceptionGroup = {
        issue: string;
        lastSeen: string;
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

    // Filters
    let searchQuery = $state('');
    let daysBack = $state("1");

    // Select options for days back
    const daysOptions = [
        { value: "1", label: "24 Hours" },
        { value: "7", label: "7 Days" },
        { value: "30", label: "30 Days" }
    ];

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
                orderBy: 'last_seen DESC', // Default sort
                pagination: {
                    page: page,
                    pageSize: pageSize
                }
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
        }
    }
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
            disabled
        />
        <!-- Note: Search disabled as backend implementation pending -->

        <!-- <Select.Root
            onOpenChangeComplete={(v) => {
                if (v) {
                    daysBack = v.value;
                    page = 1; // Reset to page 1 on filter change
                }
            }}
            selected={{ value: daysBack, label: daysOptions.find(o => o.value === daysBack)?.label }}
        >
            <Select.Trigger class="h-8 w-[180px]">
                <Select.Value placeholder="Select period" />
            </Select.Trigger>
            <Select.Content>
                {#each daysOptions as option}
                    <Select.Item value={option.value}>{option.label}</Select.Item>
                {/each}
            </Select.Content>
        </Select.Root> -->

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
                        <Table.Row>
                            <Table.Cell class="font-medium truncate max-w-[30px]"></Table.Cell>
                            <Table.Cell class="truncate" title={exception.issue}>
                                {exception.issue.substring(0, 100)}...
                            </Table.Cell>
                            <Table.Cell>{exception.count}</Table.Cell>
                            <Table.Cell>{new Date(exception.lastSeen).toLocaleString()}</Table.Cell>
                        </Table.Row>
                    {/each}
                {/if}
            </Table.Body>
        </Table.Root>
    </div>

    <div class="flex items-center justify-end space-x-2 py-4">
        <div class="flex-1 text-sm text-muted-foreground">
            Page {page} of {totalPages}
        </div>
        <div class="space-x-2">
            <Button
                variant="outline"
                size="sm"
                onclick={() => handlePageChange(page - 1)}
                disabled={page <= 1 || loading}
            >
                Previous
            </Button>
            <Button
                variant="outline"
                size="sm"
                onclick={() => handlePageChange(page + 1)}
                disabled={page >= totalPages || loading}
            >
                Next
            </Button>
        </div>
    </div>
</div>
