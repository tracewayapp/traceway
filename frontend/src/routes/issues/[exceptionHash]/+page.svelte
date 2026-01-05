<script lang="ts">
    import { onMount } from 'svelte';
    import { page } from '$app/state';
    import { goto } from '$app/navigation';
    import { api } from '$lib/api';
    import { Button } from "$lib/components/ui/button";
    import * as Card from "$lib/components/ui/card";
    import * as Table from "$lib/components/ui/table";
    import { Skeleton } from "$lib/components/ui/skeleton";
    import { ErrorDisplay } from "$lib/components/ui/error-display";

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
    };

    let group = $state<ExceptionGroup | null>(null);
    let occurrences = $state<ExceptionOccurrence[]>([]);
    let loading = $state(true);
    let error = $state('');
    let notFound = $state(false);

    // Pagination
    let currentPage = $state(1);
    let pageSize = $state(20);
    let total = $state(0);
    let totalPages = $state(0);

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
            });

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

    onMount(() => {
        loadData();
    });
</script>

<div class="space-y-6">
    <!-- Header -->
    <div class="flex items-center gap-4">
        <Button variant="outline" size="sm" onclick={() => goto('/issues')}>
            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="mr-2">
                <path d="m12 19-7-7 7-7"/><path d="M19 12H5"/>
            </svg>
            Back to Issues
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
                <Skeleton class="h-4 w-3/4 mb-2" />
                <Skeleton class="h-4 w-1/2" />
            </Card.Content>
        </Card.Root>
    {:else if notFound}
        <ErrorDisplay
            status={404}
            title="Exception Not Found"
            description="The exception you're looking for doesn't exist or may have been removed. It's possible the data has expired or the link is incorrect."
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
        <!-- Summary Card -->
        <Card.Root>
            <Card.Header>
                <Card.Title>Exception Details</Card.Title>
                <Card.Description>
                    First seen: {new Date(group.firstSeen).toLocaleString()} ·
                    Last seen: {new Date(group.lastSeen).toLocaleString()} ·
                    Occurrences: {group.count}
                </Card.Description>
            </Card.Header>
            <Card.Content>
                <div class="rounded-md bg-muted p-4 overflow-x-auto">
                    <pre class="text-sm whitespace-pre-wrap font-mono">{group.stackTrace}</pre>
                </div>
            </Card.Content>
        </Card.Root>

        <!-- Occurrences Table -->
        <Card.Root>
            <Card.Header>
                <Card.Title>Occurrences</Card.Title>
                <Card.Description>When this exception happened ({total} total)</Card.Description>
            </Card.Header>
            <Card.Content>
                <div class="rounded-md border">
                    <Table.Root>
                        <Table.Header>
                            <Table.Row>
                                <Table.Head>Recorded At</Table.Head>
                                <Table.Head>Transaction ID</Table.Head>
                            </Table.Row>
                        </Table.Header>
                        <Table.Body>
                            {#if occurrences.length === 0}
                                <Table.Row>
                                    <Table.Cell colspan={2} class="h-24 text-center">
                                        No occurrences found.
                                    </Table.Cell>
                                </Table.Row>
                            {:else}
                                {#each occurrences as occurrence}
                                    <Table.Row>
                                        <Table.Cell>{new Date(occurrence.recordedAt).toLocaleString()}</Table.Cell>
                                        <Table.Cell class="font-mono text-sm">
                                            {occurrence.transactionId || '-'}
                                        </Table.Cell>
                                    </Table.Row>
                                {/each}
                            {/if}
                        </Table.Body>
                    </Table.Root>
                </div>

                <!-- Pagination -->
                {#if totalPages > 1}
                    <div class="flex items-center justify-end space-x-2 py-4">
                        <div class="text-sm text-muted-foreground">
                            Page {currentPage} of {totalPages}
                        </div>
                        <Button
                            variant="outline"
                            size="sm"
                            onclick={() => handlePageChange(currentPage - 1)}
                            disabled={currentPage <= 1}
                        >
                            Previous
                        </Button>
                        <Button
                            variant="outline"
                            size="sm"
                            onclick={() => handlePageChange(currentPage + 1)}
                            disabled={currentPage >= totalPages}
                        >
                            Next
                        </Button>
                    </div>
                {/if}
            </Card.Content>
        </Card.Root>
    {/if}
</div>
