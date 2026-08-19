<script lang="ts">
	import { Select as SelectPrimitive } from 'bits-ui';
	import { cn } from '$lib/utils.js';

	let {
		ref = $bindable(null),
		class: className,
		...restProps
	}: SelectPrimitive.GroupProps = $props();
</script>

<SelectPrimitive.Group
	bind:ref
	data-slot="select-group"
	class={cn(
		'[&:not(:first-child)]:border-hair [&:not(:first-child)]:mt-1 [&:not(:first-child)]:border-t [&:not(:first-child)]:pt-1',
		className
	)}
	{...restProps}
/>
