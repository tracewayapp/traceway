<script lang="ts">
    import { onMount } from 'svelte';
    import { page } from '$app/state';
    import { goto } from '$app/navigation';
    import { createRowClickHandler } from '$lib/utils/navigation';
    import { api } from '$lib/api';
    import { Button } from "$lib/components/ui/button";
    import * as Card from "$lib/components/ui/card";
    import * as Table from "$lib/components/ui/table";
    import * as Select from "$lib/components/ui/select";
    import { Skeleton } from "$lib/components/ui/skeleton";
    import { ErrorDisplay } from "$lib/components/ui/error-display";
    import { projectsState } from '$lib/state/projects.svelte';
    import { CircleHelp } from "lucide-svelte";
    import * as Tooltip from "$lib/components/ui/tooltip";

    type ExceptionGroup = {
        exceptionHash: string;
        stackTrace: string;
        lastSeen: string;
        firstSeen: string;
        count: number;
    };

    type ExceptionOccurrence = {
        transactionId: string | null;
        exceptionHash: string;
        stackTrace: string;
        recordedAt: string;
        serverName: string;
        appVersion: string;
        endpoint: string;
    };

    let group = $state<ExceptionGroup | null>(null);
    let occurrences = $state<ExceptionOccurrence[]>([]);
    let loading = $state(true);
    let error = $state('');
    let notFound = $state(false);

    // Pagination state
    let currentPage = $state(1);
    let pageSize = $state(20);
    let total = $state(0);
    let totalPages = $state(0);

    // Page size options
    const pageSizeOptions = [
        { value: "10", label: "10" },
        { value: "20", label: "20" },
        { value: "50", label: "50" },
        { value: "100", label: "100" }
    ];

    const pageSizeLabel = $derived(pageSizeOptions.find(o => o.value === pageSize.toString())?.label ?? pageSize.toString());

    async function loadData() {
        loading = true;
        error = '';
        notFound = false;

        try {
            const exceptionHash = page.params.exceptionHash;
            const response = await api.post(`/exception-stack-traces/${exceptionHash}`, {
                pagination: {
                    page: currentPage,
                    pageSize: pageSize
                }
            }, { projectId: projectsState.currentProjectId ?? undefined });

            group = response.group;
            occurrences = response.occurrences || [];
            total = response.pagination.total;
            totalPages = response.pagination.totalPages;
        } catch (e: any) {
            console.error(e);
            if (e.status === 404) {
                notFound = true;
            } else {
                error = e.message || 'Failed to load exception details';
            }
        } finally {
            loading = false;
        }
    }

    function handlePageChange(newPage: number) {
        if (newPage >= 1 && newPage <= totalPages) {
            currentPage = newPage;
            loadData();
        }
    }

    function handlePageSizeChange(newPageSize: string) {
        pageSize = parseInt(newPageSize);
        currentPage = 1;
        loadData();
    }

    function truncateStackTrace(stackTrace: string): string {
        const firstLine = stackTrace.split('\n')[0];
        if (firstLine.length > 80) {
            return firstLine.slice(0, 80) + '...';
        }
        return firstLine;
    }

    onMount(() => {
        loadData();
    });
</script>

