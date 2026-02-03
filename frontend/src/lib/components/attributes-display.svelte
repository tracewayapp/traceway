<script lang="ts">
	import * as Popover from '$lib/components/ui/popover';

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
	<div class="flex flex-wrap items-center gap-1">
		{#each visibleEntries as [key, value]}
			<span
				class="inline-flex items-center rounded bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground"
			>
				<span class="font-semibold">{key}:</span>
				<span class="ml-1 max-w-[120px] truncate font-mono" title={value}>{value}</span>
			</span>
		{/each}
		{#if hasMore}
			<Popover.Root>
				<Popover.Trigger>
					<span
						class="cursor-pointer text-xs text-muted-foreground underline underline-offset-2 hover:text-foreground"
					>
						+{hiddenCount} more
					</span>
				</Popover.Trigger>
				<Popover.Content class="w-80">
					<div class="space-y-2">
						<h4 class="text-sm font-medium">All Attributes Tags</h4>
						<div class="grid gap-2">
							{#each entries as [key, value]}
								<div class="flex items-start gap-2 text-sm">
									<span class="shrink-0 font-medium text-muted-foreground">{key}:</span>
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
