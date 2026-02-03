<script lang="ts">
    import { onMount } from 'svelte';
    import { page } from '$app/state';
    import { goto } from '$app/navigation';
    import { createRowClickHandler } from '$lib/utils/navigation';
    import { createSmartBackHandler } from '$lib/utils/back-navigation';
    import { truncateStackTrace, formatDateTime } from '$lib/utils/formatters';
    import { getTimezone } from '$lib/state/timezone.svelte';
    import { api } from '$lib/api';
    import { Button } from "$lib/components/ui/button";
    import * as Card from "$lib/components/ui/card";
    import * as Table from "$lib/components/ui/table";
    import * as Select from "$lib/components/ui/select";
    import { LoadingCircle } from "$lib/components/ui/loading-circle";
    import { ErrorDisplay } from "$lib/components/ui/error-display";
    import { projectsState } from '$lib/state/projects.svelte';
    import { TracewayTableHeader } from "$lib/components/ui/traceway-table-header";
    import { TableEmptyState } from "$lib/components/ui/table-empty-state";
    import { PaginationFooter } from "$lib/components/ui/pagination-footer";
    import { toast } from 'svelte-sonner';
    import ArchiveConfirmationDialog from '$lib/components/archive-confirmation-dialog.svelte';
    import Archive from '@lucide/svelte/icons/archive';
	import { resolve } from '$app/paths';

    const timezone = $derived(getTimezone());

    type ExceptionGroup = {
        exceptionHash: string;
        stackTrace: string;
        lastSeen: string;
        firstSeen: string;
        count: number;
    };

    type ExceptionOccurrence = {
        id: string;
        traceId: string | null;
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
    let showArchiveDialog = $state(false);
    let archiving = $state(false);

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

    function handlePageSizeChange(newPageSize: number) {
        pageSize = newPageSize;
        currentPage = 1;
        loadData();
    }

    async function archiveIssue() {
        archiving = true;
        try {
            await api.post(
                '/exception-stack-traces/archive',
                { hashes: [page.params.exceptionHash] },
                { projectId: projectsState.currentProjectId ?? undefined }
            );
            toast.success('Successfully archived the Issue', { position: 'top-center' });
            goto('/issues');
        } catch (e: any) {
            console.error('Archive failed:', e);
            throw e;
        } finally {
            archiving = false;
        }
    }

    onMount(() => {
        loadData();
    });
</script>

<div class="space-y-6">
    <!-- Header -->
    <div class="flex items-center justify-between">
        <Button variant="outline" size="sm" onclick={createSmartBackHandler({ fallbackPath: resolve('/issues/[exceptionHash]', {exceptionHash: page.params.exceptionHash ?? ''}) })}>
            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="mr-2">
                <path d="m12 19-7-7 7-7"/><path d="M19 12H5"/>
            </svg>
            Back to Issue
        </Button>
        {#if group}
            <Button
                variant="outline"
                size="sm"
                onclick={() => showArchiveDialog = true}
                disabled={archiving}
                class="gap-1.5"
            >
                <Archive class="h-4 w-4" />
                Archive
            </Button>
        {/if}
    </div>

    {#if loading && !group}
        <div class="flex items-center justify-center py-20">
            <LoadingCircle size="xlg" />
        </div>
    {:else if notFound}
        <ErrorDisplay
            status={404}
            title="Exception Not Found"
            description="The exception you're looking for doesn't exist or may have been removed."
            onBack={createSmartBackHandler({ fallbackPath: resolve('/issues') })}
            backLabel="Back to Issues"
            onRetry={() => loadData()}
            identifier={page.params.exceptionHash}
        />
    {:else if error}
        <ErrorDisplay
            status={400}
            title="Something Went Wrong"
            description={error}
            onBack={createSmartBackHandler({ fallbackPath: resolve('/issues') })}
            backLabel="Back to Issues"
            onRetry={() => loadData()}
        />
    {:else if group}
        <!-- Issue Summary -->
        <Card.Root>
            <Card.Header class="pb-3">
                <Card.Title class="text-lg">All Events</Card.Title>
                <Card.Description class="font-mono text-sm">
                    {truncateStackTrace(group.stackTrace, 80)}
                </Card.Description>
            </Card.Header>
        </Card.Root>

        <!-- Events Table -->
        <div class="rounded-md border overflow-hidden">
            <Table.Root>
                {#if loading || occurrences.length > 0}
                <Table.Header>
                    <Table.Row>
                        <TracewayTableHeader
                            label="Recorded At"
                            tooltip="When this occurrence was recorded"
                        />
                        <TracewayTableHeader
                            label="Server"
                            tooltip="Server instance where error occurred"
                        />
                        <TracewayTableHeader
                            label="Trace"
                            tooltip="Trace ID if this occurred during a request"
                        />
                    </Table.Row>
                </Table.Header>
                {/if}
                <Table.Body>
                    {#if loading}
                        <Table.Row>
                            <Table.Cell colspan={3} class="h-48">
                                <div class="flex items-center justify-center">
                                    <LoadingCircle size="lg" />
                                </div>
                            </Table.Cell>
                        </Table.Row>
                    {:else if occurrences.length === 0}
                        <TableEmptyState colspan={3} message="No events found." />
                    {:else}
                        {#each occurrences as occurrence}
                            <Table.Row
                                class="cursor-pointer hover:bg-muted/50"
                                onclick={createRowClickHandler(`/issues/${page.params.exceptionHash}/${occurrence.id}`, 'preset', 'from', 'to')}
                            >
                                <Table.Cell>{formatDateTime(occurrence.recordedAt, { timezone })}</Table.Cell>
                                <Table.Cell class="font-mono text-sm text-muted-foreground">
                                    {occurrence.serverName || '-'}
                                </Table.Cell>
                                <Table.Cell class="font-mono text-sm">
                                    {occurrence.traceId || '-'}
                                </Table.Cell>
                            </Table.Row>
                        {/each}
                    {/if}
                </Table.Body>
            </Table.Root>
        </div>

        <!-- Pagination Footer -->
        <PaginationFooter
            {currentPage}
            {totalPages}
            {pageSize}
            totalItems={total}
            itemsShown={occurrences.length}
            onPageChange={handlePageChange}
            onPageSizeChange={handlePageSizeChange}
            {loading}
            itemLabel="event"
        />
    {/if}
</div>

<ArchiveConfirmationDialog
    open={showArchiveDialog}
    onOpenChange={(open) => showArchiveDialog = open}
    count={1}
    onConfirm={archiveIssue}
/>
