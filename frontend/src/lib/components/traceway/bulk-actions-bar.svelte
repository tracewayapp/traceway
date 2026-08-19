<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Button } from '$lib/components/ui/button';

	interface Props {
		count: number;
		itemLabel?: string;
		onClear: () => void;
		children: Snippet;
	}

	let { count, itemLabel = 'item', onClear, children }: Props = $props();
</script>

<div
	class="flex animate-in flex-wrap items-center gap-3 rounded-md border bg-muted/50 p-3 duration-200 fade-in slide-in-from-top-1"
>
	<span class="text-sm font-medium">{count} {itemLabel}{count === 1 ? '' : 's'} selected</span>
	{@render children()}
	<Button variant="ghost" size="sm" onclick={onClear}>Clear selection</Button>
</div>
