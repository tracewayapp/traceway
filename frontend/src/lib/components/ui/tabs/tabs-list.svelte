<script lang="ts">
	import { Tabs as TabsPrimitive } from "bits-ui";
	import { cn, type WithoutChild } from "$lib/utils.js";

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
		"bg-muted text-muted-foreground inline-flex h-9 items-center justify-center rounded-lg p-1",
		className
	)}
	{...restProps}
>
	{@render children?.()}
</TabsPrimitive.List>
