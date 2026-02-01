<script lang="ts">
    import * as Popover from "$lib/components/ui/popover";

    interface Props {
        attributes: Record<string, string> | null | undefined;
        maxInlineTags?: number;
    }

    let { attributes, maxInlineTags = 3 }: Props = $props();

    const entries = $derived(
        Object.entries(attributes || {}).sort((a, b) => a[0].localeCompare(b[0]))
    );
    const hasMore = $derived(entries.length > maxInlineTags);
    const visibleEntries = $derived(entries.slice(0, maxInlineTags));
    const hiddenCount = $derived(entries.length - maxInlineTags);
</script>

{#if entries.length > 0}
    <div class="flex flex-wrap gap-1 items-center">
        {#each visibleEntries as [key, value]}
            <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-muted text-muted-foreground">
                <span class="font-semibold">{key}:</span>
                <span class="ml-1 font-mono truncate max-w-[120px]" title={value}>{value}</span>
            </span>
        {/each}
        {#if hasMore}
            <Popover.Root>
                <Popover.Trigger>
                    <span class="text-xs text-muted-foreground hover:text-foreground cursor-pointer underline underline-offset-2">
                        +{hiddenCount} more
                    </span>
                </Popover.Trigger>
                <Popover.Content class="w-80">
                    <div class="space-y-2">
                        <h4 class="font-medium text-sm">All Context Tags</h4>
                        <div class="grid gap-2">
                            {#each entries as [key, value]}
                                <div class="flex items-start gap-2 text-sm">
                                    <span class="font-medium text-muted-foreground shrink-0">{key}:</span>
                                    <span class="font-mono break-all">{value}</span>
                                </div>
                            {/each}
                        </div>
                    </div>
                </Popover.Content>
            </Popover.Root>
        {/if}
    </div>
{:else}
    <span class="text-xs text-muted-foreground">-</span>
{/if}
