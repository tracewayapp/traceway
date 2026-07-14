<script lang="ts">
	import { Tabs as TabsPrimitive } from 'bits-ui';
	import { cn, type WithoutChild } from '$lib/utils.js';

	let {
		ref = $bindable(null),
		class: className,
		children,
		...restProps
	}: WithoutChild<TabsPrimitive.ListProps> = $props();
</script>

<TabsPrimitive.List
	bind:ref
	class={cn(
		'inline-flex h-9 items-center justify-center rounded-md bg-muted p-1 text-muted-foreground',
		className
	)}
	{...restProps}
>
	{@render children?.()}
</TabsPrimitive.List>
