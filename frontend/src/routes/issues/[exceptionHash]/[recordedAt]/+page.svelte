<script lang="ts">
    import { onMount } from 'svelte';
    import { api } from '$lib/api';
    import { Skeleton } from "$lib/components/ui/skeleton";
    import { ErrorDisplay } from "$lib/components/ui/error-display";
    import { projectsState } from '$lib/state/projects.svelte';
    import { StackTraceCard, EventCard, EventsTable, PageHeader } from '$lib/components/issues';
    import type { ExceptionGroup, ExceptionOccurrence, LinkedTransaction } from '$lib/types/exceptions';

    let { data } = $props();

    let group = $state<ExceptionGroup | null>(null);
    let occurrence = $state<ExceptionOccurrence | null>(null);
    let allOccurrences = $state<ExceptionOccurrence[]>([]);
    let loading = $state(true);
    let error = $state('');
    let notFound = $state(false);
    let total = $state(0);
    let linkedTransaction = $state<LinkedTransaction | null>(null);

    const isMessage = $derived(occurrence?.isMessage ?? false);
    const firstLineOfStackTrace = $derived(group?.stackTrace.split('\n')[0] || 'Exception');
    const hasMoreOccurrences = $derived(total > 10);

    async function loadData() {
        loading = true;
        error = '';
        notFound = false;
        linkedTransaction = null;

        try {
            // Load all occurrences for this hash
            const response = await api.post(`/exception-stack-traces/${data.exceptionHash}`, {
                pagination: {
                    page: 1,
                    pageSize: 10
                }
            }, { projectId: projectsState.currentProjectId ?? undefined });

            group = response.group;
            allOccurrences = response.occurrences || [];
            total = response.pagination.total;

            // Find the specific occurrence by recordedAt
            occurrence = allOccurrences.find(o => o.recordedAt === data.recordedAt) || null;

            if (!occurrence) {
                notFound = true;
                return;
            }

            // Load linked transaction if this occurrence has a transactionId
            if (occurrence.transactionId) {
                try {
                    const txResponse = await api.post(
                        `/transactions/${occurrence.transactionId}`,
                        {},
                        { projectId: projectsState.currentProjectId ?? undefined }
                    );
                    if (txResponse.transaction) {
                        linkedTransaction = {
                            id: txResponse.transaction.id,
                            endpoint: txResponse.transaction.endpoint,
                            duration: txResponse.transaction.duration,
                            statusCode: txResponse.transaction.statusCode,
                            recordedAt: txResponse.transaction.recordedAt
                        };
                    }
                } catch (txError) {
                    console.warn('Could not load linked transaction:', txError);
                }
            }
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

    onMount(() => {
        loadData();
    });
</script>

<div class="space-y-6">
    <PageHeader
        title={firstLineOfStackTrace}
        subtitle="Event from {new Date(data.recordedAt).toLocaleString()}"
        backHref="/issues/{data.exceptionHash}"
    />

    {#if loading && !group}
        <div class="space-y-4">
            <Skeleton class="h-48 w-full" />
            <Skeleton class="h-64 w-full" />
            <Skeleton class="h-48 w-full" />
        </div>
    {:else if notFound}
        <ErrorDisplay
            status={404}
            title="Event Not Found"
            description="The specific event you're looking for doesn't exist or may have been removed."
            backHref="/issues/{data.exceptionHash}"
            backLabel="Back to Exception"
            onRetry={() => loadData()}
        />
    {:else if error}
        <ErrorDisplay
            status={400}
            title="Something Went Wrong"
            description={error}
            backHref="/issues/{data.exceptionHash}"
            backLabel="Back to Exception"
            onRetry={() => loadData()}
        />
    {:else if group && occurrence}
        <StackTraceCard
            stackTrace={group.stackTrace}
            {isMessage}
        />

        <EventCard
            {occurrence}
            {linkedTransaction}
            title="Event"
            description="Details for this specific occurrence"
        />

        <EventsTable
            occurrences={allOccurrences}
            exceptionHash={data.exceptionHash}
            {total}
            hasMore={hasMoreOccurrences}
            showViewAll={true}
            currentRecordedAt={data.recordedAt}
        />
    {/if}
</div>
