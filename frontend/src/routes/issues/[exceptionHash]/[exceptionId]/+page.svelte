<script lang="ts">
    import { onMount } from 'svelte';
    import { api } from '$lib/api';
    import { LoadingCircle } from "$lib/components/ui/loading-circle";
    import { ErrorDisplay } from "$lib/components/ui/error-display";
    import { projectsState } from '$lib/state/projects.svelte';
    import { getTimezone } from '$lib/state/timezone.svelte';
    import { formatDateTime } from '$lib/utils/formatters';
    import { StackTraceCard, EventCard, EventsTable, PageHeader } from '$lib/components/issues';
    import type { ExceptionGroup, ExceptionOccurrence, LinkedTransaction } from '$lib/types/exceptions';
	import { createRowClickHandler } from '$lib/utils/navigation';
	import { resolve } from '$app/paths';
	import ArchiveConfirmationDialog from '$lib/components/archive-confirmation-dialog.svelte';
	import { goto } from '$app/navigation';
	import { toast } from 'svelte-sonner';

    let { data } = $props();

    const timezone = $derived(getTimezone());

    let group = $state<ExceptionGroup | null>(null);
    let occurrence = $state<ExceptionOccurrence | null>(null);
    let allOccurrences = $state<ExceptionOccurrence[]>([]);
    let loading = $state(true);
    let error = $state('');
    let notFound = $state(false);
    let total = $state(0);
    let linkedTransaction = $state<LinkedTransaction | null>(null);
    let showArchiveDialog = $state(false);
    let archiving = $state(false);


    const isMessage = $derived(occurrence?.isMessage ?? false);
    const firstLineOfStackTrace = $derived(group?.stackTrace.split('\n')[0] || 'Exception');
    const hasMoreOccurrences = $derived(total > 10);
    const subtitleText = $derived(occurrence ? `Event from ${formatDateTime(occurrence.recordedAt, { timezone })}` : 'Loading...');

    async function loadData() {
        loading = true;
        error = '';
        notFound = false;
        linkedTransaction = null;

        try {
            // Load the specific exception by ID
            const exceptionResponse = await api.post(`/exception-stack-traces/by-id/${data.exceptionId}`, {}, { projectId: projectsState.currentProjectId ?? undefined });
            occurrence = exceptionResponse.exception;

            if (!occurrence) {
                notFound = true;
                return;
            }

            // Load all occurrences for this hash (for the events table)
            const response = await api.post(`/exception-stack-traces/${data.exceptionHash}`, {
                pagination: {
                    page: 1,
                    pageSize: 10
                }
            }, { projectId: projectsState.currentProjectId ?? undefined });

            group = response.group;
            allOccurrences = response.occurrences || [];
            total = response.pagination.total;

            // Load linked transaction if this occurrence has a transactionId
            if (occurrence.transactionId) {
                try {
                    const isTask = occurrence.transactionType === 'task';
                    console.log('DEBUG linked transaction:', {
                        transactionId: occurrence.transactionId,
                        transactionType: occurrence.transactionType,
                        isTask
                    });
                    const endpoint = isTask ? '/tasks' : '/endpoints';
                    const txResponse = await api.post(
                        `${endpoint}/${occurrence.transactionId}`,
                        {},
                        { projectId: projectsState.currentProjectId ?? undefined }
                    );
                    const txData = isTask ? txResponse.task : txResponse.endpoint;
                    if (txData) {
                        linkedTransaction = {
                            id: txData.id,
                            endpoint: isTask ? txData.taskName : txData.endpoint,
                            duration: txData.duration,
                            statusCode: txData.statusCode || 0,
                            recordedAt: txData.recordedAt,
                            transactionType: isTask ? 'task' : 'endpoint'
                        };
                    }
                } catch (txError) {
                    console.error('Failed to load linked transaction:', {
                        error: txError,
                        transactionId: occurrence.transactionId,
                        transactionType: occurrence.transactionType
                    });
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

    async function archiveIssue() {
        archiving = true;
        try {
            await api.post(
                '/exception-stack-traces/archive',
                { hashes: [data.exceptionHash] },
                { projectId: projectsState.currentProjectId ?? undefined }
            );
            toast.success('Successfully archived the Issue', { position: 'top-center' });
            goto(resolve("/issues"));
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
    <PageHeader
        title={firstLineOfStackTrace}
        subtitle={subtitleText}
        onBack={createRowClickHandler(resolve("/issues/[exceptionHash]", {exceptionHash: data.exceptionHash}))}
    />

    {#if loading && !group}
        <div class="flex items-center justify-center py-20">
            <LoadingCircle size="xlg" />
        </div>
    {:else if notFound}
        <ErrorDisplay
            status={404}
            title="Event Not Found"
            description="The specific event you're looking for doesn't exist or may have been removed."
            onBack={createRowClickHandler(resolve('/issues/[exceptionHash]', {exceptionHash: data.exceptionHash}), 'presets', 'from', 'to')}
            backLabel="Back to Exception"
            onRetry={() => loadData()}
        />
    {:else if error}
        <ErrorDisplay
            status={400}
            title="Something Went Wrong"
            description={error}
            onBack={createRowClickHandler(resolve('/issues/[exceptionHash]', {exceptionHash: data.exceptionHash}), 'presets', 'from', 'to')}
            backLabel="Back to Exception"
            onRetry={() => loadData()}
        />
    {:else if group && occurrence}
        <StackTraceCard
            stackTrace={group.stackTrace}
            {isMessage}
            bind:showArchiveDialog={showArchiveDialog}
            bind:archiving={archiving}
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
            currentExceptionId={data.exceptionId}
        />
    {/if}
</div>


<ArchiveConfirmationDialog
    open={showArchiveDialog}
    onOpenChange={(open) => showArchiveDialog = open}
    count={1}
    onConfirm={archiveIssue}
/>
