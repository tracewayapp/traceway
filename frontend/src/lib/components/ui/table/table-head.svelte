<script lang="ts">
	import { cn, type WithElementRef } from '$lib/utils.js';
	import type { HTMLThAttributes } from 'svelte/elements';

	let {
		ref = $bindable(null),
		class: className,
		children,
		...restProps
	}: WithElementRef<HTMLThAttributes> = $props();
</script>

<th
	bind:this={ref}
	data-slot="table-head"
	class={cn(
		'h-10 bg-muted/60 bg-clip-padding px-2 text-start align-middle text-[15px] font-medium whitespace-nowrap text-muted-foreground dark:bg-ink-1 [&:has([role=checkbox])]:pe-0',
		className
	)}
	{...restProps}
>
	{@render children?.()}
</th>
