<script lang="ts">
    import { createRowClickHandler } from '$lib/utils/navigation';
    import * as Card from "$lib/components/ui/card";
    import * as Table from "$lib/components/ui/table";
    import { TracewayTableHeader } from "$lib/components/ui/traceway-table-header";
    import { TableEmptyState } from "$lib/components/ui/table-empty-state";
    import { ViewAllTableRow } from "$lib/components/ui/view-all-table-row";
    import type { ExceptionOccurrence } from '$lib/types/exceptions';
    import { formatDateTime } from '$lib/utils/formatters';
    import { getTimezone } from '$lib/state/timezone.svelte';

    interface Props {
        occurrences: ExceptionOccurrence[];
        exceptionHash: string;
        total: number;
        hasMore?: boolean;
        showViewAll?: boolean;
        currentExceptionId?: string;
        timezone?: string;
    }

    let {
        occurrences,
        exceptionHash,
        total,
        hasMore = false,
        showViewAll = true,
        currentExceptionId,
        timezone
    }: Props = $props();

    const tz = $derived(timezone ?? getTimezone());

    function getRowUrl(occurrence: ExceptionOccurrence): string {
        return `/issues/${exceptionHash}/${occurrence.id}`;
    }

    function isCurrentEvent(occurrence: ExceptionOccurrence): boolean {
        return currentExceptionId !== undefined && occurrence.id === currentExceptionId;
    }
</script>

<Card.Root>
    <Card.Header>
        <Card.Title>Events</Card.Title>
        <Card.Description>All occurrences of this exception ({total} total)</Card.Description>
    </Card.Header>
    <Card.Content>
        <div class="rounded-md border overflow-hidden">
            <Table.Root>
                {#if occurrences.length > 0}
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
                    {#if occurrences.length === 0}
                        <TableEmptyState colspan={3} message="No occurrences found." />
                    {:else}
                        {#each occurrences as occurrence}
                            <Table.Row
                                class="cursor-pointer hover:bg-muted/50 {isCurrentEvent(occurrence) ? 'bg-muted' : ''}"
                                onclick={createRowClickHandler(getRowUrl(occurrence))}
                            >
                                <Table.Cell>
                                    {formatDateTime(occurrence.recordedAt, { timezone: tz })}
                                    {#if isCurrentEvent(occurrence)}
                                        <span class="ml-2 text-xs text-muted-foreground">(current)</span>
                                    {/if}
                                </Table.Cell>
                                <Table.Cell class="font-mono text-sm text-muted-foreground">
                                    {occurrence.serverName || '-'}
                                </Table.Cell>
                                <Table.Cell class="font-mono text-sm">
                                    {occurrence.traceId || '-'}
                                </Table.Cell>
                            </Table.Row>
                        {/each}
                        {#if hasMore && showViewAll}
                            <ViewAllTableRow colspan={3} href={`/issues/${exceptionHash}/events`} label={`View all ${total} events`} />
                        {/if}
                    {/if}
                </Table.Body>
            </Table.Root>
        </div>
    </Card.Content>
</Card.Root>
