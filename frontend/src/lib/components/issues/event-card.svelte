<script lang="ts">
    import { goto } from '$app/navigation';
    import { Button } from "$lib/components/ui/button";
    import * as Card from "$lib/components/ui/card";
    import { ArrowRight } from "lucide-svelte";
    import type { ExceptionOccurrence, LinkedTransaction } from '$lib/types/exceptions';

    interface Props {
        occurrence: ExceptionOccurrence;
        linkedTransaction: LinkedTransaction | null;
        title?: string;
        description?: string;
    }

    let {
        occurrence,
        linkedTransaction,
        title = "Event",
        description = "Details for this specific occurrence"
    }: Props = $props();

    function formatDuration(nanoseconds: number): string {
        const ms = nanoseconds / 1_000_000;
        if (ms < 1) {
            return `${(nanoseconds / 1000).toFixed(0)}us`;
        } else if (ms < 1000) {
            return `${ms.toFixed(0)}ms`;
        } else {
            return `${(ms / 1000).toFixed(1)}s`;
        }
    }

    function getStatusColor(statusCode: number): string {
        if (statusCode >= 200 && statusCode < 300) return 'text-green-500';
        if (statusCode >= 300 && statusCode < 400) return 'text-blue-500';
        if (statusCode >= 400 && statusCode < 500) return 'text-yellow-500';
        return 'text-red-500';
    }
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
                <p class="font-mono text-sm">{new Date(occurrence.recordedAt).toLocaleString()}</p>
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
            <p class="text-sm font-medium mb-3">Context</p>
            <div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3">
                {#each Object.entries(occurrence.scope).sort((a, b) => a[0].localeCompare(b[0])) as [key, value]}
                    <div class="flex flex-col gap-1 p-3 rounded-md bg-muted">
                        <span class="text-xs font-medium text-muted-foreground">{key}</span>
                        <span class="text-sm font-mono break-all">{value}</span>
                    </div>
                {/each}
            </div>
        </div>
        {/if}

        <!-- Related Transaction -->
        {#if linkedTransaction}
        <hr class="border-border" />
        <div>
            <p class="text-sm font-medium mb-3">Related Transaction</p>
            <div class="grid grid-cols-2 gap-4 md:grid-cols-4 mb-4">
                <div class="space-y-1">
                    <p class="text-sm text-muted-foreground">Endpoint</p>
                    <p class="font-mono text-sm truncate" title={linkedTransaction.endpoint}>{linkedTransaction.endpoint}</p>
                </div>
                <div class="space-y-1">
                    <p class="text-sm text-muted-foreground">Status</p>
                    <p class="font-mono {getStatusColor(linkedTransaction.statusCode)}">{linkedTransaction.statusCode}</p>
                </div>
                <div class="space-y-1">
                    <p class="text-sm text-muted-foreground">Duration</p>
                    <p class="font-mono">{formatDuration(linkedTransaction.duration)}</p>
                </div>
                <div class="space-y-1">
                    <p class="text-sm text-muted-foreground">Recorded</p>
                    <p class="font-mono text-sm">{new Date(linkedTransaction.recordedAt).toLocaleString()}</p>
                </div>
            </div>
            <Button variant="outline" size="sm" onclick={() => goto(`/transactions/${encodeURIComponent(linkedTransaction.endpoint)}/${linkedTransaction.id}?preset=24h`)}>
                View Transaction Details
                <ArrowRight class="ml-2 h-4 w-4" />
            </Button>
        </div>
        {/if}
    </Card.Content>
</Card.Root>
