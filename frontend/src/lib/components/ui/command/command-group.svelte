<script lang="ts">
	import { Command as CommandPrimitive } from 'bits-ui';
	import type { Snippet } from 'svelte';
	import { cn, type WithoutChild } from '$lib/utils.js';

	let {
		ref = $bindable(null),
		class: className,
		children,
		heading,
		value,
		...restProps
	}: WithoutChild<CommandPrimitive.GroupProps> & {
		heading?: string;
		children: Snippet;
	} = $props();
</script>

<CommandPrimitive.Group
	bind:ref
	data-slot="command-group"
	data-command-group=""
	class={cn('overflow-hidden p-1 text-foreground', className)}
	value={value ?? heading ?? `----${crypto.randomUUID()}`}
	{...restProps}
>
	{#if heading}
		<CommandPrimitive.GroupHeading
			class="px-2 py-1.5 text-xs font-medium text-muted-foreground"
		>
			{heading}
		</CommandPrimitive.GroupHeading>
	{/if}
	<CommandPrimitive.GroupItems {children} />
</CommandPrimitive.Group>
