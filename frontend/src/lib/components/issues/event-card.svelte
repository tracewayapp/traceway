<script lang="ts">
    import { goto } from '$app/navigation';
    import { Button } from "$lib/components/ui/button";
    import * as Card from "$lib/components/ui/card";
    import { ArrowRight } from "lucide-svelte";
    import type { ExceptionOccurrence, LinkedTransaction } from '$lib/types/exceptions';
    import { formatDuration, getStatusColor, formatDateTime } from '$lib/utils/formatters';
    import { getTimezone } from '$lib/state/timezone.svelte';
    import { ContextGrid } from '$lib/components/ui/context-grid';
	import { LabelValue } from '../ui/label-value';

    interface Props {
        occurrence: ExceptionOccurrence;
        linkedTransaction: LinkedTransaction | null;
        title?: string;
        description?: string;
        timezone?: string;
    }

    let {
        occurrence,
        linkedTransaction,
        title = "Event",
        description = "Details for this specific occurrence",
        timezone
    }: Props = $props();

    const tz = $derived(timezone ?? getTimezone());
</script>

<Card.Root>
    <Card.Header>
        <Card.Title>{title}</Card.Title>
        <Card.Description>{description}</Card.Description>
    </Card.Header>
    <Card.Content class="space-y-6">
        <!-- Event Details -->
        <div class="grid grid-cols-2 gap-4 md:grid-cols-4">
            <div class="space-y-1">
                <p class="text-sm text-muted-foreground">Recorded At</p>
                <p class="font-mono text-sm">{formatDateTime(occurrence.recordedAt, { timezone: tz })}</p>
            </div>
            <div class="space-y-1">
                <p class="text-sm text-muted-foreground">Server</p>
                <p class="font-mono text-sm">{occurrence.serverName || '-'}</p>
            </div>
            <div class="space-y-1">
                <p class="text-sm text-muted-foreground">Version</p>
                <p class="font-mono text-sm">{occurrence.appVersion || '-'}</p>
            </div>
            <div class="space-y-1">
                <p class="text-sm text-muted-foreground">Transaction</p>
                <p class="font-mono text-sm">{occurrence.transactionId || '-'}</p>
            </div>
        </div>

        <!-- Context -->
        {#if occurrence.scope && Object.keys(occurrence.scope).length > 0}
        <hr class="border-border" />
        <div>
            <p class="text-sm font-medium mb-3">Context (Scope)</p>
            <ContextGrid scope={occurrence.scope} />
        </div>
        {/if}

        <!-- Related Transaction -->
        {#if linkedTransaction}
        <hr class="border-border" />
        <div>
            <p class="text-sm font-medium mb-3">
                {linkedTransaction.transactionType === 'task' ? 'Related Task' : 'Related Endpoint'}
            </p>
            <div class="grid grid-cols-2 gap-4 md:grid-cols-4 mb-4">
                <LabelValue
                    label={linkedTransaction.transactionType === 'task' ? 'Task' : 'Endpoint'}
                    value={linkedTransaction.endpoint}
                    mono
                />
                {#if linkedTransaction.transactionType !== 'task'}
                <LabelValue
                    label="Status"
                    value={linkedTransaction.statusCode}
                    mono
                    large
                    valueClass={getStatusColor(linkedTransaction.statusCode)}
                />
                {/if}
                <LabelValue
                    label="Duration"
                    value={formatDuration(linkedTransaction.duration)}
                    mono
                    large
                />
                <LabelValue
                    label="Recorded At"
                    value={formatDateTime(linkedTransaction.recordedAt, { timezone })}
                    mono
                />
            </div>
            {#if linkedTransaction.transactionType === 'task'}
            <Button variant="outline" size="sm" onclick={() => goto(`/tasks/${encodeURIComponent(linkedTransaction.endpoint)}/${linkedTransaction.id}?preset=24h`)}>
                View Task Details
                <ArrowRight class="ml-2 h-4 w-4" />
            </Button>
            {:else}
            <Button variant="outline" size="sm" onclick={() => goto(`/endpoints/${encodeURIComponent(linkedTransaction.endpoint)}/${linkedTransaction.id}?preset=24h`)}>
                View Endpoint Details
                <ArrowRight class="ml-2 h-4 w-4" />
            </Button>
            {/if}
        </div>
        {/if}
    </Card.Content>
</Card.Root>