<div class="space-y-6">
    <!-- Header -->
    <div class="flex items-center gap-4">
        <Button variant="outline" size="sm" onclick={() => goto(`/issues/${page.params.exceptionHash}`)}>
            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="mr-2">
                <path d="m12 19-7-7 7-7"/><path d="M19 12H5"/>
            </svg>
            Back to Issue
        </Button>
    </div>

    {#if loading && !group}
        <!-- Loading skeleton -->
        <Card.Root>
            <Card.Header>
                <Skeleton class="h-6 w-48" />
            </Card.Header>
            <Card.Content>
                <Skeleton class="h-4 w-full mb-2" />
                <Skeleton class="h-4 w-3/4" />
            </Card.Content>
        </Card.Root>
    {:else if notFound}
        <ErrorDisplay
            status={404}
            title="Exception Not Found"
            description="The exception you're looking for doesn't exist or may have been removed."
            backHref="/issues"
            backLabel="Back to Issues"
            onRetry={() => loadData()}
            identifier={page.params.exceptionHash}
        />
    {:else if error}
        <ErrorDisplay
            status={400}
            title="Something Went Wrong"
            description={error}
            backHref="/issues"
            backLabel="Back to Issues"
            onRetry={() => loadData()}
        />
    {:else if group}
        <!-- Issue Summary -->
        <Card.Root>
            <Card.Header class="pb-3">
                <Card.Title class="text-lg">All Events</Card.Title>
                <Card.Description class="font-mono text-sm">
                    {truncateStackTrace(group.stackTrace)}
                </Card.Description>
            </Card.Header>
        </Card.Root>

        <!-- Events Table -->
        <div class="rounded-md border overflow-hidden">
            <Table.Root>
                {#if loading || occurrences.length > 0}
                <Table.Header>
                    <Table.Row>
                        <Table.Head>
                            <span class="flex items-center gap-1.5">
                                Recorded At
                                <Tooltip.Root>
                                    <Tooltip.Trigger>
                                        <CircleHelp class="h-3.5 w-3.5 text-muted-foreground/60" />
                                    </Tooltip.Trigger>
                                    <Tooltip.Content>
                                        <p class="text-xs">When this occurrence was recorded</p>
                                    </Tooltip.Content>
                                </Tooltip.Root>
                            </span>
                        </Table.Head>
                        <Table.Head>
                            <span class="flex items-center gap-1.5">
                                Server
                                <Tooltip.Root>
                                    <Tooltip.Trigger>
                                        <CircleHelp class="h-3.5 w-3.5 text-muted-foreground/60" />
                                    </Tooltip.Trigger>
                                    <Tooltip.Content>
                                        <p class="text-xs">Server instance where error occurred</p>
                                    </Tooltip.Content>
                                </Tooltip.Root>
                            </span>
                        </Table.Head>
                        <Table.Head>
                            <span class="flex items-center gap-1.5">
                                Version
                                <Tooltip.Root>
                                    <Tooltip.Trigger>
                                        <CircleHelp class="h-3.5 w-3.5 text-muted-foreground/60" />
                                    </Tooltip.Trigger>
                                    <Tooltip.Content>
                                        <p class="text-xs">Application version when error occurred</p>
                                    </Tooltip.Content>
                                </Tooltip.Root>
                            </span>
                        </Table.Head>
                        <Table.Head>
                            <span class="flex items-center gap-1.5">
                                Endpoint
                                <Tooltip.Root>
                                    <Tooltip.Trigger>
                                        <CircleHelp class="h-3.5 w-3.5 text-muted-foreground/60" />
                                    </Tooltip.Trigger>
                                    <Tooltip.Content>
                                        <p class="text-xs">The API route where this error occurred</p>
                                    </Tooltip.Content>
                                </Tooltip.Root>
                            </span>
                        </Table.Head>
                    </Table.Row>
                </Table.Header>
                {/if}
                <Table.Body>
                    {#if loading}
                        {#each Array(5) as _}
                            <Table.Row>
                                <Table.Cell><Skeleton class="h-4 w-[150px]" /></Table.Cell>
                                <Table.Cell><Skeleton class="h-4 w-[100px]" /></Table.Cell>
                                <Table.Cell><Skeleton class="h-4 w-[80px]" /></Table.Cell>
                                <Table.Cell><Skeleton class="h-4 w-[200px]" /></Table.Cell>
                            </Table.Row>
                        {/each}
                    {:else if occurrences.length === 0}
                        <Table.Row>
                            <Table.Cell colspan={4} class="h-24 text-center">
                                No events found.
                            </Table.Cell>
                        </Table.Row>
                    {:else}
                        {#each occurrences as occurrence}
                            <Table.Row
                                class={occurrence.endpoint ? "cursor-pointer hover:bg-muted/50" : ""}
                                onclick={occurrence.endpoint ? createRowClickHandler(`/transactions/${encodeURIComponent(occurrence.endpoint)}`) : undefined}
                            >
                                <Table.Cell>{new Date(occurrence.recordedAt).toLocaleString()}</Table.Cell>
                                <Table.Cell class="font-mono text-sm text-muted-foreground">
                                    {occurrence.serverName || '-'}
                                </Table.Cell>
                                <Table.Cell class="font-mono text-sm text-muted-foreground">
                                    {occurrence.appVersion || '-'}
                                </Table.Cell>
                                <Table.Cell class="font-mono text-sm">
                                    {occurrence.endpoint || '-'}
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
                Showing {occurrences.length} of {total} events
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
                    Page {currentPage} of {totalPages || 1}
                </div>
                <div class="flex items-center space-x-2">
                    <Button
                        variant="outline"
                        size="sm"
                        class="h-8 w-8 p-0"
                        onclick={() => handlePageChange(currentPage - 1)}
                        disabled={currentPage <= 1 || loading}
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
                        onclick={() => handlePageChange(currentPage + 1)}
                        disabled={currentPage >= totalPages || loading}
                    >
                        <span class="sr-only">Go to next page</span>
                        <svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 15 15" fill="none" class="lucide lucide-chevron-right h-4 w-4">
                            <path d="M6.1584 3.13508C6.35985 2.94621 6.67627 2.95642 6.86514 3.15788L10.6151 7.15788C10.7954 7.3502 10.7954 7.64949 10.6151 7.84182L6.86514 11.8418C6.67627 12.0433 6.35985 12.0535 6.1584 11.8646C5.95694 11.6757 5.94673 11.3593 6.1356 11.1579L9.565 7.49985L6.1356 3.84182C5.94673 3.64036 5.95694 3.32394 6.1584 3.13508Z" fill="currentColor" fill-rule="evenodd" clip-rule="evenodd"></path>
                        </svg>
                    </Button>
                </div>
            </div>
        </div>
    {/if}
</div>
