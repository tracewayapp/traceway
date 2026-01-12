<script lang="ts">
    import { createRowClickHandler } from '$lib/utils/navigation';
    import * as Card from "$lib/components/ui/card";
    import * as Table from "$lib/components/ui/table";
    import * as Tooltip from "$lib/components/ui/tooltip";
    import { ArrowRight, CircleQuestionMark } from "lucide-svelte";
    import type { ExceptionOccurrence } from '$lib/types/exceptions';

    interface Props {
        occurrences: ExceptionOccurrence[];
        exceptionHash: string;
        total: number;
        hasMore?: boolean;
        showViewAll?: boolean;
        currentRecordedAt?: string;
    }

    let {
        occurrences,
        exceptionHash,
        total,
        hasMore = false,
        showViewAll = true,
        currentRecordedAt
    }: Props = $props();

    function getRowUrl(occurrence: ExceptionOccurrence): string {
        return `/issues/${exceptionHash}/${encodeURIComponent(occurrence.recordedAt)}`;
    }

    function isCurrentEvent(occurrence: ExceptionOccurrence): boolean {
        return currentRecordedAt !== undefined && occurrence.recordedAt === currentRecordedAt;
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
                        <Table.Head>
                            <span class="flex items-center gap-1.5">
                                Recorded At
                                <Tooltip.Root>
                                    <Tooltip.Trigger>
                                        <CircleQuestionMark class="h-3.5 w-3.5 text-muted-foreground/60" />
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
                                        <CircleQuestionMark class="h-3.5 w-3.5 text-muted-foreground/60" />
                                    </Tooltip.Trigger>
                                    <Tooltip.Content>
                                        <p class="text-xs">Server instance where error occurred</p>
                                    </Tooltip.Content>
                                </Tooltip.Root>
                            </span>
                        </Table.Head>
                        <Table.Head>
                            <span class="flex items-center gap-1.5">
                                Transaction
                                <Tooltip.Root>
                                    <Tooltip.Trigger>
                                        <CircleQuestionMark class="h-3.5 w-3.5 text-muted-foreground/60" />
                                    </Tooltip.Trigger>
                                    <Tooltip.Content>
                                        <p class="text-xs">Transaction ID if this occurred during a request</p>
                                    </Tooltip.Content>
                                </Tooltip.Root>
                            </span>
                        </Table.Head>
                    </Table.Row>
                </Table.Header>
                {/if}
                <Table.Body>
                    {#if occurrences.length === 0}
                        <Table.Row>
                            <Table.Cell colspan={3} class="h-24 text-center">
                                No occurrences found.
                            </Table.Cell>
                        </Table.Row>
                    {:else}
                        {#each occurrences as occurrence}
                            <Table.Row
                                class="cursor-pointer hover:bg-muted/50 {isCurrentEvent(occurrence) ? 'bg-muted' : ''}"
                                onclick={createRowClickHandler(getRowUrl(occurrence))}
                            >
                                <Table.Cell>
                                    {new Date(occurrence.recordedAt).toLocaleString()}
                                    {#if isCurrentEvent(occurrence)}
                                        <span class="ml-2 text-xs text-muted-foreground">(current)</span>
                                    {/if}
                                </Table.Cell>
                                <Table.Cell class="font-mono text-sm text-muted-foreground">
                                    {occurrence.serverName || '-'}
                                </Table.Cell>
                                <Table.Cell class="font-mono text-sm">
                                    {occurrence.transactionId || '-'}
                                </Table.Cell>
                            </Table.Row>
                        {/each}
                        {#if hasMore && showViewAll}
                            <Table.Row
                                class="cursor-pointer bg-muted/50 hover:bg-muted"
                                onclick={createRowClickHandler(`/issues/${exceptionHash}/events`)}
                            >
                                <Table.Cell colspan={3} class="py-2 text-center text-sm text-muted-foreground">
                                    View all {total} events <ArrowRight class="inline h-3.5 w-3.5" />
                                </Table.Cell>
                            </Table.Row>
                        {/if}
                    {/if}
                </Table.Body>
            </Table.Root>
        </div>
    </Card.Content>
</Card.Root>
